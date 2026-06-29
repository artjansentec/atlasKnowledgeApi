package handler

import (
	"net/http"

	"github.com/atlas/knowledge-api/internal/domain"
	"github.com/atlas/knowledge-api/internal/middleware"
	"github.com/atlas/knowledge-api/internal/service"
	"github.com/atlas/knowledge-api/pkg/httperr"
	"github.com/labstack/echo/v4"
)

type SectionHandler struct {
	sections *service.SectionService
}

func NewSectionHandler(sections *service.SectionService) *SectionHandler {
	return &SectionHandler{sections: sections}
}

type createSectionRequest struct {
	Title    string  `json:"title"`
	Content  string  `json:"content"`
	ParentID *string `json:"parentId"`
}

func (h *SectionHandler) Create(c echo.Context) error {
	var req createSectionRequest
	if err := c.Bind(&req); err != nil {
		return Error(c, httperr.BadRequest("corpo da requisição inválido"))
	}
	section, err := h.sections.Create(c.Request().Context(), middleware.GetUser(c), c.Param("slug"), service.CreateSectionInput{
		Title: req.Title, Content: req.Content, ParentID: req.ParentID,
	})
	if err != nil {
		return Error(c, err)
	}
	return JSON(c, http.StatusCreated, map[string]string{
		"id": section.ID, "title": section.Title, "content": section.Content,
	})
}

type patchSectionRequest struct {
	Title   *string `json:"title"`
	Content *string `json:"content"`
}

func (h *SectionHandler) Patch(c echo.Context) error {
	var req patchSectionRequest
	if err := c.Bind(&req); err != nil {
		return Error(c, httperr.BadRequest("corpo da requisição inválido"))
	}
	section, err := h.sections.Patch(c.Request().Context(), middleware.GetUser(c), c.Param("slug"), c.Param("sectionId"), service.PatchSectionInput{
		Title: req.Title, Content: req.Content,
	})
	if err != nil {
		return Error(c, err)
	}
	return JSON(c, http.StatusOK, map[string]string{
		"id": section.ID, "title": section.Title, "content": section.Content,
	})
}

func (h *SectionHandler) Delete(c echo.Context) error {
	if err := h.sections.Delete(c.Request().Context(), middleware.GetUser(c), c.Param("slug"), c.Param("sectionId")); err != nil {
		return Error(c, err)
	}
	return NoContent(c)
}

type reorderRequest struct {
	Items []struct {
		ID        string  `json:"id"`
		ParentID  *string `json:"parentId"`
		SortOrder int     `json:"sortOrder"`
	} `json:"items"`
}

func (h *SectionHandler) Reorder(c echo.Context) error {
	var req reorderRequest
	if err := c.Bind(&req); err != nil {
		return Error(c, httperr.BadRequest("corpo da requisição inválido"))
	}
	items := make([]domain.SectionReorderItem, len(req.Items))
	for i, item := range req.Items {
		items[i] = domain.SectionReorderItem{ID: item.ID, ParentID: item.ParentID, SortOrder: item.SortOrder}
	}
	if err := h.sections.Reorder(c.Request().Context(), middleware.GetUser(c), c.Param("slug"), items); err != nil {
		return Error(c, err)
	}
	return NoContent(c)
}
