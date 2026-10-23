// Package config loads all runtime settings from environment variables (12-factor).
package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTPAddr      string
	DatabaseURL   string
	MasterKey     string // AES-256 key material (base64 32 bytes, or passphrase → SHA-256)
	PublicBaseURL string // used to build deep links, e.g. https://platform.local
	StaticDir     string // optional: serve built frontend from the same binary
	CORSOrigins   []string

	JWTSecret    string
	AuthDisabled bool // DEV ONLY

	WorkerPoolSize     int
	WorkerQueueSize    int
	QueryTimeout       time.Duration
	HeartbeatInterval  time.Duration
	ProbeFailThreshold int
	LinkTTL            time.Duration

	SMTPHost string
	SMTPPort int
	SMTPUser string
	SMTPPass string
	SMTPFrom string
}

func Load() Config {
	return Config{
		HTTPAddr:           env("HTTP_ADDR", ":8080"),
		DatabaseURL:        env("DATABASE_URL", "postgres://sentinel:sentinel@localhost:5432/sentinel?sslmode=disable"),
		MasterKey:          env("MASTER_KEY", ""),
		PublicBaseURL:      strings.TrimRight(env("PUBLIC_BASE_URL", "https://platform.local"), "/"),
		StaticDir:          env("STATIC_DIR", ""),
		CORSOrigins:        strings.Split(env("CORS_ORIGINS", "http://localhost:5173"), ","),
		JWTSecret:          env("JWT_SECRET", ""),
		AuthDisabled:       envBool("AUTH_DISABLED", false),
		WorkerPoolSize:     envInt("WORKER_POOL_SIZE", 32),
		WorkerQueueSize:    envInt("WORKER_QUEUE_SIZE", 1024),
		QueryTimeout:       envDur("QUERY_TIMEOUT", 60*time.Second),
		HeartbeatInterval:  envDur("HEARTBEAT_INTERVAL", 60*time.Second),
		ProbeFailThreshold: envInt("PROBE_FAIL_THRESHOLD", 2),
		LinkTTL:            envDur("LINK_TTL", 30*24*time.Hour),
		SMTPHost:           env("SMTP_HOST", ""),
		SMTPPort:           envInt("SMTP_PORT", 587),
		SMTPUser:           env("SMTP_USER", ""),
		SMTPPass:           env("SMTP_PASS", ""),
		SMTPFrom:           env("SMTP_FROM", "sentinel@platform.local"),
	}
}

func env(k, d string) string {
	if v, ok := os.LookupEnv(k); ok && v != "" {
		return v
	}
	return d
}

func envInt(k string, d int) int {
	if v, err := strconv.Atoi(os.Getenv(k)); err == nil {
		return v
	}
	return d
}

func envBool(k string, d bool) bool {
	if v, err := strconv.ParseBool(os.Getenv(k)); err == nil {
		return v
	}
	return d
}

func envDur(k string, d time.Duration) time.Duration {
	if v, err := time.ParseDuration(os.Getenv(k)); err == nil {
		return v
	}
	return d
}
