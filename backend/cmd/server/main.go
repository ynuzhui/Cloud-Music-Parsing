package main

import (
	"context"
	"errors"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"go-music-aggregator/backend/internal/config"
	"go-music-aggregator/backend/internal/database"
	"go-music-aggregator/backend/internal/handler"
	"go-music-aggregator/backend/internal/middleware"
	"go-music-aggregator/backend/internal/model"
	"go-music-aggregator/backend/internal/security"
	"go-music-aggregator/backend/internal/service"
	"go-music-aggregator/backend/internal/util"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	defaultFrontendPath    = "../dist"
	defaultFrontendPathAlt = "./dist"
	defaultEnvPath         = "../data/.env"
	defaultEnvPathAlt      = "./data/.env"
	defaultBackendHTTPPort = "8098"
	backendHTTPPortEnvKey  = "APP_PORT"
	defaultRateLimit       = 30
	defaultRateWindowSec   = 60
	defaultUserGroupName   = "默认组"
	defaultUserGroupDesc   = "默认用户组"
	superUserGroupName     = "超级管理员组"
	superUserGroupDesc     = "超级管理员用户组"
)

var (
	appVersion   = "dev"
	appCommit    = "unknown"
	appBuildTime = "unknown"
)

func main() {
	util.ForceBeijingTimezone()
	backendHTTPPort := resolveBackendHTTPPort()
	envFilePath := resolveEnvFilePath()
	distPath := resolveFrontendDistPath()
	restartCh := make(chan struct{}, 1)
	exitForExternalRestart := shouldExitForExternalRestart()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	var prevDB *gorm.DB

	for {
		logStartupBootstrap(envFilePath, distPath, backendHTTPPort)
		runtimeCtx, runtimeCancel := context.WithCancel(context.Background())
		logInitStart("配置加载")
		cfg, err := config.Load(envFilePath)
		if err != nil {
			log.Fatalf("❌ 配置加载失败: %v", err)
		}
		logInitDone("配置加载")

		logInitStart("安装状态与限流器")
		state := middleware.NewInstallState(cfg.InstallDone)
		rateLimit, rateWindow := resolveRateLimitConfig()
		limiter := middleware.NewMemoryRateLimiter(rateLimit, rateWindow)
		log.Printf("✅ 安装状态与限流器初始化完成（已安装=%t，限流=%d 次/%s）", cfg.InstallDone, rateLimit, rateWindow)

		logInitStart("HTTP 路由引擎")
		router := gin.New()
		router.Use(gin.Recovery())
		router.Use(gin.Logger())
		router.Use(middleware.NoCacheAPI())
		logInitDone("HTTP 路由引擎")

		logInitStart("安装向导服务")
		installSvc := service.NewInstallService(cfg.EnvFile)
		installHandler := handler.NewInstallHandler(state, installSvc, cfg.AutoRestartInstall, restartCh)
		logInitDone("安装向导服务")

		router.GET("/api/health", func(c *gin.Context) {
			util.OK(c, gin.H{
				"status":    "ok",
				"installed": state.IsInstalled(),
			})
		})

		installRoutes := router.Group("/api/install")
		{
			installRoutes.POST("/test-db", limiter.Middleware(), installHandler.TestConnection)
			installRoutes.POST("/complete", limiter.Middleware(), installHandler.Complete)
		}
		log.Printf("✅ 安装路由注册完成（/api/install）")

		if cfg.InstallDone {
			logInitStart("已安装模块与业务路由")
			db, mountErr := mountInstalledRoutes(runtimeCtx, router, cfg, limiter, state)
			if mountErr != nil {
				log.Fatalf("❌ 已安装模块挂载失败: %v", mountErr)
			}
			logInitDone("已安装模块与业务路由")
			// Close previous DB connection pool on restart
			if prevDB != nil {
				if sqlDB, err := prevDB.DB(); err == nil {
					_ = sqlDB.Close()
				}
			}
			prevDB = db
		} else {
			router.Any("/api/auth/*path", notInstalled)
			router.Any("/api/music/*path", notInstalled)
			router.Any("/api/dashboard/*path", notInstalled)
			log.Printf("ℹ️ 系统未安装，仅开放安装与健康检查路由")
		}
		logInitStart("前端静态资源路由")
		mountFrontendRoutes(router, distPath, state)
		logInitDone("前端静态资源路由")
		logStartupSummary(cfg, backendHTTPPort)

		srv := &http.Server{
			Addr:              ":" + backendHTTPPort,
			Handler:           router,
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      60 * time.Second,
			IdleTimeout:       120 * time.Second,
		}

		go func() {
			log.Printf("🌐 后端服务监听地址: :%s", backendHTTPPort)
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("❌ 服务运行失败: %v", err)
			}
		}()

		// Block until restart or OS signal
		select {
		case <-restartCh:
			log.Println("🔄 收到重启信号，正在优雅关闭服务...")
		case sig := <-sigCh:
			log.Printf("🛑 收到系统信号 %v，正在优雅关闭服务...", sig)
			runtimeCancel()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := srv.Shutdown(ctx); err != nil {
				log.Printf("⚠️ 服务关闭异常: %v", err)
			}
			cancel()
			if prevDB != nil {
				if sqlDB, err := prevDB.DB(); err == nil {
					_ = sqlDB.Close()
				}
			}
			return
		}

		runtimeCancel()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("⚠️ 服务关闭异常: %v", err)
		}
		cancel()

		if exitForExternalRestart {
			log.Println("♻️ 当前进程为 PID 1，已退出以便由外部编排系统拉起新实例")
			return
		}
		log.Println("🔁 正在重载配置并重新启动服务...")
	}
}

