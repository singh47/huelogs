package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"strconv"
	"strings"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

// Static files and templates are compiled into the binary at build time.
// This makes the deployed artifact fully self-contained — no external file dependencies.
//
//go:embed static templates
var embeddedFS embed.FS

var tmpl = template.Must(template.ParseFS(embeddedFS, "templates/*.html"))

const maxBatchIngest = 100 // max log entries per /api/ingest request

// staticFileServer serves embedded assets at /static/*.
func staticFileServer() http.Handler {
	sub, err := fs.Sub(embeddedFS, "static")
	if err != nil {
		panic(err)
	}
	return http.FileServer(http.FS(sub))
}

// ── Health ─────────────────────────────────────────────────────────────────

func (a *App) health(w http.ResponseWriter, _ *http.Request) {
	jsonOK(w, map[string]string{"status": "ok"}, http.StatusOK)
}

// ready checks both PostgreSQL and Redis, returning 503 if either is down.
func (a *App) ready(w http.ResponseWriter, r *http.Request) {
	dbOK := a.db.Ping(r.Context()) == nil
	redisOK := a.rdb.Ping(r.Context()).Err() == nil

	code := http.StatusOK
	if !dbOK || !redisOK {
		code = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{
		"status": statusStr(dbOK && redisOK),
		"db":     statusStr(dbOK),
		"redis":  statusStr(redisOK),
	})
}

func statusStr(ok bool) string {
	if ok {
		return "ok"
	}
	return "error"
}

// ── Auth ───────────────────────────────────────────────────────────────────

type loginData struct{ Error string }

func (a *App) loginPage(w http.ResponseWriter, _ *http.Request) {
	tmpl.ExecuteTemplate(w, "login.html", loginData{})
}

