package handler

import (
	"net/http"

	"github.com/atlas/knowledge-api/internal/middleware"
	"github.com/atlas/knowledge-api/internal/service"
	"github.com/atlas/knowledge-api/pkg/httperr"
	"github.com/labstack/echo/v4"
)

type LessonHandler struct {
	lessons *service.LessonService
}

func NewLessonHandler(lessons *service.LessonService) *LessonHandler {
	return &LessonHandler{lessons: lessons}
}

type lessonRequest struct {
	Type           string   `json:"type"`
	Title          string   `json:"title"`
	Description    string   `json:"description"`
	Recommendation string   `json:"recommendation"`
	Tags           []string `json:"tags"`
}

func (h *LessonHandler) Create(c echo.Context) error {
	var req lessonRequest
	if err := c.Bind(&req); err != nil {
		return Error(c, httperr.BadRequest("corpo da requisição inválido"))
	}
	lesson, err := h.lessons.Create(c.Request().Context(), middleware.GetUser(c), c.Param("slug"), service.LessonInput{
		Type: req.Type, Title: req.Title, Description: req.Description,
		Recommendation: req.Recommendation, Tags: req.Tags,
	})
	if err != nil {
		return Error(c, err)
	}
	return JSON(c, http.StatusCreated, map[string]interface{}{
		"id": lesson.ID, "type": lesson.Type, "title": lesson.Title,
		"description": lesson.Description, "recommendation": lesson.Recommendation,
	})
}

func (h *LessonHandler) Patch(c echo.Context) error {
	var req lessonRequest
	if err := c.Bind(&req); err != nil {
		return Error(c, httperr.BadRequest("corpo da requisição inválido"))
	}
	body := map[string]interface{}{}
	_ = c.Bind(&body)
	_, hasTags := body["tags"]
	lesson, err := h.lessons.Patch(c.Request().Context(), middleware.GetUser(c), c.Param("slug"), c.Param("lessonId"), service.LessonInput{
		Type: req.Type, Title: req.Title, Description: req.Description,
		Recommendation: req.Recommendation, Tags: req.Tags,
	}, hasTags)
	if err != nil {
		return Error(c, err)
	}
	return JSON(c, http.StatusOK, map[string]interface{}{
		"id": lesson.ID, "type": lesson.Type, "title": lesson.Title,
		"description": lesson.Description, "recommendation": lesson.Recommendation,
	})
}

func (h *LessonHandler) Delete(c echo.Context) error {
	if err := h.lessons.Delete(c.Request().Context(), middleware.GetUser(c), c.Param("slug"), c.Param("lessonId")); err != nil {
		return Error(c, err)
	}
	return NoContent(c)
}
