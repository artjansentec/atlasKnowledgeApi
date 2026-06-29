package handler

import (
	"net/http"

	"github.com/atlas/knowledge-api/pkg/httperr"
	"github.com/labstack/echo/v4"
)

func JSON(c echo.Context, status int, data interface{}) error {
	return c.JSON(status, data)
}

func Error(c echo.Context, err error) error {
	httpErr := httperr.AsHTTPError(err)
	return c.JSON(httpErr.StatusCode, map[string]interface{}{
		"success": false,
		"error": map[string]string{
			"code":    httpErr.Code,
			"message": httpErr.Message,
		},
	})
}

func NoContent(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}
