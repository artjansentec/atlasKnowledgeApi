package handler

import (
	"io"
	"net/http"
	"strconv"

	"github.com/atlas/knowledge-api/internal/middleware"
	"github.com/atlas/knowledge-api/internal/service"
	"github.com/atlas/knowledge-api/pkg/httperr"
	"github.com/labstack/echo/v4"
)

type AttachmentHandler struct {
	attachments *service.AttachmentService
}

func NewAttachmentHandler(attachments *service.AttachmentService) *AttachmentHandler {
	return &AttachmentHandler{attachments: attachments}
}

func (h *AttachmentHandler) Upload(c echo.Context) error {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return Error(c, httperr.BadRequest("arquivo é obrigatório"))
	}
	src, err := fileHeader.Open()
	if err != nil {
		return Error(c, httperr.Internal("falha ao ler arquivo"))
	}
	defer src.Close()

	attachment, file, err := h.attachments.Upload(
		c.Request().Context(), middleware.GetUser(c), c.Param("slug"),
		fileHeader.Filename, fileHeader.Size, src,
	)
	if err != nil {
		return Error(c, err)
	}
	return JSON(c, http.StatusCreated, map[string]interface{}{
		"id": attachment.ID, "name": file.OriginalName, "fileId": file.ID,
	})
}

func (h *AttachmentHandler) Delete(c echo.Context) error {
	if err := h.attachments.Delete(c.Request().Context(), middleware.GetUser(c), c.Param("slug"), c.Param("attachmentId")); err != nil {
		return Error(c, err)
	}
	return NoContent(c)
}

func (h *AttachmentHandler) Download(c echo.Context) error {
	file, reader, err := h.attachments.Download(c.Request().Context(), middleware.GetUser(c), c.Param("fileId"))
	if err != nil {
		return Error(c, err)
	}
	defer reader.Close()

	c.Response().Header().Set(echo.HeaderContentType, file.MimeType)
	c.Response().Header().Set(echo.HeaderContentDisposition, `attachment; filename="`+file.OriginalName+`"`)
	c.Response().Header().Set(echo.HeaderContentLength, strconv.FormatInt(file.SizeBytes, 10))
	_, copyErr := io.Copy(c.Response().Writer, reader)
	return copyErr
}
