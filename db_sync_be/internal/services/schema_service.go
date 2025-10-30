package services

import (
	"context"
	"database/sql"
	"db-sync-scheduler/internal/models"
	"fmt"
	"log"
	"strings"
	"time"
)

type SchemaService struct {
	masterDB *sql.DB
	backupDB *sql.DB
}

func NewSchemaService(masterDB, backupDB *sql.DB) *SchemaService {
	return &SchemaService{
		masterDB: masterDB,
		backupDB: backupDB,
	}
}

func (s *SchemaService) GetForeignKeys(tableName string) ([]models.ForeignKey, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	query := `SELECT
	            kcu.TABLE_NAME,
	            kcu.COLUMN_NAME,
	            kcu.REFERENCED_TABLE_NAME,
	            kcu.REFERENCED_COLUMN_NAME,
	            kcu.CONSTRAINT_NAME
	          FROM information_schema.KEY_COLUMN_USAGE kcu
	          WHERE kcu.TABLE_SCHEMA = DATABASE()
	          AND kcu.TABLE_NAME = ?
	          AND kcu.REFERENCED_TABLE_NAME IS NOT NULL`

	rows, err := s.masterDB.QueryContext(ctx, query, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to get foreign keys: %v", err)
	}
	defer rows.Close()

	var fks []models.ForeignKey
	for rows.Next() {
		var fk models.ForeignKey
		err := rows.Scan(
			&fk.TableName,
			&fk.ColumnName,
			&fk.ReferencedTableName,
			&fk.ReferencedColumnName,
			&fk.ConstraintName,
		)
		if err != nil {
			return nil, err
		}
		fks = append(fks, fk)
	}

	return fks, rows.Err()
}

func (s *SchemaService) GetAllTablesWithDependencies() ([]models.TableDependency, error) {
	// Get all tables
	tables, err := s.GetAllTables()
	if err != nil {
		return nil, err
	}

	// Build dependency map
	depMap := make(map[string]*models.TableDependency)
	for _, table := range tables {
		depMap[table] = &models.TableDependency{
			TableName: table,
			DependsOn: []string{},
			Level:     0,
		}
	}

	// Get foreign keys for each table
	for _, table := range tables {
		fks, err := s.GetForeignKeys(table)
		if err != nil {
			log.Printf("Warning: failed to get foreign keys for table %s: %v", table, err)
			continue
		}

		for _, fk := range fks {
			// Skip self-references
			if fk.ReferencedTableName == table {
				log.Printf("Table %s has self-reference, will handle separately", table)
				continue
			}

			// Add dependency
			if _, exists := depMap[table]; exists {
				depMap[table].DependsOn = append(depMap[table].DependsOn, fk.ReferencedTableName)
			}
		}
	}

	// Topological sort to determine sync order
	sorted, err := s.topologicalSort(depMap)
	if err != nil {
		return nil, err
	}

	return sorted, nil
}

func (s *SchemaService) topologicalSort(depMap map[string]*models.TableDependency) ([]models.TableDependency, error) {
	visited := make(map[string]bool)
	recStack := make(map[string]bool)
	var result []models.TableDependency

	// Helper function to calculate depth (level) for each table
	var calculateLevel func(string) int
	calculateLevel = func(table string) int {
		if recStack[table] {
			// Circular dependency detected
			depMap[table].HasCircular = true
			log.Printf("Circular dependency detected for table: %s", table)
			return 0
		}

		// If already calculated, return cached level
		if visited[table] {
			return depMap[table].Level
		}

		visited[table] = true
		recStack[table] = true

		dep := depMap[table]
		maxDepLevel := 0

		// Calculate level based on dependencies
		// Level = max(dependency levels) + 1
		for _, depTable := range dep.DependsOn {
			if _, exists := depMap[depTable]; !exists {
				// Referenced table doesn't exist in our schema, skip
				log.Printf("Table %s references non-existent table %s", table, depTable)
				continue
			}

			depLevel := calculateLevel(depTable)
			if depLevel >= maxDepLevel {
				maxDepLevel = depLevel + 1
			}
		}

		recStack[table] = false
		dep.Level = maxDepLevel

		return maxDepLevel
	}

	// Calculate level for all tables
	for table := range depMap {
		if !visited[table] {
			calculateLevel(table)
		}
	}

	// Convert map to sorted slice by level
	for _, dep := range depMap {
		result = append(result, *dep)
	}

	// Sort by level (tables with no dependencies first, level 0 → 1 → 2...)
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[i].Level > result[j].Level {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	return result, nil
}

func (s *SchemaService) GetAllTables() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	query := `SELECT TABLE_NAME
	          FROM information_schema.TABLES
	          WHERE TABLE_SCHEMA = DATABASE()
	          AND TABLE_TYPE = 'BASE TABLE'
	          ORDER BY TABLE_NAME`

	rows, err := s.masterDB.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get tables: %v", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, err
		}
		tables = append(tables, tableName)
	}

	return tables, rows.Err()
}

