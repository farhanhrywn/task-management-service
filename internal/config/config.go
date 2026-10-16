// Package config loads application configuration from environment variables.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv  string
	AppPort string

	DB         DBConfig
	JWT        JWTConfig
	Idem       IdempotencyConfig
	Pagination PaginationConfig
	SuperAdmin SuperAdminConfig
}

type DBConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	SSLMode  string
}

// DSN returns the PostgreSQL connection string. The session time zone is
// pinned to UTC so every timestamp round-trips in UTC.
func (d DBConfig) DSN() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s TimeZone=UTC",
		d.Host, d.Port, d.User, d.Password, d.Name, d.SSLMode)
}

type JWTConfig struct {
	Secret    string
	ExpiresIn time.Duration
}

type IdempotencyConfig struct {
	TTL          time.Duration
	WaitTimeout  time.Duration
	PollInterval time.Duration
}

type PaginationConfig struct {
	DefaultLimit int
	MaxLimit     int
}

type SuperAdminConfig struct {
	Name     string
	Email    string
	Password string
}

// Load reads configuration from the environment. A local .env file is loaded
// first when present; variables already set in the environment take precedence.
func Load() (*Config, error) {
	_ = godotenv.Load()

	var errs []error
	cfg := &Config{
		AppEnv:  getString("APP_ENV", "development"),
		AppPort: getString("APP_PORT", "8080"),
		DB: DBConfig{
			Host:     getString("DB_HOST", "localhost"),
			Port:     getString("DB_PORT", "5432"),
			User:     getString("DB_USER", "postgres"),
			Password: os.Getenv("DB_PASSWORD"),
			Name:     getString("DB_NAME", "task_management"),
			SSLMode:  getString("DB_SSLMODE", "disable"),
		},
		JWT: JWTConfig{
			Secret:    os.Getenv("JWT_SECRET"),
			ExpiresIn: getDuration("JWT_EXPIRES_IN", 24*time.Hour, &errs),
		},
		Idem: IdempotencyConfig{
			TTL:          getDuration("IDEMPOTENCY_TTL", 24*time.Hour, &errs),
			WaitTimeout:  getDuration("IDEMPOTENCY_WAIT_TIMEOUT", 10*time.Second, &errs),
			PollInterval: getDuration("IDEMPOTENCY_POLL_INTERVAL", 100*time.Millisecond, &errs),
		},
		Pagination: PaginationConfig{
			DefaultLimit: getInt("PAGINATION_DEFAULT_LIMIT", 10, &errs),
			MaxLimit:     getInt("PAGINATION_MAX_LIMIT", 50, &errs),
		},
		SuperAdmin: SuperAdminConfig{
			Name:     getString("SUPERADMIN_NAME", "Super Admin"),
			Email:    os.Getenv("SUPERADMIN_EMAIL"),
			Password: os.Getenv("SUPERADMIN_PASSWORD"),
		},
	}

	required := map[string]string{
		"DB_PASSWORD":         cfg.DB.Password,
		"JWT_SECRET":          cfg.JWT.Secret,
		"SUPERADMIN_EMAIL":    cfg.SuperAdmin.Email,
		"SUPERADMIN_PASSWORD": cfg.SuperAdmin.Password,
	}
	for key, val := range required {
		if val == "" {
			errs = append(errs, fmt.Errorf("%s is required", key))
		}
	}
	if len(cfg.SuperAdmin.Password) > 0 && len(cfg.SuperAdmin.Password) < 8 {
		errs = append(errs, errors.New("SUPERADMIN_PASSWORD must be at least 8 characters"))
	}
	if cfg.Pagination.DefaultLimit < 1 || cfg.Pagination.DefaultLimit > cfg.Pagination.MaxLimit {
		errs = append(errs, errors.New("PAGINATION_DEFAULT_LIMIT must be between 1 and PAGINATION_MAX_LIMIT"))
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return cfg, nil
}

func (c *Config) IsProduction() bool { return c.AppEnv == "production" }

func getString(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func getDuration(key string, def time.Duration, errs *[]error) time.Duration {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		*errs = append(*errs, fmt.Errorf("%s must be a positive duration (e.g. 10s, 24h)", key))
		return def
	}
	return d
}

func getInt(key string, def int, errs *[]error) int {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		*errs = append(*errs, fmt.Errorf("%s must be an integer", key))
		return def
	}
	return n
}
