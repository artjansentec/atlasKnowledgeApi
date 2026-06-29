package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/atlas/knowledge-api/internal/config"
	"github.com/atlas/knowledge-api/internal/domain"
	"github.com/atlas/knowledge-api/internal/repository"
	"github.com/atlas/knowledge-api/pkg/httperr"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	cfg       *config.Config
	users     *repository.UserRepository
	refresh   *repository.RefreshTokenRepository
	jwtSecret []byte
}

type Claims struct {
	UserID string          `json:"userId"`
	Email  string          `json:"email"`
	Role   domain.UserRole `json:"role"`
	jwt.RegisteredClaims
}

type AuthResult struct {
	AccessToken  string
	RefreshToken string
	User         domain.User
}

func NewAuthService(cfg *config.Config, users *repository.UserRepository, refresh *repository.RefreshTokenRepository) *AuthService {
	return &AuthService{
		cfg:       cfg,
		users:     users,
		refresh:   refresh,
		jwtSecret: []byte(cfg.JWTSecret),
	}
}

func (s *AuthService) Login(ctx context.Context, email, password string) (*AuthResult, error) {
	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		return nil, httperr.Internal("falha ao autenticar")
	}
	if user == nil || !user.IsActive {
		return nil, httperr.Unauthorized("credenciais inválidas")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, httperr.Unauthorized("credenciais inválidas")
	}
	return s.issueTokens(ctx, *user)
}

func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (*AuthResult, error) {
	hash := hashToken(refreshToken)
	stored, err := s.refresh.GetValid(ctx, hash)
	if err != nil || stored == nil {
		return nil, httperr.Unauthorized("refresh token inválido")
	}
	user, err := s.users.GetByID(ctx, stored.UserID)
	if err != nil || user == nil {
		return nil, httperr.Unauthorized("usuário inválido")
	}
	_ = s.refresh.Revoke(ctx, hash)
	return s.issueTokens(ctx, *user)
}

func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {
	if refreshToken == "" {
		return nil
	}
	return s.refresh.Revoke(ctx, hashToken(refreshToken))
}

func (s *AuthService) ParseAccessToken(token string) (*Claims, error) {
	claims := &Claims{}
	parsed, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (interface{}, error) {
		return s.jwtSecret, nil
	})
	if err != nil || !parsed.Valid {
		return nil, httperr.Unauthorized("token inválido")
	}
	return claims, nil
}

func (s *AuthService) Me(ctx context.Context, userID string) (*domain.User, error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, httperr.Internal("falha ao buscar usuário")
	}
	if user == nil {
		return nil, httperr.Unauthorized("usuário não encontrado")
	}
	return user, nil
}

func (s *AuthService) issueTokens(ctx context.Context, user domain.User) (*AuthResult, error) {
	access, err := s.signAccess(user)
	if err != nil {
		return nil, httperr.Internal("falha ao gerar token")
	}

	refreshRaw, err := generateToken()
	if err != nil {
		return nil, httperr.Internal("falha ao gerar refresh")
	}

	rt := &domain.RefreshToken{
		UserID:    user.ID,
		TokenHash: hashToken(refreshRaw),
		ExpiresAt: time.Now().UTC().Add(s.cfg.JWTRefreshTTL),
	}
	if err := s.refresh.Create(ctx, rt); err != nil {
		return nil, httperr.Internal("falha ao persistir refresh")
	}

	return &AuthResult{AccessToken: access, RefreshToken: refreshRaw, User: user}, nil
}

func (s *AuthService) signAccess(user domain.User) (string, error) {
	claims := Claims{
		UserID: user.ID,
		Email:  user.Email,
		Role:   user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().UTC().Add(s.cfg.JWTAccessTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("rand: %w", err)
	}
	return hex.EncodeToString(b), nil
}
