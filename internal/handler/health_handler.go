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
	code := http.StatusOK

	if err := h.database.Pool.Ping(c.Request().Context()); err != nil {
		dbStatus = "disconnected"
		status = "degraded"
		code = http.StatusServiceUnavailable
	}

	return c.JSON(code, map[string]string{
		"status":   status,
		"database": dbStatus,
	})
}
