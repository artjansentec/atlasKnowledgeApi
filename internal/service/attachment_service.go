package service

import (
	"context"
	"io"
	"mime"
	"path/filepath"
	"strings"

	"github.com/atlas/knowledge-api/internal/config"
	"github.com/atlas/knowledge-api/internal/domain"
	"github.com/atlas/knowledge-api/internal/repository"
	"github.com/atlas/knowledge-api/internal/storage"
	"github.com/atlas/knowledge-api/pkg/httperr"
)

var allowedMimes = map[string]bool{
	"application/pdf": true,
	"image/png":       true,
	"image/jpeg":      true,
	"text/plain":      true,
	"text/markdown":   true,
}

type AttachmentService struct {
	cfg         *config.Config
	projects    *repository.ProjectRepository
	attachments *repository.AttachmentRepository
	files       *repository.FileRepository
	audit       *repository.AuditRepository
	storage     storage.FileStorage
	pool        pgxPool
}

func NewAttachmentService(
	cfg *config.Config,
	projects *repository.ProjectRepository,
	attachments *repository.AttachmentRepository,
	files *repository.FileRepository,
	audit *repository.AuditRepository,
	store storage.FileStorage,
	pool pgxPool,
) *AttachmentService {
	return &AttachmentService{
		cfg: cfg, projects: projects, attachments: attachments,
		files: files, audit: audit, storage: store, pool: pool,
	}
}

// authorizeWrite valida a permissão de escrita conforme o tipo de anexo:
// projeto → admin ou responsável; desenvolvimento → admin ou dev-responsável.
func (s *AttachmentService) authorizeWrite(ctx context.Context, user domain.User, project domain.Project, kind domain.AttachmentKind) error {
	if kind == domain.AttachmentDev {
		devIDs, err := s.projects.ListDevResponsibleIDs(ctx, project.ID)
		if err != nil {
			return httperr.Internal("falha ao carregar dev-responsáveis")
		}
		if !CanManageDevSections(user, devIDs) {
			return httperr.Forbidden("sem permissão para editar anexos da aba Desenvolvimento")
		}
		return nil
	}
	return requireManage(user, project)
}

func (s *AttachmentService) Upload(ctx context.Context, user domain.User, slug string, kind domain.AttachmentKind, originalName string, size int64, reader io.Reader) (*domain.Attachment, *domain.FileRecord, error) {
	project, err := s.projects.GetBySlug(ctx, slug)
	if err != nil || project == nil {
		return nil, nil, httperr.NotFound("projeto não encontrado")
	}
	if err := s.authorizeWrite(ctx, user, *project, kind); err != nil {
		return nil, nil, err
	}
	if size > s.cfg.MaxUploadBytes {
		return nil, nil, httperr.Validation("arquivo excede o tamanho máximo permitido")
	}

	ext := strings.ToLower(filepath.Ext(originalName))
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		switch ext {
		case ".md":
			mimeType = "text/markdown"
		case ".txt":
			mimeType = "text/plain"
		default:
			mimeType = "application/octet-stream"
		}
	}
	if !allowedMimes[mimeType] {
		return nil, nil, httperr.Validation("tipo de arquivo não permitido")
	}

	key, err := s.storage.Save(ctx, originalName, reader)
	if err != nil {
		return nil, nil, httperr.Internal("falha ao salvar arquivo")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, nil, httperr.Internal("falha ao iniciar transação")
	}
	defer tx.Rollback(ctx)

	file := &domain.FileRecord{
		StorageKey:   key,
		OriginalName: originalName,
		MimeType:     mimeType,
		SizeBytes:    size,
		UploadedBy:   user.ID,
	}
	if err := s.files.Create(ctx, tx, file); err != nil {
		return nil, nil, httperr.Internal("falha ao registrar arquivo")
	}

	attachment := &domain.Attachment{ProjectID: project.ID, FileID: file.ID, DisplayName: &originalName, Kind: kind}
	if err := s.attachments.Create(ctx, tx, attachment); err != nil {
		return nil, nil, httperr.Internal("falha ao vincular anexo")
	}

	actorID := user.ID
	_ = s.audit.Create(ctx, tx, &domain.AuditEvent{
		ProjectID: project.ID, ActorUserID: &actorID,
		Action: "Adicionou", Target: originalName,
		EntityType: strPtr("attachment"), EntityID: strPtr(attachment.ID),
	})

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, httperr.Internal("falha ao confirmar transação")
	}
	return attachment, file, nil
}

