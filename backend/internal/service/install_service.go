package service

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go-music-aggregator/backend/internal/database"
	"go-music-aggregator/backend/internal/model"
	"go-music-aggregator/backend/internal/security"
	"go-music-aggregator/backend/internal/util"

	"gorm.io/gorm"
)

const (
	defaultSiteName        = "云音解析"
	defaultSiteKeywords    = "云音解析,音乐解析,网易云音乐,无损音质"
	defaultSiteDescription = "专注网易云音乐聚合解析，支持搜索、歌单与多音质下载。"
)

type InstallDBConfig struct {
	Driver     string `json:"driver"`
	SQLitePath string `json:"sqlite_path"`
	MySQLHost  string `json:"mysql_host"`
	MySQLPort  string `json:"mysql_port"`
	MySQLUser  string `json:"mysql_user"`
	MySQLPass  string `json:"mysql_pass"`
	MySQLDB    string `json:"mysql_db"`
	MySQLParam string `json:"mysql_param"`
}

const fixedSQLitePath = "app.db"

type InstallTestRequest struct {
	Database InstallDBConfig `json:"database"`
}

type InstallCompleteRequest struct {
	Database      InstallDBConfig `json:"database"`
	AdminUsername string          `json:"admin_username"`
	AdminEmail    string          `json:"admin_email"`
	AdminPassword string          `json:"admin_password"`
	SiteName      string          `json:"site_name"`
}

type InstallService struct {
	envFile string
}

type InstallResult struct {
	RestartSuggested bool `json:"restart_suggested"`
}

func NewInstallService(envFile string) *InstallService {
	return &InstallService{envFile: envFile}
}

func (s *InstallService) TestConnection(req InstallTestRequest) error {
	db, err := openByInstallConfig(req.Database, s.resolveSQLitePath())
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	return sqlDB.Ping()
}

func (s *InstallService) Complete(req InstallCompleteRequest) (*InstallResult, error) {
	if !util.IsValidUsername(req.AdminUsername) {
		return nil, errors.New("admin username must be >=4 English letters or >=2 Chinese characters")
	}
	if !util.IsValidEmail(req.AdminEmail) {
		return nil, errors.New("admin email is invalid")
	}
	if len(req.AdminPassword) < 8 {
		return nil, errors.New("admin password must be at least 8 characters")
	}

	db, err := openByInstallConfig(req.Database, s.resolveSQLitePath())
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	defer sqlDB.Close()

	if err := database.AutoMigrate(db); err != nil {
		return nil, fmt.Errorf("migrate failed: %w", err)
	}

	var userCount int64
	if err := db.Model(&model.User{}).Count(&userCount).Error; err != nil {
		return nil, err
	}
	if userCount > 0 {
		return nil, errors.New("system already initialized")
	}

	admin := model.User{
		Username: strings.TrimSpace(req.AdminUsername),
		Email:    strings.TrimSpace(req.AdminEmail),
		Role:     "super_admin",
		Status:   "active",
	}
	if err := admin.SetPassword(req.AdminPassword); err != nil {
		return nil, err
	}
	if err := db.Create(&admin).Error; err != nil {
		return nil, err
	}
	defaultGroup := model.UserGroup{
		Name:        "default",
		Description: "Default group",
		DailyLimit:  100,
		Concurrency: 2,
		IsDefault:   true,
	}
	if err := db.Create(&defaultGroup).Error; err != nil {
		return nil, err
	}

	jwtSecret := randHex(32)
	masterKey := randHex(32)
	box, err := security.NewSecretBox(masterKey)
	if err != nil {
		return nil, err
	}
	settingSvc := NewSettingService(db, box)
	siteName := strings.TrimSpace(req.SiteName)
	if siteName == "" {
		siteName = defaultSiteName
	}
	if err := settingSvc.Save(SystemSettings{
		Site: SiteSettings{
			Name:        siteName,
			Keywords:    defaultSiteKeywords,
			Description: defaultSiteDescription,
			ICPNo:       "",
			PoliceNo:    "",
		},
		Feature: FeatureSettings{
			AllowRegister:       false,
			DefaultParseQuality: "standard",
			ParseRequireLogin:   true,
			DefaultDailyLimit:   100,
			DefaultConcurrency:  2,
		},
		Redis: RedisSettings{
			Enabled: false,
			Host:    "127.0.0.1",
			Port:    6379,
			Pass:    "",
			DB:      0,
		},
		Proxy: ProxySettings{
			Enabled:  false,
			Host:     "",
			Port:     0,
			Username: "",
			Password: "",
			Protocol: "http",
		},
		SMTP: SMTPSettings{
			Host: "",
			Port: 465,
			User: "",
			Pass: "",
			SSL:  true,
		},
	}); err != nil {
		return nil, err
	}

	dbDriver := strings.ToLower(strings.TrimSpace(req.Database.Driver))
	if dbDriver == "" {
		dbDriver = "sqlite"
	}

	envMap := map[string]string{
		"INSTALL_DONE":   "true",
		"DB_DRIVER":      dbDriver,
		"JWT_SECRET":     jwtSecret,
		"APP_MASTER_KEY": masterKey,
	}

	switch dbDriver {
	case "sqlite":
	case "mysql":
		envMap["MYSQL_HOST"] = fallback(req.Database.MySQLHost, "127.0.0.1")
		envMap["MYSQL_PORT"] = fallback(req.Database.MySQLPort, "3306")
		envMap["MYSQL_USER"] = fallback(req.Database.MySQLUser, "root")
		envMap["MYSQL_PASS"] = req.Database.MySQLPass
		envMap["MYSQL_DB"] = fallback(req.Database.MySQLDB, "music_parser")
		envMap["MYSQL_PARAMS"] = fallback(req.Database.MySQLParam, "charset=utf8mb4&parseTime=True&loc=Local")
	default:
		return nil, fmt.Errorf("unsupported driver: %s", dbDriver)
	}
	if err := writeEnvFile(s.envFile, envMap); err != nil {
		return nil, err
	}
	return &InstallResult{RestartSuggested: true}, nil
}

