package handler

import (
	"encoding/json"
	"net/http"

	"github.com/atlas/knowledge-api/internal/middleware"
	"github.com/atlas/knowledge-api/internal/service"
	"github.com/atlas/knowledge-api/pkg/httperr"
	"github.com/labstack/echo/v4"
)

type MnemosHandler struct {
	mnemos *service.MnemosService
}

func NewMnemosHandler(mnemos *service.MnemosService) *MnemosHandler {
	return &MnemosHandler{mnemos: mnemos}
}

func mnemosCallerFrom(c echo.Context) service.MnemosCaller {
	caller := service.MnemosCaller{ServiceAuth: middleware.IsMnemosServiceAuth(c)}
	if u, ok := middleware.TryGetUser(c); ok {
		caller.Actor = &u
	}
	return caller
}

type mnemosSyncRequest struct {
	Project           service.MnemosProjectDraft     `json:"project"`
	Sections          []service.MnemosSectionInput   `json:"sections"`
	Attachments       []service.MnemosAttachmentMeta `json:"attachments"`
	ResponsibleUserID string                         `json:"responsibleUserId"`
	ReplaceSections   *bool                          `json:"replaceSections"`
}

// Sync cria ou atualiza um projeto a partir do payload Atlas do Mnemos.
func (h *MnemosHandler) Sync(c echo.Context) error {
	var req mnemosSyncRequest
	if err := c.Bind(&req); err != nil {
		return Error(c, httperr.BadRequest("corpo da requisição inválido"))
	}
	resp, err := h.mnemos.Sync(c.Request().Context(), mnemosCallerFrom(c), service.MnemosSyncInput{
		Project:           req.Project,
		Sections:          req.Sections,
		Attachments:       req.Attachments,
		ResponsibleUserID: req.ResponsibleUserID,
		ReplaceSections:   req.ReplaceSections,
	})
	if err != nil {
		return Error(c, err)
	}
	status := http.StatusOK
	if resp.Created {
		status = http.StatusCreated
	}
	return JSON(c, status, resp)
}

type mnemosPatchRequest struct {
	Name              *string `json:"name"`
	Description       *string `json:"description"`
	Status            *string `json:"status"`
	Client            *string `json:"client"`
	ResponsibleUserID string  `json:"responsibleUserId"`
}

func (h *MnemosHandler) Patch(c echo.Context) error {
	var req mnemosPatchRequest
	if err := c.Bind(&req); err != nil {
		return Error(c, httperr.BadRequest("corpo da requisição inválido"))
	}
	draft := service.MnemosProjectDraft{Client: req.Client}
	if req.Name != nil {
		draft.Name = *req.Name
	}
	if req.Description != nil {
		draft.Description = *req.Description
	}
	if req.Status != nil {
		draft.Status = *req.Status
	}
	resp, err := h.mnemos.PatchProject(c.Request().Context(), mnemosCallerFrom(c), c.Param("slug"), draft, req.ResponsibleUserID)
	if err != nil {
		return Error(c, err)
	}
	return JSON(c, http.StatusOK, resp)
}

type mnemosStructureRequest struct {
	Sections          []service.MnemosSectionInput `json:"sections"`
	ReplaceSections   *bool                        `json:"replaceSections"`
	ResponsibleUserID string                       `json:"responsibleUserId"`
}

func (h *MnemosHandler) ApplyStructure(c echo.Context) error {
	var req mnemosStructureRequest
	if err := c.Bind(&req); err != nil {
		return Error(c, httperr.BadRequest("corpo da requisição inválido"))
	}
	replace := true
	if req.ReplaceSections != nil {
		replace = *req.ReplaceSections
	}
	resp, err := h.mnemos.ApplyStructure(
		c.Request().Context(), mnemosCallerFrom(c), c.Param("slug"), req.Sections, replace, req.ResponsibleUserID,
	)
	if err != nil {
		return Error(c, err)
	}
	return JSON(c, http.StatusOK, resp)
}

func (h *MnemosHandler) UploadAttachments(c echo.Context) error {
	form, err := c.MultipartForm()
	if err != nil {
		return Error(c, httperr.BadRequest("multipart inválido"))
	}

	files := form.File["files"]
	if len(files) == 0 {
		files = form.File["files[]"]
	}
	if len(files) == 0 {
		files = form.File["file"]
	}
	if len(files) == 0 {
		return Error(c, httperr.Validation("pelo menos um arquivo deve ser enviado (campo files)"))
	}

	var meta []service.MnemosAttachmentMeta
	if raw := c.FormValue("meta"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &meta); err != nil {
			var wrap struct {
				Attachments []service.MnemosAttachmentMeta `json:"attachments"`
			}
			if err2 := json.Unmarshal([]byte(raw), &wrap); err2 != nil {
				return Error(c, httperr.BadRequest("meta deve ser JSON válido (array ou {attachments:[]})"))
			}
			meta = wrap.Attachments
		}
	} else if raw := c.FormValue("attachments"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &meta); err != nil {
			return Error(c, httperr.BadRequest("attachments deve ser um JSON array válido"))
		}
	}

	actorUserID := c.FormValue("responsibleUserId")
	if actorUserID == "" {
		actorUserID = c.FormValue("actorUserId")
	}

	items, err := h.mnemos.UploadAttachments(
		c.Request().Context(), mnemosCallerFrom(c), c.Param("slug"), files, meta, actorUserID,
	)
	if err != nil {
		return Error(c, err)
	}
	return JSON(c, http.StatusCreated, map[string]interface{}{
		"slug":        c.Param("slug"),
		"attachments": items,
	})
}