func logInitStart(name string) {
	log.Printf("[DEBUG] 正在初始化 %s...", name)
}

func logInitDone(name string) {
	log.Printf("[DEBUG] %s 初始化完成", name)
}

func logStartupBootstrap(envFilePath, distPath, backendHTTPPort string) {
	log.Println("========================================")
	log.Println("🚀 云音解析后端启动中...")
	log.Printf("📍 环境文件: %s", envFilePath)
	log.Printf("📁 前端目录: %s", distPath)
	log.Printf("🧷 监听端口: %s", backendHTTPPort)
}

func logStartupSummary(cfg config.Config, backendHTTPPort string) {
	log.Println("========================================")
	log.Println("✅ 初始化阶段全部完成")
	log.Printf("📦 版本: %s", appVersion)
	log.Printf("📝 Commit: %s", appCommit)
	log.Printf("📅 构建时间: %s", appBuildTime)
	log.Printf("🐹 Go 版本: %s", runtime.Version())
	log.Printf("⚙️ 运行模式: %s", gin.Mode())
	log.Printf("🗄️ 数据库驱动: %s", strings.ToUpper(cfg.DBDriver))
	log.Printf("🧱 安装状态: %t", cfg.InstallDone)
	log.Printf("🌐 服务地址: :%s", backendHTTPPort)
	log.Println("========================================")
}

func resolveBackendHTTPPort() string {
	port := strings.TrimSpace(os.Getenv(backendHTTPPortEnvKey))
	if port == "" {
		return defaultBackendHTTPPort
	}
	return port
}

func resolveRateLimitConfig() (int, time.Duration) {
	limit := defaultRateLimit
	if raw := strings.TrimSpace(os.Getenv("RATE_LIMIT")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			limit = n
		}
	}
	windowSec := defaultRateWindowSec
	if raw := strings.TrimSpace(os.Getenv("RATE_WINDOW_SEC")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			windowSec = n
		}
	}
	return limit, time.Duration(windowSec) * time.Second
}

