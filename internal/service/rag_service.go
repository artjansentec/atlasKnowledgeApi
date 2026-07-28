package service

import (
	"context"
	"strings"

	"github.com/atlas/knowledge-api/internal/ai"
	"github.com/atlas/knowledge-api/internal/domain"
	"github.com/atlas/knowledge-api/internal/repository"
	"github.com/atlas/knowledge-api/pkg/httperr"
)

// RAGService orquestra busca semântica no Mnemos com filtro de permissões do Atlas.
type RAGService struct {
	projects *repository.ProjectRepository
	ai       *ai.Client
}

func NewRAGService(projects *repository.ProjectRepository, aiClient *ai.Client) *RAGService {
	return &RAGService{projects: projects, ai: aiClient}
}

// Search filtra project_ids pelo acesso do usuário e consulta o Mnemos.
// Se requestedProjectIDs estiver vazio, usa todos os projetos acessíveis.
func (s *RAGService) Search(ctx context.Context, user domain.User, question string, requestedProjectIDs []string) (*ai.RAGSearchResponse, error) {
	question = strings.TrimSpace(question)
	if question == "" {
		return nil, httperr.Validation("question é obrigatório")
	}

	allowed, err := s.projects.AccessibleProjectIDs(ctx, user.ID, IsAdmin(user))
	if err != nil {
		return nil, httperr.Internal("falha ao resolver projetos acessíveis")
	}

	// Admin (nil) → materializa todos os IDs; Mnemos exige project_ids não vazio.
	scopeIDs := allowed
	if allowed == nil {
		projects, err := s.projects.List(ctx, domain.ProjectListFilter{}, nil)
		if err != nil {
			return nil, httperr.Internal("falha ao listar projetos")
		}
		scopeIDs = make([]string, 0, len(projects))
		for _, p := range projects {
			scopeIDs = append(scopeIDs, p.ID)
		}
	}

	if len(scopeIDs) == 0 {
		return nil, httperr.Forbidden("nenhum projeto acessível para busca RAG")
	}

	projectIDs := intersectProjectIDs(scopeIDs, requestedProjectIDs)
	if len(projectIDs) == 0 {
		return nil, httperr.Forbidden("nenhum dos project_ids informados é acessível")
	}

	return s.ai.SearchRAG(ctx, question, projectIDs)
}

func intersectProjectIDs(allowed, requested []string) []string {
	if len(requested) == 0 {
		out := make([]string, len(allowed))
		copy(out, allowed)
		return out
	}
	set := make(map[string]struct{}, len(allowed))
	for _, id := range allowed {
		set[id] = struct{}{}
	}
	seen := make(map[string]struct{})
	var out []string
	for _, id := range requested {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := set[id]; !ok {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