func (s *SchemaService) GetTableSchema(tableName string) ([]models.ColumnInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	query := `SELECT
	            COLUMN_NAME,
	            DATA_TYPE,
	            COLUMN_TYPE,
	            IS_NULLABLE,
	            COLUMN_KEY,
	            COLUMN_DEFAULT,
	            EXTRA
	          FROM information_schema.COLUMNS
	          WHERE TABLE_SCHEMA = DATABASE()
	          AND TABLE_NAME = ?
	          ORDER BY ORDINAL_POSITION`

	rows, err := s.masterDB.QueryContext(ctx, query, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to get table schema: %v", err)
	}
	defer rows.Close()

	var columns []models.ColumnInfo
	for rows.Next() {
		var col models.ColumnInfo
		err := rows.Scan(
			&col.ColumnName,
			&col.DataType,
			&col.ColumnType,
			&col.IsNullable,
			&col.ColumnKey,
			&col.ColumnDefault,
			&col.Extra,
		)
		if err != nil {
			return nil, err
		}
		columns = append(columns, col)
	}

	return columns, rows.Err()
}

func (s *SchemaService) GetTableCreateStatement(tableName string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	query := fmt.Sprintf("SHOW CREATE TABLE `%s`", tableName)

	var table, createStmt string
	err := s.masterDB.QueryRowContext(ctx, query).Scan(&table, &createStmt)
	if err != nil {
		return "", fmt.Errorf("failed to get create statement: %v", err)
	}

	return createStmt, nil
}

func (s *SchemaService) TableExists(tableName string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	query := `SELECT COUNT(*)
	          FROM information_schema.TABLES
	          WHERE TABLE_SCHEMA = DATABASE()
	          AND TABLE_NAME = ?`

	var count int
	err := s.backupDB.QueryRowContext(ctx, query, tableName).Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func (s *SchemaService) CreateTable(tableName string) error {
	log.Printf("Creating table: %s", tableName)

	// Dapatkan CREATE TABLE statement dari master
	createStmt, err := s.GetTableCreateStatement(tableName)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Execute CREATE TABLE di backup database
	_, err = s.backupDB.ExecContext(ctx, createStmt)
	if err != nil {
		return fmt.Errorf("failed to create table %s: %v", tableName, err)
	}

	log.Printf("Table created successfully: %s", tableName)
	return nil
}

func (s *SchemaService) CompareSchemas(tableName string) ([]string, error) {
	masterColumns, err := s.GetTableSchema(tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to get master schema: %v", err)
	}

	backupColumns, err := s.getBackupTableSchema(tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to get backup schema: %v", err)
	}

	backupColMap := make(map[string]models.ColumnInfo)
	for _, col := range backupColumns {
		backupColMap[col.ColumnName] = col
	}

	// Generate ALTER statements
	var alterStatements []string

	for _, masterCol := range masterColumns {
		backupCol, exists := backupColMap[masterCol.ColumnName]

		if !exists {
			// Kolom baru, perlu ditambahkan
			alterStmt := s.generateAddColumnStatement(tableName, masterCol)
			alterStatements = append(alterStatements, alterStmt)
		} else if s.columnsDifferent(masterCol, backupCol) {
			// Kolom sudah ada tapi berbeda, perlu dimodifikasi
			alterStmt := s.generateModifyColumnStatement(tableName, masterCol)
			alterStatements = append(alterStatements, alterStmt)
		}
	}

	return alterStatements, nil
}

func (s *SchemaService) getBackupTableSchema(tableName string) ([]models.ColumnInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	query := `SELECT
	            COLUMN_NAME,
	            DATA_TYPE,
	            COLUMN_TYPE,
	            IS_NULLABLE,
	            COLUMN_KEY,
	            COLUMN_DEFAULT,
	            EXTRA
	          FROM information_schema.COLUMNS
	          WHERE TABLE_SCHEMA = DATABASE()
	          AND TABLE_NAME = ?
	          ORDER BY ORDINAL_POSITION`

	rows, err := s.backupDB.QueryContext(ctx, query, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []models.ColumnInfo
	for rows.Next() {
		var col models.ColumnInfo
		err := rows.Scan(
			&col.ColumnName,
			&col.DataType,
			&col.ColumnType,
			&col.IsNullable,
			&col.ColumnKey,
			&col.ColumnDefault,
			&col.Extra,
		)
		if err != nil {
			return nil, err
		}
		columns = append(columns, col)
	}

	return columns, rows.Err()
}

func (s *SchemaService) columnsDifferent(col1, col2 models.ColumnInfo) bool {
	return col1.ColumnType != col2.ColumnType ||
		col1.IsNullable != col2.IsNullable ||
		col1.Extra != col2.Extra
}

func (s *SchemaService) generateAddColumnStatement(tableName string, col models.ColumnInfo) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("ALTER TABLE `%s` ADD COLUMN `%s` %s",
		tableName, col.ColumnName, col.ColumnType))

	if col.IsNullable == "NO" {
		parts = append(parts, "NOT NULL")
	}

	if col.ColumnDefault != nil {
		parts = append(parts, fmt.Sprintf("DEFAULT %s", *col.ColumnDefault))
	}

	if col.Extra != "" {
		parts = append(parts, col.Extra)
	}

	return strings.Join(parts, " ")
}

