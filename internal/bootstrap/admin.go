package bootstrap

import (
	"context"
	"fmt"

	"github.com/atlas/knowledge-api/internal/db"
	"github.com/atlas/knowledge-api/internal/domain"
	"github.com/atlas/knowledge-api/internal/repository"
	"github.com/atlas/knowledge-api/internal/service"
)

type EnsureAdminOptions struct {
	Email    string
	Password string
	Name     string
}

type EnsureAdminResult struct {
	Created bool
	Email   string
	Name    string
}

func EnsureDefaultAdmin(ctx context.Context, database *db.DB, opts EnsureAdminOptions) (*EnsureAdminResult, error) {
	var count int
	if err := database.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return nil, fmt.Errorf("consulta users: %w", err)
	}
	if count > 0 {
		return &EnsureAdminResult{Created: false}, nil
	}

	hash, err := service.HashPassword(opts.Password)
	if err != nil {
		return nil, fmt.Errorf("hash senha: %w", err)
	}

	user := domain.User{
		Email:        opts.Email,
		PasswordHash: hash,
		Name:         opts.Name,
		Role:         domain.RoleAdmin,
		IsActive:     true,
	}

	tx, err := database.Pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("transação: %w", err)
	}
	defer tx.Rollback(ctx)

	users := repository.NewUserRepository(database)
	if err := users.Create(ctx, tx, &user); err != nil {
		return nil, fmt.Errorf("criar admin: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return &EnsureAdminResult{Created: true, Email: user.Email, Name: user.Name}, nil
}
