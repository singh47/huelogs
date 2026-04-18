package main

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

// DB wraps a pgxpool connection pool.
type DB struct {
	pool *pgxpool.Pool
}

// Log is a single log entry returned by queries and emitted over WebSocket.
type Log struct {
	ID          int64     `json:"id"`
	Timestamp   time.Time `json:"timestamp"`
	Message     string    `json:"message"`
	ServiceName *string   `json:"service_name"` // nullable
	Level       string    `json:"level"`
}

var validLevels = map[string]bool{
	"DEBUG": true, "INFO": true, "WARNING": true, "ERROR": true, "CRITICAL": true,
}

// logEntry is a validated-but-unpersisted log entry read from the Redis stream.
type logEntry struct {
	message     string
	serviceName string
	level       string
}

// NewDB creates a connection pool and verifies connectivity.
func NewDB(url string) (*DB, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	cfg.MinConns = 2
	cfg.MaxConns = 10

	// ConnectConfig creates the pool and establishes initial connections.
	pool, err := pgxpool.ConnectConfig(context.Background(), cfg)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	return &DB{pool: pool}, nil
}

// Close releases all pool connections.
func (db *DB) Close() { db.pool.Close() }

// Ping verifies the database is reachable.
func (db *DB) Ping(ctx context.Context) error {
	_, err := db.pool.Exec(ctx, "SELECT 1")
	return err
}

// InitSchema creates the TimescaleDB extension, the logs hypertable, and indexes.
// Each Exec runs in its own implicit transaction (autocommit), which is what
// TimescaleDB requires for CREATE EXTENSION and create_hypertable.
func (db *DB) InitSchema() error {
	ctx := context.Background()

	steps := []struct {
		name string
		sql  string
	}{
		{
			"create extension",
			"CREATE EXTENSION IF NOT EXISTS timescaledb CASCADE",
		},
		{
			"create table",
			`CREATE TABLE IF NOT EXISTS logs (
				id           BIGSERIAL    NOT NULL,
				timestamp    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
				message      TEXT         NOT NULL,
				service_name TEXT,
				level        TEXT         NOT NULL DEFAULT 'INFO',
				PRIMARY KEY  (id, timestamp)
			)`,
		},
		{
			"create hypertable",
			"SELECT create_hypertable('logs', 'timestamp', if_not_exists => TRUE)",
		},
		{
			"index service",
			"CREATE INDEX IF NOT EXISTS idx_logs_service ON logs (service_name, timestamp DESC)",
		},
		{
			"index level",
			"CREATE INDEX IF NOT EXISTS idx_logs_level ON logs (level, timestamp DESC)",
		},
	}

	for _, s := range steps {
		if _, err := db.pool.Exec(ctx, s.sql); err != nil {
			return fmt.Errorf("%s: %w", s.name, err)
		}
	}
	return nil
}

// InsertLog persists a new entry and returns the full record (including DB-generated timestamp).
func (db *DB) InsertLog(ctx context.Context, message, serviceName, level string) (Log, error) {
	var sn *string
	if serviceName != "" {
		sn = &serviceName
	}

	var l Log
	err := db.pool.QueryRow(ctx,
		`INSERT INTO logs (message, service_name, level)
		 VALUES ($1, $2, $3)
		 RETURNING id, timestamp, message, service_name, level`,
		message, sn, level,
	).Scan(&l.ID, &l.Timestamp, &l.Message, &l.ServiceName, &l.Level)
	return l, err
}

// GetLogs returns entries ordered newest-first with limit/offset pagination.
func (db *DB) GetLogs(ctx context.Context, limit, offset int) ([]Log, error) {
	rows, err := db.pool.Query(ctx,
		`SELECT id, timestamp, message, service_name, level
		 FROM logs
		 ORDER BY timestamp DESC
		 LIMIT $1 OFFSET $2`,
		limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectLogs(rows)
}

// SearchLogs returns entries whose message or service_name matches the query.
func (db *DB) SearchLogs(ctx context.Context, query string, limit int) ([]Log, error) {
	rows, err := db.pool.Query(ctx,
		`SELECT id, timestamp, message, service_name, level
		 FROM logs
		 WHERE message ILIKE $1 OR service_name ILIKE $1
		 ORDER BY timestamp DESC
		 LIMIT $2`,
		"%"+query+"%", limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectLogs(rows)
}

// BatchInsertLogs inserts multiple entries in a single PostgreSQL round trip
// using pgx SendBatch, then returns all inserted rows with their DB timestamps.
func (db *DB) BatchInsertLogs(ctx context.Context, entries []logEntry) ([]Log, error) {
	if len(entries) == 0 {
		return nil, nil
	}

	batch := &pgx.Batch{}
	for _, e := range entries {
		var sn *string
		if e.serviceName != "" {
			sn = &e.serviceName
		}
		batch.Queue(
			`INSERT INTO logs (message, service_name, level)
			 VALUES ($1, $2, $3)
			 RETURNING id, timestamp, message, service_name, level`,
			e.message, sn, e.level,
		)
	}

	br := db.pool.SendBatch(ctx, batch)
	defer br.Close()

	var logs []Log
	for range entries {
		var l Log
		if err := br.QueryRow().Scan(&l.ID, &l.Timestamp, &l.Message, &l.ServiceName, &l.Level); err != nil {
			return nil, fmt.Errorf("scan batch row: %w", err)
		}
		logs = append(logs, l)
	}
	return logs, nil
}

func collectLogs(rows pgx.Rows) ([]Log, error) {
	var logs []Log
	for rows.Next() {
		var l Log
		if err := rows.Scan(&l.ID, &l.Timestamp, &l.Message, &l.ServiceName, &l.Level); err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}
	return logs, rows.Err()
}
