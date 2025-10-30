package models

type ForeignKey struct {
	TableName            string `json:"table_name"`
	ColumnName           string `json:"column_name"`
	ReferencedTableName  string `json:"referenced_table_name"`
	ReferencedColumnName string `json:"referenced_column_name"`
	ConstraintName       string `json:"constraint_name"`
}

type TableDependency struct {
	TableName   string   `json:"table_name"`
	DependsOn   []string `json:"depends_on"`   // Tables that must be synced first
	Level       int      `json:"level"`        // Depth level in dependency tree
	HasCircular bool     `json:"has_circular"` // Has circular dependency
}
