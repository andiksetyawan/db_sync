package config

type AppConfig struct {
	Server   ServerConfig   `envPrefix:"SERVER_"`
	Sync     SyncConfig     `envPrefix:"SYNC_"`
	MasterDB DatabaseConfig `envPrefix:"MASTER_DB_"`
	BackupDB DatabaseConfig `envPrefix:"BACKUP_DB_"`
}

type ServerConfig struct {
	Port string `env:"PORT" envDefault:"3000"`
}

type SyncConfig struct {
	Schedule string `env:"SCHEDULE" envDefault:"*/1 * * * *"`

	BatchSize int `env:"BATCH_SIZE" envDefault:"100"`

	AutoSchemaSync bool `env:"AUTO_SCHEMA_SYNC" envDefault:"true"`

	EnableChecksumSync bool `env:"ENABLE_CHECKSUM_SYNC" envDefault:"true"`
}

type DatabaseConfig struct {
	Host     string `env:"HOST" envDefault:"localhost"`
	Port     string `env:"PORT" envDefault:"3306"`
	User     string `env:"USER" envDefault:"root"`
	Password string `env:"PASSWORD" envDefault:"password"`
	Name     string `env:"NAME" envDefault:"master_db"`
}
