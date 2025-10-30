package app

import (
	"database/sql"
	"db-sync-scheduler/internal/config"
	"db-sync-scheduler/internal/services"
)

type Application struct {
	Config        *config.AppConfig
	MasterDB      *sql.DB
	BackupDB      *sql.DB
	SyncService   *services.SyncService
	SchemaService *services.SchemaService
}

func NewApplication(cfg *config.AppConfig, masterDB, backupDB *sql.DB) *Application {
	app := &Application{
		Config:   cfg,
		MasterDB: masterDB,
		BackupDB: backupDB,
	}

	app.SchemaService = services.NewSchemaService(masterDB, backupDB)
	app.SyncService = services.NewSyncService(
		masterDB,
		backupDB,
		app.SchemaService,
		cfg.Sync.Schedule,
		cfg.Sync.BatchSize,
		cfg.Sync.AutoSchemaSync,
		cfg,
	)

	return app
}

func (app *Application) Close() {
	if app.MasterDB != nil {
		app.MasterDB.Close()
	}

	if app.BackupDB != nil {
		app.BackupDB.Close()
	}
}
