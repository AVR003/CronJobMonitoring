package checks

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"time"
)

type httpConfig struct {
	URL            string `json:"url"`
	Method         string `json:"method"`
	ExpectedStatus int    `json:"expected_status"`
	BodyPattern    string `json:"body_pattern"`
}

var httpCl = &http.Client{Timeout: 30 * time.Second}

func runHTTP(ctx context.Context, raw json.RawMessage) Result {
	var cfg httpConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return Result{Status: StatusUnknown, Error: "invalid config: " + err.Error()}
	}
	if cfg.URL == "" {
		return Result{Status: StatusUnknown, Error: "url is required"}
	}
	if cfg.Method == "" {
		cfg.Method = "GET"
	}
	if cfg.ExpectedStatus == 0 {
		cfg.ExpectedStatus = 200
	}

	req, err := http.NewRequestWithContext(ctx, cfg.Method, cfg.URL, nil)
	if err != nil {
		return Result{Status: StatusDown, Error: "build request: " + err.Error()}
	}

	start := time.Now()
	resp, err := httpCl.Do(req)
	latency := float64(time.Since(start).Milliseconds())
	if err != nil {
		return Result{Status: StatusDown, LatencyMs: &latency, Error: err.Error()}
	}
	defer resp.Body.Close()

	detail := map[string]any{"status_code": resp.StatusCode, "url": cfg.URL}

	if resp.StatusCode != cfg.ExpectedStatus {
		return Result{
			Status:    StatusDown,
			LatencyMs: &latency,
			Detail:    detail,
			Error:     fmt.Sprintf("expected %d got %d", cfg.ExpectedStatus, resp.StatusCode),
		}
	}

	if cfg.BodyPattern != "" {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
		if matched, _ := regexp.Match(cfg.BodyPattern, body); !matched {
			return Result{
				Status:    StatusDown,
				LatencyMs: &latency,
				Detail:    detail,
				Error:     fmt.Sprintf("body pattern %q not matched", cfg.BodyPattern),
			}
		}
	}

	return Result{Status: StatusUp, LatencyMs: &latency, Detail: detail}
}
