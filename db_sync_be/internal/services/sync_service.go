package services

import (
	"context"
	"database/sql"
	"db-sync-scheduler/internal/config"
	"db-sync-scheduler/internal/models"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

type SyncService struct {
	masterDB      *sql.DB
	backupDB      *sql.DB
	isRunning     bool
	mutex         sync.RWMutex
	cron          *cron.Cron
	cronSchedule  string
	batchSize     int
	tableStatus   map[string]*models.SyncStatus
	schemaService *SchemaService
	syncSchema    bool
	lastRunTime   time.Time
	nextRunTime   time.Time
	config        *config.AppConfig
}

// NewSyncService creates a new sync service
func NewSyncService(masterDB, backupDB *sql.DB, schemaService *SchemaService, cronSchedule string, batchSize int, autoSchemaSync bool, cfg *config.AppConfig) *SyncService {
	return &SyncService{
		masterDB:      masterDB,
		backupDB:      backupDB,
		isRunning:     false,
		cron:          cron.New(),
		cronSchedule:  cronSchedule,
		batchSize:     batchSize,
		tableStatus:   make(map[string]*models.SyncStatus),
		schemaService: schemaService,
		syncSchema:    autoSchemaSync,
		config:        cfg,
	}
}

// StartSync memulai proses sinkronisasi dengan cron scheduler
func (s *SyncService) StartSync() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.isRunning {
		return fmt.Errorf("sync already running")
	}

	log.Printf("Starting synchronization service with schedule: %s", s.cronSchedule)

	// Add cron job
	entryID, err := s.cron.AddFunc(s.cronSchedule, func() {
		s.mutex.Lock()
		s.lastRunTime = time.Now()
		s.mutex.Unlock()

		log.Printf("\nCron triggered at %s\n", time.Now().Format("2006-01-02 15:04:05"))

		// Sync schema jika diaktifkan
		if s.syncSchema {
			if err := s.schemaService.SyncAllSchemas(); err != nil {
				log.Printf("Schema sync warning: %v", err)
			}
		}

		// Sync data
		s.syncAllTables()
	})

	if err != nil {
		return fmt.Errorf("failed to add cron job: %v", err)
	}

	// Start cron scheduler
	s.cron.Start()
	s.isRunning = true

	// Get next run time
	entries := s.cron.Entries()
	if len(entries) > 0 {
		s.nextRunTime = entries[0].Next
		log.Printf("Next sync scheduled at: %s", s.nextRunTime.Format("2006-01-02 15:04:05"))
	}

	log.Printf("Sync service started (Entry ID: %d)", entryID)

	// Run immediately on start (optional)
	go func() {
		log.Println("Running initial sync...")
		if s.syncSchema {
			if err := s.schemaService.SyncAllSchemas(); err != nil {
				log.Printf("Schema sync warning: %v", err)
			}
		}
		s.syncAllTables()

		// Update next run time after initial sync
		s.mutex.Lock()
		entries := s.cron.Entries()
		if len(entries) > 0 {
			s.nextRunTime = entries[0].Next
		}
		s.mutex.Unlock()
	}()

	return nil
}

// StopSync menghentikan proses sinkronisasi
func (s *SyncService) StopSync() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !s.isRunning {
		return fmt.Errorf("sync is not running")
	}

	// Stop cron scheduler
	ctx := s.cron.Stop()
	<-ctx.Done() // Wait for running jobs to finish

	s.isRunning = false
	log.Println("Synchronization service stopped")

	return nil
}