func resolveEnvFilePath() string {
	// Running from backend source directory.
	if fileExists("go.mod") && dirExists("cmd/server") {
		return absOrFallback(defaultEnvPath)
	}
	// Running from project root directory.
	if fileExists("backend/go.mod") {
		return absOrFallback(defaultEnvPathAlt)
	}

	exeDir := executableDir()
	envCandidates := []string{
		defaultEnvPathAlt,
		defaultEnvPath,
		filepath.Join(exeDir, "data", ".env"),
		filepath.Join(exeDir, "..", "data", ".env"),
	}

	// Prefer an existing .env file from common runtime layouts.
	for _, candidate := range envCandidates {
		if fileExists(candidate) {
			return absOrFallback(candidate)
		}
	}

	// If .env doesn't exist yet, select path under an existing data directory.
	dataDirCandidates := []string{
		filepath.Dir(defaultEnvPathAlt),
		filepath.Dir(defaultEnvPath),
		filepath.Join(exeDir, "data"),
		filepath.Join(exeDir, "..", "data"),
	}
	for idx, dirPath := range dataDirCandidates {
		if dirExists(dirPath) {
			return absOrFallback(envCandidates[idx])
		}
	}

	// Final fallback for standalone binary mode.
	return absOrFallback(filepath.Join(exeDir, "data", ".env"))
}

func resolveFrontendDistPath() string {
	candidates := []string{
		defaultFrontendPathAlt,
		defaultFrontendPath,
		filepath.Join(executableDir(), "dist"),
	}
	for _, p := range candidates {
		if fileExists(filepath.Join(p, "index.html")) {
			return absOrFallback(p)
		}
	}
	return absOrFallback(defaultFrontendPath)
}

func absOrFallback(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return abs
}

func executableDir() string {
	exePath, err := os.Executable()
	if err != nil {
		return "."
	}
	resolved, err := filepath.EvalSymlinks(exePath)
	if err == nil && strings.TrimSpace(resolved) != "" {
		exePath = resolved
	}
	return filepath.Dir(exePath)
}

