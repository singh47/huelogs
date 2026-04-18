package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

// Worker reads log entries from the Redis Stream in batches, writes them to
// PostgreSQL in a single batch, and broadcasts them over WebSocket.
//
// Using Redis Streams (not Pub/Sub) gives us:
//   - Persistence: messages survive a worker restart
//   - At-least-once delivery: unACKed entries stay in the PEL and are retried
//   - Consumer groups: multiple worker instances compete, each entry processed once
type Worker struct {
	rdb        *redis.Client
	db         *DB
	hub        *Hub
	consumerID string
}

// NewWorker returns a Worker with a unique consumer ID based on hostname+PID.
func NewWorker(rdb *redis.Client, db *DB, hub *Hub) *Worker {
	hostname, _ := os.Hostname()
	return &Worker{
		rdb:        rdb,
		db:         db,
		hub:        hub,
		consumerID: fmt.Sprintf("%s-%d", hostname, os.Getpid()),
	}
}

// Start runs the consumer loop until ctx is cancelled. Call in a goroutine.
func (w *Worker) Start(ctx context.Context) {
	// Create the consumer group once. MKSTREAM creates the stream if absent.
	// "0" = start from the beginning (so pending/unACKed entries on restart are retried).
	err := w.rdb.XGroupCreateMkStream(ctx, streamKey, consumerGroup, "0").Err()
	if err != nil && !isBusyGroup(err) {
		log.Fatal().Err(err).Msg("failed to create redis consumer group")
	}

	log.Info().Str("consumer", w.consumerID).Msg("worker started")

	for {
		// Exit cleanly when the server shuts down.
		select {
		case <-ctx.Done():
			log.Info().Msg("worker shutting down")
			return
		default:
		}

		streams, err := w.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    consumerGroup,
			Consumer: w.consumerID,
			Streams:  []string{streamKey, ">"},
			Count:    workerBatchSize,
			Block:    2 * time.Second, // block until a message arrives or timeout
		}).Result()

		if err == redis.Nil {
			// Timeout — no new messages, loop again.
			continue
		}
		if err != nil {
			if ctx.Err() != nil {
				return // context cancelled, exit cleanly
			}
			log.Error().Err(err).Msg("redis read error — retrying in 1s")
			time.Sleep(time.Second)
			continue
		}

		if len(streams) == 0 || len(streams[0].Messages) == 0 {
			continue
		}

		w.processBatch(ctx, streams[0].Messages)
	}
}

// processBatch converts a slice of stream messages into DB rows, broadcasts
// them over WebSocket, then ACKs the messages.
func (w *Worker) processBatch(ctx context.Context, msgs []redis.XMessage) {
	entries := make([]logEntry, 0, len(msgs))
	ids := make([]string, 0, len(msgs))

	for _, msg := range msgs {
		entries = append(entries, logEntry{
			message:     streamStr(msg.Values, "message"),
			serviceName: streamStr(msg.Values, "service_name"),
			level:       streamStr(msg.Values, "level"),
		})
		ids = append(ids, msg.ID)
	}

	logs, err := w.db.BatchInsertLogs(ctx, entries)
	if err != nil {
		// Do NOT ACK — messages remain in the PEL and will be retried.
		log.Error().Err(err).Int("count", len(entries)).Msg("batch insert failed — will retry")
		return
	}

	// Broadcast each inserted log to connected WebSocket clients.
	for _, l := range logs {
		if data, err := json.Marshal(l); err == nil {
			select {
			case w.hub.broadcast <- data:
			default:
				log.Warn().Msg("broadcast channel full — WebSocket push dropped")
			}
		}
	}

	// ACK only after a successful DB write. This is the at-least-once guarantee:
	// if the process crashes after insert but before ACK, the messages are
	// reprocessed on restart (duplicates are acceptable in a logging system).
	if err := w.rdb.XAck(ctx, streamKey, consumerGroup, ids...).Err(); err != nil {
		log.Warn().Err(err).Msg("failed to ACK messages — may be reprocessed")
	}

	log.Info().Int("count", len(logs)).Msg("batch written")
}

// streamStr safely extracts a string value from a stream message payload.
func streamStr(vals map[string]interface{}, key string) string {
	if v, ok := vals[key]; ok {
		return fmt.Sprintf("%v", v)
	}
	return ""
}

// isBusyGroup returns true when the error is a Redis BUSYGROUP response,
// which means the consumer group already exists (idempotent create is fine).
func isBusyGroup(err error) bool {
	return strings.Contains(err.Error(), "BUSYGROUP")
}
