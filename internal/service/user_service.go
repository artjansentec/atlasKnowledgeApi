package service

import (
	"context"

	"github.com/atlas/knowledge-api/internal/repository"
	"github.com/atlas/knowledge-api/pkg/httperr"
)

type UserListService struct {
	users *repository.UserRepository
}

func NewUserListService(users *repository.UserRepository) *UserListService {
	return &UserListService{users: users}
}

type UserListItem struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func (s *UserListService) ListActive(ctx context.Context) ([]UserListItem, error) {
	users, err := s.users.ListActive(ctx)
	if err != nil {
		return nil, httperr.Internal("falha ao listar usuários")
	}
	items := make([]UserListItem, 0, len(users))
	for _, u := range users {
		items = append(items, UserListItem{ID: u.ID, Name: u.Name, Email: u.Email})
	}
	return items, nil
}
