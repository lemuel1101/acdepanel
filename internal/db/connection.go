package db

import (
	"fmt"
	"log"

	"github.com/novapanel/novapanel/internal/config"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func InitDatabase(cfg *config.Config) error {
	var dial gorm.Dialector

	switch cfg.Database.Driver {
	case "postgres", "postgresql":
		dial = postgres.Open(cfg.DSN())
	case "sqlite":
		dial = sqlite.Open(cfg.DSN())
	default:
		return fmt.Errorf("unsupported database driver: %s", cfg.Database.Driver)
	}

	logLevel := logger.Silent
	if cfg.Server.LogLevel == "debug" {
		logLevel = logger.Info
	}

	var err error
	DB, err = gorm.Open(dial, &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying DB: %w", err)
	}

	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(5)

	log.Println("Database connected successfully")
	return nil
}

func RunMigrations() error {
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}

	return DB.AutoMigrate(
		&User{},
		&Domain{},
		&DomainAlias{},
		&Database{},
		&DatabaseUser{},
		&EmailAccount{},
		&FirewallRule{},
		&Backup{},
		&CronJob{},
		&SSLRequest{},
		&AuditLog{},
		&Setting{},
	)
}