func shouldExitForExternalRestart() bool {
	return os.Getpid() == 1
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

func dirExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

func mountInstalledRoutes(runtimeCtx context.Context, router *gin.Engine, cfg config.Config, limiter *middleware.MemoryRateLimiter, state *middleware.InstallState) (*gorm.DB, error) {
	logInitStart("数据库连接")
	db, err := database.Open(cfg)
	if err != nil {
		log.Printf("❌ 数据库连接失败: %v", err)
		return nil, err
	}
	logInitDone("数据库连接")

	logInitStart("数据库自动迁移")
	if err := database.AutoMigrate(db); err != nil {
		log.Printf("❌ 数据库自动迁移失败: %v", err)
		return nil, err
	}
	logInitDone("数据库自动迁移")

	logInitStart("固定超级管理员校验")
	if err := ensureFixedSuperAdmin(db); err != nil {
		log.Printf("❌ 固定超级管理员校验失败: %v", err)
		return nil, err
	}
	logInitDone("固定超级管理员校验")

	logInitStart("内置用户组校验")
	if err := ensureBuiltinUserGroups(db); err != nil {
		log.Printf("❌ 内置用户组校验失败: %v", err)
		return nil, err
	}
	logInitDone("内置用户组校验")

	logInitStart("SecretBox")
	box, err := security.NewSecretBox(cfg.MasterKey)
	if err != nil {
		log.Printf("❌ SecretBox 初始化失败: %v", err)
		return nil, err
	}
	logInitDone("SecretBox")

	logInitStart("JWT 管理器")
	jwtMgr := security.NewJWTManager(cfg.JWTSecret, cfg.JWTIssuer)
	logInitDone("JWT 管理器")

	logInitStart("SettingService")
	settingSvc := service.NewSettingService(db, box)
	logInitDone("SettingService")

	logInitStart("ParseService")
	parseSvc := service.NewParseService(db, settingSvc, box)
	logInitDone("ParseService")

	logInitStart("StatsService")
	statsSvc := service.NewStatsService(db)
	logInitDone("StatsService")

	logInitStart("QuotaService")
	quotaSvc := service.NewQuotaService(db, settingSvc)
	logInitDone("QuotaService")

	logInitStart("CaptchaService")
	captchaSvc := service.NewCaptchaService(settingSvc)
	logInitDone("CaptchaService")

	logInitStart("MailService")
	mailSvc := service.NewMailService(settingSvc)
	logInitDone("MailService")

	logInitStart("EmailCodeService")
	emailCodeSvc := service.NewEmailCodeService(db, mailSvc)
	logInitDone("EmailCodeService")

	logInitStart("CookieAutoVerifyJob")
	cookieAutoVerifyJob := service.NewCookieAutoVerifyJob(db, box, parseSvc, settingSvc, mailSvc)
	logInitDone("CookieAutoVerifyJob")

	logInitStart("ParseService 缓存后端刷新")
	if err := parseSvc.RefreshCacheBackend(runtimeCtx); err != nil {
		log.Printf("⚠️ ParseService 缓存后端刷新失败: %v", err)
	} else {
		logInitDone("ParseService 缓存后端刷新")
	}

	logInitStart("Cookie 自动校验后台任务")
	go cookieAutoVerifyJob.Run(runtimeCtx)
	logInitDone("Cookie 自动校验后台任务")

	logInitStart("CleanupJob")
	cleanupJob := service.NewCleanupJob(db)
	logInitDone("CleanupJob")

	logInitStart("数据清理后台任务")
	go cleanupJob.Run(runtimeCtx)
	logInitDone("数据清理后台任务")

	logInitStart("PublicHandler")
	publicHandler := handler.NewPublicHandler(settingSvc)
	logInitDone("PublicHandler")

	logInitStart("审计日志中间件")
	router.Use(middleware.AuditLog(db))
	logInitDone("审计日志中间件")

	router.GET("/api/site", middleware.RequireInstalled(state), publicHandler.Site)
	log.Printf("✅ 公共站点路由注册完成（/api/site）")

	logInitStart("AuthHandler")
	authHandler := handler.NewAuthHandler(db, jwtMgr, settingSvc, captchaSvc, emailCodeSvc)
	logInitDone("AuthHandler")

	logInitStart("ParseHandler")
	parseHandler := handler.NewParseHandler(parseSvc, quotaSvc)
	logInitDone("ParseHandler")

	logInitStart("AdminHandler")
	adminHandler := handler.NewAdminHandler(db, box, settingSvc, statsSvc, parseSvc, mailSvc)
	logInitDone("AdminHandler")

	logInitStart("UserAdminHandler")
	userAdminHandler := handler.NewUserAdminHandler(db)
	logInitDone("UserAdminHandler")

	logInitStart("UserHandler")
	userHandler := handler.NewUserHandler(db, quotaSvc)
	logInitDone("UserHandler")

	logInitStart("认证路由")
	authRoutes := router.Group("/api/auth")
	authRoutes.Use(middleware.RequireInstalled(state))
	{
		authRoutes.POST("/login", limiter.Middleware(), authHandler.Login)
		authRoutes.POST("/register/email-code", limiter.Middleware(), authHandler.SendRegisterEmailCode)
		authRoutes.POST("/register", limiter.Middleware(), authHandler.Register)
		authRoutes.GET("/me", middleware.JWTAuth(jwtMgr, db), userHandler.Me)
		authRoutes.POST("/refresh", middleware.JWTAuth(jwtMgr, db), authHandler.RefreshToken)
	}
	logInitDone("认证路由")

	logInitStart("音乐路由")
	musicRoutes := router.Group("/api/music")
	musicRoutes.Use(middleware.RequireInstalled(state), middleware.OptionalJWTAuth(jwtMgr, db, settingSvc))
	{
		musicRoutes.GET("/providers", parseHandler.Providers)
		musicRoutes.POST("/parse", limiter.Middleware(), parseHandler.ParseNetease)
		musicRoutes.POST("/search", limiter.Middleware(), parseHandler.SearchSong)
		musicRoutes.POST("/playlist", limiter.Middleware(), parseHandler.PlaylistDetail)
		musicRoutes.POST("/lyric", limiter.Middleware(), parseHandler.GetLyric)
		musicRoutes.POST("/comment", limiter.Middleware(), parseHandler.GetComments)
		musicRoutes.POST("/recommend/playlist", limiter.Middleware(), parseHandler.RecommendPlaylists)
		musicRoutes.POST("/toplist", limiter.Middleware(), parseHandler.Toplist)
		musicRoutes.POST("/artist", limiter.Middleware(), parseHandler.Artist)
		musicRoutes.POST("/lyric/download", limiter.Middleware(), parseHandler.DownloadLyric)
		musicRoutes.POST("/cover/download", limiter.Middleware(), parseHandler.DownloadCover)
	}
	logInitDone("音乐路由")

	logInitStart("后台管理路由")
	adminRoutes := router.Group("/api/dashboard")
	adminRoutes.Use(middleware.RequireInstalled(state), middleware.JWTAuth(jwtMgr, db), middleware.AdminOnly())
	{
		adminRoutes.GET("/stats", adminHandler.Stats)
		adminRoutes.GET("/settings", adminHandler.GetSettings)
		adminRoutes.PUT("/settings", adminHandler.SaveSettings)
		adminRoutes.POST("/cookies", adminHandler.AddCookie)
		adminRoutes.GET("/cookies", adminHandler.ListCookies)
		adminRoutes.GET("/cookies/qr/key", adminHandler.CookieQRCodeKey)
		adminRoutes.POST("/cookies/qr/check", adminHandler.CookieQRCodeCheck)
		adminRoutes.POST("/cookies/verify-all", adminHandler.VerifyAllCookies)
		adminRoutes.POST("/cookies/:id/verify", adminHandler.VerifyCookie)
		adminRoutes.PATCH("/cookies/:id", adminHandler.UpdateCookie)
		adminRoutes.DELETE("/cookies/:id", adminHandler.DeleteCookie)
		adminRoutes.GET("/audit-logs", adminHandler.AuditLogs)
		adminRoutes.POST("/smtp/test", adminHandler.SmtpTest)

		adminRoutes.GET("/users", userAdminHandler.ListUsers)
		adminRoutes.POST("/users", userAdminHandler.CreateUser)
		adminRoutes.PATCH("/users/:id", userAdminHandler.UpdateUser)
		adminRoutes.PATCH("/users/:id/status", userAdminHandler.UpdateUserStatus)
		adminRoutes.PATCH("/users/:id/role", middleware.SuperAdminOnly(), userAdminHandler.SetUserRole)
		adminRoutes.POST("/users/:id/reset-password", userAdminHandler.ResetUserPassword)
		adminRoutes.DELETE("/users/:id", userAdminHandler.DeleteUser)

		adminRoutes.GET("/user-groups", userAdminHandler.ListUserGroups)
		adminRoutes.POST("/user-groups", middleware.SuperAdminOnly(), userAdminHandler.CreateUserGroup)
		adminRoutes.PATCH("/user-groups/:id", middleware.SuperAdminOnly(), userAdminHandler.UpdateUserGroup)
		adminRoutes.DELETE("/user-groups/:id", middleware.SuperAdminOnly(), userAdminHandler.DeleteUserGroup)
	}
	logInitDone("后台管理路由")

	logInitStart("用户中心路由")
	userRoutes := router.Group("/api/user")
	userRoutes.Use(middleware.RequireInstalled(state), middleware.JWTAuth(jwtMgr, db))
	{
		userRoutes.GET("/me", userHandler.Me)
		userRoutes.GET("/quota/today", userHandler.QuotaToday)
		userRoutes.GET("/usage/trend", userHandler.UsageTrend)
	}
	logInitDone("用户中心路由")

	return db, nil
}

func notInstalled(c *gin.Context) {
	c.JSON(http.StatusPreconditionRequired, gin.H{
		"code": http.StatusPreconditionRequired,
		"msg":  "系统尚未安装，请先完成安装流程",
	})
}

func mountFrontendRoutes(router *gin.Engine, distDir string, state *middleware.InstallState) {
	if embeddedFS, ok := loadEmbeddedFrontend(); ok {
		if err := mountFrontendRoutesFromFS(router, embeddedFS, state); err == nil {
			log.Printf("✅ 已启用内嵌前端静态资源")
			return
		}
	}

	indexFile := filepath.Join(distDir, "index.html")
	if _, err := os.Stat(indexFile); err != nil {
		log.Printf("⚠️ 未找到前端构建产物：%s，已跳过静态资源路由", distDir)
		return
	}
	log.Printf("✅ 前端静态资源目录已挂载：%s", distDir)

	router.NoRoute(func(c *gin.Context) {
		requestPath := c.Request.URL.Path
		if strings.HasPrefix(requestPath, "/api/") {
			util.Err(c, http.StatusNotFound, "接口不存在")
			return
		}

		cleanedPath := normalizeRoutePath(requestPath)
		cleaned := strings.TrimPrefix(cleanedPath, "/")
		if cleaned != "" && cleaned != "." {
			candidate := filepath.Join(distDir, filepath.FromSlash(cleaned))
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				setStaticCacheHeaders(c, cleaned)
				c.File(candidate)
				return
			}
			if strings.Contains(filepath.Base(cleaned), ".") {
				util.Err(c, http.StatusNotFound, "静态资源不存在")
				return
			}
		}
		if target, ok := resolveInstallRedirect(cleanedPath, state); ok {
			c.Redirect(http.StatusFound, target)
			return
		}
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		c.File(indexFile)
	})
}

