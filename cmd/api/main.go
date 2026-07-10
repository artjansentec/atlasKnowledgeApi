package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/atlas/knowledge-api/internal/config"
	"github.com/atlas/knowledge-api/internal/db"
	"github.com/atlas/knowledge-api/internal/handler"
	"github.com/atlas/knowledge-api/internal/middleware"
	"github.com/atlas/knowledge-api/internal/repository"
	"github.com/atlas/knowledge-api/internal/service"
	"github.com/atlas/knowledge-api/internal/storage"
	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
)

func printStartupBanner(cfg *config.Config) {
	base := fmt.Sprintf("http://localhost:%s", cfg.Port)
	lines := []string{
		"",
		"  Atlas Knowledge API",
		"  ─────────────────────────────────────────",
		fmt.Sprintf("  Status      online"),
		fmt.Sprintf("  Porta       %s", cfg.Port),
		fmt.Sprintf("  API         %s/api/v1", base),
		fmt.Sprintf("  Swagger     %s/swagger", base),
		fmt.Sprintf("  Health      %s/api/v1/health", base),
		fmt.Sprintf("  Storage     %s", cfg.StoragePath),
		"  ─────────────────────────────────────────",
		"  Parar       Ctrl+C",
		"",
	}
	fmt.Println(strings.Join(lines, "\n"))
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("❌ config: %v", err)
	}

	ctx := context.Background()
	if err := db.MigrateUp(cfg.DatabaseURL); err != nil {
		log.Fatalf("❌ migrations: %v", err)
	}

	database, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("❌ banco de dados: %v\n   Verifique DATABASE_URL e se o Postgres local está rodando", err)
	}
	defer database.Close()

	fileStore, err := storage.NewLocalFileStorage(cfg.StoragePath)
	if err != nil {
		log.Fatalf("❌ storage: %v", err)
	}

	userRepo := repository.NewUserRepository(database)
	refreshRepo := repository.NewRefreshTokenRepository(database)
	projectRepo := repository.NewProjectRepository(database)
	sectionRepo := repository.NewSectionRepository(database)
	lessonRepo := repository.NewLessonRepository(database)
	fileRepo := repository.NewFileRepository(database)
	attachmentRepo := repository.NewAttachmentRepository(database)
	auditRepo := repository.NewAuditRepository(database)
	tagRepo := repository.NewTagRepository(database)

	authSvc := service.NewAuthService(cfg, userRepo, refreshRepo)
	projectSvc := service.NewProjectService(projectRepo, sectionRepo, lessonRepo, attachmentRepo, fileRepo, tagRepo, auditRepo, userRepo, database.Pool)
	sectionSvc := service.NewSectionService(projectRepo, sectionRepo, auditRepo)
	lessonSvc := service.NewLessonService(projectRepo, lessonRepo, tagRepo, auditRepo, database.Pool)
	attachmentSvc := service.NewAttachmentService(cfg, projectRepo, attachmentRepo, fileRepo, auditRepo, fileStore, database.Pool)
	searchSvc := service.NewSearchService(projectRepo, sectionRepo, lessonRepo, auditRepo, userRepo)
	dashboardSvc := service.NewDashboardService(projectRepo, sectionRepo, lessonRepo, auditRepo, userRepo, tagRepo)
	userSvc := service.NewUserListService(userRepo)

	authMW := middleware.NewAuthMiddleware(authSvc)
	loginLimiter := middleware.NewRateLimiter(10)
	searchLimiter := middleware.NewRateLimiter(10)

	authHandler := handler.NewAuthHandler(cfg, authSvc)
	projectHandler := handler.NewProjectHandler(cfg, projectSvc, userRepo, tagRepo, fileRepo)
	sectionHandler := handler.NewSectionHandler(sectionSvc)
	devSectionHandler := handler.NewDevSectionHandler(sectionSvc)
	lessonHandler := handler.NewLessonHandler(lessonSvc)
	attachmentHandler := handler.NewAttachmentHandler(attachmentSvc)
	devAttachmentHandler := handler.NewDevAttachmentHandler(attachmentSvc)
	searchHandler := handler.NewSearchHandler(searchSvc)
	dashboardHandler := handler.NewDashboardHandler(dashboardSvc)
	userHandler := handler.NewUserHandler(userSvc)

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.Use(echomw.Recover())
	e.Use(echomw.RequestID())
	e.Use(echomw.Logger())
	e.Use(middleware.CORS(cfg))

	handler.RegisterSwagger(e)

	healthHandler := handler.NewHealthHandler(database)
	api := e.Group("/api/v1")
	api.GET("/health", healthHandler.Check)

	auth := api.Group("/auth")
	auth.POST("/login", authHandler.Login, loginLimiter.Middleware())
	auth.POST("/refresh", authHandler.Refresh)
	auth.POST("/logout", authHandler.Logout, authMW.RequireAuth)
	auth.GET("/me", authHandler.Me, authMW.RequireAuth)

	protected := api.Group("", authMW.RequireAuth)
	protected.GET("/users", userHandler.List)
	protected.GET("/dashboard/summary", dashboardHandler.Summary)
	protected.GET("/search", searchHandler.Search, searchLimiter.Middleware())

	protected.GET("/project-statuses", projectHandler.ListStatuses)
	protected.GET("/projects", projectHandler.List)
	protected.GET("/projects/:slug", projectHandler.Get)
	protected.POST("/projects", projectHandler.Create)
	protected.PATCH("/projects/:slug", projectHandler.Patch)
	protected.DELETE("/projects/:slug", projectHandler.Delete)
	protected.PUT("/projects/:slug/readers", projectHandler.SetReaders)

	protected.POST("/projects/:slug/sections", sectionHandler.Create)
	protected.PATCH("/projects/:slug/sections/:sectionId", sectionHandler.Patch)
	protected.DELETE("/projects/:slug/sections/:sectionId", sectionHandler.Delete)
	protected.PUT("/projects/:slug/sections/reorder", sectionHandler.Reorder)

	protected.POST("/projects/:slug/dev-sections", devSectionHandler.Create)
	protected.PATCH("/projects/:slug/dev-sections/:sectionId", devSectionHandler.Patch)
	protected.DELETE("/projects/:slug/dev-sections/:sectionId", devSectionHandler.Delete)
	protected.PUT("/projects/:slug/dev-sections/reorder", devSectionHandler.Reorder)

	protected.POST("/projects/:slug/lessons", lessonHandler.Create)
	protected.PATCH("/projects/:slug/lessons/:lessonId", lessonHandler.Patch)
	protected.DELETE("/projects/:slug/lessons/:lessonId", lessonHandler.Delete)

	protected.POST("/projects/:slug/attachments", attachmentHandler.Upload)
	protected.DELETE("/projects/:slug/attachments/:attachmentId", attachmentHandler.Delete)

	protected.POST("/projects/:slug/dev-attachments", devAttachmentHandler.Upload)
	protected.DELETE("/projects/:slug/dev-attachments/:attachmentId", devAttachmentHandler.Delete)

	protected.GET("/files/:fileId/download", attachmentHandler.Download)

	go func() {
		printStartupBanner(cfg)
		if err := e.Start(cfg.ServerAddress()); err != nil && err != http.ErrServerClosed {
			if strings.Contains(err.Error(), "bind") {
				log.Fatalf("\n❌ Porta %s em uso.\n   Feche a instância anterior ou rode: Get-NetTCPConnection -LocalPort %s -State Listen | %% {{ Stop-Process -Id $_.OwningProcess -Force }}\n   Ou altere PORT no arquivo .env\n", cfg.Port, cfg.Port)
			}
			log.Fatalf("❌ servidor: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("\n⏹  Encerrando API...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("❌ shutdown: %v", err)
	}
	fmt.Println("✅ API encerrada.")
}