func (s *SchemaService) generateModifyColumnStatement(tableName string, col models.ColumnInfo) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("ALTER TABLE `%s` MODIFY COLUMN `%s` %s",
		tableName, col.ColumnName, col.ColumnType))

	if col.IsNullable == "NO" {
		parts = append(parts, "NOT NULL")
	}

	if col.ColumnDefault != nil {
		parts = append(parts, fmt.Sprintf("DEFAULT %s", *col.ColumnDefault))
	}

	if col.Extra != "" {
		parts = append(parts, col.Extra)
	}

	return strings.Join(parts, " ")
}

func (s *SchemaService) SyncSchema(tableName string) error {
	log.Printf("Checking schema for table: %s", tableName)

	// Cek apakah tabel sudah ada di backup
	exists, err := s.TableExists(tableName)
	if err != nil {
		return err
	}

	if !exists {
		// Tabel belum ada, buat baru
		return s.CreateTable(tableName)
	}

	// Tabel sudah ada, bandingkan schema
	alterStatements, err := s.CompareSchemas(tableName)
	if err != nil {
		return err
	}

	if len(alterStatements) == 0 {
		log.Printf("Schema already in sync for table: %s", tableName)
		return nil
	}

	// Execute ALTER statements
	log.Printf("Found %d schema differences for table: %s", len(alterStatements), tableName)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for _, stmt := range alterStatements {
		log.Printf("  Executing: %s", stmt)
		_, err := s.backupDB.ExecContext(ctx, stmt)
		if err != nil {
			return fmt.Errorf("failed to execute alter statement: %v", err)
		}
	}

	log.Printf("Schema synchronized for table: %s", tableName)
	return nil
}

// SyncAllSchemas melakukan sinkronisasi schema untuk semua tabel dengan FK-aware ordering
func (s *SchemaService) SyncAllSchemas() error {
	log.Println("Starting schema synchronization...")

	// Get tables with dependency ordering
	tableDeps, err := s.GetAllTablesWithDependencies()
	if err != nil {
		return err
	}

	log.Printf("Found %d tables to sync (ordered by FK dependencies)", len(tableDeps))

	// Sync tables in dependency order
	for _, dep := range tableDeps {
		if len(dep.DependsOn) > 0 {
			log.Printf("Syncing schema: %s (Level %d, depends on: %v)",
				dep.TableName, dep.Level, dep.DependsOn)
		} else {
			log.Printf("Syncing schema: %s (Level %d, no dependencies)",
				dep.TableName, dep.Level)
		}

		if err := s.SyncSchema(dep.TableName); err != nil {
			log.Printf("Error syncing schema for table %s: %v", dep.TableName, err)
			// Continue dengan tabel lainnya
			continue
		}
	}

	log.Println("Schema synchronization completed")
	return nil
}
