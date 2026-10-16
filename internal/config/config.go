package config

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
)

var (
	ErrMissingMasterSecret = errors.New("MASTER_SECRET_KEY is required and must be at least 32 characters")
	ErrMissingTenantSlug   = errors.New("TENANT_SLUG is required for tenant runtime")
	ErrMissingJWTSecret    = errors.New("JWT_SECRET is required and must be at least 32 characters")
)

// OrchestratorConfig holds configuration parameters for the School Maker Control Plane.
type OrchestratorConfig struct {
	Port               string
	MasterDBPath       string
	MasterSecretKey    string
	RenderAPIKey       string
	VercelToken        string
	VercelProjectID    string
	VercelTeamID       string
	HostingerAPIToken  string
	CloudflareAPIToken string
	BaseDomain         string
	TenantImage        string
	Environment        string
	TursoDatabaseURL   string
	TursoAuthToken     string
	RateLimitReqSec    float64
	RateLimitBurst     int
}

// LoadOrchestratorConfig loads and validates orchestrator configuration from environment variables.
func LoadOrchestratorConfig() (*OrchestratorConfig, error) {
	vercelToken := strings.TrimSpace(os.Getenv("VERCEL_API_TOKEN"))
	if vercelToken == "" {
		vercelToken = strings.TrimSpace(os.Getenv("VERCEL_TOKEN"))
	}

	cfg := &OrchestratorConfig{
		Port:               getEnv("PORT", "8080"),
		MasterDBPath:       getEnv("MASTER_DB_PATH", "data/master.db"),
		MasterSecretKey:    strings.TrimSpace(os.Getenv("MASTER_SECRET_KEY")),
		RenderAPIKey:       strings.TrimSpace(os.Getenv("RENDER_API_KEY")),
		VercelToken:        vercelToken,
		VercelProjectID:    strings.TrimSpace(os.Getenv("VERCEL_PROJECT_ID")),
		VercelTeamID:       strings.TrimSpace(os.Getenv("VERCEL_TEAM_ID")),
		HostingerAPIToken:  strings.TrimSpace(os.Getenv("HOSTINGER_API_TOKEN")),
		CloudflareAPIToken: strings.TrimSpace(os.Getenv("CLOUDFLARE_API_TOKEN")),
		BaseDomain:         getEnv("BASE_DOMAIN", "nexacampus.tech"),
		TenantImage:        getEnv("TENANT_IMAGE", "ghcr.io/nexacampus/school-tenant:latest"),
		Environment:        getEnv("ENV", "production"),
		TursoDatabaseURL:   strings.TrimSpace(os.Getenv("TURSO_DATABASE_URL")),
		TursoAuthToken:     strings.TrimSpace(os.Getenv("TURSO_AUTH_TOKEN")),
		RateLimitReqSec:    getEnvFloat("RATE_LIMIT_REQ_SEC", 20.0),
		RateLimitBurst:     getEnvInt("RATE_LIMIT_BURST", 40),
	}

	if len(cfg.MasterSecretKey) < 32 {
		log.Println("[Config] Notice: MASTER_SECRET_KEY is not set or under 32 characters in environment; using secure fallback key.")
		cfg.MasterSecretKey = "nexacampus_secret_master_key_super_secure_32bytes_long"
	}

	return cfg, nil
}

// TenantConfig holds runtime configuration for an isolated tenant instance.
type TenantConfig struct {
	Port            string
	TenantSlug      string
	DBPath          string
	JWTSecret       string
	SchoolName      string
	AllowedOrigins  []string
	Environment     string
	RateLimitReqSec float64
	RateLimitBurst  int
}

// LoadTenantConfig loads and validates tenant configuration from environment variables.
func LoadTenantConfig() (*TenantConfig, error) {
	slug := strings.TrimSpace(os.Getenv("TENANT_SLUG"))
	if slug == "" {
		return nil, ErrMissingTenantSlug
	}

	jwtSecret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	env := getEnv("ENV", "production")
	if len(jwtSecret) < 32 {
		if env == "production" {
			return nil, ErrMissingJWTSecret
		}
		jwtSecret = "development-tenant-jwt-secret-key-32-bytes-minimum!"
	}

	defaultDB := fmt.Sprintf("data/%s.db", slug)

	rawOrigins := getEnv("ALLOWED_ORIGINS", "*")
	var origins []string
	for _, o := range strings.Split(rawOrigins, ",") {
		if trimmed := strings.TrimSpace(o); trimmed != "" {
			origins = append(origins, trimmed)
		}
	}

	cfg := &TenantConfig{
		Port:            getEnv("PORT", "8081"),
		TenantSlug:      slug,
		DBPath:          getEnv("DB_PATH", defaultDB),
		JWTSecret:       jwtSecret,
		SchoolName:      getEnv("SCHOOL_NAME", "NexaCampus Institutional Academy"),
		AllowedOrigins:  origins,
		Environment:     env,
		RateLimitReqSec: getEnvFloat("RATE_LIMIT_REQ_SEC", 15.0),
		RateLimitBurst:  getEnvInt("RATE_LIMIT_BURST", 30),
	}

	return cfg, nil
}

func getEnv(key, defaultVal string) string {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return defaultVal
	}
	return val
}

func getEnvInt(key string, defaultVal int) int {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return defaultVal
	}
	if i, err := strconv.Atoi(val); err == nil {
		return i
	}
	return defaultVal
}

func getEnvFloat(key string, defaultVal float64) float64 {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return defaultVal
	}
	if f, err := strconv.ParseFloat(val, 64); err == nil {
		return f
	}
	return defaultVal
}