// syncAllTables melakukan sinkronisasi semua tabel dengan mempertimbangkan foreign key dependencies
func (s *SyncService) syncAllTables() {
	log.Printf("\nStarting sync at %s\n", time.Now().Format("2006-01-02 15:04:05"))

	// Dapatkan semua tabel dengan dependency order
	tableDeps, err := s.schemaService.GetAllTablesWithDependencies()
	if err != nil {
		log.Printf("Error getting tables with dependencies: %v\n", err)
		return
	}

	log.Printf("Found %d tables to sync (ordered by FK dependencies)\n", len(tableDeps))

	// Sync setiap tabel berdasarkan dependency order
	for _, dep := range tableDeps {
		if !s.IsRunning() {
			break
		}

		// Log dependency info
		if len(dep.DependsOn) > 0 {
			log.Printf("Syncing table: %s (Level: %d, Dependencies: %v)",
				dep.TableName, dep.Level, dep.DependsOn)
		} else {
			log.Printf("Syncing table: %s (Level: %d, No dependencies)",
				dep.TableName, dep.Level)
		}

		if dep.HasCircular {
			log.Printf("Table %s has circular dependency, syncing with caution", dep.TableName)
		}

		s.syncTable(dep.TableName)
	}

	log.Println("All tables sync completed")
}

// syncTable melakukan sinkronisasi satu tabel
func (s *SyncService) syncTable(tableName string) {

	// Get atau create status untuk tabel ini
	s.mutex.Lock()
	if s.tableStatus[tableName] == nil {
		s.tableStatus[tableName] = &models.SyncStatus{
			TableName:    tableName,
			LastSyncID:   0,
			LastSyncTime: time.Time{}, // Zero time
			Status:       "syncing",
		}
	}
	status := s.tableStatus[tableName]
	status.Status = "syncing"
	s.mutex.Unlock()

	totalSynced := 0
	currentOffset := status.LastSyncID
	lastSyncTime := status.LastSyncTime

	// Dapatkan primary key column
	pkColumn, err := s.getPrimaryKeyColumn(tableName)
	if err != nil {
		log.Printf("Error getting primary key for %s: %v", tableName, err)
		s.updateTableStatus(tableName, "error", err.Error(), currentOffset, totalSynced)
		return
	}

	if pkColumn == "" {
		log.Printf("Table %s has no primary key, skipping...", tableName)
		s.updateTableStatus(tableName, "skipped", "no primary key", currentOffset, totalSynced)
		return
	}

	// Cek apakah tabel punya kolom updated_at
	hasUpdatedAt := s.hasUpdatedAtColumn(tableName)

	// STEP 1: Sync data baru (incremental by ID)
	for s.IsRunning() {
		rows, err := s.fetchDataFromMaster(tableName, pkColumn, currentOffset, s.batchSize)
		if err != nil {
			log.Printf("Error fetching data from %s: %v", tableName, err)
			s.updateTableStatus(tableName, "error", err.Error(), currentOffset, totalSynced)
			return
		}

		if len(rows) == 0 {
			break
		}

		synced, lastID, err := s.upsertDataToBackup(tableName, pkColumn, rows)
		if err != nil {
			log.Printf("Error upserting data to %s: %v", tableName, err)
			s.updateTableStatus(tableName, "error", err.Error(), currentOffset, totalSynced)
			return
		}

		totalSynced += synced
		currentOffset = lastID

		log.Printf("  New data batch: %d records (Total: %d)", synced, totalSynced)

		if len(rows) < s.batchSize {
			break
		}
	}

	// STEP 2: Sync updated data (by updated_at timestamp or checksum)
	if hasUpdatedAt && !lastSyncTime.IsZero() {
		log.Printf("  Checking for updated records since %s", lastSyncTime.Format("2006-01-02 15:04:05"))

		updatedRows, err := s.fetchUpdatedDataFromMaster(tableName, pkColumn, lastSyncTime)
		if err != nil {
			log.Printf("Error fetching updated data from %s: %v", tableName, err)
		} else if len(updatedRows) > 0 {
			synced, _, err := s.upsertDataToBackup(tableName, pkColumn, updatedRows)
			if err != nil {
				log.Printf("Error upserting updated data to %s: %v", tableName, err)
			} else {
				totalSynced += synced
				log.Printf("  Updated data: %d records synced", synced)
			}
		}
	} else if s.config.Sync.EnableChecksumSync {
		log.Printf("performing checksum-based sync for changed records")

		changedRows, err := s.fetchChangedDataByChecksum(tableName, pkColumn)
		if err != nil {
			log.Printf("error fetching changed data from %s: %v", tableName, err)
		} else if len(changedRows) > 0 {
			synced, _, err := s.upsertDataToBackup(tableName, pkColumn, changedRows)
			if err != nil {
				log.Printf("error upserting changed data to %s: %v", tableName, err)
			} else {
				totalSynced += synced
				log.Printf("changed data: %d records synced", synced)
			}
		} else {
			log.Printf("No changed records found")
		}
	} else {
		log.Printf("Checksum sync disabled, skipping update detection for table without updated_at")
	}

	s.updateTableStatus(tableName, "success", "", currentOffset, totalSynced)
	log.Printf("Table %s synced: %d records\n", tableName, totalSynced)
}

