package handler

import (
	"net/http"
	"strconv"

	"github.com/atlas/knowledge-api/internal/ai"
	"github.com/atlas/knowledge-api/internal/middleware"
	"github.com/atlas/knowledge-api/internal/service"
	"github.com/atlas/knowledge-api/pkg/httperr"
	"github.com/labstack/echo/v4"
)

type ObservabilityHandler struct {
	obs *service.ObservabilityService
}

func NewObservabilityHandler(obs *service.ObservabilityService) *ObservabilityHandler {
	return &ObservabilityHandler{obs: obs}
}

// ListExecutions proxies Mnemos GET /v1/observability/executions.
// GET /api/v1/observability/executions
func (h *ObservabilityHandler) ListExecutions(c echo.Context) error {
	filter := ai.ExecutionListFilter{
		Operation: c.QueryParam("operation"),
		Provider:  c.QueryParam("provider"),
		ProjectID: c.QueryParam("project_id"),
		Status:    c.QueryParam("status"),
		Model:     c.QueryParam("model"),
	}
	if lim := c.QueryParam("limit"); lim != "" {
		n, err := strconv.Atoi(lim)
		if err != nil || n < 1 {
			return Error(c, httperr.Validation("limit deve ser um inteiro positivo"))
		}
		filter.Limit = n
	}

	result, err := h.obs.ListExecutions(c.Request().Context(), middleware.GetUser(c), filter)
	if err != nil {
		return Error(c, err)
	}
	return JSON(c, http.StatusOK, result)
}

// GetExecution proxies Mnemos GET /v1/observability/executions/:id.
// GET /api/v1/observability/executions/:id
func (h *ObservabilityHandler) GetExecution(c echo.Context) error {
	result, err := h.obs.GetExecution(c.Request().Context(), middleware.GetUser(c), c.Param("id"))
	if err != nil {
		return Error(c, err)
	}
	return JSON(c, http.StatusOK, result)
}
