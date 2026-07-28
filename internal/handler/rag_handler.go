package handler

import (
	"net/http"

	"github.com/atlas/knowledge-api/internal/middleware"
	"github.com/atlas/knowledge-api/internal/service"
	"github.com/atlas/knowledge-api/pkg/httperr"
	"github.com/labstack/echo/v4"
)

type RAGHandler struct {
	rag *service.RAGService
}

func NewRAGHandler(rag *service.RAGService) *RAGHandler {
	return &RAGHandler{rag: rag}
}

type ragSearchBody struct {
	Question   string   `json:"question"`
	ProjectIDs []string `json:"project_ids"`
}

// Search proxies semantic Q&A to Mnemos after filtering by user permissions.
// POST /api/v1/rag/search
func (h *RAGHandler) Search(c echo.Context) error {
	var body ragSearchBody
	if err := c.Bind(&body); err != nil {
		return Error(c, httperr.BadRequest("corpo da requisição inválido"))
	}

	result, err := h.rag.Search(
		c.Request().Context(),
		middleware.GetUser(c),
		body.Question,
		body.ProjectIDs,
	)
	if err != nil {
		return Error(c, err)
	}
	return JSON(c, http.StatusOK, result)
}
