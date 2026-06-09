package checks

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"
)

type tcpConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

func runTCP(ctx context.Context, raw json.RawMessage) Result {
	var cfg tcpConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return Result{Status: StatusUnknown, Error: "invalid config: " + err.Error()}
	}
	if cfg.Host == "" || cfg.Port == 0 {
		return Result{Status: StatusUnknown, Error: "host and port are required"}
	}

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	timeout := 10 * time.Second
	if dl, ok := ctx.Deadline(); ok {
		timeout = time.Until(dl)
	}

	start := time.Now()
	conn, err := net.DialTimeout("tcp", addr, timeout)
	latency := float64(time.Since(start).Milliseconds())

	if err != nil {
		return Result{Status: StatusDown, LatencyMs: &latency, Error: err.Error()}
	}
	conn.Close()
	return Result{Status: StatusUp, LatencyMs: &latency, Detail: map[string]any{"addr": addr}}
}
