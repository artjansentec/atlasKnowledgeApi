package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/atlas/knowledge-api/internal/ai"
	"github.com/atlas/knowledge-api/internal/config"
	"github.com/atlas/knowledge-api/internal/domain"
	"github.com/atlas/knowledge-api/internal/repository"
	"github.com/atlas/knowledge-api/internal/storage"
	"github.com/atlas/knowledge-api/pkg/httperr"
)

var docAllowedExt = map[string]bool{
	".pdf":  true,
	".txt":  true,
	".md":   true,
	".markdown": true,
	".doc":  true,
	".docx": true,
	".rtf":  true,
	".png":  true,
	".jpg":  true,
	".jpeg": true,
	".gif":  true,
	".webp": true,
	".csv":  true,
	".json": true,
	".xml":  true,
	".html": true,
	".htm":  true,
}

var docBlockedExt = map[string]bool{
	".exe": true, ".bat": true, ".cmd": true, ".com": true, ".msi": true,
	".scr": true, ".ps1": true, ".sh": true, ".bash": true, ".dll": true,
	".so": true, ".dylib": true, ".jar": true, ".apk": true, ".bin": true,
}

var docAllowedMimes = map[string]bool{
	"application/pdf":    true,
	"text/plain":         true,
	"text/markdown":      true,
	"text/csv":           true,
	"text/html":          true,
	"text/xml":           true,
	"application/json":   true,
	"application/xml":    true,
	"application/msword": true,
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
	"application/rtf": true,
	"image/png":       true,
	"image/jpeg":      true,
	"image/gif":       true,
	"image/webp":      true,
}

type DocumentationService struct {
	cfg      *config.Config
	projects *repository.ProjectRepository
	docs     *repository.DocumentationRepository
	files    *repository.FileRepository
	users    *repository.UserRepository
	audit    *repository.AuditRepository
	storage  storage.FileStorage
	ai       *ai.Client
	pool     pgxPool
}

func NewDocumentationService(
	cfg *config.Config,
	projects *repository.ProjectRepository,
	docs *repository.DocumentationRepository,
	files *repository.FileRepository,
	users *repository.UserRepository,
	audit *repository.AuditRepository,
	store storage.FileStorage,
	aiClient *ai.Client,
	pool pgxPool,
) *DocumentationService {
	return &DocumentationService{
		cfg: cfg, projects: projects, docs: docs, files: files,
		users: users, audit: audit, storage: store, ai: aiClient, pool: pool,
	}
}

type GenerateInput struct {
	ProjectName       string
	Description       string
	GenerationOptions json.RawMessage
	Files             []*multipart.FileHeader
}

type JobStatusResponse struct {
	JobID       string `json:"job_id"`
	Status      string `json:"status"`
	Progress    int    `json:"progress,omitempty"`
	CurrentStep string `json:"current_step,omitempty"`
	Message     string `json:"message,omitempty"`
	Error       string `json:"error,omitempty"`
}

