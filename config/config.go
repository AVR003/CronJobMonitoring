package config

import (
	"log/slog"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port            string
	DBURL           string
	VaultAddr       string
	VaultRoleID     string
	VaultSecretID   string
	VaultMount      string
	InitialAPIToken string
	LogLevel        slog.Level
}

func Load() *Config {
	_ = godotenv.Load()

	level := slog.LevelInfo
	if os.Getenv("LOG_LEVEL") == "debug" {
		level = slog.LevelDebug
	}

	host := getenv("DB_HOST", "localhost")
	port := getenv("DB_PORT", "5432")
	user := getenv("DB_USER", "postgres")
	pass := getenv("DB_PASSWORD", "postgres")
	name := getenv("DB_NAME", "cronjobs")
	ssl := getenv("DB_SSLMODE", "disable")

	dbURL := "postgres://" + user + ":" + pass + "@" + host + ":" + port + "/" + name + "?sslmode=" + ssl

	return &Config{
		Port:            getenv("PORT", "8080"),
		DBURL:           dbURL,
		VaultAddr:       getenv("VAULT_ADDR", ""),
		VaultRoleID:     getenv("VAULT_ROLE_ID", ""),
		VaultSecretID:   getenv("VAULT_SECRET_ID", ""),
		VaultMount:      getenv("VAULT_MOUNT", "secret"),
		InitialAPIToken: getenv("INITIAL_API_TOKEN", ""),
		LogLevel:        level,
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
