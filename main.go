package main

import (
	"context"
	"embed"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"monitoring/api"
	"monitoring/config"
	"monitoring/db"
	"monitoring/runner"
	"monitoring/vault"
)

//go:embed frontend/dist
var frontendFS embed.FS

func main() {
	cfg := config.Load()

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.LogLevel,
	})))

	pool, err := db.Connect(cfg)
	if err != nil {
		slog.Error("db connect", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := db.Migrate(pool); err != nil {
		slog.Error("db migrate", "err", err)
		os.Exit(1)
	}

	var vc *vault.Client
	if cfg.VaultAddr != "" && cfg.VaultRoleID != "" {
		vc, err = vault.NewClient(cfg)
		if err != nil {
			slog.Error("vault init", "err", err)
			os.Exit(1)
		}
	}

	if cfg.InitialAPIToken != "" {
		if err := db.SeedToken(pool, cfg.InitialAPIToken); err != nil {
			slog.Warn("seed token", "err", err)
		}
	}

	r := runner.New(pool, vc)
	r.Start()

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      api.NewRouter(pool, vc, r, frontendFS),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		slog.Info("listening", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server", "err", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	r.Stop()
}
