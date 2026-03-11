package main

import (
	"context"
	"errors"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
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
	defaultUserGroupName   = "默认组"
	defaultUserGroupDesc   = "默认用户组"
	superUserGroupName     = "超级管理员组"
	superUserGroupDesc     = "超级管理员用户组"
)

func main() {
	util.ForceBeijingTimezone()
	backendHTTPPort := resolveBackendHTTPPort()
	envFilePath := resolveEnvFilePath()
	distPath := resolveFrontendDistPath()
	restartCh := make(chan struct{}, 1)
	exitForExternalRestart := shouldExitForExternalRestart()

	for {
		cfg, err := config.Load(envFilePath)
		if err != nil {
			log.Fatalf("load config failed: %v", err)
		}

		state := middleware.NewInstallState(cfg.InstallDone)
		limiter := middleware.NewMemoryRateLimiter(30, time.Minute)

		router := gin.New()
		router.Use(gin.Recovery())
		router.Use(gin.Logger())

		installSvc := service.NewInstallService(cfg.EnvFile)
		installHandler := handler.NewInstallHandler(state, installSvc, cfg.AutoRestartInstall, restartCh)

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

		if cfg.InstallDone {
			if err := mountInstalledRoutes(router, cfg, limiter, state); err != nil {
				log.Fatalf("mount installed routes failed: %v", err)
			}
		} else {
			router.Any("/api/auth/*path", notInstalled)
			router.Any("/api/music/*path", notInstalled)
			router.Any("/api/admin/*path", notInstalled)
		}
		mountFrontendRoutes(router, distPath, state)

		srv := &http.Server{
			Addr:    ":" + backendHTTPPort,
			Handler: router,
		}

		go func() {
			log.Printf("backend listening on :%s", backendHTTPPort)
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("server run failed: %v", err)
			}
		}()

		// Block until restart signal
		<-restartCh
		log.Println("received restart signal, shutting down...")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = srv.Shutdown(ctx)
		cancel()

		if exitForExternalRestart {
			log.Println("running as pid 1, exiting process for external restart")
			return
		}
		log.Println("reloading configuration and restarting server...")
	}
}

