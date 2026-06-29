package handler

import (
	"net/http"

	"github.com/atlas/knowledge-api/internal/config"
	"github.com/atlas/knowledge-api/internal/middleware"
	"github.com/atlas/knowledge-api/internal/service"
	"github.com/atlas/knowledge-api/pkg/httperr"
	"github.com/labstack/echo/v4"
)

type AuthHandler struct {
	cfg  *config.Config
	auth *service.AuthService
}

func NewAuthHandler(cfg *config.Config, auth *service.AuthService) *AuthHandler {
	return &AuthHandler{cfg: cfg, auth: auth}
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *AuthHandler) Login(c echo.Context) error {
	var req loginRequest
	if err := c.Bind(&req); err != nil {
		return Error(c, httperr.BadRequest("corpo da requisição inválido"))
	}
	result, err := h.auth.Login(c.Request().Context(), req.Email, req.Password)
	if err != nil {
		return Error(c, err)
	}
	h.setRefreshCookie(c, result.RefreshToken)
	return JSON(c, http.StatusOK, map[string]interface{}{
		"accessToken": result.AccessToken,
		"user": map[string]string{
			"id": result.User.ID, "name": result.User.Name,
			"email": result.User.Email, "role": string(result.User.Role),
		},
	})
}

func (h *AuthHandler) Refresh(c echo.Context) error {
	cookie, err := c.Cookie(h.cfg.RefreshCookie)
	if err != nil || cookie.Value == "" {
		return Error(c, httperr.Unauthorized("refresh token ausente"))
	}
	result, err := h.auth.Refresh(c.Request().Context(), cookie.Value)
	if err != nil {
		return Error(c, err)
	}
	h.setRefreshCookie(c, result.RefreshToken)
	return JSON(c, http.StatusOK, map[string]string{"accessToken": result.AccessToken})
}

func (h *AuthHandler) Logout(c echo.Context) error {
	cookie, _ := c.Cookie(h.cfg.RefreshCookie)
	if cookie != nil {
		_ = h.auth.Logout(c.Request().Context(), cookie.Value)
	}
	h.clearRefreshCookie(c)
	return NoContent(c)
}

func (h *AuthHandler) Me(c echo.Context) error {
	user := middleware.GetUser(c)
	return JSON(c, http.StatusOK, map[string]string{
		"id": user.ID, "name": user.Name, "email": user.Email, "role": string(user.Role),
	})
}

func (h *AuthHandler) setRefreshCookie(c echo.Context, token string) {
	c.SetCookie(&http.Cookie{
		Name:     h.cfg.RefreshCookie,
		Value:    token,
		Path:     "/api/v1/auth",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(h.cfg.JWTRefreshTTL.Seconds()),
	})
}

func (h *AuthHandler) clearRefreshCookie(c echo.Context) {
	c.SetCookie(&http.Cookie{
		Name:     h.cfg.RefreshCookie,
		Value:    "",
		Path:     "/api/v1/auth",
		HttpOnly: true,
		MaxAge:   -1,
	})
}
