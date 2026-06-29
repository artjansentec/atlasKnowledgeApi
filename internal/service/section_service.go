package service

import (
	"context"

	"github.com/atlas/knowledge-api/internal/domain"
	"github.com/atlas/knowledge-api/internal/repository"
	"github.com/atlas/knowledge-api/pkg/httperr"
)

type SectionService struct {
	projects *repository.ProjectRepository
	sections *repository.SectionRepository
	audit    *repository.AuditRepository
}

func NewSectionService(projects *repository.ProjectRepository, sections *repository.SectionRepository, audit *repository.AuditRepository) *SectionService {
	return &SectionService{projects: projects, sections: sections, audit: audit}
}

type CreateSectionInput struct {
	Title    string
	Content  string
	ParentID *string
}

type PatchSectionInput struct {
	Title   *string
	Content *string
}

func (s *SectionService) Create(ctx context.Context, user domain.User, slug string, input CreateSectionInput) (*domain.Section, error) {
	project, err := s.projects.GetBySlug(ctx, slug)
	if err != nil || project == nil {
		return nil, httperr.NotFound("projeto não encontrado")
	}
	if err := requireManage(user, *project); err != nil {
		return nil, err
	}
	if input.Title == "" {
		return nil, httperr.Validation("título é obrigatório")
	}

	order, err := s.sections.NextSortOrder(ctx, project.ID, input.ParentID)
	if err != nil {
		return nil, httperr.Internal("falha ao ordenar seção")
	}

	section := &domain.Section{
		ProjectID: project.ID,
		ParentID:  input.ParentID,
		Title:     input.Title,
		Content:   input.Content,
		SortOrder: order,
	}
	if err := s.sections.Create(ctx, nil, section); err != nil {
		return nil, httperr.Internal("falha ao criar seção")
	}

	actorID := user.ID
	_ = s.audit.Create(ctx, nil, &domain.AuditEvent{
		ProjectID: project.ID, ActorUserID: &actorID,
		Action: "Adicionou", Target: input.Title,
		EntityType: strPtr("section"), EntityID: strPtr(section.ID),
	})
	return section, nil
}

func (s *SectionService) Patch(ctx context.Context, user domain.User, slug, sectionID string, input PatchSectionInput) (*domain.Section, error) {
	project, err := s.projects.GetBySlug(ctx, slug)
	if err != nil || project == nil {
		return nil, httperr.NotFound("projeto não encontrado")
	}
	if err := requireManage(user, *project); err != nil {
		return nil, err
	}

	section, err := s.sections.GetByID(ctx, project.ID, sectionID)
	if err != nil || section == nil {
		return nil, httperr.NotFound("seção não encontrada")
	}

	if err := s.sections.Update(ctx, sectionID, input.Title, input.Content); err != nil {
		return nil, httperr.Internal("falha ao atualizar seção")
	}

	actorID := user.ID
	_ = s.audit.Create(ctx, nil, &domain.AuditEvent{
		ProjectID: project.ID, ActorUserID: &actorID,
		Action: "Atualizou", Target: section.Title,
		EntityType: strPtr("section"), EntityID: strPtr(sectionID),
	})
	return s.sections.GetByID(ctx, project.ID, sectionID)
}

func (s *SectionService) Delete(ctx context.Context, user domain.User, slug, sectionID string) error {
	project, err := s.projects.GetBySlug(ctx, slug)
	if err != nil || project == nil {
		return httperr.NotFound("projeto não encontrado")
	}
	if err := requireManage(user, *project); err != nil {
		return err
	}
	section, err := s.sections.GetByID(ctx, project.ID, sectionID)
	if err != nil || section == nil {
		return httperr.NotFound("seção não encontrada")
	}
	if err := s.sections.SoftDelete(ctx, sectionID); err != nil {
		return httperr.Internal("falha ao remover seção")
	}
	actorID := user.ID
	_ = s.audit.Create(ctx, nil, &domain.AuditEvent{
		ProjectID: project.ID, ActorUserID: &actorID,
		Action: "Removeu", Target: section.Title,
		EntityType: strPtr("section"), EntityID: strPtr(sectionID),
	})
	return nil
}

func (s *SectionService) Reorder(ctx context.Context, user domain.User, slug string, items []domain.SectionReorderItem) error {
	project, err := s.projects.GetBySlug(ctx, slug)
	if err != nil || project == nil {
		return httperr.NotFound("projeto não encontrado")
	}
	if err := requireManage(user, *project); err != nil {
		return err
	}
	return s.sections.Reorder(ctx, project.ID, items)
}

func requireManage(user domain.User, project domain.Project) error {
	if !CanManageProject(user, project) {
		return httperr.Forbidden("sem permissão para gerenciar este projeto")
	}
	return nil
}