func resolveBackendHTTPPort() string {
	port := strings.TrimSpace(os.Getenv(backendHTTPPortEnvKey))
	if port == "" {
		return defaultBackendHTTPPort
	}
	return port
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

func mountInstalledRoutes(router *gin.Engine, cfg config.Config, limiter *middleware.MemoryRateLimiter, state *middleware.InstallState) error {
	db, err := database.Open(cfg)
	if err != nil {
		return err
	}
	if err := database.AutoMigrate(db); err != nil {
		return err
	}
	if err := ensureSingleSuperAdmin(db); err != nil {
		return err
	}
	if err := ensureBuiltinUserGroups(db); err != nil {
		return err
	}

	box, err := security.NewSecretBox(cfg.MasterKey)
	if err != nil {
		return err
	}
	jwtMgr := security.NewJWTManager(cfg.JWTSecret, cfg.JWTIssuer)
	settingSvc := service.NewSettingService(db, box)
	parseSvc := service.NewParseService(db, settingSvc, box)
	statsSvc := service.NewStatsService(db)
	quotaSvc := service.NewQuotaService(db, settingSvc)
	captchaSvc := service.NewCaptchaService(settingSvc)
	_ = parseSvc.RefreshCacheBackend(context.Background())
	publicHandler := handler.NewPublicHandler(settingSvc)

	router.Use(middleware.AuditLog(db))
	router.GET("/api/site", middleware.RequireInstalled(state), publicHandler.Site)

	authHandler := handler.NewAuthHandler(db, jwtMgr, settingSvc, captchaSvc)
	parseHandler := handler.NewParseHandler(parseSvc, quotaSvc)
	adminHandler := handler.NewAdminHandler(db, box, settingSvc, statsSvc, parseSvc)
	userAdminHandler := handler.NewUserAdminHandler(db)
	userHandler := handler.NewUserHandler(db, quotaSvc)

	authRoutes := router.Group("/api/auth")
	authRoutes.Use(middleware.RequireInstalled(state))
	{
		authRoutes.POST("/login", limiter.Middleware(), authHandler.Login)
		authRoutes.POST("/register", limiter.Middleware(), authHandler.Register)
		authRoutes.GET("/me", middleware.JWTAuth(jwtMgr, db), userHandler.Me)
	}

	musicRoutes := router.Group("/api/music")
	musicRoutes.Use(middleware.RequireInstalled(state), middleware.OptionalJWTAuth(jwtMgr, db, settingSvc))
	{
		musicRoutes.GET("/providers", parseHandler.Providers)
		musicRoutes.POST("/parse", limiter.Middleware(), parseHandler.ParseNetease)
		musicRoutes.POST("/search", limiter.Middleware(), parseHandler.SearchSong)
		musicRoutes.POST("/playlist", limiter.Middleware(), parseHandler.PlaylistDetail)
		musicRoutes.POST("/lyric", limiter.Middleware(), parseHandler.GetLyric)
		musicRoutes.POST("/lyric/download", limiter.Middleware(), parseHandler.DownloadLyric)
		musicRoutes.POST("/cover/download", limiter.Middleware(), parseHandler.DownloadCover)
	}

	adminRoutes := router.Group("/api/admin")
	adminRoutes.Use(middleware.RequireInstalled(state), middleware.JWTAuth(jwtMgr, db), middleware.AdminOnly())
	{
		adminRoutes.GET("/stats", adminHandler.Stats)
		adminRoutes.GET("/settings", adminHandler.GetSettings)
		adminRoutes.PUT("/settings", adminHandler.SaveSettings)
		adminRoutes.POST("/cookies", adminHandler.AddCookie)
		adminRoutes.GET("/cookies", adminHandler.ListCookies)
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

		adminRoutes.GET("/user-groups", userAdminHandler.ListUserGroups)
		adminRoutes.POST("/user-groups", middleware.SuperAdminOnly(), userAdminHandler.CreateUserGroup)
		adminRoutes.PATCH("/user-groups/:id", middleware.SuperAdminOnly(), userAdminHandler.UpdateUserGroup)
		adminRoutes.DELETE("/user-groups/:id", middleware.SuperAdminOnly(), userAdminHandler.DeleteUserGroup)
	}

	userRoutes := router.Group("/api/user")
	userRoutes.Use(middleware.RequireInstalled(state), middleware.JWTAuth(jwtMgr, db))
	{
		userRoutes.GET("/me", userHandler.Me)
		userRoutes.GET("/quota/today", userHandler.QuotaToday)
		userRoutes.GET("/usage/trend", userHandler.UsageTrend)
	}
	return nil
}

func notInstalled(c *gin.Context) {
	c.JSON(http.StatusPreconditionRequired, gin.H{
		"code": http.StatusPreconditionRequired,
		"msg":  "system not installed, visit /install first",
	})
}

func mountFrontendRoutes(router *gin.Engine, distDir string, state *middleware.InstallState) {
	if embeddedFS, ok := loadEmbeddedFrontend(); ok {
		if err := mountFrontendRoutesFromFS(router, embeddedFS, state); err == nil {
			log.Printf("serving embedded frontend dist")
			return
		}
	}

	indexFile := filepath.Join(distDir, "index.html")
	if _, err := os.Stat(indexFile); err != nil {
		log.Printf("frontend dist not found at %s, skip static routes", distDir)
		return
	}
	log.Printf("serving frontend dist from %s", distDir)

	router.NoRoute(func(c *gin.Context) {
		requestPath := c.Request.URL.Path
		if strings.HasPrefix(requestPath, "/api/") {
			util.Err(c, http.StatusNotFound, "endpoint not found")
			return
		}

		cleanedPath := normalizeRoutePath(requestPath)
		cleaned := strings.TrimPrefix(cleanedPath, "/")
		if cleaned != "" && cleaned != "." {
			candidate := filepath.Join(distDir, filepath.FromSlash(cleaned))
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				c.File(candidate)
				return
			}
			if strings.Contains(filepath.Base(cleaned), ".") {
				util.Err(c, http.StatusNotFound, "asset not found")
				return
			}
		}
		if target, ok := resolveInstallRedirect(cleanedPath, state); ok {
			c.Redirect(http.StatusFound, target)
			return
		}
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
			util.Err(c, http.StatusNotFound, "endpoint not found")
			return
		}

		cleanedPath := normalizeRoutePath(requestPath)
		cleaned := strings.TrimPrefix(cleanedPath, "/")
		if cleaned != "" && cleaned != "." {
			if fileBytes, readErr := fs.ReadFile(frontendFS, cleaned); readErr == nil {
				c.Data(http.StatusOK, detectContentType(cleaned, fileBytes), fileBytes)
				return
			}
			if strings.Contains(path.Base(cleaned), ".") {
				util.Err(c, http.StatusNotFound, "asset not found")
				return
			}
		}
		if target, ok := resolveInstallRedirect(cleanedPath, state); ok {
			c.Redirect(http.StatusFound, target)
			return
		}
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

func ensureSingleSuperAdmin(db *gorm.DB) error {
	var supers []model.User
	if err := db.Where("role = ?", "super_admin").Order("id asc").Find(&supers).Error; err != nil {
		return err
	}
	if len(supers) == 0 {
		var firstAdmin model.User
		if err := db.Where("role = ?", "admin").Order("id asc").First(&firstAdmin).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		return db.Model(&model.User{}).Where("id = ?", firstAdmin.ID).Update("role", "super_admin").Error
	}
	if len(supers) == 1 {
		return nil
	}
	keeperID := supers[0].ID
	return db.Model(&model.User{}).
		Where("role = ? AND id <> ?", "super_admin", keeperID).
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
		if err := tx.Where("role = ?", "super_admin").Order("id asc").First(&superAdmin).Error; err != nil {
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
