package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/atlas/knowledge-api/internal/domain"
	"github.com/atlas/knowledge-api/internal/repository"
	"github.com/atlas/knowledge-api/pkg/httperr"
	"github.com/jackc/pgx/v5"
)

type AuditService struct {
	repo *repository.AuditRepository
}

func NewAuditService(repo *repository.AuditRepository) *AuditService {
	return &AuditService{repo: repo}
}

func (s *AuditService) Record(ctx context.Context, tx pgx.Tx, projectID string, actorID *string, action, target, entityType, entityID string) error {
	e := &domain.AuditEvent{
		ProjectID:   projectID,
		ActorUserID: actorID,
		Action:      action,
		Target:      target,
		EntityType:  strPtr(entityType),
		EntityID:    strPtr(entityID),
	}
	return s.repo.Create(ctx, tx, e)
}

func strPtr(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

var slugSanitizer = regexp.MustCompile(`[^a-z0-9]+`)

func Slugify(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = slugSanitizer.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

type ProjectService struct {
	db          *repository.ProjectRepository
	sections    *repository.SectionRepository
	lessons     *repository.LessonRepository
	attachments *repository.AttachmentRepository
	files       *repository.FileRepository
	tags        *repository.TagRepository
	audit       *repository.AuditRepository
	users       *repository.UserRepository
	pool        pgxPool
}

type pgxPool interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

func NewProjectService(
	projects *repository.ProjectRepository,
	sections *repository.SectionRepository,
	lessons *repository.LessonRepository,
	attachments *repository.AttachmentRepository,
	files *repository.FileRepository,
	tags *repository.TagRepository,
	audit *repository.AuditRepository,
	users *repository.UserRepository,
	pool pgxPool,
) *ProjectService {
	return &ProjectService{
		db: projects, sections: sections, lessons: lessons,
		attachments: attachments, files: files, tags: tags,
		audit: audit, users: users, pool: pool,
	}
}

type CreateProjectInput struct {
	Name                  string
	Slug                  string
	Description           string
	Status                string
	ResponsibleUserID     string
	DevResponsibleUserIDs []string
	Client                *string
	Tags                  []string
	Tech                  []string
	SectionTitle          string
	SectionContent        string
}

type PatchProjectInput struct {
	Name                  *string
	Description           *string
	Status                *string
	ResponsibleUserID     *string
	DevResponsibleUserIDs []string
	HasDevResponsibles    bool
	Client                *string
	HasClient             bool
	Tags                  []string
	Tech                  []string
	HasTags               bool
	HasTech               bool
}

func (s *ProjectService) accessibleIDs(ctx context.Context, user domain.User) ([]string, error) {
	return s.db.AccessibleProjectIDs(ctx, user.ID, IsAdmin(user))
}

func (s *ProjectService) requireRead(ctx context.Context, user domain.User, project *domain.Project) ([]domain.ProjectMember, error) {
	members, err := s.db.ListMembers(ctx, project.ID)
	if err != nil {
		return nil, httperr.Internal("falha ao carregar membros")
	}
	if CanReadProject(user, *project, members) {
		return members, nil
	}
	// Dev-responsáveis também têm acesso de leitura ao projeto (aba Desenvolvimento).
	devIDs, err := s.db.ListDevResponsibleIDs(ctx, project.ID)
	if err != nil {
		return nil, httperr.Internal("falha ao carregar dev-responsáveis")
	}
	for _, id := range devIDs {
		if id == user.ID {
			return members, nil
		}
	}
	return nil, httperr.Forbidden("sem permissão para acessar este projeto")
}

func (s *ProjectService) requireManage(ctx context.Context, user domain.User, project *domain.Project) error {
	if !CanManageProject(user, *project) {
		return httperr.Forbidden("sem permissão para gerenciar este projeto")
	}
	return nil
}

func (s *ProjectService) GetBySlug(ctx context.Context, user domain.User, slug string) (*domain.Project, []domain.ProjectMember, error) {
	project, err := s.db.GetBySlug(ctx, slug)
	if err != nil {
		return nil, nil, httperr.Internal("falha ao buscar projeto")
	}
	if project == nil {
		return nil, nil, httperr.NotFound("projeto não encontrado")
	}
	members, err := s.requireRead(ctx, user, project)
	if err != nil {
		return nil, nil, err
	}
	return project, members, nil
}

func (s *ProjectService) ListStatuses(ctx context.Context) ([]domain.ProjectStatusMeta, error) {
	statuses, err := s.db.ListStatuses(ctx)
	if err != nil {
		return nil, httperr.Internal("falha ao listar status")
	}
	return statuses, nil
}

func (s *ProjectService) resolveStatus(ctx context.Context, code string) (*domain.ProjectStatusMeta, error) {
	meta, err := s.db.GetStatus(ctx, code)
	if err != nil {
		return nil, httperr.Internal("falha ao validar status")
	}
	if meta == nil {
		return nil, httperr.InvalidStatus("status inválido")
	}
	return meta, nil
}

func (s *ProjectService) List(ctx context.Context, user domain.User, filter domain.ProjectListFilter) ([]domain.Project, error) {
	if filter.Status != "" {
		if _, err := s.resolveStatus(ctx, filter.Status); err != nil {
			return nil, err
		}
	}
	allowed, err := s.accessibleIDs(ctx, user)
	if err != nil {
		return nil, httperr.Internal("falha ao listar projetos")
	}
	projects, err := s.db.List(ctx, filter, allowed)
	if err != nil {
		return nil, httperr.Internal("falha ao listar projetos")
	}
	return projects, nil
}

func (s *ProjectService) Create(ctx context.Context, user domain.User, input CreateProjectInput) (*domain.Project, error) {
	if !IsAdmin(user) {
		return nil, httperr.Forbidden("apenas administradores podem criar projetos")
	}
	if strings.TrimSpace(input.Name) == "" {
		return nil, httperr.Validation("nome é obrigatório")
	}
	slug := input.Slug
	if slug == "" {
		slug = Slugify(input.Name)
	}
	status := domain.ProjectStatus(input.Status)
	if status == "" {
		status = domain.StatusActive
	}
	if _, err := s.resolveStatus(ctx, string(status)); err != nil {
		return nil, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, httperr.Internal("falha ao iniciar transação")
	}
	defer tx.Rollback(ctx)

	project := &domain.Project{
		Slug:              slug,
		Name:              input.Name,
		Description:       input.Description,
		Status:            status,
		ResponsibleUserID: input.ResponsibleUserID,
		Client:            input.Client,
	}
	if err := s.db.Create(ctx, tx, project); err != nil {
		return nil, httperr.Validation("não foi possível criar o projeto (slug pode estar em uso)")
	}

	tagIDs, err := s.tags.ResolveNames(ctx, tx, input.Tags, domain.TagGeneral)
	if err != nil {
		return nil, httperr.Internal("falha ao salvar tags")
	}
	if err := s.tags.SetProjectTags(ctx, tx, project.ID, tagIDs, "project_tags"); err != nil {
		return nil, httperr.Internal("falha ao salvar tags")
	}

	techIDs, err := s.tags.ResolveNames(ctx, tx, input.Tech, domain.TagTech)
	if err != nil {
		return nil, httperr.Internal("falha ao salvar tecnologias")
	}
	if err := s.tags.SetProjectTags(ctx, tx, project.ID, techIDs, "project_tech"); err != nil {
		return nil, httperr.Internal("falha ao salvar tecnologias")
	}

	if err := s.db.SetDevResponsibles(ctx, tx, project.ID, input.DevResponsibleUserIDs); err != nil {
		return nil, httperr.Internal("falha ao salvar dev-responsáveis")
	}

	sectionTitle := input.SectionTitle
	if sectionTitle == "" {
		sectionTitle = "Visão geral"
	}
	section := &domain.Section{
		ProjectID: project.ID,
		Title:     sectionTitle,
		Content:   input.SectionContent,
		SortOrder: 0,
	}
	if err := s.sections.Create(ctx, tx, section); err != nil {
		return nil, httperr.Internal("falha ao criar seção inicial")
	}

	actorID := user.ID
	if err := s.audit.Create(ctx, tx, &domain.AuditEvent{
		ProjectID: project.ID, ActorUserID: &actorID,
		Action: "Criou o projeto", Target: sectionTitle,
		EntityType: strPtr("project"), EntityID: strPtr(project.ID),
	}); err != nil {
		return nil, httperr.Internal("falha ao registrar auditoria")
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, httperr.Internal("falha ao confirmar transação")
	}
	return project, nil
}

func (s *ProjectService) Patch(ctx context.Context, user domain.User, slug string, input PatchProjectInput) (*domain.Project, error) {
	project, err := s.db.GetBySlug(ctx, slug)
	if err != nil || project == nil {
		return nil, httperr.NotFound("projeto não encontrado")
	}
	if err := s.requireManage(ctx, user, project); err != nil {
		return nil, err
	}

	fields := map[string]interface{}{}
	if input.Name != nil {
		fields["name"] = *input.Name
	}
	if input.Description != nil {
		fields["description"] = *input.Description
	}
	statusChanged := false
	var newStatusLabel string
	if input.Status != nil {
		meta, err := s.resolveStatus(ctx, *input.Status)
		if err != nil {
			return nil, err
		}
		fields["status"] = meta.Code
		if domain.ProjectStatus(meta.Code) != project.Status {
			statusChanged = true
			newStatusLabel = meta.Label
		}
	}
	if input.ResponsibleUserID != nil {
		fields["responsible_user_id"] = *input.ResponsibleUserID
	}
	if input.HasClient {
		fields["client"] = input.Client
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, httperr.Internal("falha ao iniciar transação")
	}
	defer tx.Rollback(ctx)

	if len(fields) > 0 {
		sets := []string{}
		args := []interface{}{project.ID}
		i := 2
		for k, v := range fields {
			sets = append(sets, fmt.Sprintf("%s = $%d", k, i))
			args = append(args, v)
			i++
		}
		query := fmt.Sprintf("UPDATE projects SET %s WHERE id = $1 AND deleted_at IS NULL", strings.Join(sets, ", "))
		if _, err := tx.Exec(ctx, query, args...); err != nil {
			return nil, httperr.Internal("falha ao atualizar projeto")
		}
	}

	if input.HasTags {
		tagIDs, err := s.tags.ResolveNames(ctx, tx, input.Tags, domain.TagGeneral)
		if err != nil {
			return nil, httperr.Internal("falha ao atualizar tags")
		}
		if err := s.tags.SetProjectTags(ctx, tx, project.ID, tagIDs, "project_tags"); err != nil {
			return nil, httperr.Internal("falha ao atualizar tags")
		}
	}
	if input.HasTech {
		techIDs, err := s.tags.ResolveNames(ctx, tx, input.Tech, domain.TagTech)
		if err != nil {
			return nil, httperr.Internal("falha ao atualizar tecnologias")
		}
		if err := s.tags.SetProjectTags(ctx, tx, project.ID, techIDs, "project_tech"); err != nil {
			return nil, httperr.Internal("falha ao atualizar tecnologias")
		}
	}
	if input.HasDevResponsibles {
		if err := s.db.SetDevResponsibles(ctx, tx, project.ID, input.DevResponsibleUserIDs); err != nil {
			return nil, httperr.Internal("falha ao atualizar dev-responsáveis")
		}
	}

	actorID := user.ID
	if err := s.audit.Create(ctx, tx, &domain.AuditEvent{
		ProjectID: project.ID, ActorUserID: &actorID,
		Action: "Atualizou", Target: project.Name,
		EntityType: strPtr("project"), EntityID: strPtr(project.ID),
	}); err != nil {
		return nil, httperr.Internal("falha ao registrar auditoria")
	}

	if statusChanged {
		if err := s.audit.Create(ctx, tx, &domain.AuditEvent{
			ProjectID: project.ID, ActorUserID: &actorID,
			Action: "Alterou o status para", Target: newStatusLabel,
			EntityType: strPtr("project"), EntityID: strPtr(project.ID),
		}); err != nil {
			return nil, httperr.Internal("falha ao registrar auditoria")
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, httperr.Internal("falha ao confirmar transação")
	}
	return s.db.GetBySlug(ctx, slug)
}

func (s *ProjectService) Delete(ctx context.Context, user domain.User, slug string) error {
	if !IsAdmin(user) {
		return httperr.Forbidden("apenas administradores podem remover projetos")
	}
	project, err := s.db.GetBySlug(ctx, slug)
	if err != nil || project == nil {
		return httperr.NotFound("projeto não encontrado")
	}
	return s.db.SoftDelete(ctx, project.ID)
}

func (s *ProjectService) SetReaders(ctx context.Context, user domain.User, slug string, userIDs []string) error {
	project, err := s.db.GetBySlug(ctx, slug)
	if err != nil || project == nil {
		return httperr.NotFound("projeto não encontrado")
	}
	if err := s.requireManage(ctx, user, project); err != nil {
		return err
	}
	return s.db.ReplaceReaders(ctx, project.ID, userIDs)
}

func (s *ProjectService) ListMembers(ctx context.Context, projectID string) ([]domain.ProjectMember, error) {
	return s.db.ListMembers(ctx, projectID)
}

func (s *ProjectService) DevResponsibleIDs(ctx context.Context, projectID string) ([]string, error) {
	return s.db.ListDevResponsibleIDs(ctx, projectID)
}

func (s *ProjectService) LoadDevSections(ctx context.Context, projectID string) ([]domain.Section, error) {
	return s.sections.ListByProject(ctx, projectID, domain.SectionDev)
}

func (s *ProjectService) LoadDevAttachments(ctx context.Context, projectID string) ([]domain.Attachment, error) {
	return s.attachments.ListByProject(ctx, projectID, domain.AttachmentDev)
}

func (s *ProjectService) LoadProjectData(ctx context.Context, projectID string) (
	[]domain.Section, []domain.Lesson, []domain.Attachment, []domain.AuditEvent, error,
) {
	sections, err := s.sections.ListByProject(ctx, projectID, domain.SectionDoc)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	lessons, err := s.lessons.ListByProject(ctx, projectID)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	attachments, err := s.attachments.ListByProject(ctx, projectID, domain.AttachmentProject)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	history, err := s.audit.ListByProject(ctx, projectID)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	return sections, lessons, attachments, history, nil
}
