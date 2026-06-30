package handler

import (
	"net/http"
	"time"

	"github.com/atlas/knowledge-api/internal/domain"
	"github.com/atlas/knowledge-api/internal/middleware"
	"github.com/atlas/knowledge-api/internal/service"
	"github.com/atlas/knowledge-api/pkg/httperr"
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
	period, err := parseDashboardPeriod(c.QueryParam("from"), c.QueryParam("to"))
	if err != nil {
		return Error(c, err)
	}

	summary, err := h.dashboard.Summary(c.Request().Context(), middleware.GetUser(c), period)
	if err != nil {
		return Error(c, err)
	}
	return JSON(c, http.StatusOK, summary)
}

func parseDashboardPeriod(fromStr, toStr string) (*domain.DateRange, error) {
	if fromStr == "" && toStr == "" {
		return nil, nil
	}
	if fromStr == "" || toStr == "" {
		return nil, httperr.Validation("informe from e to no formato YYYY-MM-DD")
	}

	from, err := time.Parse("2006-01-02", fromStr)
	if err != nil {
		return nil, httperr.Validation("from inválido; use YYYY-MM-DD")
	}
	to, err := time.Parse("2006-01-02", toStr)
	if err != nil {
		return nil, httperr.Validation("to inválido; use YYYY-MM-DD")
	}

	period := domain.DateRange{From: from, To: to}
	if !period.Valid() {
		return nil, httperr.Validation("from não pode ser posterior a to")
	}
	return &period, nil
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