type ActiveJobItem struct {
	JobID        string     `json:"job_id"`
	Status       string     `json:"status"`
	Progress     int        `json:"progress"`
	CurrentStep  string     `json:"current_step"`
	ProjectID    string     `json:"project_id"`
	ProjectSlug  string     `json:"project_slug"`
	ProjectName  string     `json:"project_name"`
	FileCount    int        `json:"file_count"`
	CreatedBy    string     `json:"created_by"`
	UserName     string     `json:"user_name"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

type VersionListItem struct {
	ID             string    `json:"id"`
	Version        int       `json:"version"`
	CreatedAt      time.Time `json:"created_at"`
	UserID         string    `json:"user_id"`
	UserName       string    `json:"user_name"`
	ModelUsed      string    `json:"model_used"`
	ProcessingMs   int64     `json:"processing_ms"`
	FileCount      int       `json:"file_count"`
	TotalSizeBytes int64     `json:"total_size_bytes"`
	Language       string    `json:"language"`
}

type VersionDetailResponse struct {
	ID                string          `json:"id"`
	Version           int             `json:"version"`
	ProjectID         string          `json:"project_id"`
	JobID             string          `json:"job_id"`
	CreatedAt         time.Time       `json:"created_at"`
	UserID            string          `json:"user_id"`
	UserName          string          `json:"user_name"`
	ModelUsed         string          `json:"model_used"`
	Language          string          `json:"language"`
	ProcessingMs      int64           `json:"processing_ms"`
	FileCount         int             `json:"file_count"`
	TotalSizeBytes    int64           `json:"total_size_bytes"`
	GenerationOptions json.RawMessage `json:"generation_options"`
	Content           json.RawMessage `json:"content"`
}

func (s *DocumentationService) loadProject(ctx context.Context, user domain.User, slug string, manage bool) (*domain.Project, error) {
	project, err := s.projects.GetBySlug(ctx, slug)
	if err != nil || project == nil {
		return nil, httperr.NotFound("projeto não encontrado")
	}
	members, err := s.projects.ListMembers(ctx, project.ID)
	if err != nil {
		return nil, httperr.Internal("falha ao verificar permissão")
	}
	if manage {
		if err := requireManage(user, *project); err != nil {
			return nil, err
		}
	} else if !CanReadProject(user, *project, members) {
		return nil, httperr.Forbidden("sem permissão para acessar este projeto")
	}
	return project, nil
}

func (s *DocumentationService) Generate(ctx context.Context, user domain.User, slug string, input GenerateInput) (*JobStatusResponse, error) {
	project, err := s.loadProject(ctx, user, slug, true)
	if err != nil {
		return nil, err
	}

	active, err := s.docs.HasActiveJob(ctx, project.ID)
	if err != nil {
		return nil, httperr.Internal("falha ao verificar processamentos em andamento")
	}
	if active {
		return nil, httperr.Conflict("já existe um processamento em andamento para o projeto")
	}

	if len(input.Files) == 0 {
		return nil, httperr.Validation("pelo menos um arquivo deve ser enviado")
	}
	if len(input.Files) > s.cfg.DocMaxFiles {
		return nil, httperr.Validation("quantidade de arquivos excede o limite permitido")
	}

	var totalSize int64
	for _, fh := range input.Files {
		if err := s.validateFileHeader(fh); err != nil {
			return nil, err
		}
		totalSize += fh.Size
	}
	if totalSize > s.cfg.DocMaxTotalBytes {
		return nil, httperr.PayloadTooLarge("tamanho total dos arquivos excede o limite permitido")
	}

	opts := input.GenerationOptions
	if len(opts) == 0 {
		opts = json.RawMessage("{}")
	} else if !json.Valid(opts) {
		return nil, httperr.BadRequest("generation_options deve ser um JSON válido")
	}

	projectName := strings.TrimSpace(input.ProjectName)
	if projectName == "" {
		projectName = project.Name
	}
	description := strings.TrimSpace(input.Description)
	if description == "" {
		description = project.Description
	}

	now := time.Now().UTC()
	job := &domain.DocumentationJob{
		ProjectID:         project.ID,
		CreatedBy:         user.ID,
		Status:            domain.DocJobPending,
		Progress:          0,
		CurrentStep:       "Aguardando processamento",
		ProjectName:       projectName,
		Description:       description,
		GenerationOptions: opts,
		FileCount:         len(input.Files),
		TotalSizeBytes:    totalSize,
		StartedAt:         &now,
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, httperr.Internal("falha ao iniciar transação")
	}
	defer tx.Rollback(ctx)

	if err := s.docs.CreateJob(ctx, tx, job); err != nil {
		if isUniqueViolation(err) {
			return nil, httperr.Conflict("já existe um processamento em andamento para o projeto")
		}
		return nil, httperr.Internal("falha ao criar job de documentação")
	}

	savedKeys := make([]string, 0, len(input.Files))
	for _, fh := range input.Files {
		src, err := fh.Open()
		if err != nil {
			s.cleanupKeys(ctx, savedKeys)
			return nil, httperr.Internal("falha ao ler arquivo enviado")
		}

		hasher := sha256.New()
		reader := io.TeeReader(src, hasher)
		key, err := s.storage.Save(ctx, fh.Filename, reader)
		src.Close()
		if err != nil {
			s.cleanupKeys(ctx, savedKeys)
			return nil, httperr.Internal("falha ao armazenar arquivo")
		}
		savedKeys = append(savedKeys, key)

		hash := hex.EncodeToString(hasher.Sum(nil))
		mimeType := detectDocMime(fh.Filename)
		file := &domain.FileRecord{
			StorageKey:   key,
			OriginalName: fh.Filename,
			MimeType:     mimeType,
			SizeBytes:    fh.Size,
			UploadedBy:   user.ID,
		}
		if err := s.files.Create(ctx, tx, file); err != nil {
			s.cleanupKeys(ctx, savedKeys)
			return nil, httperr.Internal("falha ao registrar arquivo")
		}
		docFile := &domain.DocumentationFile{
			JobID:       job.ID,
			FileID:      file.ID,
			ContentHash: &hash,
		}
		if err := s.docs.AddFile(ctx, tx, docFile); err != nil {
			s.cleanupKeys(ctx, savedKeys)
			return nil, httperr.Internal("falha ao vincular arquivo ao job")
		}
	}

	actorID := user.ID
	_ = s.audit.Create(ctx, tx, &domain.AuditEvent{
		ProjectID: project.ID, ActorUserID: &actorID,
		Action: "Iniciou geração", Target: "documentação",
		EntityType: strPtr("documentation_job"), EntityID: strPtr(job.ID),
	})

	if err := tx.Commit(ctx); err != nil {
		s.cleanupKeys(ctx, savedKeys)
		return nil, httperr.Internal("falha ao confirmar geração")
	}

	go s.processJob(job.ID)

	return &JobStatusResponse{
		JobID:   job.ID,
		Status:  string(domain.DocJobPending),
		Message: "Processamento iniciado.",
	}, nil
}

func (s *DocumentationService) processJob(jobID string) {
	ctx, cancel := context.WithTimeout(context.Background(), s.cfg.AIServiceTimeout+2*time.Minute)
	defer cancel()

	job, err := s.docs.GetJobByID(ctx, jobID)
	if err != nil || job == nil {
		log.Printf("documentation job %s: não encontrado", jobID)
		return
	}
	if job.Status == domain.DocJobCancelled {
		return
	}

	fail := func(msg string) {
		_ = s.docs.UpdateJobStatus(ctx, jobID, domain.DocJobFailed, job.Progress, "Falhou", &msg)
	}

	if err := s.docs.UpdateJobStatus(ctx, jobID, domain.DocJobValidating, 10, "Validando arquivos", nil); err != nil {
		fail("falha ao atualizar status")
		return
	}
	if s.isCancelled(ctx, jobID) {
		return
	}

	if err := s.docs.UpdateJobStatus(ctx, jobID, domain.DocJobUploadingFiles, 25, "Preparando arquivos", nil); err != nil {
		fail("falha ao atualizar status")
		return
	}

	docFiles, err := s.docs.ListFilesByJob(ctx, jobID)
	if err != nil {
		fail("falha ao carregar arquivos do job")
		return
	}

	aiFiles := make([]ai.FileInput, 0, len(docFiles))
	closers := make([]io.Closer, 0, len(docFiles))
	defer func() {
		for _, c := range closers {
			_ = c.Close()
		}
	}()

	for _, df := range docFiles {
		file, err := s.files.GetByID(ctx, df.FileID)
		if err != nil || file == nil {
			fail("arquivo do job não encontrado")
			return
		}
		reader, err := s.storage.Open(ctx, file.StorageKey)
		if err != nil {
			fail("falha ao abrir arquivo no storage")
			return
		}
		closers = append(closers, reader)
		aiFiles = append(aiFiles, ai.FileInput{
			Name:     file.OriginalName,
			MimeType: file.MimeType,
			Reader:   reader,
		})
	}

	if s.isCancelled(ctx, jobID) {
		return
	}
	if err := s.docs.UpdateJobStatus(ctx, jobID, domain.DocJobWaitingAI, 40, "Enviando ao Serviço de IA", nil); err != nil {
		fail("falha ao atualizar status")
		return
	}

	started := time.Now()
	aiCtx, aiCancel := context.WithTimeout(ctx, s.cfg.AIServiceTimeout)
	defer aiCancel()

	if err := s.docs.UpdateJobStatus(ctx, jobID, domain.DocJobProcessing, 60, "Aguardando Serviço de IA", nil); err != nil {
		fail("falha ao atualizar status")
		return
	}

	project, err := s.projects.GetByID(ctx, job.ProjectID)
	if err != nil || project == nil {
		fail("projeto do job não encontrado")
		return
	}

	result, err := s.ai.Generate(aiCtx, ai.GenerateRequest{
		ProjectID:         job.ProjectID,
		ProjectSlug:       project.Slug,
		ProjectName:       job.ProjectName,
		Description:       job.Description,
		GenerationOptions: job.GenerationOptions,
		Files:             aiFiles,
		RequestedBy:       job.CreatedBy,
		OnProgress: func(st ai.JobStatus) {
			if s.isCancelled(ctx, jobID) {
				return
			}
			// Mapeia progresso remoto (0–100) para a faixa Atlas 40–95.
			progress := 40 + (st.Progress * 55 / 100)
			if progress < 40 {
				progress = 40
			}
			if progress > 95 {
				progress = 95
			}
			step := st.CurrentStage
			if step == "" {
				step = st.Status
			}
			if step == "" {
				step = "Processando no Serviço de IA"
			}
			_ = s.docs.UpdateJobStatus(ctx, jobID, domain.DocJobProcessing, progress, step, nil)
		},
	})
	if err != nil {
		msg := err.Error()
		var httpErr *httperr.Error
		if errors.As(err, &httpErr) {
			msg = httpErr.Message
		}
		fail(msg)
		return
	}

	if s.isCancelled(ctx, jobID) {
		return
	}

	processingMs := time.Since(started).Milliseconds()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		fail("falha ao iniciar transação de persistência")
		return
	}
	defer tx.Rollback(ctx)

	versionNum, err := s.docs.NextVersionNumber(ctx, tx, job.ProjectID)
	if err != nil {
		fail("falha ao calcular número da versão")
		return
	}

	version := &domain.DocumentationVersion{
		ProjectID:         job.ProjectID,
		JobID:             job.ID,
		CreatedBy:         job.CreatedBy,
		VersionNumber:     versionNum,
		Content:           result.Content,
		ModelUsed:         result.ModelUsed,
		Language:          result.Language,
		ProcessingMs:      processingMs,
		FileCount:         job.FileCount,
		TotalSizeBytes:    job.TotalSizeBytes,
		GenerationOptions: job.GenerationOptions,
	}
	if err := s.docs.CreateVersion(ctx, tx, version); err != nil {
		fail("falha ao salvar documentação gerada")
		return
	}

	actorID := job.CreatedBy
	_ = s.audit.Create(ctx, tx, &domain.AuditEvent{
		ProjectID: job.ProjectID, ActorUserID: &actorID,
		Action: "Gerou documentação", Target: "versão " + strconv.Itoa(versionNum),
		EntityType: strPtr("documentation_version"), EntityID: strPtr(version.ID),
	})

	if err := tx.Commit(ctx); err != nil {
		fail("falha ao confirmar documentação")
		return
	}

	_ = s.docs.LinkFilesToVersion(ctx, job.ID, version.ID)
	_ = s.docs.SetJobVersion(ctx, job.ID, version.ID)
	_ = s.docs.UpdateJobStatus(ctx, job.ID, domain.DocJobCompleted, 100, "Concluído", nil)
}

func (s *DocumentationService) isCancelled(ctx context.Context, jobID string) bool {
	job, err := s.docs.GetJobByID(ctx, jobID)
	return err == nil && job != nil && job.Status == domain.DocJobCancelled
}

func (s *DocumentationService) GetJob(ctx context.Context, user domain.User, jobID string) (*JobStatusResponse, error) {
	job, err := s.docs.GetJobByID(ctx, jobID)
	if err != nil || job == nil {
		return nil, httperr.NotFound("job não encontrado")
	}
	if _, err := s.authorizeJobAccess(ctx, user, job); err != nil {
		return nil, err
	}

	resp := &JobStatusResponse{
		JobID:  job.ID,
		Status: string(job.Status),
	}
	if job.Status != domain.DocJobCompleted {
		resp.Progress = job.Progress
		resp.CurrentStep = job.CurrentStep
	}
	if job.ErrorMessage != nil {
		resp.Error = *job.ErrorMessage
	}
	return resp, nil
}

func (s *DocumentationService) ListActiveJobs(ctx context.Context, user domain.User, projectSlug string) ([]ActiveJobItem, error) {
	var projectID *string
	if projectSlug != "" {
		project, err := s.loadProject(ctx, user, projectSlug, false)
		if err != nil {
			return nil, err
		}
		projectID = &project.ID
	}

	rows, err := s.docs.ListActiveJobs(ctx, projectID)
	if err != nil {
		return nil, httperr.Internal("falha ao listar jobs em andamento")
	}

	items := make([]ActiveJobItem, 0, len(rows))
	for _, row := range rows {
		project, err := s.projects.GetByID(ctx, row.Job.ProjectID)
		if err != nil || project == nil {
			continue
		}
		members, err := s.projects.ListMembers(ctx, project.ID)
		if err != nil {
			return nil, httperr.Internal("falha ao verificar permissão")
		}
		if !CanReadProject(user, *project, members) {
			continue
		}

		userName := ""
		if u, _ := s.users.GetByID(ctx, row.Job.CreatedBy); u != nil {
			userName = u.Name
		}
		name := row.Job.ProjectName
		if name == "" {
			name = row.ProjectName
		}
		items = append(items, ActiveJobItem{
			JobID:       row.Job.ID,
			Status:      string(row.Job.Status),
			Progress:    row.Job.Progress,
			CurrentStep: row.Job.CurrentStep,
			ProjectID:   row.Job.ProjectID,
			ProjectSlug: row.ProjectSlug,
			ProjectName: name,
			FileCount:   row.Job.FileCount,
			CreatedBy:   row.Job.CreatedBy,
			UserName:    userName,
			StartedAt:   row.Job.StartedAt,
			CreatedAt:   row.Job.CreatedAt,
		})
	}
	return items, nil
}

func (s *DocumentationService) CancelJob(ctx context.Context, user domain.User, jobID string) (*JobStatusResponse, error) {
	job, err := s.docs.GetJobByID(ctx, jobID)
	if err != nil || job == nil {
		return nil, httperr.NotFound("job não encontrado")
	}
	project, err := s.authorizeJobAccess(ctx, user, job)
	if err != nil {
		return nil, err
	}
	if err := requireManage(user, *project); err != nil {
		return nil, err
	}
	if job.Status.IsTerminal() {
		return nil, httperr.Conflict("o processamento já foi finalizado e não pode ser cancelado")
	}

	cancelled, err := s.docs.CancelJob(ctx, jobID)
	if err != nil || cancelled == nil {
		return nil, httperr.Conflict("não foi possível cancelar o processamento")
	}

	actorID := user.ID
	_ = s.audit.Create(ctx, nil, &domain.AuditEvent{
		ProjectID: job.ProjectID, ActorUserID: &actorID,
		Action: "Cancelou geração", Target: "documentação",
		EntityType: strPtr("documentation_job"), EntityID: strPtr(jobID),
	})

	return &JobStatusResponse{
		JobID:       cancelled.ID,
		Status:      string(cancelled.Status),
		Progress:    cancelled.Progress,
		CurrentStep: cancelled.CurrentStep,
		Message:     "Processamento cancelado.",
	}, nil
}

func (s *DocumentationService) GetLatest(ctx context.Context, user domain.User, slug string) (*VersionDetailResponse, error) {
	project, err := s.loadProject(ctx, user, slug, false)
	if err != nil {
		return nil, err
	}
	version, err := s.docs.GetLatestVersion(ctx, project.ID)
	if err != nil {
		return nil, httperr.Internal("falha ao buscar documentação")
	}
	if version == nil {
		return nil, httperr.NotFound("nenhuma documentação gerada para este projeto")
	}
	return s.toVersionDetail(ctx, version)
}

func (s *DocumentationService) ListVersions(ctx context.Context, user domain.User, slug string) ([]VersionListItem, error) {
	project, err := s.loadProject(ctx, user, slug, false)
	if err != nil {
		return nil, err
	}
	versions, err := s.docs.ListVersions(ctx, project.ID)
	if err != nil {
		return nil, httperr.Internal("falha ao listar versões")
	}

	items := make([]VersionListItem, 0, len(versions))
	for _, v := range versions {
		name := ""
		if u, _ := s.users.GetByID(ctx, v.CreatedBy); u != nil {
			name = u.Name
		}
		items = append(items, VersionListItem{
			ID:             v.ID,
			Version:        v.VersionNumber,
			CreatedAt:      v.CreatedAt,
			UserID:         v.CreatedBy,
			UserName:       name,
			ModelUsed:      v.ModelUsed,
			ProcessingMs:   v.ProcessingMs,
			FileCount:      v.FileCount,
			TotalSizeBytes: v.TotalSizeBytes,
			Language:       v.Language,
		})
	}
	return items, nil
}

func (s *DocumentationService) GetVersion(ctx context.Context, user domain.User, slug, versionID string) (*VersionDetailResponse, error) {
	project, err := s.loadProject(ctx, user, slug, false)
	if err != nil {
		return nil, err
	}
	version, err := s.docs.GetVersionByID(ctx, project.ID, versionID)
	if err != nil {
		return nil, httperr.Internal("falha ao buscar versão")
	}
	if version == nil {
		return nil, httperr.NotFound("versão de documentação não encontrada")
	}
	return s.toVersionDetail(ctx, version)
}

func (s *DocumentationService) DeleteVersion(ctx context.Context, user domain.User, slug, versionID string) error {
	project, err := s.loadProject(ctx, user, slug, true)
	if err != nil {
		return err
	}
	version, err := s.docs.SoftDeleteVersion(ctx, project.ID, versionID)
	if err != nil {
		return httperr.Internal("falha ao remover documentação")
	}
	if version == nil {
		return httperr.NotFound("versão de documentação não encontrada")
	}

	actorID := user.ID
	_ = s.audit.Create(ctx, nil, &domain.AuditEvent{
		ProjectID: project.ID, ActorUserID: &actorID,
		Action: "Removeu documentação", Target: "versão " + strconv.Itoa(version.VersionNumber),
		EntityType: strPtr("documentation_version"), EntityID: strPtr(versionID),
	})
	return nil
}

func (s *DocumentationService) Regenerate(ctx context.Context, user domain.User, slug, versionID string) (*JobStatusResponse, error) {
	project, err := s.loadProject(ctx, user, slug, true)
	if err != nil {
		return nil, err
	}

	active, err := s.docs.HasActiveJob(ctx, project.ID)
	if err != nil {
		return nil, httperr.Internal("falha ao verificar processamentos em andamento")
	}
	if active {
		return nil, httperr.Conflict("já existe um processamento em andamento para o projeto")
	}

	source, err := s.docs.GetVersionByID(ctx, project.ID, versionID)
	if err != nil || source == nil {
		return nil, httperr.NotFound("versão de documentação não encontrada")
	}

	sourceFiles, err := s.docs.ListFilesByVersion(ctx, source.ID)
	if err != nil {
		return nil, httperr.Internal("falha ao carregar arquivos da versão")
	}
	if len(sourceFiles) == 0 {
		// Fallback: arquivos ligados ao job original.
		sourceFiles, err = s.docs.ListFilesByJob(ctx, source.JobID)
		if err != nil || len(sourceFiles) == 0 {
			return nil, httperr.Validation("nenhum arquivo disponível para reprocessar")
		}
	}

	now := time.Now().UTC()
	job := &domain.DocumentationJob{
		ProjectID:         project.ID,
		CreatedBy:         user.ID,
		Status:            domain.DocJobPending,
		Progress:          0,
		CurrentStep:       "Aguardando reprocessamento",
		ProjectName:       project.Name,
		Description:       project.Description,
		GenerationOptions: source.GenerationOptions,
		FileCount:         len(sourceFiles),
		TotalSizeBytes:    source.TotalSizeBytes,
		StartedAt:         &now,
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, httperr.Internal("falha ao iniciar transação")
	}
	defer tx.Rollback(ctx)

	if err := s.docs.CreateJob(ctx, tx, job); err != nil {
		if isUniqueViolation(err) {
			return nil, httperr.Conflict("já existe um processamento em andamento para o projeto")
		}
		return nil, httperr.Internal("falha ao criar job de reprocessamento")
	}

	for _, sf := range sourceFiles {
		docFile := &domain.DocumentationFile{
			JobID:       job.ID,
			FileID:      sf.FileID,
			ContentHash: sf.ContentHash,
		}
		if err := s.docs.AddFile(ctx, tx, docFile); err != nil {
			return nil, httperr.Internal("falha ao vincular arquivos ao reprocessamento")
		}
	}

	actorID := user.ID
	_ = s.audit.Create(ctx, tx, &domain.AuditEvent{
		ProjectID: project.ID, ActorUserID: &actorID,
		Action: "Reprocessou documentação", Target: "versão " + strconv.Itoa(source.VersionNumber),
		EntityType: strPtr("documentation_job"), EntityID: strPtr(job.ID),
	})

	if err := tx.Commit(ctx); err != nil {
		return nil, httperr.Internal("falha ao confirmar reprocessamento")
	}

	go s.processJob(job.ID)

	return &JobStatusResponse{
		JobID:   job.ID,
		Status:  string(domain.DocJobPending),
		Message: "Reprocessamento iniciado.",
	}, nil
}

func (s *DocumentationService) authorizeJobAccess(ctx context.Context, user domain.User, job *domain.DocumentationJob) (*domain.Project, error) {
	project, err := s.projects.GetByID(ctx, job.ProjectID)
	if err != nil || project == nil {
		return nil, httperr.NotFound("projeto não encontrado")
	}
	members, err := s.projects.ListMembers(ctx, project.ID)
	if err != nil {
		return nil, httperr.Internal("falha ao verificar permissão")
	}
	if !CanReadProject(user, *project, members) {
		return nil, httperr.Forbidden("sem permissão para acessar este job")
	}
	return project, nil
}

func (s *DocumentationService) toVersionDetail(ctx context.Context, v *domain.DocumentationVersion) (*VersionDetailResponse, error) {
	name := ""
	if u, _ := s.users.GetByID(ctx, v.CreatedBy); u != nil {
		name = u.Name
	}
	opts := v.GenerationOptions
	if len(opts) == 0 {
		opts = []byte("{}")
	}
	return &VersionDetailResponse{
		ID:                v.ID,
		Version:           v.VersionNumber,
		ProjectID:         v.ProjectID,
		JobID:             v.JobID,
		CreatedAt:         v.CreatedAt,
		UserID:            v.CreatedBy,
		UserName:          name,
		ModelUsed:         v.ModelUsed,
		Language:          v.Language,
		ProcessingMs:      v.ProcessingMs,
		FileCount:         v.FileCount,
		TotalSizeBytes:    v.TotalSizeBytes,
		GenerationOptions: opts,
		Content:           v.Content,
	}, nil
}

func (s *DocumentationService) validateFileHeader(fh *multipart.FileHeader) error {
	if fh.Size <= 0 {
		return httperr.Validation("arquivo vazio não é permitido")
	}
	if fh.Size > s.cfg.DocMaxFileBytes {
		return httperr.PayloadTooLarge("arquivo excede o tamanho permitido")
	}

	ext := strings.ToLower(filepath.Ext(fh.Filename))
	if docBlockedExt[ext] {
		return httperr.UnsupportedMediaType("upload de arquivos executáveis não é permitido")
	}
	if !docAllowedExt[ext] {
		return httperr.UnsupportedMediaType("tipo de arquivo não suportado")
	}

	mimeType := detectDocMime(fh.Filename)
	if !docAllowedMimes[mimeType] {
		return httperr.UnsupportedMediaType("tipo de arquivo não suportado")
	}
	return nil
}

func (s *DocumentationService) cleanupKeys(ctx context.Context, keys []string) {
	for _, key := range keys {
		_ = s.storage.Delete(ctx, key)
	}
}

func detectDocMime(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	mimeType := mime.TypeByExtension(ext)
	if mimeType != "" {
		if i := strings.Index(mimeType, ";"); i >= 0 {
			mimeType = mimeType[:i]
		}
		return mimeType
	}
	switch ext {
	case ".md", ".markdown":
		return "text/markdown"
	case ".txt":
		return "text/plain"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".doc":
		return "application/msword"
	default:
		return "application/octet-stream"
	}
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "duplicate key") || strings.Contains(msg, "unique constraint") || strings.Contains(msg, "idx_documentation_jobs_active_project")
}
