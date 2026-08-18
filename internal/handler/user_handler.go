package handler

import (
	"net/http"

	"github.com/atlas/knowledge-api/internal/middleware"
	"github.com/atlas/knowledge-api/internal/service"
	"github.com/atlas/knowledge-api/pkg/httperr"
	"github.com/labstack/echo/v4"
)

type UserHandler struct {
	users *service.UserService
}

func NewUserHandler(users *service.UserService) *UserHandler {
	return &UserHandler{users: users}
}

type createUserRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

type updateUserRequest struct {
	Name     string  `json:"name"`
	Email    string  `json:"email"`
	Role     string  `json:"role"`
	Password *string `json:"password"`
}

func (h *UserHandler) List(c echo.Context) error {
	users, err := h.users.ListActive(c.Request().Context())
	if err != nil {
		return Error(c, err)
	}
	return JSON(c, http.StatusOK, users)
}

func (h *UserHandler) Get(c echo.Context) error {
	user, err := h.users.Get(c.Request().Context(), c.Param("id"))
	if err != nil {
		return Error(c, err)
	}
	return JSON(c, http.StatusOK, user)
}

func (h *UserHandler) Create(c echo.Context) error {
	var req createUserRequest
	if err := c.Bind(&req); err != nil {
		return Error(c, httperr.BadRequest("corpo da requisição inválido"))
	}

	user, err := h.users.Create(c.Request().Context(), middleware.GetUser(c), service.CreateUserInput{
		Name:     req.Name,
		Email:    req.Email,
		Password: req.Password,
		Role:     req.Role,
	})
	if err != nil {
		return Error(c, err)
	}
	return JSON(c, http.StatusCreated, user)
}

func (h *UserHandler) Patch(c echo.Context) error {
	var req updateUserRequest
	if err := c.Bind(&req); err != nil {
		return Error(c, httperr.BadRequest("corpo da requisição inválido"))
	}

	user, err := h.users.Update(c.Request().Context(), middleware.GetUser(c), c.Param("id"), service.UpdateUserInput{
		Name:     req.Name,
		Email:    req.Email,
		Role:     req.Role,
		Password: req.Password,
	})
	if err != nil {
		return Error(c, err)
	}
	return JSON(c, http.StatusOK, user)
}

func (h *UserHandler) Delete(c echo.Context) error {
	if err := h.users.Delete(c.Request().Context(), middleware.GetUser(c), c.Param("id")); err != nil {
		return Error(c, err)
	}
	return NoContent(c)
}
