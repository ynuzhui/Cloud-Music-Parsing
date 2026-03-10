package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
)

const (
	fixedSQLiteFileName       = "app.db"
	defaultJWTIssuer          = "Cloud Music Parsing"
	fixedAutoRestartOnInstall = true
)

type Config struct {
	InstallDone        bool
	AutoRestartInstall bool
	DBDriver           string
	SQLitePath         string
	MySQLHost          string
	MySQLPort          string
	MySQLUser          string
	MySQLPass          string
	MySQLDB            string
	MySQLParams        string
	JWTSecret          string
	JWTIssuer          string
	MasterKey          string
	InstallMarker      string
	EnvFile            string
}

func Load(envFile string) (Config, error) {
	fileValues, _ := godotenv.Read(envFile)
	read := func(key, fallback string) string {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return v
		}
		if v := strings.TrimSpace(fileValues[key]); v != "" {
			return v
		}
		return fallback
	}

	cfg := Config{
		InstallDone:        read("INSTALL_DONE", "false") == "true",
		AutoRestartInstall: fixedAutoRestartOnInstall,
		DBDriver:           strings.ToLower(read("DB_DRIVER", "sqlite")),
		SQLitePath:         resolveSQLitePath(envFile),
		MySQLHost:          read("MYSQL_HOST", "127.0.0.1"),
		MySQLPort:          read("MYSQL_PORT", "3306"),
		MySQLUser:          read("MYSQL_USER", "root"),
		MySQLPass:          read("MYSQL_PASS", ""),
		MySQLDB:            read("MYSQL_DB", "music_parser"),
		MySQLParams:        read("MYSQL_PARAMS", "charset=utf8mb4&parseTime=True&loc=Local"),
		JWTSecret:          read("JWT_SECRET", ""),
		JWTIssuer:          defaultJWTIssuer,
		MasterKey:          read("APP_MASTER_KEY", ""),
		InstallMarker:      read("INSTALL_MARKER_FILE", "../data/.installed"),
		EnvFile:            envFile,
	}

	if cfg.JWTSecret == "" && cfg.InstallDone {
		return Config{}, errors.New("JWT_SECRET is required when INSTALL_DONE=true")
	}
	if cfg.MasterKey == "" && cfg.InstallDone {
		return Config{}, errors.New("APP_MASTER_KEY is required when INSTALL_DONE=true")
	}
	if cfg.DBDriver != "sqlite" && cfg.DBDriver != "mysql" {
		return Config{}, fmt.Errorf("unsupported DB_DRIVER: %s", cfg.DBDriver)
	}

	return cfg, nil
}

func resolveSQLitePath(envFile string) string {
	base := filepath.Dir(envFile)
	if strings.TrimSpace(base) == "" {
		base = "."
	}
	return filepath.Clean(filepath.Join(base, fixedSQLiteFileName))
}

func (c Config) MySQLDSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?%s",
		c.MySQLUser,
		c.MySQLPass,
		c.MySQLHost,
		c.MySQLPort,
		c.MySQLDB,
		c.MySQLParams,
	)
}
