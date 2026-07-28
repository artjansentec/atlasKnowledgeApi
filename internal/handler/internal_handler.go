package handler

import (
	"net/http"
	"strconv"

	"github.com/atlas/knowledge-api/internal/service"
	"github.com/atlas/knowledge-api/pkg/httperr"
	"github.com/labstack/echo/v4"
)

// InternalHandler serves machine-to-machine endpoints for Mnemos.
type InternalHandler struct {
	knowledge *service.ProjectKnowledgeService
}

func NewInternalHandler(knowledge *service.ProjectKnowledgeService) *InternalHandler {
	return &InternalHandler{knowledge: knowledge}
}

// ListProjectIDs GET /api/v1/internal/projects?page=1&page_size=50
func (h *InternalHandler) ListProjectIDs(c echo.Context) error {
	page, _ := strconv.Atoi(c.QueryParam("page"))
	pageSize, _ := strconv.Atoi(c.QueryParam("page_size"))
	if pageSize == 0 {
		pageSize, _ = strconv.Atoi(c.QueryParam("pageSize"))
	}
	result, err := h.knowledge.ListProjectIDs(c.Request().Context(), page, pageSize)
	if err != nil {
		return Error(c, err)
	}
	return JSON(c, http.StatusOK, result)
}

// GetProjectKnowledge GET /api/v1/internal/projects/:id/knowledge
func (h *InternalHandler) GetProjectKnowledge(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return Error(c, httperr.Validation("id do projeto é obrigatório"))
	}
	result, err := h.knowledge.GetByID(c.Request().Context(), id)
	if err != nil {
		return Error(c, err)
	}
	return JSON(c, http.StatusOK, result)
}
