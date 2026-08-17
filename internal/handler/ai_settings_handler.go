package handler

import (
	"encoding/json"
	"net/http"

	"github.com/atlas/knowledge-api/internal/mapper"
	"github.com/atlas/knowledge-api/internal/middleware"
	"github.com/atlas/knowledge-api/internal/service"
	"github.com/atlas/knowledge-api/pkg/httperr"
	"github.com/labstack/echo/v4"
)

type AISettingsHandler struct {
	settings *service.AISettingsService
}

func NewAISettingsHandler(settings *service.AISettingsService) *AISettingsHandler {
	return &AISettingsHandler{settings: settings}
}

type updateAISettingsRequest struct {
	Provider string  `json:"provider"`
	Model    string  `json:"model"`
	BaseURL  string  `json:"baseUrl"`
	APIKey   *string `json:"apiKey"`
}

// Get GET /api/v1/ai-settings (admin)
func (h *AISettingsHandler) Get(c echo.Context) error {
	user := middleware.GetUser(c)
	settings, err := h.settings.GetForAdmin(c.Request().Context(), user)
	if err != nil {
		return Error(c, err)
	}
	return JSON(c, http.StatusOK, mapper.ToAISettingsPublic(settings))
}

// Update PUT /api/v1/ai-settings (admin)
func (h *AISettingsHandler) Update(c echo.Context) error {
	user := middleware.GetUser(c)
	var req updateAISettingsRequest
	dec := json.NewDecoder(c.Request().Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		return Error(c, httperr.Validation("JSON inválido"))
	}

	settings, err := h.settings.Update(c.Request().Context(), user, service.UpdateAISettingsInput{
		Provider: req.Provider,
		Model:    req.Model,
		BaseURL:  req.BaseURL,
		APIKey:   req.APIKey,
	})
	if err != nil {
		return Error(c, err)
	}
	return JSON(c, http.StatusOK, mapper.ToAISettingsPublic(settings))
}

// GetInternal GET /api/v1/internal/ai-settings (Mnemos X-Api-Key / admin JWT)
func (h *AISettingsHandler) GetInternal(c echo.Context) error {
	settings, err := h.settings.Get(c.Request().Context())
	if err != nil {
		return Error(c, err)
	}
	return JSON(c, http.StatusOK, mapper.ToAISettingsInternal(settings))
}
