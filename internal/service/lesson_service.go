package service

import (
	"context"

	"github.com/atlas/knowledge-api/internal/domain"
	"github.com/atlas/knowledge-api/internal/repository"
	"github.com/atlas/knowledge-api/pkg/httperr"
)

type LessonService struct {
	projects *repository.ProjectRepository
	lessons  *repository.LessonRepository
	tags     *repository.TagRepository
	audit    *repository.AuditRepository
	pool     pgxPool
}

func NewLessonService(
	projects *repository.ProjectRepository,
	lessons *repository.LessonRepository,
	tags *repository.TagRepository,
	audit *repository.AuditRepository,
	pool pgxPool,
) *LessonService {
	return &LessonService{projects: projects, lessons: lessons, tags: tags, audit: audit, pool: pool}
}

type LessonInput struct {
	Type           string
	Title          string
	Description    string
	Recommendation string
	Tags           []string
}

func (s *LessonService) Create(ctx context.Context, user domain.User, slug string, input LessonInput) (*domain.Lesson, error) {
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
	if input.Type == "" {
		input.Type = string(domain.LessonSuccess)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, httperr.Internal("falha ao iniciar transação")
	}
	defer tx.Rollback(ctx)

	actorID := user.ID
	lesson := &domain.Lesson{
		ProjectID:      project.ID,
		Type:           domain.LessonType(input.Type),
		Title:          input.Title,
		Description:    input.Description,
		Recommendation: input.Recommendation,
		CreatedBy:      &actorID,
	}
	if err := s.lessons.Create(ctx, tx, lesson); err != nil {
		return nil, httperr.Internal("falha ao criar lição")
	}

	if len(input.Tags) > 0 {
		tagIDs, err := s.tags.ResolveNames(ctx, tx, input.Tags, domain.TagGeneral)
		if err != nil {
			return nil, httperr.Internal("falha ao salvar tags")
		}
		for _, tagID := range tagIDs {
			if _, err := tx.Exec(ctx, `INSERT INTO lesson_tags (lesson_id, tag_id) VALUES ($1, $2)`, lesson.ID, tagID); err != nil {
				return nil, httperr.Internal("falha ao salvar tags")
			}
		}
	}

	_ = s.audit.Create(ctx, tx, &domain.AuditEvent{
		ProjectID: project.ID, ActorUserID: &actorID,
		Action: "Adicionou", Target: input.Title,
		EntityType: strPtr("lesson"), EntityID: strPtr(lesson.ID),
	})

	if err := tx.Commit(ctx); err != nil {
		return nil, httperr.Internal("falha ao confirmar transação")
	}
	return lesson, nil
}

func (s *LessonService) Patch(ctx context.Context, user domain.User, slug, lessonID string, input LessonInput, hasTags bool) (*domain.Lesson, error) {
	project, err := s.projects.GetBySlug(ctx, slug)
	if err != nil || project == nil {
		return nil, httperr.NotFound("projeto não encontrado")
	}
	if err := requireManage(user, *project); err != nil {
		return nil, err
	}
	lesson, err := s.lessons.GetByID(ctx, project.ID, lessonID)
	if err != nil || lesson == nil {
		return nil, httperr.NotFound("lição não encontrada")
	}

	fields := map[string]interface{}{}
	if input.Type != "" {
		fields["type"] = input.Type
	}
	if input.Title != "" {
		fields["title"] = input.Title
	}
	if input.Description != "" {
		fields["description"] = input.Description
	}
	if input.Recommendation != "" {
		fields["recommendation"] = input.Recommendation
	}
	if len(fields) > 0 {
		if err := s.lessons.Update(ctx, lessonID, fields); err != nil {
			return nil, httperr.Internal("falha ao atualizar lição")
		}
	}

	if hasTags {
		tx, err := s.pool.Begin(ctx)
		if err != nil {
			return nil, httperr.Internal("falha ao iniciar transação")
		}
		defer tx.Rollback(ctx)
		tagIDs, err := s.tags.ResolveNames(ctx, tx, input.Tags, domain.TagGeneral)
		if err != nil {
			return nil, httperr.Internal("falha ao atualizar tags")
		}
		if _, err := tx.Exec(ctx, `DELETE FROM lesson_tags WHERE lesson_id = $1`, lessonID); err != nil {
			return nil, httperr.Internal("falha ao atualizar tags")
		}
		for _, tagID := range tagIDs {
			if _, err := tx.Exec(ctx, `INSERT INTO lesson_tags (lesson_id, tag_id) VALUES ($1, $2)`, lessonID, tagID); err != nil {
				return nil, httperr.Internal("falha ao atualizar tags")
			}
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, httperr.Internal("falha ao confirmar transação")
		}
	}

	actorID := user.ID
	_ = s.audit.Create(ctx, nil, &domain.AuditEvent{
		ProjectID: project.ID, ActorUserID: &actorID,
		Action: "Atualizou", Target: lesson.Title,
		EntityType: strPtr("lesson"), EntityID: strPtr(lessonID),
	})
	return s.lessons.GetByID(ctx, project.ID, lessonID)
}

func (s *LessonService) Delete(ctx context.Context, user domain.User, slug, lessonID string) error {
	project, err := s.projects.GetBySlug(ctx, slug)
	if err != nil || project == nil {
		return httperr.NotFound("projeto não encontrado")
	}
	if err := requireManage(user, *project); err != nil {
		return err
	}
	lesson, err := s.lessons.GetByID(ctx, project.ID, lessonID)
	if err != nil || lesson == nil {
		return httperr.NotFound("lição não encontrada")
	}
	if err := s.lessons.SoftDelete(ctx, lessonID); err != nil {
		return httperr.Internal("falha ao remover lição")
	}
	actorID := user.ID
	_ = s.audit.Create(ctx, nil, &domain.AuditEvent{
		ProjectID: project.ID, ActorUserID: &actorID,
		Action: "Removeu", Target: lesson.Title,
		EntityType: strPtr("lesson"), EntityID: strPtr(lessonID),
	})
	return nil
}
