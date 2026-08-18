package config

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv           string
	Port             string
	DatabaseURL      string
	AdminEmail       string
	AdminPassword    string
	AdminName        string
	JWTSecret        string
	JWTAccessTTL     time.Duration
	JWTRefreshTTL    time.Duration
	StoragePath      string
	MaxUploadBytes   int64
	CORSOrigins      []string
	RefreshCookie    string
	CookieSecure     bool
	CookieSameSite   http.SameSite
	APIBaseURL       string
	AIServiceURL     string
	AIServiceTimeout time.Duration
	DocMaxFiles      int
	DocMaxTotalBytes int64
	DocMaxFileBytes  int64
	MnemosAPIKey     string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	accessTTL, err := time.ParseDuration(getEnv("JWT_ACCESS_TTL", "15m"))
	if err != nil {
		return nil, fmt.Errorf("JWT_ACCESS_TTL inválido: %w", err)
	}

	refreshTTL, err := time.ParseDuration(getEnv("JWT_REFRESH_TTL", "168h"))
	if err != nil {
		return nil, fmt.Errorf("JWT_REFRESH_TTL inválido: %w", err)
	}

	maxUpload, err := strconv.ParseInt(getEnv("MAX_UPLOAD_BYTES", "20971520"), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("MAX_UPLOAD_BYTES inválido: %w", err)
	}

	corsRaw := getEnv("CORS_ORIGINS", "http://localhost:5173")
	var origins []string
	for _, o := range strings.Split(corsRaw, ",") {
		if trimmed := strings.TrimSpace(o); trimmed != "" {
			origins = append(origins, trimmed)
		}
	}

	port := getEnv("PORT", "8080")
	appEnv := strings.ToLower(getEnv("APP_ENV", "development"))
	jwtSecret := getEnv("JWT_SECRET", "change-me-in-production")
	adminPassword := getEnv("ADMIN_PASSWORD", "CHANGE_ME")

	if isProduction(appEnv) {
		if len(jwtSecret) < 32 || jwtSecret == "change-me-in-production" || jwtSecret == "CHANGE_ME" {
			return nil, fmt.Errorf("JWT_SECRET deve ter pelo menos 32 caracteres em produção")
		}
		if adminPassword == "" || adminPassword == "CHANGE_ME" {
			return nil, fmt.Errorf("ADMIN_PASSWORD deve ser definido com um valor forte em produção")
		}
	}

	cookieSecure := parseBool(getEnv("COOKIE_SECURE", ""), isProduction(appEnv))
	cookieSameSite, err := parseSameSite(getEnv("COOKIE_SAMESITE", "lax"))
	if err != nil {
		return nil, err
	}
	if cookieSameSite == http.SameSiteNoneMode {
		cookieSecure = true
	}

	aiTimeout, err := time.ParseDuration(getEnv("AI_SERVICE_TIMEOUT", "30m"))
	if err != nil {
		return nil, fmt.Errorf("AI_SERVICE_TIMEOUT inválido: %w", err)
	}

	docMaxFiles, err := strconv.Atoi(getEnv("DOC_MAX_FILES", "20"))
	if err != nil || docMaxFiles < 1 {
		return nil, fmt.Errorf("DOC_MAX_FILES inválido")
	}

	docMaxTotal, err := strconv.ParseInt(getEnv("DOC_MAX_TOTAL_BYTES", "104857600"), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("DOC_MAX_TOTAL_BYTES inválido: %w", err)
	}

	docMaxFile, err := strconv.ParseInt(getEnv("DOC_MAX_FILE_BYTES", getEnv("MAX_UPLOAD_BYTES", "20971520")), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("DOC_MAX_FILE_BYTES inválido: %w", err)
	}

	databaseURL, err := resolveDatabaseURL()
	if err != nil {
		return nil, err
	}

	return &Config{
		AppEnv:           appEnv,
		Port:             port,
		DatabaseURL:      databaseURL,
		AdminEmail:       getEnv("ADMIN_EMAIL", "arthur.oliveira@aquila.com.br"),
		AdminPassword:    adminPassword,
		AdminName:        getEnv("ADMIN_NAME", "Arthur Oliveira"),
		JWTSecret:        jwtSecret,
		JWTAccessTTL:     accessTTL,
		JWTRefreshTTL:    refreshTTL,
		StoragePath:      getEnv("STORAGE_PATH", "./storage"),
		MaxUploadBytes:   maxUpload,
		CORSOrigins:      origins,
		RefreshCookie:    "refresh_token",
		CookieSecure:     cookieSecure,
		CookieSameSite:   cookieSameSite,
		APIBaseURL:       getEnv("API_BASE_URL", fmt.Sprintf("http://localhost:%s", port)),
		AIServiceURL:     getEnv("AI_SERVICE_URL", "http://localhost:8081"),
		AIServiceTimeout: aiTimeout,
		DocMaxFiles:      docMaxFiles,
		DocMaxTotalBytes: docMaxTotal,
		DocMaxFileBytes:  docMaxFile,
		MnemosAPIKey:     getEnv("MNEMOS_API_KEY", ""),
	}, nil
}

func (c *Config) ServerAddress() string {
	return ":" + c.Port
}

func resolveDatabaseURL() (string, error) {
	if raw := strings.TrimSpace(os.Getenv("DATABASE_URL")); raw != "" {
		return raw, nil
	}

	user := getEnv("POSTGRES_USER", "postgres")
	password := getEnv("POSTGRES_PASSWORD", "postgres")
	host := getEnv("POSTGRES_HOST", "localhost")
	port := getEnv("POSTGRES_PORT", "5432")
	name := getEnv("POSTGRES_DB", "atlas_knowledge")
	sslMode := getEnv("POSTGRES_SSLMODE", "disable")

	if host == "" {
		return "", fmt.Errorf("POSTGRES_HOST vazio — informe o host do banco ou DATABASE_URL")
	}

	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(user, password),
		Host:   net.JoinHostPort(host, port),
		Path:   "/" + strings.TrimPrefix(name, "/"),
	}
	q := url.Values{}
	q.Set("sslmode", sslMode)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func isProduction(appEnv string) bool {
	return appEnv == "production" || appEnv == "prod"
}

func parseBool(raw string, fallback bool) bool {
	if raw == "" {
		return fallback
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return v
}

func parseSameSite(raw string) (http.SameSite, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "lax":
		return http.SameSiteLaxMode, nil
	case "strict":
		return http.SameSiteStrictMode, nil
	case "none":
		return http.SameSiteNoneMode, nil
	default:
		return 0, fmt.Errorf("COOKIE_SAMESITE inválido: use lax, strict ou none")
	}
}
