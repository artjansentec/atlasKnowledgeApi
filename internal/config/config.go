package config

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
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
	StorageDriver    string
	S3Bucket         string
	S3Prefix         string
	S3Region         string
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
	// Docker na EC2 costuma herdar APP_ENV=development do .env local via env_file.
	if runningInDocker() && !isProduction(appEnv) {
		appEnv = "production"
	}
	jwtSecret := getEnv("JWT_SECRET", "change-me-in-production")
	adminPassword := getEnv("ADMIN_PASSWORD", "CHANGE_ME")

	if isProduction(appEnv) {
		if len(jwtSecret) < 32 || jwtSecret == "change-me-in-production" || jwtSecret == "CHANGE_ME" {
			return nil, fmt.Errorf("JWT_SECRET deve ter pelo menos 32 caracteres em produção")
		}
		if adminPassword == "" || adminPassword == "CHANGE_ME" || len(adminPassword) < 8 {
			return nil, fmt.Errorf("ADMIN_PASSWORD deve ser definido com um valor forte em produção (mín. 8 caracteres)")
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

	storageDriver, s3Bucket, s3Prefix, s3Region, err := resolveStorageConfig()
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
		StoragePath:      resolveStoragePath(),
		StorageDriver:    storageDriver,
		S3Bucket:         s3Bucket,
		S3Prefix:         s3Prefix,
		S3Region:         s3Region,
		MaxUploadBytes:   maxUpload,
		CORSOrigins:      origins,
		RefreshCookie:    "refresh_token",
		CookieSecure:     cookieSecure,
		CookieSameSite:   cookieSameSite,
		APIBaseURL:       getEnv("API_BASE_URL", fmt.Sprintf("http://localhost:%s", port)),
		AIServiceURL:     resolveAIServiceURL(),
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

func (c *Config) StorageLabel() string {
	if strings.EqualFold(c.StorageDriver, "s3") {
		if prefix := strings.Trim(c.S3Prefix, "/"); prefix != "" {
			return "s3://" + c.S3Bucket + "/" + prefix
		}
		return "s3://" + c.S3Bucket
	}
	return c.StoragePath
}

// StartupWarnings lista valores de .env de desenvolvimento que quebram o front na EC2
// (CORS, downloads) mas não impedem a API de subir.
func (c *Config) StartupWarnings() []string {
	if !runningInDocker() {
		return nil
	}
	var warnings []string
	if isLoopbackHost(hostnameFromURL(c.APIBaseURL)) {
		warnings = append(warnings, "API_BASE_URL ainda é localhost — links de download de anexos vão apontar para o container. Defina a URL pública (https://api.suaempresa.com)")
	}
	onlyLocalCORS := len(c.CORSOrigins) > 0
	for _, origin := range c.CORSOrigins {
		if origin == "*" || (!strings.Contains(origin, "localhost") && !strings.Contains(origin, "127.0.0.1")) {
			onlyLocalCORS = false
			break
		}
	}
	if onlyLocalCORS {
		warnings = append(warnings, "CORS_ORIGINS só tem localhost — o front em produção será bloqueado pelo navegador. Use a origem HTTPS do front")
	}
	return warnings
}

func resolveDatabaseURL() (string, error) {
	if raw := strings.TrimSpace(os.Getenv("DATABASE_URL")); raw != "" {
		return normalizeDatabaseURL(raw)
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
	if err := rejectDockerLoopback("POSTGRES_HOST", host); err != nil {
		return "", err
	}
	if isRDSHost(host) && (sslMode == "" || sslMode == "disable") {
		sslMode = "require"
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

func normalizeDatabaseURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("DATABASE_URL inválida: %w", err)
	}
	if err := rejectDockerLoopback("DATABASE_URL", u.Hostname()); err != nil {
		return "", err
	}
	if isRDSHost(u.Hostname()) {
		q := u.Query()
		if mode := q.Get("sslmode"); mode == "" || mode == "disable" {
			q.Set("sslmode", "require")
			u.RawQuery = q.Encode()
		}
	}
	return u.String(), nil
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func runningInDocker() bool {
	_, err := os.Stat("/.dockerenv")
	return err == nil
}

func isLoopbackHost(host string) bool {
	h := strings.ToLower(strings.TrimSpace(host))
	return h == "localhost" || h == "127.0.0.1" || h == "::1" || strings.HasPrefix(h, "127.")
}

func isRDSHost(host string) bool {
	return strings.Contains(strings.ToLower(host), "rds.amazonaws.com")
}

func hostnameFromURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

func rejectDockerLoopback(label, host string) error {
	if runningInDocker() && isLoopbackHost(host) {
		return fmt.Errorf("%s=%s não funciona no Docker (localhost é o próprio container). Use o endpoint do RDS, host.docker.internal (Postgres na EC2) ou postgres (compose --profile db)", label, host)
	}
	return nil
}

// resolveStoragePath evita mkdir ./storage no Docker (cwd /, user atlas → permission denied).
func resolveStoragePath() string {
	path := getEnv("STORAGE_PATH", "./storage")
	if filepath.IsAbs(path) {
		return path
	}
	if runningInDocker() {
		return "/data/storage"
	}
	return path
}

// resolveStorageConfig: na EC2 basta S3_BUCKET. Credenciais vêm da IAM role da instância.
func resolveStorageConfig() (driver, bucket, prefix, region string, err error) {
	bucket = strings.TrimSpace(getEnv("S3_BUCKET", ""))
	prefix = strings.TrimSpace(getEnv("S3_PREFIX", ""))
	region = strings.TrimSpace(getEnv("AWS_REGION", getEnv("AWS_DEFAULT_REGION", "")))
	driver = strings.ToLower(strings.TrimSpace(getEnv("STORAGE_DRIVER", "")))
	if driver == "" {
		if bucket != "" {
			driver = "s3"
		} else {
			driver = "local"
		}
	}
	if driver != "local" && driver != "s3" {
		return "", "", "", "", fmt.Errorf("STORAGE_DRIVER inválido: use local ou s3")
	}
	if driver == "s3" && bucket == "" {
		return "", "", "", "", fmt.Errorf("S3_BUCKET é obrigatório para usar o bucket S3")
	}
	return driver, bucket, prefix, region, nil
}

func resolveAIServiceURL() string {
	raw := getEnv("AI_SERVICE_URL", "http://localhost:8081")
	if runningInDocker() && isLoopbackHost(hostnameFromURL(raw)) {
		return "http://mnemos:8081"
	}
	return raw
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