func openByInstallConfig(dbCfg InstallDBConfig, sqlitePath string) (*gorm.DB, error) {
	driver := strings.ToLower(strings.TrimSpace(dbCfg.Driver))
	switch driver {
	case "", "sqlite":
		return database.OpenForTest("sqlite", sqlitePath, "")
	case "mysql":
		mysqlParam := fallback(dbCfg.MySQLParam, "charset=utf8mb4&parseTime=True&loc=Local")
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?%s",
			fallback(dbCfg.MySQLUser, "root"),
			dbCfg.MySQLPass,
			fallback(dbCfg.MySQLHost, "127.0.0.1"),
			fallback(dbCfg.MySQLPort, "3306"),
			fallback(dbCfg.MySQLDB, "music_parser"),
			mysqlParam,
		)
		return database.OpenForTest("mysql", "", dsn)
	default:
		return nil, fmt.Errorf("unsupported driver: %s", driver)
	}
}

func (s *InstallService) resolveSQLitePath() string {
	baseDir := filepath.Dir(s.envFile)
	if strings.TrimSpace(baseDir) == "" {
		baseDir = "."
	}
	return filepath.Clean(filepath.Join(baseDir, fixedSQLitePath))
}

func writeEnvFile(path string, values map[string]string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && filepath.Dir(path) != "." {
		return err
	}

	dbDriver := strings.ToLower(strings.TrimSpace(values["DB_DRIVER"]))
	if dbDriver == "" {
		dbDriver = "sqlite"
	}

	lines := []string{
		"# 安装状态（true 表示已安装）",
		"INSTALL_DONE=" + values["INSTALL_DONE"],
		"",
		"# 数据库驱动（sqlite 或 mysql）",
		"DB_DRIVER=" + dbDriver,
		"",
	}

	switch dbDriver {
	case "sqlite":
	case "mysql":
		lines = append(lines,
			"# MySQL 连接配置",
			"MYSQL_HOST="+fallback(values["MYSQL_HOST"], "127.0.0.1"),
			"MYSQL_PORT="+fallback(values["MYSQL_PORT"], "3306"),
			"MYSQL_USER="+fallback(values["MYSQL_USER"], "root"),
			"MYSQL_PASS="+values["MYSQL_PASS"],
			"MYSQL_DB="+fallback(values["MYSQL_DB"], "music_parser"),
			"MYSQL_PARAMS="+fallback(values["MYSQL_PARAMS"], "charset=utf8mb4&parseTime=True&loc=Local"),
		)
	default:
		return fmt.Errorf("unsupported DB_DRIVER: %s", dbDriver)
	}

	lines = append(lines,
		"",
		"# 安全配置（安装时自动生成）",
		"JWT_SECRET="+values["JWT_SECRET"],
		"APP_MASTER_KEY="+values["APP_MASTER_KEY"],
	)

	content := strings.Join(lines, "\n") + "\n"
	return os.WriteFile(path, []byte(content), 0o600)
}

func randHex(size int) string {
	buf := make([]byte, size)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}

func fallback(v, defaultValue string) string {
	if strings.TrimSpace(v) == "" {
		return defaultValue
	}
	return strings.TrimSpace(v)
}
