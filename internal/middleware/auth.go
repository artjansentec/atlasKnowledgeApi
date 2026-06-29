package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/atlas/knowledge-api/internal/config"
	"github.com/atlas/knowledge-api/internal/domain"
	"github.com/atlas/knowledge-api/internal/service"
	"github.com/labstack/echo/v4"
	"golang.org/x/time/rate"
)

const UserContextKey = "user"

type AuthMiddleware struct {
	auth *service.AuthService
}

func NewAuthMiddleware(auth *service.AuthService) *AuthMiddleware {
	return &AuthMiddleware{auth: auth}
}

func (m *AuthMiddleware) RequireAuth(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		header := c.Request().Header.Get(echo.HeaderAuthorization)
		if len(header) < 8 || header[:7] != "Bearer " {
			return c.JSON(http.StatusUnauthorized, map[string]interface{}{
				"success": false,
				"error":   map[string]string{"code": "UNAUTHORIZED", "message": "token ausente"},
			})
		}
		claims, err := m.auth.ParseAccessToken(header[7:])
		if err != nil {
			return c.JSON(http.StatusUnauthorized, map[string]interface{}{
				"success": false,
				"error":   map[string]string{"code": "UNAUTHORIZED", "message": "token inválido"},
			})
		}
		user, err := m.auth.Me(c.Request().Context(), claims.UserID)
		if err != nil {
			return c.JSON(http.StatusUnauthorized, map[string]interface{}{
				"success": false,
				"error":   map[string]string{"code": "UNAUTHORIZED", "message": "usuário inválido"},
			})
		}
		c.Set(UserContextKey, *user)
		return next(c)
	}
}

func GetUser(c echo.Context) domain.User {
	return c.Get(UserContextKey).(domain.User)
}

func CORS(cfg *config.Config) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			origin := c.Request().Header.Get(echo.HeaderOrigin)
			for _, allowed := range cfg.CORSOrigins {
				if allowed == "*" || allowed == origin {
					c.Response().Header().Set(echo.HeaderAccessControlAllowOrigin, origin)
					break
				}
			}
			c.Response().Header().Set(echo.HeaderAccessControlAllowCredentials, "true")
			c.Response().Header().Set(echo.HeaderAccessControlAllowHeaders, "Content-Type, Authorization")
			c.Response().Header().Set(echo.HeaderAccessControlAllowMethods, "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			if c.Request().Method == http.MethodOptions {
				return c.NoContent(http.StatusNoContent)
			}
			return next(c)
		}
	}
}

type ipLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type RateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*ipLimiter
	limit    rate.Limit
	burst    int
}

func NewRateLimiter(perMinute int) *RateLimiter {
	rl := &RateLimiter{
		limiters: make(map[string]*ipLimiter),
		limit:    rate.Limit(float64(perMinute) / 60.0),
		burst:    perMinute,
	}
	go rl.cleanup()
	return rl
}

func (rl *RateLimiter) cleanup() {
	for {
		time.Sleep(time.Minute)
		rl.mu.Lock()
		for ip, l := range rl.limiters {
			if time.Since(l.lastSeen) > 3*time.Minute {
				delete(rl.limiters, ip)
			}
		}
		rl.mu.Unlock()
	}
}

func (rl *RateLimiter) Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ip := c.RealIP()
			rl.mu.Lock()
			l, ok := rl.limiters[ip]
			if !ok {
				l = &ipLimiter{limiter: rate.NewLimiter(rl.limit, rl.burst)}
				rl.limiters[ip] = l
			}
			l.lastSeen = time.Now()
			rl.mu.Unlock()

			if !l.limiter.Allow() {
				return c.JSON(http.StatusTooManyRequests, map[string]interface{}{
					"success": false,
					"error":   map[string]string{"code": "RATE_LIMIT", "message": "muitas requisições, tente novamente em instantes"},
				})
			}
			return next(c)
		}
	}
}