func mountFrontendRoutesFromFS(router *gin.Engine, frontendFS fs.FS, state *middleware.InstallState) error {
	indexBytes, err := fs.ReadFile(frontendFS, "index.html")
	if err != nil {
		return err
	}

	router.NoRoute(func(c *gin.Context) {
		requestPath := c.Request.URL.Path
		if strings.HasPrefix(requestPath, "/api/") {
			util.Err(c, http.StatusNotFound, "接口不存在")
			return
		}

		cleanedPath := normalizeRoutePath(requestPath)
		cleaned := strings.TrimPrefix(cleanedPath, "/")
		if cleaned != "" && cleaned != "." {
			if fileBytes, readErr := fs.ReadFile(frontendFS, cleaned); readErr == nil {
				setStaticCacheHeaders(c, cleaned)
				c.Data(http.StatusOK, detectContentType(cleaned, fileBytes), fileBytes)
				return
			}
			if strings.Contains(path.Base(cleaned), ".") {
				util.Err(c, http.StatusNotFound, "静态资源不存在")
				return
			}
		}
		if target, ok := resolveInstallRedirect(cleanedPath, state); ok {
			c.Redirect(http.StatusFound, target)
			return
		}
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Data(http.StatusOK, "text/html; charset=utf-8", indexBytes)
	})
	return nil
}

