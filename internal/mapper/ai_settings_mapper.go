package mapper

import (
	"strings"
	"time"

	"github.com/atlas/knowledge-api/internal/domain"
)

// AISettingsPublic é a visão admin (sem expor a chave completa).
type AISettingsPublic struct {
	Provider         string  `json:"provider"`
	Model            string  `json:"model"`
	BaseURL          string  `json:"baseUrl"`
	APIKeyConfigured bool    `json:"apiKeyConfigured"`
	APIKeyPreview    string  `json:"apiKeyPreview,omitempty"`
	UpdatedBy        *string `json:"updatedBy,omitempty"`
	UpdatedAt        string  `json:"updatedAt"`
}

// AISettingsInternal é a visão máquina-a-máquina (Mnemos) com a chave completa.
type AISettingsInternal struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	APIKey   string `json:"api_key"`
	BaseURL  string `json:"base_url"`
}

func ToAISettingsPublic(s *domain.AISettings) AISettingsPublic {
	if s == nil {
		return AISettingsPublic{
			Provider:  "openai",
			Model:     "gpt-4o-mini",
			UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		}
	}
	configured := strings.TrimSpace(s.APIKey) != ""
	preview := ""
	if configured {
		preview = maskAPIKey(s.APIKey)
	}
	return AISettingsPublic{
		Provider:         s.Provider,
		Model:            s.Model,
		BaseURL:          s.BaseURL,
		APIKeyConfigured: configured,
		APIKeyPreview:    preview,
		UpdatedBy:        s.UpdatedBy,
		UpdatedAt:        s.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func ToAISettingsInternal(s *domain.AISettings) AISettingsInternal {
	if s == nil {
		return AISettingsInternal{Provider: "openai", Model: "gpt-4o-mini"}
	}
	return AISettingsInternal{
		Provider: s.Provider,
		Model:    s.Model,
		APIKey:   s.APIKey,
		BaseURL:  s.BaseURL,
	}
}

func maskAPIKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	if len(key) <= 8 {
		return "••••"
	}
	return key[:4] + "…" + key[len(key)-4:]
}
