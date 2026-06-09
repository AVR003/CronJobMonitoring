package checks

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type postgresConfig struct {
	Host      string `json:"host"`
	Port      int    `json:"port"`
	Database  string `json:"database"`
	User      string `json:"user"`
	Password  string `json:"password"`  // plaintext fallback (dev only)
	VaultPath string `json:"vault_path"` // overrides password if set
	Query     string `json:"query"`
	SSLMode   string `json:"sslmode"`
}

func runPostgres(ctx context.Context, raw json.RawMessage, vault VaultReader) Result {
	var cfg postgresConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return Result{Status: StatusUnknown, Error: "invalid config: " + err.Error()}
	}
	if cfg.Host == "" || cfg.Database == "" || cfg.User == "" {
		return Result{Status: StatusUnknown, Error: "host, database and user are required"}
	}
	if cfg.Port == 0 {
		cfg.Port = 5432
	}
	if cfg.SSLMode == "" {
		cfg.SSLMode = "disable"
	}

	password := cfg.Password
	if cfg.VaultPath != "" && vault != nil {
		secrets, err := vault.ReadSecret(ctx, cfg.VaultPath)
		if err != nil {
			return Result{Status: StatusUnknown, Error: "vault: " + err.Error()}
		}
		password = secrets["password"]
	}

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		cfg.User, password, cfg.Host, cfg.Port, cfg.Database, cfg.SSLMode)

	start := time.Now()
	conn, err := pgx.Connect(ctx, dsn)
	latency := float64(time.Since(start).Milliseconds())
	if err != nil {
		return Result{Status: StatusDown, LatencyMs: &latency, Error: err.Error()}
	}
	defer conn.Close(ctx)

	if cfg.Query != "" {
		if _, err := conn.Exec(ctx, cfg.Query); err != nil {
			return Result{Status: StatusDegraded, LatencyMs: &latency, Error: "query: " + err.Error()}
		}
	}

	return Result{Status: StatusUp, LatencyMs: &latency}
}