// getPrimaryKeyColumn mendapatkan nama kolom primary key
func (s *SyncService) getPrimaryKeyColumn(tableName string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	query := `SELECT COLUMN_NAME
	          FROM information_schema.KEY_COLUMN_USAGE
	          WHERE TABLE_SCHEMA = DATABASE()
	          AND TABLE_NAME = ?
	          AND CONSTRAINT_NAME = 'PRIMARY'
	          LIMIT 1`

	var pkColumn string
	err := s.masterDB.QueryRowContext(ctx, query, tableName).Scan(&pkColumn)
	if err != nil {
		return "", nil // Tidak ada primary key
	}

	return pkColumn, nil
}

// hasUpdatedAtColumn mengecek apakah tabel punya kolom updated_at
func (s *SyncService) hasUpdatedAtColumn(tableName string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	query := `SELECT COUNT(*)
	          FROM information_schema.COLUMNS
	          WHERE TABLE_SCHEMA = DATABASE()
	          AND TABLE_NAME = ?
	          AND COLUMN_NAME = 'updated_at'`

	var count int
	err := s.masterDB.QueryRowContext(ctx, query, tableName).Scan(&count)
	if err != nil {
		return false
	}

	return count > 0
}

// fetchDataFromMaster mengambil data dari master database
func (s *SyncService) fetchDataFromMaster(tableName, pkColumn string, offset, limit int) ([]map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	operator := ">"
	if offset == 0 {
		operator = ">="
	}

	query := fmt.Sprintf("SELECT * FROM `%s` WHERE `%s` %s ? ORDER BY `%s` LIMIT ?",
		tableName, pkColumn, operator, pkColumn)

	rows, err := s.masterDB.QueryContext(ctx, query, offset, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var results []map[string]interface{}

	for rows.Next() {
		// Create slice untuk scan
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		// Konversi ke map
		rowMap := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			// Convert []byte to string
			if b, ok := val.([]byte); ok {
				rowMap[col] = string(b)
			} else {
				rowMap[col] = val
			}
		}

		results = append(results, rowMap)
	}

	return results, rows.Err()
}

// fetchUpdatedDataFromMaster mengambil data yang di-update sejak lastSyncTime
func (s *SyncService) fetchUpdatedDataFromMaster(tableName, pkColumn string, lastSyncTime time.Time) ([]map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Query untuk ambil data yang updated_at > lastSyncTime
	query := fmt.Sprintf("SELECT * FROM `%s` WHERE `updated_at` > ? ORDER BY `updated_at` LIMIT 1000",
		tableName)

	rows, err := s.masterDB.QueryContext(ctx, query, lastSyncTime)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var results []map[string]interface{}

	for rows.Next() {
		// Create slice untuk scan
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		// Konversi ke map
		rowMap := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			// Convert []byte to string
			if b, ok := val.([]byte); ok {
				rowMap[col] = string(b)
			} else {
				rowMap[col] = val
			}
		}

		results = append(results, rowMap)
	}

	return results, rows.Err()
}

