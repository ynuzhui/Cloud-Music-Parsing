package database

import (
	"fmt"
	"os"
	"path/filepath"

	"go-music-aggregator/backend/internal/config"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Open(cfg config.Config) (*gorm.DB, error) {
	switch cfg.DBDriver {
	case "sqlite":
		if err := os.MkdirAll(filepath.Dir(cfg.SQLitePath), 0o755); err != nil {
			return nil, fmt.Errorf("create sqlite dir: %w", err)
		}
		return gorm.Open(sqlite.Open(cfg.SQLitePath), &gorm.Config{Logger: logger.Default.LogMode(logger.Warn)})
	case "mysql":
		return gorm.Open(mysql.Open(cfg.MySQLDSN()), &gorm.Config{Logger: logger.Default.LogMode(logger.Warn)})
	default:
		return nil, fmt.Errorf("unsupported db driver: %s", cfg.DBDriver)
	}
}

func OpenForTest(driver, sqlitePath, mysqlDSN string) (*gorm.DB, error) {
	switch driver {
	case "sqlite":
		if err := os.MkdirAll(filepath.Dir(sqlitePath), 0o755); err != nil {
			return nil, fmt.Errorf("create sqlite dir: %w", err)
		}
		return gorm.Open(sqlite.Open(sqlitePath), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	case "mysql":
		return gorm.Open(mysql.Open(mysqlDSN), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	default:
		return nil, fmt.Errorf("unsupported db driver: %s", driver)
	}
}
