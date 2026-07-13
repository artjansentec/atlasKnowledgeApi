package handler

import (
	"encoding/json"
	"net/http"

	"github.com/atlas/knowledge-api/internal/middleware"
	"github.com/atlas/knowledge-api/internal/service"
	"github.com/atlas/knowledge-api/pkg/httperr"
	"github.com/labstack/echo/v4"
)

type DocumentationHandler struct {
	docs *service.DocumentationService
}

func NewDocumentationHandler(docs *service.DocumentationService) *DocumentationHandler {
	return &DocumentationHandler{docs: docs}
}

func (h *DocumentationHandler) Generate(c echo.Context) error {
	form, err := c.MultipartForm()
	if err != nil {
		return Error(c, httperr.BadRequest("requisição deve ser multipart/form-data"))
	}

	var opts json.RawMessage
	if raw := c.FormValue("generation_options"); raw != "" {
		if !json.Valid([]byte(raw)) {
			return Error(c, httperr.BadRequest("generation_options deve ser um JSON válido"))
		}
		opts = json.RawMessage(raw)
	}

	files := form.File["files"]
	if len(files) == 0 {
		files = form.File["files[]"]
	}

	resp, err := h.docs.Generate(c.Request().Context(), middleware.GetUser(c), c.Param("slug"), service.GenerateInput{
		ProjectName:       c.FormValue("project_name"),
		Description:       c.FormValue("description"),
		GenerationOptions: opts,
		Files:             files,
	})
	if err != nil {
		return Error(c, err)
	}
	return JSON(c, http.StatusAccepted, resp)
}

func (h *DocumentationHandler) ListJobs(c echo.Context) error {
	items, err := h.docs.ListActiveJobs(c.Request().Context(), middleware.GetUser(c), c.QueryParam("project"))
	if err != nil {
		return Error(c, err)
	}
	if items == nil {
		items = []service.ActiveJobItem{}
	}
	return JSON(c, http.StatusOK, items)
}

func (h *DocumentationHandler) GetJob(c echo.Context) error {
	resp, err := h.docs.GetJob(c.Request().Context(), middleware.GetUser(c), c.Param("jobId"))
	if err != nil {
		return Error(c, err)
	}
	return JSON(c, http.StatusOK, resp)
}

func (h *DocumentationHandler) CancelJob(c echo.Context) error {
	resp, err := h.docs.CancelJob(c.Request().Context(), middleware.GetUser(c), c.Param("jobId"))
	if err != nil {
		return Error(c, err)
	}
	return JSON(c, http.StatusOK, resp)
}

func (h *DocumentationHandler) GetLatest(c echo.Context) error {
	resp, err := h.docs.GetLatest(c.Request().Context(), middleware.GetUser(c), c.Param("slug"))
	if err != nil {
		return Error(c, err)
	}
	return JSON(c, http.StatusOK, resp)
}

func (h *DocumentationHandler) ListVersions(c echo.Context) error {
	items, err := h.docs.ListVersions(c.Request().Context(), middleware.GetUser(c), c.Param("slug"))
	if err != nil {
		return Error(c, err)
	}
	if items == nil {
		items = []service.VersionListItem{}
	}
	return JSON(c, http.StatusOK, items)
}

func (h *DocumentationHandler) GetVersion(c echo.Context) error {
	resp, err := h.docs.GetVersion(c.Request().Context(), middleware.GetUser(c), c.Param("slug"), c.Param("versionId"))
	if err != nil {
		return Error(c, err)
	}
	return JSON(c, http.StatusOK, resp)
}

func (h *DocumentationHandler) DeleteVersion(c echo.Context) error {
	if err := h.docs.DeleteVersion(c.Request().Context(), middleware.GetUser(c), c.Param("slug"), c.Param("versionId")); err != nil {
		return Error(c, err)
	}
	return NoContent(c)
}

func (h *DocumentationHandler) Regenerate(c echo.Context) error {
	resp, err := h.docs.Regenerate(c.Request().Context(), middleware.GetUser(c), c.Param("slug"), c.Param("versionId"))
	if err != nil {
		return Error(c, err)
	}
	return JSON(c, http.StatusAccepted, resp)
}