// getTableColumns retrieves all column names for a table
func (s *SyncService) getTableColumns(tableName string) ([]string, error) {
	query := `SELECT COLUMN_NAME FROM INFORMATION_SCHEMA.COLUMNS 
			  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? 
			  ORDER BY ORDINAL_POSITION`

	rows, err := s.masterDB.Query(query, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to get columns for table %s: %w", tableName, err)
	}
	defer rows.Close()

	var columns []string
	for rows.Next() {
		var columnName string
		if err := rows.Scan(&columnName); err != nil {
			return nil, fmt.Errorf("failed to scan column name: %w", err)
		}
		columns = append(columns, columnName)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating columns: %w", err)
	}

	return columns, nil
}

// fetchChangedDataByChecksum membandingkan checksum data antara master dan backup untuk mendeteksi perubahan
func (s *SyncService) fetchChangedDataByChecksum(tableName, pkColumn string) ([]map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Get table columns
	columns, err := s.getTableColumns(tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to get table columns: %w", err)
	}

	// Build CONCAT_WS expression with COALESCE for NULL handling
	var concatColumns []string
	for _, col := range columns {
		concatColumns = append(concatColumns, fmt.Sprintf("COALESCE(`%s`, '')", col))
	}
	concatExpr := strings.Join(concatColumns, ", ")

	// Get all records from master with their checksums
	masterQuery := fmt.Sprintf("SELECT *, CRC32(CONCAT_WS('|', %s)) as row_checksum FROM `%s` ORDER BY `%s`",
		concatExpr, tableName, pkColumn)

	masterRows, err := s.masterDB.QueryContext(ctx, masterQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to query master data: %w", err)
	}
	defer masterRows.Close()

	masterData, err := s.scanRowsToMaps(masterRows)
	if err != nil {
		return nil, fmt.Errorf("failed to scan master rows: %w", err)
	}

	// Get all records from backup with their checksums
	backupQuery := fmt.Sprintf("SELECT *, CRC32(CONCAT_WS('|', %s)) as row_checksum FROM `%s` ORDER BY `%s`",
		concatExpr, tableName, pkColumn)

	backupRows, err := s.backupDB.QueryContext(ctx, backupQuery)
	if err != nil {
		if strings.Contains(err.Error(), "doesn't exist") {
			for i := range masterData {
				delete(masterData[i], "row_checksum")
			}
			return masterData, nil
		}
		return nil, fmt.Errorf("failed to query backup data: %w", err)
	}
	defer backupRows.Close()

	backupData, err := s.scanRowsToMaps(backupRows)
	if err != nil {
		return nil, fmt.Errorf("failed to scan backup rows: %w", err)
	}

	backupChecksums := make(map[interface{}]interface{})
	for _, row := range backupData {
		if pkValue, exists := row[pkColumn]; exists {
			if checksum, exists := row["row_checksum"]; exists {
				backupChecksums[pkValue] = checksum
			}
		}
	}

	var changedRows []map[string]interface{}
	for _, masterRow := range masterData {
		pkValue, exists := masterRow[pkColumn]
		if !exists {
			continue
		}

		masterChecksum, exists := masterRow["row_checksum"]
		if !exists {
			continue
		}

		backupChecksum, exists := backupChecksums[pkValue]

		if !exists || backupChecksum != masterChecksum {
			rowCopy := make(map[string]interface{})
			for k, v := range masterRow {
				if k != "row_checksum" {
					rowCopy[k] = v
				}
			}
			changedRows = append(changedRows, rowCopy)
		}
	}

	return changedRows, nil
}

func (s *SyncService) scanRowsToMaps(rows *sql.Rows) ([]map[string]interface{}, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var results []map[string]interface{}

	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		rowMap := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			if b, ok := val.([]byte); ok {
				rowMap[col] = string(b)
			} else {
				rowMap[col] = val
			}
		}

		results = append(results, rowMap)
	}

	return results, rows.Err()
}

