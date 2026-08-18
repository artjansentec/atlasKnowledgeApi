package service

import (
	"context"
	"strings"

	"github.com/atlas/knowledge-api/internal/domain"
	"github.com/atlas/knowledge-api/internal/repository"
	"github.com/atlas/knowledge-api/pkg/httperr"
)

var allowedAIProviders = map[string]bool{
	"openai":    true,
	"anthropic": true,
	"gemini":    true,
	"ollama":    true,
	"azure":     true,
}

// AISettingsService gerencia a configuração global do provedor de IA.
type AISettingsService struct {
	repo *repository.AISettingsRepository
}

func NewAISettingsService(repo *repository.AISettingsRepository) *AISettingsService {
	return &AISettingsService{repo: repo}
}

func (s *AISettingsService) Get(ctx context.Context) (*domain.AISettings, error) {
	settings, err := s.repo.Get(ctx)
	if err != nil {
		return nil, httperr.Internal("falha ao carregar configurações de IA")
	}
	if settings == nil {
		return &domain.AISettings{
			Provider: "openai",
			Model:    "gpt-5.6-luna",
		}, nil
	}
	return settings, nil
}

// GetForAdmin exige perfil admin (resposta mascarada fica no mapper/handler).
func (s *AISettingsService) GetForAdmin(ctx context.Context, user domain.User) (*domain.AISettings, error) {
	if user.Role != domain.RoleAdmin {
		return nil, httperr.Forbidden("apenas administradores podem ver configurações de IA")
	}
	return s.Get(ctx)
}

type UpdateAISettingsInput struct {
	Provider string
	Model    string
	BaseURL  string
	// APIKey nil = manter a atual; ponteiro para string (mesmo vazia) = gravar.
	APIKey *string
}

func (s *AISettingsService) Update(ctx context.Context, user domain.User, in UpdateAISettingsInput) (*domain.AISettings, error) {
	if user.Role != domain.RoleAdmin {
		return nil, httperr.Forbidden("apenas administradores podem alterar configurações de IA")
	}

	provider := strings.ToLower(strings.TrimSpace(in.Provider))
	model := strings.TrimSpace(in.Model)
	if provider == "" {
		return nil, httperr.Validation("provider é obrigatório")
	}
	if !allowedAIProviders[provider] {
		return nil, httperr.Validation("provider inválido (use openai, anthropic, gemini, ollama ou azure)")
	}
	if model == "" {
		return nil, httperr.Validation("model é obrigatório")
	}
	if provider != "ollama" && in.APIKey != nil && strings.TrimSpace(*in.APIKey) == "" {
		// permitir limpar chave explicitamente; ok
	}

	settings, err := s.repo.Upsert(ctx, provider, model, in.BaseURL, in.APIKey, user.ID)
	if err != nil {
		return nil, httperr.Internal("falha ao salvar configurações de IA")
	}
	return settings, nil
}
