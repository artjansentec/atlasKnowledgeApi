package handler

import (
	"net/http"

	"github.com/atlas/knowledge-api/internal/middleware"
	"github.com/atlas/knowledge-api/internal/service"
	"github.com/labstack/echo/v4"
)

type SearchHandler struct {
	search *service.SearchService
}

func NewSearchHandler(search *service.SearchService) *SearchHandler {
	return &SearchHandler{search: search}
}

func (h *SearchHandler) Search(c echo.Context) error {
	result, err := h.search.Search(c.Request().Context(), middleware.GetUser(c), c.QueryParam("q"))
	if err != nil {
		return Error(c, err)
	}
	return JSON(c, http.StatusOK, result)
}

type DashboardHandler struct {
	dashboard *service.DashboardService
}

func NewDashboardHandler(dashboard *service.DashboardService) *DashboardHandler {
	return &DashboardHandler{dashboard: dashboard}
}

func (h *DashboardHandler) Summary(c echo.Context) error {
	summary, err := h.dashboard.Summary(c.Request().Context(), middleware.GetUser(c))
	if err != nil {
		return Error(c, err)
	}
	return JSON(c, http.StatusOK, summary)
}

type UserHandler struct {
	users *service.UserListService
}

func NewUserHandler(users *service.UserListService) *UserHandler {
	return &UserHandler{users: users}
}

func (h *UserHandler) List(c echo.Context) error {
	users, err := h.users.ListActive(c.Request().Context())
	if err != nil {
		return Error(c, err)
	}
	return JSON(c, http.StatusOK, users)
}