// upsertDataToBackup melakukan insert atau update data ke backup database
func (s *SyncService) upsertDataToBackup(tableName, pkColumn string, rows []map[string]interface{}) (int, int, error) {
	if len(rows) == 0 {
		return 0, 0, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	synced := 0
	lastID := 0

	for _, row := range rows {
		// Build column names and placeholders
		var columns []string
		var placeholders []string
		var values []interface{}
		var updates []string

		for col, val := range row {
			columns = append(columns, fmt.Sprintf("`%s`", col))
			placeholders = append(placeholders, "?")
			values = append(values, val)

			// Untuk ON DUPLICATE KEY UPDATE
			if col != pkColumn {
				updates = append(updates, fmt.Sprintf("`%s` = VALUES(`%s`)", col, col))
			}
		}

		// Build query
		query := fmt.Sprintf(
			"INSERT INTO `%s` (%s) VALUES (%s) ON DUPLICATE KEY UPDATE %s",
			tableName,
			strings.Join(columns, ", "),
			strings.Join(placeholders, ", "),
			strings.Join(updates, ", "),
		)

		_, err := s.backupDB.ExecContext(ctx, query, values...)
		if err != nil {
			return synced, lastID, fmt.Errorf("failed to upsert row: %v", err)
		}

		synced++

		// Update last ID
		if pkVal, ok := row[pkColumn]; ok {
			switch v := pkVal.(type) {
			case int:
				lastID = v
			case int64:
				lastID = int(v)
			case string:
				fmt.Sscanf(v, "%d", &lastID)
			}
		}
	}

	return synced, lastID, nil
}

func (s *SyncService) updateTableStatus(tableName, status, errMsg string, lastID, totalSynced int) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.tableStatus[tableName] == nil {
		s.tableStatus[tableName] = &models.SyncStatus{TableName: tableName}
	}

	s.tableStatus[tableName].Status = status
	s.tableStatus[tableName].LastSyncID = lastID
	s.tableStatus[tableName].TotalSynced = totalSynced
	s.tableStatus[tableName].LastSyncTime = time.Now()
	s.tableStatus[tableName].ErrorMessage = errMsg
}

func (s *SyncService) IsRunning() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.isRunning
}

func (s *SyncService) GetStatus() map[string]interface{} {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// Copy table status
	tableStatusCopy := make(map[string]models.SyncStatus)
	for k, v := range s.tableStatus {
		if v != nil {
			tableStatusCopy[k] = *v
		}
	}

	// Format times
	var lastRun, nextRun string
	if !s.lastRunTime.IsZero() {
		lastRun = s.lastRunTime.Format("2006-01-02 15:04:05")
	}
	if !s.nextRunTime.IsZero() {
		nextRun = s.nextRunTime.Format("2006-01-02 15:04:05")
	}

	return map[string]interface{}{
		"isRunning":      s.isRunning,
		"cronSchedule":   s.cronSchedule,
		"batchSize":      s.batchSize,
		"autoSchemaSync": s.syncSchema,
		"lastRun":        lastRun,
		"nextRun":        nextRun,
		"tables":         tableStatusCopy,
	}
}

func (s *SyncService) UpdateConfig(cronSchedule string, batchSize int, syncSchema *bool) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	needsRestart := false

	// Update cron schedule if provided and different
	if cronSchedule != "" && cronSchedule != s.cronSchedule {
		s.cronSchedule = cronSchedule
		needsRestart = s.isRunning
	}

	// Update batch size
	if batchSize > 0 {
		s.batchSize = batchSize
	}

	// Update schema sync flag
	if syncSchema != nil {
		s.syncSchema = *syncSchema
	}

	log.Printf("Configuration updated - Schedule: %s, Batch Size: %d, Auto Schema Sync: %v\n",
		s.cronSchedule, s.batchSize, s.syncSchema)

	// Restart service if schedule changed
	if needsRestart {
		log.Println("Schedule changed - restart service to apply new schedule")
	}

	return nil
}

func (s *SyncService) TriggerSchemaSync() error {
	log.Println("Manual schema sync triggered")
	return s.schemaService.SyncAllSchemas()
}