func (a *App) loginSubmit(w http.ResponseWriter, r *http.Request) {
	if r.FormValue("api_key") == a.apiKey {
		sess, _ := a.store.Get(r, "session")
		sess.Values["authenticated"] = true
		if err := sess.Save(r, w); err != nil {
			log.Error().Err(err).Msg("session save failed")
			jsonError(w, "internal server error", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	log.Warn().Str("ip", r.RemoteAddr).Msg("failed login attempt")
	tmpl.ExecuteTemplate(w, "login.html", loginData{Error: "Invalid API key"})
}

func (a *App) logout(w http.ResponseWriter, r *http.Request) {
	sess, _ := a.store.Get(r, "session")
	sess.Options.MaxAge = -1
	sess.Save(r, w)
	http.Redirect(w, r, "/login", http.StatusFound)
}

func (a *App) dashboard(w http.ResponseWriter, _ *http.Request) {
	tmpl.ExecuteTemplate(w, "index.html", nil)
}

// ── Log API ────────────────────────────────────────────────────────────────

func (a *App) getLogs(w http.ResponseWriter, r *http.Request) {
	limit, offset, err := parsePagination(r, 100, 500)
	if err != nil {
		jsonError(w, "limit and offset must be integers", http.StatusBadRequest)
		return
	}

	logs, err := a.db.GetLogs(r.Context(), limit, offset)
	if err != nil {
		log.Error().Err(err).Msg("get logs failed")
		jsonError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if logs == nil {
		logs = []Log{}
	}
	jsonOK(w, map[string]any{"logs": logs, "limit": limit, "offset": offset}, http.StatusOK)
}

// addLog accepts a single log entry, validates it, and pushes it onto the
// Redis Stream. The worker picks it up asynchronously and writes it to the DB.
// Returns 202 Accepted — the log is queued, not yet persisted.
func (a *App) addLog(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Message     string `json:"message"`
		ServiceName string `json:"service_name"`
		Level       string `json:"level"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "JSON body required", http.StatusBadRequest)
		return
	}

	req.Message = strings.TrimSpace(req.Message)
	if req.Message == "" {
		jsonError(w, "message is required", http.StatusBadRequest)
		return
	}

	req.Level = strings.ToUpper(strings.TrimSpace(req.Level))
	if !validLevels[req.Level] {
		req.Level = "INFO"
	}

	if err := a.pushToStream(r.Context(), []logEntry{{
		message:     req.Message,
		serviceName: req.ServiceName,
		level:       req.Level,
	}}); err != nil {
		log.Error().Err(err).Msg("redis push failed")
		jsonError(w, "failed to queue log", http.StatusServiceUnavailable)
		return
	}

	jsonOK(w, map[string]bool{"accepted": true}, http.StatusAccepted)
}

// ingest accepts a batch of log entries (up to maxBatchIngest) and pushes them
// all onto the Redis Stream in a single pipeline round trip.
// Returns 202 Accepted — the logs are queued, not yet persisted.
func (a *App) ingest(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Logs []struct {
			Message     string `json:"message"`
			ServiceName string `json:"service_name"`
			Level       string `json:"level"`
		} `json:"logs"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "JSON body required", http.StatusBadRequest)
		return
	}
	if len(req.Logs) == 0 {
		jsonError(w, "logs array must not be empty", http.StatusBadRequest)
		return
	}
	if len(req.Logs) > maxBatchIngest {
		jsonError(w, fmt.Sprintf("batch exceeds maximum of %d entries", maxBatchIngest), http.StatusBadRequest)
		return
	}

	// Validate and normalise — skip blank messages silently.
	entries := make([]logEntry, 0, len(req.Logs))
	for _, l := range req.Logs {
		msg := strings.TrimSpace(l.Message)
		if msg == "" {
			continue
		}
		level := strings.ToUpper(strings.TrimSpace(l.Level))
		if !validLevels[level] {
			level = "INFO"
		}
		entries = append(entries, logEntry{
			message:     msg,
			serviceName: l.ServiceName,
			level:       level,
		})
	}

	if len(entries) == 0 {
		jsonError(w, "no valid log entries in batch", http.StatusBadRequest)
		return
	}

	if err := a.pushToStream(r.Context(), entries); err != nil {
		log.Error().Err(err).Msg("redis pipeline failed")
		jsonError(w, "failed to queue logs", http.StatusServiceUnavailable)
		return
	}

	jsonOK(w, map[string]int{"accepted": len(entries)}, http.StatusAccepted)
}

// pushToStream writes one or more log entries to the Redis Stream using a
// pipeline so all XADDs are sent in a single network round trip.
func (a *App) pushToStream(ctx context.Context, entries []logEntry) error {
	pipe := a.rdb.Pipeline()
	for _, e := range entries {
		pipe.XAdd(ctx, &redis.XAddArgs{
			Stream: streamKey,
			MaxLen: streamMaxLen,
			Approx: true, // approximate trim is O(1) vs O(n) for exact
			Values: map[string]interface{}{
				"message":      e.message,
				"service_name": e.serviceName,
				"level":        e.level,
			},
		})
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (a *App) searchLogs(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		jsonError(w, "search query is required", http.StatusBadRequest)
		return
	}

	limit := clampInt(parseIntOr(r.URL.Query().Get("limit"), 200), 1, 500)

	logs, err := a.db.SearchLogs(r.Context(), q, limit)
	if err != nil {
		log.Error().Err(err).Msg("search logs failed")
		jsonError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if logs == nil {
		logs = []Log{}
	}
	jsonOK(w, map[string]any{"logs": logs}, http.StatusOK)
}

// ── Helpers ────────────────────────────────────────────────────────────────

func parsePagination(r *http.Request, defaultLimit, maxLimit int) (limit, offset int, err error) {
	limit = defaultLimit
	if l := r.URL.Query().Get("limit"); l != "" {
		if limit, err = strconv.Atoi(l); err != nil {
			return
		}
		limit = clampInt(limit, 1, maxLimit)
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if offset, err = strconv.Atoi(o); err != nil {
			return
		}
		if offset < 0 {
			offset = 0
		}
	}
	return
}

func parseIntOr(s string, fallback int) int {
	if v, err := strconv.Atoi(s); err == nil {
		return v
	}
	return fallback
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func jsonOK(w http.ResponseWriter, data any, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func jsonError(w http.ResponseWriter, msg string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
