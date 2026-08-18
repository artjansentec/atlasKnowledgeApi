package service

import (
	"context"
	"net/mail"
	"strings"

	"github.com/atlas/knowledge-api/internal/domain"
	"github.com/atlas/knowledge-api/internal/repository"
	"github.com/atlas/knowledge-api/pkg/httperr"
)

const minUserPasswordLength = 8

type UserService struct {
	users   *repository.UserRepository
	refresh *repository.RefreshTokenRepository
}

func NewUserService(users *repository.UserRepository, refresh *repository.RefreshTokenRepository) *UserService {
	return &UserService{users: users, refresh: refresh}
}

// NewUserListService mantém o construtor antigo usado em testes e wiring parcial.
func NewUserListService(users *repository.UserRepository) *UserService {
	return NewUserService(users, nil)
}

type UserListItem struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

type CreateUserInput struct {
	Name     string
	Email    string
	Password string
	Role     string
}

type UpdateUserInput struct {
	Name     string
	Email    string
	Role     string
	Password *string
}

func (s *UserService) ListActive(ctx context.Context) ([]UserListItem, error) {
	users, err := s.users.ListActive(ctx)
	if err != nil {
		return nil, httperr.Internal("falha ao listar usuários")
	}
	items := make([]UserListItem, 0, len(users))
	for _, u := range users {
		items = append(items, toUserListItem(u))
	}
	return items, nil
}

func (s *UserService) Get(ctx context.Context, id string) (*UserListItem, error) {
	user, err := s.users.GetByID(ctx, id)
	if err != nil {
		return nil, httperr.Internal("falha ao buscar usuário")
	}
	if user == nil {
		return nil, httperr.NotFound("usuário não encontrado")
	}
	item := toUserListItem(*user)
	return &item, nil
}

func (s *UserService) Create(ctx context.Context, actor domain.User, in CreateUserInput) (*UserListItem, error) {
	if err := requireAdmin(actor); err != nil {
		return nil, err
	}

	name, email, role, err := validateUserIdentity(in.Name, in.Email, in.Role)
	if err != nil {
		return nil, err
	}
	if err := validatePassword(in.Password); err != nil {
		return nil, err
	}

	existing, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		return nil, httperr.Internal("falha ao verificar e-mail")
	}
	if existing != nil {
		return nil, httperr.Conflict("e-mail já está em uso")
	}

	hash, err := HashPassword(in.Password)
	if err != nil {
		return nil, httperr.Internal("falha ao gerar senha")
	}

	user := domain.User{
		Email:        email,
		PasswordHash: hash,
		Name:         name,
		Role:         role,
		IsActive:     true,
	}
	if err := s.users.Insert(ctx, &user); err != nil {
		return nil, httperr.Internal("falha ao criar usuário")
	}

	item := toUserListItem(user)
	return &item, nil
}

func (s *UserService) Update(ctx context.Context, actor domain.User, id string, in UpdateUserInput) (*UserListItem, error) {
	if err := requireAdmin(actor); err != nil {
		return nil, err
	}

	user, err := s.users.GetByID(ctx, id)
	if err != nil {
		return nil, httperr.Internal("falha ao buscar usuário")
	}
	if user == nil {
		return nil, httperr.NotFound("usuário não encontrado")
	}

	name, email, role, err := validateUserIdentity(in.Name, in.Email, in.Role)
	if err != nil {
		return nil, err
	}

	if user.Role == domain.RoleAdmin && role != domain.RoleAdmin {
		if err := s.ensureNotLastAdmin(ctx); err != nil {
			return nil, err
		}
	}

	if !strings.EqualFold(user.Email, email) {
		existing, err := s.users.GetByEmail(ctx, email)
		if err != nil {
			return nil, httperr.Internal("falha ao verificar e-mail")
		}
		if existing != nil && existing.ID != user.ID {
			return nil, httperr.Conflict("e-mail já está em uso")
		}
	}

	user.Name = name
	user.Email = email
	user.Role = role

	if in.Password != nil && strings.TrimSpace(*in.Password) != "" {
		if err := validatePassword(*in.Password); err != nil {
			return nil, err
		}
		hash, err := HashPassword(*in.Password)
		if err != nil {
			return nil, httperr.Internal("falha ao gerar senha")
		}
		user.PasswordHash = hash
	}

	if err := s.users.Update(ctx, user); err != nil {
		return nil, httperr.Internal("falha ao atualizar usuário")
	}

	item := toUserListItem(*user)
	return &item, nil
}

func (s *UserService) Delete(ctx context.Context, actor domain.User, id string) error {
	if err := requireAdmin(actor); err != nil {
		return err
	}
	if actor.ID == id {
		return httperr.Validation("você não pode excluir a própria conta")
	}

	user, err := s.users.GetByID(ctx, id)
	if err != nil {
		return httperr.Internal("falha ao buscar usuário")
	}
	if user == nil {
		return httperr.NotFound("usuário não encontrado")
	}
	if user.Role == domain.RoleAdmin {
		if err := s.ensureNotLastAdmin(ctx); err != nil {
			return err
		}
	}

	ok, err := s.users.Deactivate(ctx, id)
	if err != nil {
		return httperr.Internal("falha ao excluir usuário")
	}
	if !ok {
		return httperr.NotFound("usuário não encontrado")
	}
	if s.refresh != nil {
		_ = s.refresh.RevokeAllForUser(ctx, id)
	}
	return nil
}

func (s *UserService) ensureNotLastAdmin(ctx context.Context) error {
	count, err := s.users.CountActiveAdmins(ctx)
	if err != nil {
		return httperr.Internal("falha ao verificar administradores")
	}
	if count <= 1 {
		return httperr.Validation("não é possível remover ou rebaixar o último administrador")
	}
	return nil
}

func requireAdmin(actor domain.User) error {
	if actor.Role != domain.RoleAdmin {
		return httperr.Forbidden("apenas administradores podem gerenciar usuários")
	}
	return nil
}

func validateUserIdentity(name, email, role string) (string, string, domain.UserRole, error) {
	name = strings.TrimSpace(name)
	email = strings.ToLower(strings.TrimSpace(email))
	if name == "" {
		return "", "", "", httperr.Validation("nome é obrigatório")
	}
	if email == "" {
		return "", "", "", httperr.Validation("e-mail é obrigatório")
	}
	parsed, err := mail.ParseAddress(email)
	if err != nil || !strings.EqualFold(parsed.Address, email) {
		return "", "", "", httperr.Validation("e-mail inválido")
	}

	parsedRole, err := parseUserRole(role)
	if err != nil {
		return "", "", "", err
	}
	return name, email, parsedRole, nil
}

func validatePassword(password string) error {
	if len(strings.TrimSpace(password)) < minUserPasswordLength {
		return httperr.Validation("a senha deve ter no mínimo 8 caracteres")
	}
	return nil
}

func parseUserRole(role string) (domain.UserRole, error) {
	switch domain.UserRole(strings.TrimSpace(role)) {
	case domain.RoleAdmin:
		return domain.RoleAdmin, nil
	case domain.RoleConsultor:
		return domain.RoleConsultor, nil
	case domain.RoleDeveloper:
		return domain.RoleDeveloper, nil
	default:
		return "", httperr.Validation("perfil inválido")
	}
}

func toUserListItem(u domain.User) UserListItem {
	return UserListItem{
		ID:    u.ID,
		Name:  u.Name,
		Email: u.Email,
		Role:  string(u.Role),
	}
}
