# Universal Monitoring System — Implementation Plan

## What We're Building

A self-hosted monitoring platform — written in Go — that runs as a **single binary** containing three concerns:

1. **Monitor Runner** — background goroutines that execute checks (ping, DB, HTTP, etc.) on configurable schedules and write results to PostgreSQL.
2. **REST API** — token-authenticated HTTP API consumed by the React UI and any third-party integrations.
3. **React Frontend** — embedded in the binary at build time; served as static files. Used to configure monitors (IP, port, credentials, schedule) and view live status.

---

## Decisions — Locked In

| Decision | Choice |
|----------|--------|
| Language | Go |
| Credential storage | HashiCorp Vault |
| Binary shape | Single binary (API + runner in one process) |
| Result retention | 30 days (cleanup goroutine runs nightly) |
| API authentication | Static bearer token (set via env var) |
| ICMP ping | `fping` subprocess (`fping -c 1 -t 500 <host>`) |
| Frontend hosting | Embedded in Go binary via `embed.FS` |

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    React Frontend (embedded)                      │
│     Configure monitors · Dashboard · Alert history               │
└──────────────────────────────┬──────────────────────────────────┘
                               │ REST  (Bearer token)
                               │
┌──────────────────────────────▼──────────────────────────────────┐
│                        Single Go Binary                          │
│                                                                  │
│  ┌───────────────────────┐   ┌──────────────────────────────┐   │
│  │    API Server          │   │      Monitor Runner           │   │
│  │   (chi router)         │   │  (goroutine per monitor)      │   │
│  │                        │   │                               │   │
│  │  /api/monitors  CRUD   │   │  ticker fires → dispatch      │   │
│  │  /api/status    read   │◄──│  check → write result to DB   │   │
│  │  /api/alerts    read   │   │  evaluate alert rules         │   │
│  │  /              React  │   │  fire notifications           │   │
│  └──────────┬────────────┘   └─────────────┬─────────────────┘   │
│             │                              │                      │
└─────────────┼──────────────────────────────┼──────────────────────┘
              │                              │
    ┌─────────▼──────────────────────────────▼──────┐
    │              PostgreSQL                        │
    │  monitors · check_results · alert_rules        │
    │  alert_events · notification_channels · tokens │
    └────────────────────────────────────────────────┘
              │
    ┌─────────▼──────────────┐
    │    HashiCorp Vault      │
    │  DB passwords · API     │
    │  keys · monitor creds   │
    └────────────────────────┘
