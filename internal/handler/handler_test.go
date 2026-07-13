package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/atlas/knowledge-api/internal/config"
	"github.com/atlas/knowledge-api/internal/db"
	"github.com/atlas/knowledge-api/internal/domain"
	"github.com/atlas/knowledge-api/internal/handler"
	"github.com/atlas/knowledge-api/internal/middleware"
	"github.com/atlas/knowledge-api/internal/repository"
	"github.com/atlas/knowledge-api/internal/service"
	"github.com/atlas/knowledge-api/internal/storage"
	"github.com/labstack/echo/v4"
)

func setupTestEnv(t *testing.T) (*echo.Echo, *db.DB, func()) {
	t.Helper()
	databaseURL := getenv("TEST_DATABASE_URL", "postgres://postgres:postgres@localhost:5432/atlas_knowledge?sslmode=disable")
	cfg := &config.Config{
		Port: "8080", DatabaseURL: databaseURL, JWTSecret: "test-secret",
		JWTAccessTTL: mustDuration("15m"), JWTRefreshTTL: mustDuration("168h"),
		StoragePath: t.TempDir(), MaxUploadBytes: 20971520,
		CORSOrigins: []string{"http://localhost:5173"}, RefreshCookie: "refresh_token",
		APIBaseURL: "http://localhost:8080",
	}

	ctx := context.Background()
	database, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Skipf("postgres indisponível: %v", err)
	}
	cleanup := func() { database.Close() }

	userRepo := repository.NewUserRepository(database)
	refreshRepo := repository.NewRefreshTokenRepository(database)
	projectRepo := repository.NewProjectRepository(database)
	sectionRepo := repository.NewSectionRepository(database)
	lessonRepo := repository.NewLessonRepository(database)
	fileRepo := repository.NewFileRepository(database)
	attachmentRepo := repository.NewAttachmentRepository(database)
	auditRepo := repository.NewAuditRepository(database)
	tagRepo := repository.NewTagRepository(database)
	_, _ = storage.NewLocalFileStorage(cfg.StoragePath)

	authSvc := service.NewAuthService(cfg, userRepo, refreshRepo)
	projectSvc := service.NewProjectService(projectRepo, sectionRepo, lessonRepo, attachmentRepo, fileRepo, tagRepo, auditRepo, userRepo, database.Pool)
	sectionSvc := service.NewSectionService(projectRepo, sectionRepo, auditRepo)
	authMW := middleware.NewAuthMiddleware(authSvc, cfg)

	authHandler := handler.NewAuthHandler(cfg, authSvc)
	projectHandler := handler.NewProjectHandler(cfg, projectSvc, userRepo, tagRepo, fileRepo)
	sectionHandler := handler.NewSectionHandler(sectionSvc)

	e := echo.New()
	api := e.Group("/api/v1")
	api.POST("/auth/login", authHandler.Login)
	protected := api.Group("", authMW.RequireAuth)
	protected.GET("/projects", projectHandler.List)
	protected.POST("/projects", projectHandler.Create)
	protected.PATCH("/projects/:slug/sections/:sectionId", sectionHandler.Patch)
	protected.PUT("/projects/:slug/sections/reorder", sectionHandler.Reorder)

	seedTestData(t, ctx, database, userRepo, projectRepo, sectionRepo)

	return e, database, cleanup
}

