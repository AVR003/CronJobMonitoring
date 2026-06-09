package checks

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type pingConfig struct {
	Host      string `json:"host"`
	Count     int    `json:"count"`
	TimeoutMs int    `json:"timeout_ms"`
}

// Windows: "Average = 4ms"
var winRTTRe = regexp.MustCompile(`Average = (\d+)ms`)

// Linux/macOS: "rtt min/avg/max/mdev = 0.1/0.2/0.3/0.0 ms"
var unixRTTRe = regexp.MustCompile(`min/avg/max(?:/mdev)? = [\d.]+/([\d.]+)/`)

func runPing(ctx context.Context, raw json.RawMessage) Result {
	var cfg pingConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return Result{Status: StatusUnknown, Error: "invalid config: " + err.Error()}
	}
	if cfg.Host == "" {
		return Result{Status: StatusUnknown, Error: "host is required"}
	}
	if cfg.Count == 0 {
		cfg.Count = 3
	}
	if cfg.TimeoutMs == 0 {
		cfg.TimeoutMs = 1000
	}

	var args []string
	if runtime.GOOS == "windows" {
		// -n count  -w timeout_ms
		args = []string{"-n", strconv.Itoa(cfg.Count), "-w", strconv.Itoa(cfg.TimeoutMs), cfg.Host}
	} else {
		// -c count  -W timeout_seconds (minimum 1)
		timeoutSec := cfg.TimeoutMs / 1000
		if timeoutSec < 1 {
			timeoutSec = 1
		}
		args = []string{"-c", strconv.Itoa(cfg.Count), "-W", strconv.Itoa(timeoutSec), cfg.Host}
	}

	start := time.Now()
	out, err := exec.CommandContext(ctx, "ping", args...).CombinedOutput()
	elapsed := float64(time.Since(start).Milliseconds())
	outStr := string(out)

	// On Windows, ping exits 0 even on "100% loss" — check output text too.
	down := err != nil
	if runtime.GOOS == "windows" && !down {
		if strings.Contains(outStr, "100% loss") ||
			strings.Contains(outStr, "Request timed out") ||
			strings.Contains(outStr, "could not find host") ||
			strings.Contains(outStr, "Destination host unreachable") {
			down = true
		}
	}

	if down {
		return Result{
			Status:    StatusDown,
			LatencyMs: &elapsed,
			Error:     fmt.Sprintf("unreachable: %s", outStr),
		}
	}

	latency := elapsed
	re := winRTTRe
	if runtime.GOOS != "windows" {
		re = unixRTTRe
	}
	if m := re.FindStringSubmatch(outStr); len(m) > 1 {
		if v, parseErr := strconv.ParseFloat(m[1], 64); parseErr == nil {
			latency = v
		}
	}

	return Result{
		Status:    StatusUp,
		LatencyMs: &latency,
		Detail:    map[string]any{"output": outStr},
	}
}
