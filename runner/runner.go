package runner

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"monitoring/checks"
	"monitoring/models"
	vaultpkg "monitoring/vault"
)

type job struct {
	cancel context.CancelFunc
}

type Runner struct {
	pool  *pgxpool.Pool
	vault *vaultpkg.Client
	mu    sync.Mutex
	jobs  map[uuid.UUID]*job
}

func New(pool *pgxpool.Pool, vc *vaultpkg.Client) *Runner {
	return &Runner{
		pool:  pool,
		vault: vc,
		jobs:  make(map[uuid.UUID]*job),
	}
}

func (r *Runner) Start() {
	rows, err := r.pool.Query(context.Background(), `
		SELECT id, monitor_type, enabled, interval_secs, timeout_secs, config
		FROM monitors WHERE enabled = true
	`)
	if err != nil {
		slog.Error("runner: load monitors", "err", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var m models.Monitor
		if err := rows.Scan(&m.ID, &m.MonitorType, &m.Enabled, &m.IntervalSecs, &m.TimeoutSecs, &m.Config); err != nil {
			slog.Error("runner: scan monitor", "err", err)
			continue
		}
		r.startJob(m)
	}

	go r.cleanupLoop()
	slog.Info("runner started", "jobs", len(r.jobs))
}

func (r *Runner) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, j := range r.jobs {
		j.cancel()
	}
	r.jobs = make(map[uuid.UUID]*job)
	slog.Info("runner stopped")
}

// Reload is called by the API after a monitor is created, updated, or toggled.
func (r *Runner) Reload(id uuid.UUID) {
	var m models.Monitor
	err := r.pool.QueryRow(context.Background(), `
		SELECT id, monitor_type, enabled, interval_secs, timeout_secs, config
		FROM monitors WHERE id = $1
	`, id).Scan(&m.ID, &m.MonitorType, &m.Enabled, &m.IntervalSecs, &m.TimeoutSecs, &m.Config)
	if err != nil {
		slog.Error("runner: reload monitor", "id", id, "err", err)
		return
	}
	if !m.Enabled {
		r.Remove(id)
		return
	}
	r.startJob(m)
}

// Remove is called by the API when a monitor is deleted.
func (r *Runner) Remove(id uuid.UUID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if j, ok := r.jobs[id]; ok {
		j.cancel()
		delete(r.jobs, id)
	}
}

func (r *Runner) startJob(m models.Monitor) {
	ctx, cancel := context.WithCancel(context.Background())

	r.mu.Lock()
	if old, ok := r.jobs[m.ID]; ok {
		old.cancel()
	}
	r.jobs[m.ID] = &job{cancel: cancel}
	r.mu.Unlock()

	go func() {
		ticker := time.NewTicker(time.Duration(m.IntervalSecs) * time.Second)
		defer ticker.Stop()
		r.runCheck(ctx, m) // run once immediately on start
		for {
			select {
			case <-ticker.C:
				r.runCheck(ctx, m)
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (r *Runner) runCheck(ctx context.Context, m models.Monitor) {
	checkCtx, cancel := context.WithTimeout(ctx, time.Duration(m.TimeoutSecs)*time.Second)
	defer cancel()

	result := checks.Run(checkCtx, m.MonitorType, m.Config, safeVault(r.vault))

	var detailJSON []byte
	if result.Detail != nil {
		detailJSON, _ = json.Marshal(result.Detail)
	}

	_, err := r.pool.Exec(ctx, `
		INSERT INTO check_results (monitor_id, status, latency_ms, detail, error_message)
		VALUES ($1, $2, $3, $4, $5)
	`, m.ID, string(result.Status), result.LatencyMs, detailJSON, result.Error)
	if err != nil {
		slog.Error("runner: write result", "monitor_id", m.ID, "err", err)
	}

	slog.Debug("check done", "monitor_id", m.ID, "status", result.Status, "latency_ms", result.LatencyMs)
}

// cleanupLoop deletes check_results older than 30 days, runs nightly.
func (r *Runner) cleanupLoop() {
	ticker := time.NewTicker(24 * time.Hour)
	for range ticker.C {
		tag, err := r.pool.Exec(context.Background(),
			`DELETE FROM check_results WHERE checked_at < now() - interval '30 days'`)
		if err != nil {
			slog.Error("cleanup", "err", err)
		} else {
			slog.Info("cleanup done", "rows_deleted", tag.RowsAffected())
		}
	}
}

func safeVault(c *vaultpkg.Client) checks.VaultReader {
	if c == nil {
		return nil
	}
	return c
}
