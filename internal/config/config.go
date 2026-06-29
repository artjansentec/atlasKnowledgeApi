package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Port            string
	DatabaseURL     string
	JWTSecret       string
	JWTAccessTTL    time.Duration
	JWTRefreshTTL   time.Duration
	StoragePath     string
	MaxUploadBytes  int64
	CORSOrigins     []string
	RefreshCookie   string
	APIBaseURL      string
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

	return &Config{
		Port:           port,
		DatabaseURL:    getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/atlas_knowledge?sslmode=disable"),
		JWTSecret:      getEnv("JWT_SECRET", "change-me-in-production"),
		JWTAccessTTL:   accessTTL,
		JWTRefreshTTL:  refreshTTL,
		StoragePath:    getEnv("STORAGE_PATH", "./storage"),
		MaxUploadBytes: maxUpload,
		CORSOrigins:    origins,
		RefreshCookie:  "refresh_token",
		APIBaseURL:     getEnv("API_BASE_URL", fmt.Sprintf("http://localhost:%s", port)),
	}, nil
}

func (c *Config) ServerAddress() string {
	return ":" + c.Port
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
