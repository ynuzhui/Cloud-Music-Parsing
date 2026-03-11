package service

import (
	"fmt"
	"strconv"
	"strings"

	"go-music-aggregator/backend/internal/model"
	"go-music-aggregator/backend/internal/security"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SiteSettings struct {
	Name        string `json:"name"`
	Keywords    string `json:"keywords"`
	Description string `json:"description"`
	ICPNo       string `json:"icp_no"`
	PoliceNo    string `json:"police_no"`
}

type FeatureSettings struct {
	AllowRegister       bool   `json:"allow_register"`
	RegisterEmailVerify bool   `json:"register_email_verify"`
	DefaultParseQuality string `json:"default_parse_quality"`
	ParseRequireLogin   bool   `json:"parse_require_login"`
	DefaultDailyLimit   int    `json:"default_daily_parse_limit"`
	DefaultConcurrency  int    `json:"default_concurrency_limit"`
	CookieAutoVerify    bool   `json:"cookie_auto_verify"`
}

type CaptchaSettings struct {
	Enabled             bool   `json:"enabled"`
	Provider            string `json:"provider"`
	GeetestCaptchaID    string `json:"geetest_captcha_id"`
	GeetestCaptchaKey   string `json:"geetest_captcha_key"`
	CloudflareSiteKey   string `json:"cloudflare_site_key"`
	CloudflareSecretKey string `json:"cloudflare_secret_key"`
}

type RedisSettings struct {
	Enabled bool   `json:"enabled"`
	Host    string `json:"host"`
	Port    int    `json:"port"`
	Pass    string `json:"pass"`
	DB      int    `json:"db"`
}

// Addr returns "host:port" for redis connection.
func (r RedisSettings) Addr() string {
	host := r.Host
	if host == "" {
		host = "127.0.0.1"
	}
	port := r.Port
	if port <= 0 {
		port = 6379
	}
	return fmt.Sprintf("%s:%d", host, port)
}

type ProxySettings struct {
	Enabled  bool   `json:"enabled"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	Protocol string `json:"protocol"` // http/https/socks4/socks5/socks5h
}

// BuildURL returns the full proxy URL, e.g. "socks5://user:pass@host:port".
func (p ProxySettings) BuildURL() string {
	if p.Host == "" {
		return ""
	}
	scheme := strings.ToLower(strings.TrimSpace(p.Protocol))
	if scheme == "" {
		scheme = "http"
	}
	hostPort := p.Host
	if p.Port > 0 {
		hostPort = fmt.Sprintf("%s:%d", p.Host, p.Port)
	}
	if p.Username != "" {
		return fmt.Sprintf("%s://%s:%s@%s", scheme, p.Username, p.Password, hostPort)
	}
	return fmt.Sprintf("%s://%s", scheme, hostPort)
}

type SMTPSettings struct {
	Host string `json:"host"`
	Port int    `json:"port"`
	User string `json:"user"`
	Pass string `json:"pass"`
	SSL  bool   `json:"ssl"`
}

type SystemSettings struct {
	Site    SiteSettings    `json:"site"`
	Feature FeatureSettings `json:"feature"`
	Captcha CaptchaSettings `json:"captcha"`
	Redis   RedisSettings   `json:"redis"`
	Proxy   ProxySettings   `json:"proxy"`
	SMTP    SMTPSettings    `json:"smtp"`
}

type SettingService struct {
	db  *gorm.DB
	box *security.SecretBox
}

func NewSettingService(db *gorm.DB, box *security.SecretBox) *SettingService {
	return &SettingService{db: db, box: box}
}

func (s *SettingService) Save(settings SystemSettings) error {
	settings.Captcha = normalizeCaptchaSettings(settings.Captcha)
	if err := validateCaptchaSettings(settings.Captcha); err != nil {
		return err
	}
	if settings.Feature.CookieAutoVerify {
		if strings.TrimSpace(settings.SMTP.Host) == "" || settings.SMTP.Port <= 0 || strings.TrimSpace(settings.SMTP.User) == "" {
			return fmt.Errorf("开启 Cookie 自动校验前，请先完成 SMTP 配置")
		}
	}
	if err := s.db.Where("key IN ?", []string{"captcha_login_enabled", "captcha_register_enabled"}).Delete(&model.Setting{}).Error; err != nil {
		return err
	}

	ops := []struct {
		Key       string
		Value     string
		Encrypted bool
	}{
		{"site_name", settings.Site.Name, false},
		{"site_keywords", settings.Site.Keywords, false},
		{"site_description", settings.Site.Description, false},
		{"site_icp_no", settings.Site.ICPNo, false},
		{"site_police_no", settings.Site.PoliceNo, false},
		{"allow_register", strconv.FormatBool(settings.Feature.AllowRegister), false},
		{"register_email_verify", strconv.FormatBool(settings.Feature.RegisterEmailVerify), false},
		{"default_parse_quality", normalizeQuality(settings.Feature.DefaultParseQuality, "standard"), false},
		{"parse_require_login", strconv.FormatBool(settings.Feature.ParseRequireLogin), false},
		{"default_daily_parse_limit", strconv.Itoa(nonNegative(settings.Feature.DefaultDailyLimit)), false},
		{"default_concurrency_limit", strconv.Itoa(nonNegative(settings.Feature.DefaultConcurrency)), false},
		{"cookie_auto_verify", strconv.FormatBool(settings.Feature.CookieAutoVerify), false},
		{"captcha_enabled", strconv.FormatBool(settings.Captcha.Enabled), false},
		{"captcha_provider", settings.Captcha.Provider, false},
		{"captcha_geetest_captcha_id", settings.Captcha.GeetestCaptchaID, false},
		{"captcha_geetest_captcha_key", settings.Captcha.GeetestCaptchaKey, true},
		{"captcha_cloudflare_site_key", settings.Captcha.CloudflareSiteKey, false},
		{"captcha_cloudflare_secret_key", settings.Captcha.CloudflareSecretKey, true},
		{"redis_enabled", strconv.FormatBool(settings.Redis.Enabled), false},
		{"redis_host", settings.Redis.Host, false},
		{"redis_port", strconv.Itoa(settings.Redis.Port), false},
		{"redis_pass", settings.Redis.Pass, true},
		{"redis_db", strconv.Itoa(settings.Redis.DB), false},
		{"proxy_enabled", strconv.FormatBool(settings.Proxy.Enabled), false},
		{"proxy_host", settings.Proxy.Host, false},
		{"proxy_port", strconv.Itoa(settings.Proxy.Port), false},
		{"proxy_username", settings.Proxy.Username, false},
		{"proxy_password", settings.Proxy.Password, true},
		{"proxy_protocol", settings.Proxy.Protocol, false},
		{"smtp_host", settings.SMTP.Host, false},
		{"smtp_port", strconv.Itoa(settings.SMTP.Port), false},
		{"smtp_user", settings.SMTP.User, false},
		{"smtp_pass", settings.SMTP.Pass, true},
		{"smtp_ssl", strconv.FormatBool(settings.SMTP.SSL), false},
	}
	for _, op := range ops {
		if err := s.upsert(op.Key, op.Value, op.Encrypted); err != nil {
			return err
		}
	}
	return nil
}

func (s *SettingService) Load() (SystemSettings, error) {
	defaults := SystemSettings{
		Site: SiteSettings{
			Name:        "Music Parser",
			Keywords:    "music,parser,netease",
			Description: "Music parsing service",
			ICPNo:       "",
			PoliceNo:    "",
		},
		Feature: FeatureSettings{
			AllowRegister:       false,
			RegisterEmailVerify: false,
			DefaultParseQuality: "standard",
			ParseRequireLogin:   true,
			DefaultDailyLimit:   100,
			DefaultConcurrency:  2,
			CookieAutoVerify:    false,
		},
		Captcha: CaptchaSettings{
			Enabled:             false,
			Provider:            "geetest",
			GeetestCaptchaID:    "",
			GeetestCaptchaKey:   "",
			CloudflareSiteKey:   "",
			CloudflareSecretKey: "",
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
	}

	settings := defaults
	settings.Site.Name = s.mustGetString("site_name", defaults.Site.Name)
	settings.Site.Keywords = s.mustGetString("site_keywords", defaults.Site.Keywords)
	settings.Site.Description = s.mustGetString("site_description", defaults.Site.Description)
	settings.Site.ICPNo = s.mustGetString("site_icp_no", defaults.Site.ICPNo)
	settings.Site.PoliceNo = s.mustGetString("site_police_no", defaults.Site.PoliceNo)
	settings.Feature.AllowRegister = s.mustGetBool("allow_register", defaults.Feature.AllowRegister)
	settings.Feature.RegisterEmailVerify = s.mustGetBool("register_email_verify", defaults.Feature.RegisterEmailVerify)
	settings.Feature.DefaultParseQuality = normalizeQuality(
		s.mustGetString("default_parse_quality", defaults.Feature.DefaultParseQuality),
		defaults.Feature.DefaultParseQuality,
	)
	settings.Feature.ParseRequireLogin = s.mustGetBool("parse_require_login", defaults.Feature.ParseRequireLogin)
	settings.Feature.DefaultDailyLimit = s.mustGetInt("default_daily_parse_limit", defaults.Feature.DefaultDailyLimit)
	settings.Feature.DefaultConcurrency = s.mustGetInt("default_concurrency_limit", defaults.Feature.DefaultConcurrency)
	settings.Feature.CookieAutoVerify = s.mustGetBool("cookie_auto_verify", defaults.Feature.CookieAutoVerify)
	settings.Captcha.Enabled = s.mustGetBool("captcha_enabled", defaults.Captcha.Enabled)
	settings.Captcha.Provider = normalizeCaptchaProvider(s.mustGetString("captcha_provider", defaults.Captcha.Provider))
	settings.Captcha.GeetestCaptchaID = s.mustGetString("captcha_geetest_captcha_id", defaults.Captcha.GeetestCaptchaID)
	settings.Captcha.GeetestCaptchaKey = s.mustGetString("captcha_geetest_captcha_key", defaults.Captcha.GeetestCaptchaKey)
	settings.Captcha.CloudflareSiteKey = s.mustGetString("captcha_cloudflare_site_key", defaults.Captcha.CloudflareSiteKey)
	settings.Captcha.CloudflareSecretKey = s.mustGetString("captcha_cloudflare_secret_key", defaults.Captcha.CloudflareSecretKey)
	settings.Captcha = normalizeCaptchaSettings(settings.Captcha)
	settings.Redis.Enabled = s.mustGetBool("redis_enabled", defaults.Redis.Enabled)
	settings.Redis.Host = s.mustGetString("redis_host", defaults.Redis.Host)
	settings.Redis.Port = s.mustGetInt("redis_port", defaults.Redis.Port)
	settings.Redis.Pass = s.mustGetString("redis_pass", defaults.Redis.Pass)
	settings.Redis.DB = s.mustGetInt("redis_db", defaults.Redis.DB)
	settings.Proxy.Enabled = s.mustGetBool("proxy_enabled", defaults.Proxy.Enabled)
	settings.Proxy.Host = s.mustGetString("proxy_host", defaults.Proxy.Host)
	settings.Proxy.Port = s.mustGetInt("proxy_port", defaults.Proxy.Port)
	settings.Proxy.Username = s.mustGetString("proxy_username", defaults.Proxy.Username)
	settings.Proxy.Password = s.mustGetString("proxy_password", defaults.Proxy.Password)
	settings.Proxy.Protocol = s.mustGetString("proxy_protocol", defaults.Proxy.Protocol)
	settings.SMTP.Host = s.mustGetString("smtp_host", defaults.SMTP.Host)
	settings.SMTP.Port = s.mustGetInt("smtp_port", defaults.SMTP.Port)
	settings.SMTP.User = s.mustGetString("smtp_user", defaults.SMTP.User)
	settings.SMTP.Pass = s.mustGetString("smtp_pass", defaults.SMTP.Pass)
	settings.SMTP.SSL = s.mustGetBool("smtp_ssl", defaults.SMTP.SSL)

	return settings, nil
}

func (s *SettingService) CanRegister() bool {
	return s.mustGetBool("allow_register", false)
}

func (s *SettingService) RegisterEmailVerifyEnabled() bool {
	return s.mustGetBool("register_email_verify", false)
}

func (s *SettingService) ParseRequireLogin() bool {
	return s.mustGetBool("parse_require_login", true)
}

func (s *SettingService) upsert(key string, plainValue string, encrypt bool) error {
	storeValue := plainValue
	if encrypt {
		enc, err := s.box.Encrypt(plainValue)
		if err != nil {
			return err
		}
		storeValue = enc
	}

	row := model.Setting{
		Key:       key,
		Value:     storeValue,
		Encrypted: encrypt,
	}
	return s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "encrypted", "updated_at"}),
	}).Create(&row).Error
}

func (s *SettingService) mustGetString(key, fallback string) string {
	var row model.Setting
	tx := s.db.Where("key = ?", key).Limit(1).Find(&row)
	if tx.Error != nil || tx.RowsAffected == 0 {
		return fallback
	}
	value := row.Value
	if row.Encrypted {
		plain, err := s.box.Decrypt(row.Value)
		if err != nil {
			return fallback
		}
		value = plain
	}
	return strings.TrimSpace(value)
}

func (s *SettingService) mustGetBool(key string, fallback bool) bool {
	v := strings.ToLower(s.mustGetString(key, strconv.FormatBool(fallback)))
	return v == "true" || v == "1" || v == "yes"
}

func (s *SettingService) mustGetInt(key string, fallback int) int {
	raw := s.mustGetString(key, strconv.Itoa(fallback))
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return fallback
	}
	return n
}

func nonNegative(v int) int {
	if v < 0 {
		return 0
	}
	return v
}

func normalizeCaptchaSettings(raw CaptchaSettings) CaptchaSettings {
	cfg := raw
	cfg.Provider = normalizeCaptchaProvider(cfg.Provider)
	cfg.GeetestCaptchaID = strings.TrimSpace(cfg.GeetestCaptchaID)
	cfg.GeetestCaptchaKey = strings.TrimSpace(cfg.GeetestCaptchaKey)
	cfg.CloudflareSiteKey = strings.TrimSpace(cfg.CloudflareSiteKey)
	cfg.CloudflareSecretKey = strings.TrimSpace(cfg.CloudflareSecretKey)
	return cfg
}

func normalizeCaptchaProvider(raw string) string {
	provider := strings.ToLower(strings.TrimSpace(raw))
	switch provider {
	case "cloudflare":
		return "cloudflare"
	default:
		return "geetest"
	}
}

func validateCaptchaSettings(cfg CaptchaSettings) error {
	if !cfg.Enabled {
		return nil
	}

	switch normalizeCaptchaProvider(cfg.Provider) {
	case "cloudflare":
		if strings.TrimSpace(cfg.CloudflareSiteKey) == "" || strings.TrimSpace(cfg.CloudflareSecretKey) == "" {
			return fmt.Errorf("启用 Cloudflare 验证码时，Site Key 与 Secret Key 不能为空")
		}
	default:
		if strings.TrimSpace(cfg.GeetestCaptchaID) == "" || strings.TrimSpace(cfg.GeetestCaptchaKey) == "" {
			return fmt.Errorf("启用极验验证码时，Captcha ID 与 Private Key 不能为空")
		}
	}
	return nil
}
