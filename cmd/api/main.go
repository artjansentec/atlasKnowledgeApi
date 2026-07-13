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

	"github.com/atlas/knowledge-api/internal/ai"
	"github.com/atlas/knowledge-api/internal/bootstrap"
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
	aiBase := strings.TrimRight(cfg.AIServiceURL, "/")
	aiGenerateURL := aiBase + "/v1/knowledge/document"
	aiJobsURL := aiBase + "/v1/jobs/:id"
	aiStatus, aiHealthURL := probeMnimosHealth(aiBase)

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
		fmt.Sprintf("  Mnimos AI   %s", aiGenerateURL),
		fmt.Sprintf("  Mnimos Jobs %s", aiJobsURL),
		fmt.Sprintf("  Mnimos      %s  (%s)", aiStatus, aiHealthURL),
		"  ─────────────────────────────────────────",
		"  Parar       Ctrl+C",
		"",
	}
	fmt.Println(strings.Join(lines, "\n"))
}

// probeMnimosHealth verifica /health e /v1/health no host de AI_SERVICE_URL.
func probeMnimosHealth(aiBase string) (status, checkedURL string) {
	client := &http.Client{Timeout: 2 * time.Second}
	candidates := []string{
		aiBase + "/health",
		aiBase + "/v1/health",
	}

	var lastURL string
	for _, url := range candidates {
		lastURL = url
		resp, err := client.Get(url)
		if err != nil {
			continue
		}
		_ = resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return "online", url
		}
	}
	return "offline", lastURL
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

	adminResult, err := bootstrap.EnsureDefaultAdmin(ctx, database, bootstrap.EnsureAdminOptions{
		Email:    cfg.AdminEmail,
		Password: cfg.AdminPassword,
		Name:     cfg.AdminName,
	})
	if err != nil {
		log.Fatalf("❌ admin inicial: %v", err)
	}
	if adminResult.Created {
		fmt.Printf("admin criado: %s (%s)\n", adminResult.Name, adminResult.Email)
	}

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
	documentationRepo := repository.NewDocumentationRepository(database)

	aiClient := ai.NewClient(cfg.AIServiceURL, cfg.AIServiceTimeout)

	authSvc := service.NewAuthService(cfg, userRepo, refreshRepo)
	projectSvc := service.NewProjectService(projectRepo, sectionRepo, lessonRepo, attachmentRepo, fileRepo, tagRepo, auditRepo, userRepo, database.Pool)
	sectionSvc := service.NewSectionService(projectRepo, sectionRepo, auditRepo)
	lessonSvc := service.NewLessonService(projectRepo, lessonRepo, tagRepo, auditRepo, database.Pool)
	attachmentSvc := service.NewAttachmentService(cfg, projectRepo, attachmentRepo, fileRepo, auditRepo, fileStore, database.Pool)
	searchSvc := service.NewSearchService(projectRepo, sectionRepo, lessonRepo, auditRepo, userRepo)
	dashboardSvc := service.NewDashboardService(projectRepo, sectionRepo, lessonRepo, auditRepo, userRepo, tagRepo)
	userSvc := service.NewUserListService(userRepo)
	documentationSvc := service.NewDocumentationService(
		cfg, projectRepo, documentationRepo, fileRepo, userRepo, auditRepo, fileStore, aiClient, database.Pool,
	)

	authMW := middleware.NewAuthMiddleware(authSvc, cfg)
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
	documentationHandler := handler.NewDocumentationHandler(documentationSvc)
	mnemosSvc := service.NewMnemosService(
		cfg, projectRepo, sectionRepo, attachmentRepo, fileRepo, userRepo, auditRepo, fileStore, database.Pool,
	)
	mnemosHandler := handler.NewMnemosHandler(mnemosSvc)

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.Use(echomw.Recover())
	e.Use(echomw.RequestID())
	e.Use(echomw.Logger())
	e.Use(middleware.CORS(cfg))
	e.Use(echomw.BodyLimit(fmt.Sprintf("%dB", cfg.DocMaxTotalBytes+1024*1024)))

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

	protected.POST("/projects/:slug/documentation/generate", documentationHandler.Generate)
	protected.GET("/projects/:slug/documentation", documentationHandler.GetLatest)
	protected.GET("/projects/:slug/documentation/versions", documentationHandler.ListVersions)
	protected.GET("/projects/:slug/documentation/:versionId", documentationHandler.GetVersion)
	protected.DELETE("/projects/:slug/documentation/:versionId", documentationHandler.DeleteVersion)
	protected.POST("/projects/:slug/documentation/:versionId/regenerate", documentationHandler.Regenerate)
	protected.GET("/documentation/jobs", documentationHandler.ListJobs)
	protected.GET("/documentation/jobs/:jobId", documentationHandler.GetJob)
	protected.POST("/documentation/jobs/:jobId/cancel", documentationHandler.CancelJob)

	// Integração Mnemos: admin JWT ou X-Api-Key (MNEMOS_API_KEY)
	mnemosAPI := api.Group("/mnemos", authMW.RequireMnemosAuth)
	mnemosAPI.POST("/projects", mnemosHandler.Sync)
	mnemosAPI.PATCH("/projects/:slug", mnemosHandler.Patch)
	mnemosAPI.PUT("/projects/:slug/structure", mnemosHandler.ApplyStructure)
	mnemosAPI.POST("/projects/:slug/attachments", mnemosHandler.UploadAttachments)

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