func normalizeRoutePath(raw string) string {
	cleaned := path.Clean("/" + strings.TrimSpace(raw))
	if cleaned == "." {
		return "/"
	}
	return cleaned
}

func resolveInstallRedirect(routePath string, state *middleware.InstallState) (string, bool) {
	if state == nil {
		return "", false
	}
	normalized := normalizeRoutePath(routePath)
	if !state.IsInstalled() {
		if normalized != "/install" {
			return "/install", true
		}
		return "", false
	}
	if normalized == "/install" {
		return "/", true
	}
	return "", false
}

func detectContentType(filePath string, content []byte) string {
	if ext := filepath.Ext(filePath); ext != "" {
		if t := mime.TypeByExtension(ext); t != "" {
			return t
		}
	}
	return http.DetectContentType(content)
}

// setStaticCacheHeaders 根据文件路径设置分级缓存策略：
// - /assets/ 下带 hash 的构建产物：长期缓存（1年，immutable）
// - 其他静态文件（favicon 等）：短期缓存（1天）
func setStaticCacheHeaders(c *gin.Context, filePath string) {
	if strings.HasPrefix(filePath, "assets/") {
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		c.Header("Cache-Control", "public, max-age=86400")
	}
}

func ensureFixedSuperAdmin(db *gorm.DB) error {
	var root model.User
	if err := db.First(&root, 1).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("未找到 ID=1 用户，无法固定超级管理员")
		}
		return err
	}

	updates := map[string]any{}
	if root.Role != "super_admin" {
		updates["role"] = "super_admin"
	}
	if strings.ToLower(strings.TrimSpace(root.Status)) != "active" {
		updates["status"] = "active"
	}
	if len(updates) > 0 {
		if err := db.Model(&model.User{}).Where("id = ?", 1).Updates(updates).Error; err != nil {
			return err
		}
	}

	return db.Model(&model.User{}).
		Where("role = ? AND id <> ?", "super_admin", 1).
		Updates(map[string]any{
			"role":          "admin",
			"token_version": gorm.Expr("token_version + 1"),
		}).Error
}

