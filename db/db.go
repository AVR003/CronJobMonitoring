package db

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"monitoring/config"
)

//go:embed schema.sql
var schema string

func Connect(cfg *config.Config) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(context.Background(), cfg.DBURL)
	if err != nil {
		return nil, fmt.Errorf("pgxpool.New: %w", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("ping: %w", err)
	}
	slog.Info("database connected")
	return pool, nil
}

func Migrate(pool *pgxpool.Pool) error {
	if _, err := pool.Exec(context.Background(), schema); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}
	slog.Info("schema ready")
	return nil
}

func SeedToken(pool *pgxpool.Pool, token string) error {
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(token)))
	_, err := pool.Exec(context.Background(), `
		INSERT INTO api_tokens (id, name, token_hash)
		VALUES ($1, 'default', $2)
		ON CONFLICT (token_hash) DO NOTHING
	`, uuid.New(), hash)
	return err
}
