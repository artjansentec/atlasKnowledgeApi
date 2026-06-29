package handler

import (
	"net/http"

	"github.com/atlas/knowledge-api/internal/db"
	"github.com/labstack/echo/v4"
)

type HealthHandler struct {
	database *db.DB
}

func NewHealthHandler(database *db.DB) *HealthHandler {
	return &HealthHandler{database: database}
}

func (h *HealthHandler) Check(c echo.Context) error {
	status := "ok"
	dbStatus := "connected"

	if err := h.database.Pool.Ping(c.Request().Context()); err != nil {
		dbStatus = "disconnected"
		status = "degraded"
	}

	return c.JSON(http.StatusOK, map[string]string{
		"status":   status,
		"database": dbStatus,
	})
}
