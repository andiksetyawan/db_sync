package models

import "time"

type TableInfo struct {
	TableName string       `json:"table_name"`
	Columns   []ColumnInfo `json:"columns"`
}

type ColumnInfo struct {
	ColumnName    string  `json:"column_name"`
	DataType      string  `json:"data_type"`
	ColumnType    string  `json:"column_type"`
	IsNullable    string  `json:"is_nullable"`
	ColumnKey     string  `json:"column_key"`
	ColumnDefault *string `json:"column_default"`
	Extra         string  `json:"extra"`
}

type SyncStatus struct {
	TableName    string    `json:"table_name"`
	LastSyncID   int       `json:"last_sync_id"`
	TotalSynced  int       `json:"total_synced"`
	LastSyncTime time.Time `json:"last_sync_time"`
	Status       string    `json:"status"`
	ErrorMessage string    `json:"error_message,omitempty"`
}
