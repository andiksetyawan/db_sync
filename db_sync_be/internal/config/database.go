package config

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/go-sql-driver/mysql"
)

func InitDatabase(cfg *AppConfig) (*sql.DB, *sql.DB, error) {
	var err error

	masterDSN := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true",
		cfg.MasterDB.User,
		cfg.MasterDB.Password,
		cfg.MasterDB.Host,
		cfg.MasterDB.Port,
		cfg.MasterDB.Name,
	)

	masterDB, err := sql.Open("mysql", masterDSN)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open master database: %v", err)
	}

	if err = masterDB.Ping(); err != nil {
		return nil, nil, fmt.Errorf("failed to ping master database: %v", err)
	}

	masterDB.SetMaxOpenConns(10)
	masterDB.SetMaxIdleConns(5)

	log.Printf("Connected to Master Database (%s:%s/%s)",
		cfg.MasterDB.Host, cfg.MasterDB.Port, cfg.MasterDB.Name)

	backupDSN := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true",
		cfg.BackupDB.User,
		cfg.BackupDB.Password,
		cfg.BackupDB.Host,
		cfg.BackupDB.Port,
		cfg.BackupDB.Name,
	)

	backupDB, err := sql.Open("mysql", backupDSN)
	if err != nil {
		masterDB.Close() // Close master DB jika backup gagal
		return nil, nil, fmt.Errorf("failed to open backup database: %v", err)
	}

	if err = backupDB.Ping(); err != nil {
		masterDB.Close() // Close master DB jika backup gagal
		return nil, nil, fmt.Errorf("failed to ping backup database: %v", err)
	}

	// Set connection pool settings
	backupDB.SetMaxOpenConns(10)
	backupDB.SetMaxIdleConns(5)

	log.Printf("Connected to Backup Database (%s:%s/%s)",
		cfg.BackupDB.Host, cfg.BackupDB.Port, cfg.BackupDB.Name)

	return masterDB, backupDB, nil
}