func (s *AttachmentService) Delete(ctx context.Context, user domain.User, slug string, kind domain.AttachmentKind, attachmentID string) error {
	project, err := s.projects.GetBySlug(ctx, slug)
	if err != nil || project == nil {
		return httperr.NotFound("projeto não encontrado")
	}
	if err := s.authorizeWrite(ctx, user, *project, kind); err != nil {
		return err
	}

	existing, err := s.attachments.GetByID(ctx, project.ID, attachmentID)
	if err != nil || existing == nil || existing.Kind != kind {
		return httperr.NotFound("anexo não encontrado")
	}

	attachment, err := s.attachments.Delete(ctx, attachmentID)
	if err != nil || attachment == nil {
		return httperr.NotFound("anexo não encontrado")
	}

	file, err := s.files.GetByID(ctx, attachment.FileID)
	if err == nil && file != nil {
		_ = s.storage.Delete(ctx, file.StorageKey)
	}

	name := "anexo"
	if attachment.DisplayName != nil {
		name = *attachment.DisplayName
	}
	actorID := user.ID
	_ = s.audit.Create(ctx, nil, &domain.AuditEvent{
		ProjectID: project.ID, ActorUserID: &actorID,
		Action: "Removeu", Target: name,
		EntityType: strPtr("attachment"), EntityID: strPtr(attachmentID),
	})
	return nil
}

// canReadAttachment define quem baixa um arquivo. Arquivos de projeto exigem
// acesso de leitura ao projeto; arquivos de desenvolvimento exigem, além disso,
// visibilidade da aba Desenvolvimento (admin/desenvolvedor ou dev-responsável).
func (s *AttachmentService) canReadAttachment(ctx context.Context, user domain.User, project domain.Project, members []domain.ProjectMember, kind domain.AttachmentKind) bool {
	baseRead := CanReadProject(user, project, members)
	if !baseRead {
		devIDs, err := s.projects.ListDevResponsibleIDs(ctx, project.ID)
		if err == nil {
			for _, id := range devIDs {
				if id == user.ID {
					baseRead = true
					break
				}
			}
		}
	}
	if !baseRead {
		return false
	}
	if kind == domain.AttachmentDev {
		return CanSeeDevSections(user)
	}
	return true
}

func (s *AttachmentService) Download(ctx context.Context, user domain.User, fileID string) (*domain.FileRecord, io.ReadCloser, error) {
	file, err := s.files.GetByID(ctx, fileID)
	if err != nil || file == nil {
		return nil, nil, httperr.NotFound("arquivo não encontrado")
	}

	projectID, kind, err := s.attachments.GetProjectAndKindByFileID(ctx, fileID)
	if err != nil || projectID == "" {
		return nil, nil, httperr.NotFound("arquivo não vinculado a projeto")
	}

	project, err := s.projects.GetByID(ctx, projectID)
	if err != nil || project == nil {
		return nil, nil, httperr.NotFound("projeto não encontrado")
	}
	members, err := s.projects.ListMembers(ctx, project.ID)
	if err != nil {
		return nil, nil, httperr.Internal("falha ao verificar permissão")
	}
	if !s.canReadAttachment(ctx, user, *project, members, kind) {
		return nil, nil, httperr.Forbidden("sem permissão para baixar este arquivo")
	}

	reader, err := s.storage.Open(ctx, file.StorageKey)
	if err != nil {
		return nil, nil, httperr.NotFound("arquivo não encontrado no storage")
	}
	return file, reader, nil
}
