package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/sessions"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// App holds shared application dependencies injected into every handler.
type App struct {
	db     *DB
	hub    *Hub
	store  *sessions.CookieStore
	apiKey string
	rdb    *redis.Client
}

func main() {
	// JSON structured logging — every line is a valid JSON object.
	zerolog.TimeFieldFormat = time.RFC3339
	log.Logger = zerolog.New(os.Stdout).With().Timestamp().Logger()

	// ── Config ──────────────────────────────────────────────────────────────

	apiKey := os.Getenv("LOGGER_API_KEY")
	if apiKey == "" {
		apiKey = randomHex(32)
		log.Warn().Msg("LOGGER_API_KEY not set — generated ephemeral key; set this env var in production")
	}

	secretKey := mustEnv("SECRET_KEY")
	dbURL := mustEnv("DATABASE_URL")
	redisURL := mustEnv("REDIS_URL")

	// ── Graceful shutdown context ─────────────────────────────────────────────
	// ctx is cancelled when SIGINT or SIGTERM is received.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// ── Database ─────────────────────────────────────────────────────────────

	db, err := NewDB(dbURL)
	if err != nil {
		log.Fatal().Err(err).Msg("database connection failed")
	}
	defer db.Close()

	if err := db.InitSchema(); err != nil {
		log.Fatal().Err(err).Msg("schema init failed")
	}
	log.Info().Msg("database ready")

	// ── Redis ─────────────────────────────────────────────────────────────────

	rdb, err := NewRedisClient(redisURL)
	if err != nil {
		log.Fatal().Err(err).Msg("redis config failed")
	}
	defer rdb.Close()

	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatal().Err(err).Msg("redis ping failed")
	}
	log.Info().Msg("redis ready")

	// ── WebSocket hub ─────────────────────────────────────────────────────────

	hub := NewHub()
	go hub.Run()

	// ── Background worker ─────────────────────────────────────────────────────
	// Reads from the Redis Stream, batch-inserts to PostgreSQL, and broadcasts
	// to WebSocket clients. Shuts down cleanly when ctx is cancelled.

	worker := NewWorker(rdb, db, hub)
	go worker.Start(ctx)

	// ── Sessions ──────────────────────────────────────────────────────────────

	secure := os.Getenv("SESSION_COOKIE_SECURE") != "false"
	store := sessions.NewCookieStore([]byte(secretKey))
	store.Options = &sessions.Options{
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
		MaxAge:   7 * 24 * 3600,
	}

	app := &App{db: db, hub: hub, store: store, apiKey: apiKey, rdb: rdb}

	// ── Router ────────────────────────────────────────────────────────────────

	r := chi.NewRouter()
	r.Use(chiMiddleware.RealIP)
	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.Logger)
	r.Use(chiMiddleware.Recoverer)

	// Static files are compiled into the binary via go:embed.
	r.Handle("/static/*", http.StripPrefix("/static/", staticFileServer()))

	// Public
	r.Get("/health", app.health)
	r.Get("/ready", app.ready)
	r.With(RateLimiter(20, 20)).Get("/login", app.loginPage)
	r.With(RateLimiter(20, 20)).Post("/login", app.loginSubmit)
	r.Get("/logout", app.logout)

	// Authenticated — dashboard and WebSocket
	r.With(app.RequireAuth).Get("/", app.dashboard)
	r.With(app.RequireAuth).Get("/ws", hub.ServeWS)

	// Authenticated — log API
	r.Route("/api", func(r chi.Router) {
		r.Use(app.RequireAuth)
		r.Get("/logs", app.getLogs)
		r.Get("/search-logs", app.searchLogs)

		// Single-entry ingestion (backward compatible, async)
		r.With(RateLimiter(1000, 100)).Post("/add-log", app.addLog)

		// Batch ingestion — up to 100 entries per request, single pipeline push
		r.With(RateLimiter(200, 50)).Post("/ingest", app.ingest)
	})

	// ── Server ────────────────────────────────────────────────────────────────

	port := os.Getenv("PORT")
	if port == "" {
		port = "5000"
	}

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	// Shut the HTTP server down when the signal context is cancelled.
	go func() {
		<-ctx.Done()
		log.Info().Msg("shutdown signal received — draining connections")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Error().Err(err).Msg("http shutdown error")
		}
	}()

	log.Info().Str("port", port).Msg("server listening")
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal().Err(err).Msg("server error")
	}
	log.Info().Msg("server stopped")
}

// mustEnv returns the env var value or exits with a clear error.
func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatal().Str("var", key).Msg("required environment variable not set")
	}
	return v
}

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}