func seedTestData(t *testing.T, ctx context.Context, database *db.DB, users *repository.UserRepository, projects *repository.ProjectRepository, sections *repository.SectionRepository) {
	t.Helper()
	_, _ = database.Pool.Exec(ctx, `TRUNCATE users, projects, project_sections, project_members, project_dev_responsibles, project_lessons, project_attachments, files, tags, project_tags, project_tech, lesson_tags, audit_events, refresh_tokens CASCADE`)

	adminHash, _ := service.HashPassword("admin123")
	userHash, _ := service.HashPassword("user123")

	tx, err := database.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)

	admin := domain.User{Email: "admin@test.com", PasswordHash: adminHash, Name: "Admin", Role: domain.RoleAdmin, IsActive: true}
	reader := domain.User{Email: "reader@test.com", PasswordHash: userHash, Name: "Leitor", Role: domain.RoleConsultor, IsActive: true}
	responsible := domain.User{Email: "resp@test.com", PasswordHash: userHash, Name: "Responsavel", Role: domain.RoleConsultor, IsActive: true}
	for _, u := range []*domain.User{&admin, &reader, &responsible} {
		if err := users.Create(ctx, tx, u); err != nil {
			t.Fatal(err)
		}
	}

	project := &domain.Project{
		Slug: "test-project", Name: "Test Project", Description: "Desc",
		Status: domain.StatusActive, ResponsibleUserID: responsible.ID,
	}
	if err := projects.Create(ctx, tx, project); err != nil {
		t.Fatal(err)
	}
	_, _ = tx.Exec(ctx, `INSERT INTO project_members (project_id, user_id, role) VALUES ($1, $2, 'reader')`, project.ID, reader.ID)

	section := &domain.Section{ProjectID: project.ID, Title: "Overview", Content: "# Wiki test", SortOrder: 0}
	if err := sections.Create(ctx, tx, section); err != nil {
		t.Fatal(err)
	}

	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}

func login(t *testing.T, e *echo.Echo, email, password string) string {
	t.Helper()
	body := bytes.NewBufferString(`{"email":"` + email + `","password":"` + password + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("login status %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		AccessToken string `json:"accessToken"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	return resp.AccessToken
}

func TestLoginValid(t *testing.T) {
	e, _, cleanup := setupTestEnv(t)
	defer cleanup()
	token := login(t, e, "admin@test.com", "admin123")
	if token == "" {
		t.Fatal("token vazio")
	}
}

func TestLoginInvalid(t *testing.T) {
	e, _, cleanup := setupTestEnv(t)
	defer cleanup()
	body := bytes.NewBufferString(`{"email":"admin@test.com","password":"wrong"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("esperado 401, obteve %d", rec.Code)
	}
}

func TestUserCannotCreateProject(t *testing.T) {
	e, _, cleanup := setupTestEnv(t)
	defer cleanup()
	token := login(t, e, "reader@test.com", "user123")
	body := bytes.NewBufferString(`{"name":"Novo","description":"d","responsibleUserId":"00000000-0000-0000-0000-000000000001"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("esperado 403, obteve %d", rec.Code)
	}
}

func TestResponsibleCanPatchSectionReaderCannot(t *testing.T) {
	e, database, cleanup := setupTestEnv(t)
	defer cleanup()

	var sectionID string
	_ = database.Pool.QueryRow(context.Background(), `SELECT id FROM project_sections LIMIT 1`).Scan(&sectionID)

	respToken := login(t, e, "resp@test.com", "user123")
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/projects/test-project/sections/"+sectionID, bytes.NewBufferString(`{"content":"# ok"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+respToken)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("responsável: esperado 200, obteve %d: %s", rec.Code, rec.Body.String())
	}

	readerToken := login(t, e, "reader@test.com", "user123")
	req2 := httptest.NewRequest(http.MethodPatch, "/api/v1/projects/test-project/sections/"+sectionID, bytes.NewBufferString(`{"content":"# fail"}`))
	req2.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req2.Header.Set(echo.HeaderAuthorization, "Bearer "+readerToken)
	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusForbidden {
		t.Fatalf("leitor: esperado 403, obteve %d", rec2.Code)
	}
}

func TestSearchReturnsProjectByTitle(t *testing.T) {
	e, _, cleanup := setupTestEnv(t)
	defer cleanup()

	token := login(t, e, "admin@test.com", "admin123")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects?q=Test", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, obteve %d", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("Test Project")) {
		t.Fatalf("projeto não encontrado na listagem: %s", rec.Body.String())
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func mustDuration(s string) time.Duration {
	d, _ := time.ParseDuration(s)
	return d
}
