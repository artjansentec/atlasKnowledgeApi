package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/atlas/knowledge-api/internal/db"
	"github.com/atlas/knowledge-api/internal/domain"
	"github.com/jackc/pgx/v5"
)

// AISettingsRepository persiste a configuração singleton do provedor de IA.
type AISettingsRepository struct {
	db *db.DB
}

func NewAISettingsRepository(database *db.DB) *AISettingsRepository {
	return &AISettingsRepository{db: database}
}

func (r *AISettingsRepository) Get(ctx context.Context) (*domain.AISettings, error) {
	var s domain.AISettings
	var updatedBy *string
	err := r.db.Pool.QueryRow(ctx, `
		SELECT provider, model, api_key, base_url, updated_by, created_at, updated_at
		FROM ai_settings WHERE id = 1
	`).Scan(&s.Provider, &s.Model, &s.APIKey, &s.BaseURL, &updatedBy, &s.CreatedAt, &s.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get ai_settings: %w", err)
	}
	s.UpdatedBy = updatedBy
	return &s, nil
}

// Upsert atualiza a linha singleton. apiKey nil mantém a chave atual.
func (r *AISettingsRepository) Upsert(ctx context.Context, provider, model, baseURL string, apiKey *string, updatedBy string) (*domain.AISettings, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	model = strings.TrimSpace(model)
	baseURL = strings.TrimSpace(baseURL)

	var updatedByArg any
	if strings.TrimSpace(updatedBy) != "" {
		updatedByArg = updatedBy
	}

	var s domain.AISettings
	var updatedByOut *string
	var err error

	if apiKey != nil {
		err = r.db.Pool.QueryRow(ctx, `
			INSERT INTO ai_settings (id, provider, model, api_key, base_url, updated_by, updated_at)
			VALUES (1, $1, $2, $3, $4, $5, NOW())
			ON CONFLICT (id) DO UPDATE SET
				provider = EXCLUDED.provider,
				model = EXCLUDED.model,
				api_key = EXCLUDED.api_key,
				base_url = EXCLUDED.base_url,
				updated_by = EXCLUDED.updated_by,
				updated_at = NOW()
			RETURNING provider, model, api_key, base_url, updated_by, created_at, updated_at
		`, provider, model, *apiKey, baseURL, updatedByArg).Scan(
			&s.Provider, &s.Model, &s.APIKey, &s.BaseURL, &updatedByOut, &s.CreatedAt, &s.UpdatedAt,
		)
	} else {
		err = r.db.Pool.QueryRow(ctx, `
			INSERT INTO ai_settings (id, provider, model, api_key, base_url, updated_by, updated_at)
			VALUES (1, $1, $2, '', $3, $4, NOW())
			ON CONFLICT (id) DO UPDATE SET
				provider = EXCLUDED.provider,
				model = EXCLUDED.model,
				base_url = EXCLUDED.base_url,
				updated_by = EXCLUDED.updated_by,
				updated_at = NOW()
			RETURNING provider, model, api_key, base_url, updated_by, created_at, updated_at
		`, provider, model, baseURL, updatedByArg).Scan(
			&s.Provider, &s.Model, &s.APIKey, &s.BaseURL, &updatedByOut, &s.CreatedAt, &s.UpdatedAt,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("upsert ai_settings: %w", err)
	}
	s.UpdatedBy = updatedByOut
	return &s, nil
}
