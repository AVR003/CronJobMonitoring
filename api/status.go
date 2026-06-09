package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"monitoring/models"
)

type statusHandlers struct {
	pool *pgxpool.Pool
}

type monitorStatus struct {
	MonitorID   uuid.UUID          `json:"monitor_id"`
	Name        string             `json:"name"`
	MonitorType string             `json:"monitor_type"`
	Enabled     bool               `json:"enabled"`
	LastResult  *models.CheckResult `json:"last_result"`
}

func (h *statusHandlers) all(w http.ResponseWriter, r *http.Request) {
	rows, err := h.pool.Query(r.Context(), `
		SELECT m.id, m.name, m.monitor_type, m.enabled,
		       cr.id, cr.checked_at, cr.status, cr.latency_ms, cr.detail, cr.error_message
		FROM monitors m
		LEFT JOIN LATERAL (
			SELECT id, checked_at, status, latency_ms, detail, error_message
			FROM check_results
			WHERE monitor_id = m.id
			ORDER BY checked_at DESC LIMIT 1
		) cr ON true
		ORDER BY m.name
	`)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	statuses := make([]monitorStatus, 0)
	for rows.Next() {
		var s monitorStatus
		// nullable columns from the LATERAL join
		var crID *int64
		var crCheckedAt *time.Time
		var crStatus *string
		var crLatencyMs *float64
		var crDetail []byte
		var crErrMsg *string

		if err := rows.Scan(
			&s.MonitorID, &s.Name, &s.MonitorType, &s.Enabled,
			&crID, &crCheckedAt, &crStatus, &crLatencyMs, &crDetail, &crErrMsg,
		); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if crID != nil {
			cr := models.CheckResult{
				ID:        *crID,
				MonitorID: s.MonitorID,
				CheckedAt: *crCheckedAt,
				Status:    *crStatus,
				LatencyMs: crLatencyMs,
				Detail:    json.RawMessage(crDetail),
			}
			if crErrMsg != nil {
				cr.ErrorMessage = *crErrMsg
			}
			s.LastResult = &cr
		}
		statuses = append(statuses, s)
	}
	writeJSON(w, http.StatusOK, statuses)
}

func (h *statusHandlers) one(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	var cr models.CheckResult
	err = h.pool.QueryRow(r.Context(), `
		SELECT id, monitor_id, checked_at, status, latency_ms, detail, error_message
		FROM check_results WHERE monitor_id = $1
		ORDER BY checked_at DESC LIMIT 1
	`, id).Scan(&cr.ID, &cr.MonitorID, &cr.CheckedAt, &cr.Status,
		&cr.LatencyMs, &cr.Detail, &cr.ErrorMessage)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no results yet"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, cr)
}

func (h *statusHandlers) results(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	rows, err := h.pool.Query(r.Context(), `
		SELECT id, monitor_id, checked_at, status, latency_ms, detail, error_message
		FROM check_results WHERE monitor_id = $1
		ORDER BY checked_at DESC LIMIT 100
	`, id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	list := make([]models.CheckResult, 0)
	for rows.Next() {
		var cr models.CheckResult
		if err := rows.Scan(&cr.ID, &cr.MonitorID, &cr.CheckedAt, &cr.Status,
			&cr.LatencyMs, &cr.Detail, &cr.ErrorMessage); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		list = append(list, cr)
	}
	writeJSON(w, http.StatusOK, list)
}