func ensureBuiltinUserGroups(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var defaultGroup model.UserGroup
		defaultErr := tx.Where("is_default = ?", true).Order("id asc").First(&defaultGroup).Error
		switch {
		case defaultErr == nil:
			updates := map[string]any{}
			if strings.TrimSpace(defaultGroup.Name) == "default" || strings.TrimSpace(defaultGroup.Name) == "" {
				var conflictCount int64
				if err := tx.Model(&model.UserGroup{}).Where("name = ? AND id <> ?", defaultUserGroupName, defaultGroup.ID).Count(&conflictCount).Error; err != nil {
					return err
				}
				if conflictCount == 0 {
					updates["name"] = defaultUserGroupName
				}
			}
			if strings.TrimSpace(defaultGroup.Description) == "Default group" || strings.TrimSpace(defaultGroup.Description) == "" {
				updates["description"] = defaultUserGroupDesc
			}
			if len(updates) > 0 {
				if err := tx.Model(&model.UserGroup{}).Where("id = ?", defaultGroup.ID).Updates(updates).Error; err != nil {
					return err
				}
			}
		case errors.Is(defaultErr, gorm.ErrRecordNotFound):
			nameErr := tx.Where("name = ?", defaultUserGroupName).Order("id asc").First(&defaultGroup).Error
			switch {
			case nameErr == nil:
				if err := tx.Model(&model.UserGroup{}).Where("id = ?", defaultGroup.ID).Updates(map[string]any{
					"is_default":  true,
					"description": defaultUserGroupDesc,
					"unlimited":   false,
					"daily_limit": defaultGroup.DailyLimit,
					"concurrency": defaultGroup.Concurrency,
				}).Error; err != nil {
					return err
				}
				if err := tx.Model(&model.UserGroup{}).Where("id <> ? AND is_default = ?", defaultGroup.ID, true).Update("is_default", false).Error; err != nil {
					return err
				}
			case errors.Is(nameErr, gorm.ErrRecordNotFound):
				defaultGroup = model.UserGroup{
					Name:        defaultUserGroupName,
					Description: defaultUserGroupDesc,
					DailyLimit:  100,
					Concurrency: 2,
					Unlimited:   false,
					IsDefault:   true,
				}
				if err := tx.Create(&defaultGroup).Error; err != nil {
					return err
				}
			default:
				return nameErr
			}
		default:
			return defaultErr
		}

		var superGroup model.UserGroup
		superErr := tx.Where("name = ?", superUserGroupName).Order("id asc").First(&superGroup).Error
		switch {
		case superErr == nil:
		case errors.Is(superErr, gorm.ErrRecordNotFound):
			superGroup = model.UserGroup{
				Name:        superUserGroupName,
				Description: superUserGroupDesc,
				DailyLimit:  0,
				Concurrency: 0,
				Unlimited:   true,
				IsDefault:   false,
			}
			if err := tx.Create(&superGroup).Error; err != nil {
				return err
			}
		default:
			return superErr
		}

		var superAdmin model.User
		if err := tx.First(&superAdmin, 1).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		if superAdmin.GroupID == nil || *superAdmin.GroupID != superGroup.ID {
			if err := tx.Model(&model.User{}).Where("id = ?", superAdmin.ID).Update("group_id", superGroup.ID).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