```

### How credentials flow

- Monitor config in DB stores a **Vault path** (e.g. `secret/monitors/prod-postgres`) instead of a plaintext password.
- When the runner executes a check, it fetches the secret from Vault using that path.
- Vault token for the service is set via `VAULT_TOKEN` env var (or AppRole auth for production).
- The React UI sends credential fields to the API, which writes them to Vault and stores only the Vault path in the DB.

---

## Monitor Types

### Phase 1 — Core Checks
| Type | What It Checks | Key Config |
|------|---------------|------------|
| ICMP Ping | RTT, packet loss via `fping` | Host/IP, count, timeout |
| TCP Port | Port open/closed | Host, port, timeout |
| HTTP/HTTPS | Status code, response time, optional body match | URL, method, expected code, body pattern |
| PostgreSQL | Connection + optional query | Host, port, db, user → Vault path for password |
| MySQL/MariaDB | Connection | Host, port, db, user → Vault path |
| Redis | PING response | Host, port → Vault path for password |

### Phase 2 — Advanced Checks
| Type | What It Checks | Key Config |
|------|---------------|------------|
| SSL Certificate | Days until expiry | Host, port, warn/critical thresholds |
| DNS Resolution | Records present | Hostname, record type, expected value |
| SSH | Auth success | Host, port, user → Vault path for key/password |
| ClickHouse | Connection + query | Host, port, db, user → Vault path |
| Zabbix API | Host/item status via Zabbix API | Zabbix URL → Vault path for API token |
| Tailscale | Peer reachability via Tailscale API | Tailnet → Vault path for API key, peer IP/tag |
| Custom Script | Exit code / stdout match | Script path or inline shell command |
| SNMP | OID value | Host, community string, OID |

---

## PostgreSQL Schema

```sql
-- Monitor definitions
CREATE TABLE monitors (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name          TEXT NOT NULL,
    description   TEXT,
    monitor_type  TEXT NOT NULL,       -- 'ping', 'tcp', 'http', 'postgres', 'redis', etc.
    enabled       BOOLEAN NOT NULL DEFAULT true,
    interval_secs INT NOT NULL DEFAULT 60,
    timeout_secs  INT NOT NULL DEFAULT 10,
    config        JSONB NOT NULL,      -- type-specific: host, port, vault_path, url, etc.
                                       -- NEVER stores plaintext passwords; use vault_path
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Check results — time-series, one row per execution
CREATE TABLE check_results (
    id            BIGSERIAL PRIMARY KEY,
    monitor_id    UUID NOT NULL REFERENCES monitors(id) ON DELETE CASCADE,
    checked_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    status        TEXT NOT NULL,       -- 'up', 'down', 'degraded', 'unknown'
    latency_ms    DOUBLE PRECISION,    -- null if check failed before latency was measured
    detail        JSONB,              -- type-specific output (packet loss %, HTTP code, etc.)
    error_message TEXT
);

CREATE INDEX ON check_results (monitor_id, checked_at DESC);

-- Nightly cleanup: DELETE FROM check_results WHERE checked_at < now() - interval '30 days';

-- Notification channels
CREATE TABLE notification_channels (
    id        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name      TEXT NOT NULL,
    type      TEXT NOT NULL,           -- 'webhook', 'slack', 'email'
    config    JSONB NOT NULL           -- url / addresses / headers (no passwords here; Slack webhook URLs go to Vault)
);

-- Alert rules attached to a monitor
CREATE TABLE alert_rules (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    monitor_id      UUID NOT NULL REFERENCES monitors(id) ON DELETE CASCADE,
    condition       TEXT NOT NULL,     -- 'down', 'latency_gt', 'consecutive_failures'
    threshold       DOUBLE PRECISION,  -- ms for latency_gt; count for consecutive_failures
    channel_id      UUID REFERENCES notification_channels(id),
    enabled         BOOLEAN NOT NULL DEFAULT true
);

-- Alert event log
CREATE TABLE alert_events (
    id          BIGSERIAL PRIMARY KEY,
    monitor_id  UUID NOT NULL REFERENCES monitors(id),
    rule_id     UUID REFERENCES alert_rules(id),
    fired_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at TIMESTAMPTZ,
    detail      JSONB
);

-- API tokens for bearer auth
CREATE TABLE api_tokens (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,         -- label: 'frontend', 'grafana', 'third-party-app'
    token_hash  TEXT NOT NULL UNIQUE,  -- SHA-256 of the token; never store plaintext
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at  TIMESTAMPTZ,           -- null = no expiry
    enabled     BOOLEAN NOT NULL DEFAULT true
);
```

---

## API Endpoints

All endpoints (except `/` and `/api/health`) require `Authorization: Bearer <token>` header.

### Auth
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/health` | None | Liveness check — returns `{"status":"ok"}` |
| GET | `/api/status` | Token | Current status of ALL monitors (latest result per monitor) — primary third-party endpoint |

### Monitors
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/monitors` | List all monitors with current status |
| POST | `/api/monitors` | Create monitor (credentials written to Vault, path stored in DB) |
| GET | `/api/monitors/:id` | Get monitor config |
| PUT | `/api/monitors/:id` | Update monitor config |
| DELETE | `/api/monitors/:id` | Delete monitor (also deletes Vault secret) |
| PATCH | `/api/monitors/:id/toggle` | Enable / disable |
| POST | `/api/monitors/:id/check-now` | Trigger an immediate check, returns result |

### Results
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/monitors/:id/results` | Historical results (`?limit=100&from=&to=`) |
| GET | `/api/monitors/:id/status` | Latest result for one monitor |

### Alerts
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/alerts/active` | Currently open (unresolved) alert events |
| GET | `/api/alerts/history` | All past alert events (`?from=&to=`) |
| GET | `/api/notification-channels` | List notification channels |
| POST | `/api/notification-channels` | Create channel |
| PUT | `/api/notification-channels/:id` | Update channel |
| DELETE | `/api/notification-channels/:id` | Delete channel |

### Tokens
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/tokens` | List tokens (names only, never hashes) |
| POST | `/api/tokens` | Generate new token — returned once, then only hash stored |
| DELETE | `/api/tokens/:id` | Revoke token |

### Metadata
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/monitor-types` | Returns supported types + their config field schema (drives UI form) |

---

## React Frontend — Pages

### Dashboard (`/`)
- Grid of monitor cards: name, type, current status badge (UP / DOWN / DEGRADED), last checked timestamp, latency
- Summary bar: X up · Y down · Z unknown
- Auto-refresh every 30 s via polling `/api/status`

### Monitor Detail (`/monitors/:id`)
- Latency chart over time (last 24 h by default, adjustable)
- Recent check results table with status, latency, error message
- Alert events for this monitor

### Add / Edit Monitor (`/monitors/new`, `/monitors/:id/edit`)
- Step 1: pick monitor type from a card grid (Ping, TCP, HTTP, PostgreSQL, etc.)
- Step 2: type-specific form rendered from `/api/monitor-types` field schema
  - Credential fields (password, API key) sent to API, stored in Vault — never shown again (masked after save)
- Step 3: set schedule (interval), timeout, alert rules
- "Test Now" button → calls `POST /api/monitors/:id/check-now`, shows result inline

### Settings (`/settings`)
- Notification channels CRUD
- API token management (generate, list, revoke)

---

## Project Structure (Go)

```
cron-job-monitoring/
├── go.mod
├── go.sum
├── main.go                    # starts DB, Vault, API server, runner
├── config/
│   └── config.go              # env-var based config (DB_URL, VAULT_ADDR, PORT, etc.)
├── db/
│   ├── db.go                  # pgxpool setup
│   └── migrations/
│       └── 001_initial.sql
├── vault/
│   └── vault.go               # Vault client: Read/Write/Delete secret
├── models/
│   ├── monitor.go             # Monitor, MonitorConfig (typed per check type)
│   └── check_result.go
├── api/
│   ├── server.go              # chi router setup, middleware (auth, CORS, logger)
│   ├── middleware.go          # bearer token validation
│   ├── monitors.go            # CRUD handlers
│   ├── status.go              # /api/status, /api/monitors/:id/status
│   ├── alerts.go              # alert + notification channel handlers
│   └── tokens.go              # token management handlers
├── checks/
│   ├── check.go               # CheckResult type + Checker interface
│   ├── ping.go                # fping subprocess
│   ├── tcp.go
│   ├── http.go
│   ├── postgres.go
│   ├── mysql.go
│   ├── redis.go
│   ├── ssl.go
│   ├── dns.go
│   ├── ssh.go
│   ├── clickhouse.go
│   ├── tailscale.go
│   └── zabbix.go
├── runner/
│   ├── runner.go              # loads monitors from DB, starts/stops goroutines per monitor
│   └── alerter.go             # evaluates alert rules after each check, fires notifications
├── notifiers/
│   ├── webhook.go
│   ├── slack.go
│   └── email.go
└── frontend/                  # React app (Vite + React + Tailwind + shadcn/ui)
    ├── src/
    │   ├── pages/
    │   │   ├── Dashboard.tsx
    │   │   ├── MonitorDetail.tsx
    │   │   ├── MonitorForm.tsx
    │   │   └── Settings.tsx
    │   ├── components/
    │   └── api/               # Typed fetch wrappers (one function per endpoint)
    ├── package.json
    └── vite.config.ts
```

The Go binary embeds the `frontend/dist/` build output at compile time:

```go
//go:embed frontend/dist
var frontendFS embed.FS
```

---

## Implementation Phases

### Phase 1 — Foundation (Week 1–2)
1. DB schema + migration runner (golang-migrate)
2. Vault client (Read/Write/Delete at `secret/data/monitors/<id>`)
3. chi API: monitor CRUD + `/api/status` + bearer token middleware
4. Runner: goroutine-per-monitor with `time.Ticker`, dispatches checks, writes to DB
5. Checks: Ping (fping), TCP, HTTP, PostgreSQL
6. React: Dashboard, monitor list, create/edit form for the 4 types, "Test Now"
7. Embed frontend in binary
- **Verify:** Create a ping monitor → it fires every 60 s → status shows in dashboard → third-party GET `/api/status` with token returns correct JSON

### Phase 2 — More Check Types (Week 3)
8. Add checks: Redis, MySQL, SSL cert, DNS
9. React: dynamic form per type driven by `/api/monitor-types` schema
- **Verify:** Each type can be created, shows results, credentials stored in Vault not DB

### Phase 3 — Alerts (Week 4)
10. Alert rule evaluation in `alerter.go` after every check write
11. Notifiers: webhook (POST JSON) → Slack → email
12. Nightly cleanup goroutine (DELETE check_results older than 30 days)
13. React: Alert history, active alerts badge, notification channel config
- **Verify:** Set a monitor DOWN → webhook fires within one check interval

### Phase 4 — Advanced Monitors (Week 5+)
14. ClickHouse check
15. Tailscale API check
16. Zabbix API check
17. SSH check
18. Custom script execution (subprocess with timeout, stdout match)
19. SNMP (optional, requires `snmpget` binary)

---

## Key Go Dependencies

```go
// go.mod — key packages

github.com/go-chi/chi/v5          // HTTP router
github.com/jackc/pgx/v5           // PostgreSQL driver + pgxpool
github.com/golang-migrate/migrate/v4 // DB migrations
github.com/hashicorp/vault/api    // Vault client
github.com/google/uuid            // UUID generation
encoding/json                     // stdlib — JSON encode/decode

// Checks
net/http                          // stdlib — HTTP/HTTPS checks
net                               // stdlib — TCP dial
os/exec                           // stdlib — fping subprocess
crypto/tls                        // stdlib — SSL cert inspection
net                               // stdlib — DNS lookup (net.LookupHost etc.)
github.com/go-redis/redis/v9      // Redis PING check
github.com/go-sql-driver/mysql    // MySQL check
github.com/ClickHouse/clickhouse-go/v2 // ClickHouse check
golang.org/x/crypto/ssh           // SSH check

// Auth
crypto/sha256                     // stdlib — token hashing
crypto/subtle                     // stdlib — constant-time compare

// Frontend embed
embed                             // stdlib — embed frontend/dist

// Observability
log/slog                          // stdlib — structured logging (Go 1.21+)
```

No heavy frameworks. `net/http` + `chi` for routing; stdlib for most checks; Vault SDK for secrets.

---

## Environment Variables

```env
PORT=8080
DATABASE_URL=postgres://user:pass@localhost:5432/monitoring?sslmode=disable
VAULT_ADDR=http://vault:8200
VAULT_ROLE_ID=<role-id>        # AppRole role_id (non-secret, safe to bake into config)
VAULT_SECRET_ID=<secret-id>    # AppRole secret_id (treat like a password — inject at runtime)
VAULT_MOUNT=secret             # KV v2 mount path
INITIAL_API_TOKEN=             # if set, seeded into api_tokens on first startup
LOG_LEVEL=info
```

### AppRole Auth Flow

The service **never holds a long-lived Vault root token**. On startup:

```
1. POST /v1/auth/approle/login  { role_id, secret_id }
         ↓
2. Vault returns a short-lived client token  (TTL e.g. 1 h)
         ↓
3. Service uses that token for all secret reads/writes
         ↓
4. Background goroutine watches token TTL, re-authenticates before expiry
```

**Vault side setup (one-time, done by ops):**

```hcl
# Policy: monitoring-svc can only read/write/delete under secret/monitors/*
path "secret/data/monitors/*" {
  capabilities = ["create", "read", "update", "delete"]
}
path "secret/metadata/monitors/*" {
  capabilities = ["list", "delete"]
}

# Enable AppRole and create role
vault auth enable approle
vault policy write monitoring-svc monitoring-svc.hcl
vault write auth/approle/role/monitoring-svc \
    token_policies="monitoring-svc" \
    token_ttl=1h \
    token_max_ttl=4h \
    secret_id_ttl=0   # non-expiring secret_id; rotate manually

# Get role_id and secret_id to inject into service config
vault read auth/approle/role/monitoring-svc/role-id
vault write -f auth/approle/role/monitoring-svc/secret-id
```

`VAULT_ROLE_ID` is non-sensitive (it's a stable identifier) and can live in a config file or K8s ConfigMap. `VAULT_SECRET_ID` is the secret — inject it via K8s Secret, Docker secret, or CI/CD secret store.

---

## Vault Secret Layout

```
secret/
└── monitors/
    └── {monitor-uuid}/
        └── data:
              password: "..."       # for DB checks
              api_key: "..."        # for Tailscale / Zabbix
              ssh_key: "..."        # for SSH checks
```

The `monitors.config` JSONB column stores `"vault_path": "secret/data/monitors/{uuid}"`. The runner calls `vault.Read(path)` before each check execution.

---

## Docker Deployment

```dockerfile
# Build frontend
FROM node:20-alpine AS ui
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# Build Go binary
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=ui /app/frontend/dist ./frontend/dist
RUN go build -o monitoring-svc .

# Final image
FROM alpine:3.19
RUN apk add --no-cache fping ca-certificates
COPY --from=builder /app/monitoring-svc /usr/local/bin/
EXPOSE 8080
ENTRYPOINT ["monitoring-svc"]
```

`fping` must be in the runtime image — the ping check shells out to it. No raw socket privileges needed.
