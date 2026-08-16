package service

import (
	"context"
	"strings"

	"github.com/atlas/knowledge-api/internal/ai"
	"github.com/atlas/knowledge-api/internal/domain"
	"github.com/atlas/knowledge-api/internal/repository"
	"github.com/atlas/knowledge-api/pkg/httperr"
)

// ObservabilityService proxy autenticado das execuções do Mnemos.
type ObservabilityService struct {
	projects *repository.ProjectRepository
	ai       *ai.Client
}

func NewObservabilityService(projects *repository.ProjectRepository, aiClient *ai.Client) *ObservabilityService {
	return &ObservabilityService{projects: projects, ai: aiClient}
}

// ListExecutions lista execuções do Mnemos filtrando por projetos acessíveis ao usuário.
func (s *ObservabilityService) ListExecutions(ctx context.Context, user domain.User, filter ai.ExecutionListFilter) (*ai.ExecutionListResponse, error) {
	allowed, err := s.projects.AccessibleProjectIDs(ctx, user.ID, IsAdmin(user))
	if err != nil {
		return nil, httperr.Internal("falha ao resolver projetos acessíveis")
	}

	projectID := strings.TrimSpace(filter.ProjectID)
	if projectID != "" {
		if !projectAccessible(allowed, projectID) {
			return nil, httperr.Forbidden("projeto não acessível")
		}
	}

	out, err := s.ai.ListExecutions(ctx, filter)
	if err != nil {
		return nil, err
	}

	// Admin (allowed == nil) vê tudo; demais usuários só execuções dos projetos acessíveis.
	if allowed != nil {
		filtered := make([]ai.ExecutionSummary, 0, len(out.Items))
		for _, item := range out.Items {
			if projectAccessible(allowed, item.ProjectID) {
				filtered = append(filtered, item)
			}
		}
		out.Items = filtered
		out.Count = len(filtered)
	}
	return out, nil
}

// GetExecution retorna o detalhe de uma execução se o projeto for acessível.
func (s *ObservabilityService) GetExecution(ctx context.Context, user domain.User, id string) (*ai.ExecutionDetail, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, httperr.Validation("id da execução é obrigatório")
	}

	detail, err := s.ai.GetExecution(ctx, id)
	if err != nil {
		return nil, err
	}

	allowed, err := s.projects.AccessibleProjectIDs(ctx, user.ID, IsAdmin(user))
	if err != nil {
		return nil, httperr.Internal("falha ao resolver projetos acessíveis")
	}
	if !projectAccessible(allowed, detail.ProjectID) {
		return nil, httperr.Forbidden("execução de projeto não acessível")
	}
	return detail, nil
}

// projectAccessible: allowed == nil significa admin (acesso total).
func projectAccessible(allowed []string, projectID string) bool {
	if allowed == nil {
		return true
	}
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return false
	}
	for _, id := range allowed {
		if id == projectID {
			return true
		}
	}
	return false
}
