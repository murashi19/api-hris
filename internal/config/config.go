package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Environment         string
	HTTPAddress         string
	Timezone            string
	DatabaseURL         string
	RedisAddress        string
	RedisPassword       string
	RedisDB             int
	JWTSecret           string
	AccessTokenTTL      time.Duration
	RefreshTokenTTL     time.Duration
	RefreshCookieName   string
	RefreshCookieSecure bool
	CORSOrigins         []string
}

func Load() (Config, error) {
	cfg := Config{
		Environment:         env("APP_ENV", "development"),
		HTTPAddress:         env("HTTP_ADDRESS", ":8080"),
		Timezone:            env("APP_TIMEZONE", "Asia/Jakarta"),
		DatabaseURL:         os.Getenv("DATABASE_URL"),
		RedisAddress:        env("REDIS_ADDRESS", "localhost:6379"),
		RedisPassword:       os.Getenv("REDIS_PASSWORD"),
		RefreshCookieName:   env("REFRESH_COOKIE_NAME", "hris_refresh_token"),
		RefreshCookieSecure: envBool("REFRESH_COOKIE_SECURE", false),
		CORSOrigins:         splitCSV(env("CORS_ORIGINS", "http://localhost:5173")),
	}
	var err error
	if cfg.RedisDB, err = strconv.Atoi(env("REDIS_DB", "0")); err != nil {
		return Config{}, fmt.Errorf("REDIS_DB must be an integer: %w", err)
	}
	if cfg.AccessTokenTTL, err = time.ParseDuration(env("ACCESS_TOKEN_TTL", "15m")); err != nil {
		return Config{}, fmt.Errorf("invalid ACCESS_TOKEN_TTL: %w", err)
	}
	if cfg.RefreshTokenTTL, err = time.ParseDuration(env("REFRESH_TOKEN_TTL", "168h")); err != nil {
		return Config{}, fmt.Errorf("invalid REFRESH_TOKEN_TTL: %w", err)
	}
	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	if len(cfg.JWTSecret) == 0 {
		cfg.JWTSecret = os.Getenv("JWT_SECRET")
	}
	if len(cfg.JWTSecret) < 32 {
		return Config{}, errors.New("JWT_SECRET must be at least 32 characters")
	}
	if cfg.AccessTokenTTL <= 0 || cfg.RefreshTokenTTL <= 0 {
		return Config{}, errors.New("token TTL values must be positive")
	}
	if _, err := time.LoadLocation(cfg.Timezone); err != nil {
		return Config{}, fmt.Errorf("invalid APP_TIMEZONE: %w", err)
	}
	return cfg, nil
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
