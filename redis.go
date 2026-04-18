package main

import (
	"fmt"

	"github.com/redis/go-redis/v9"
)

const (
	// streamKey is the Redis Stream that ingestion endpoints write to and the
	// worker reads from.
	streamKey = "huelogs:logs"

	// consumerGroup is the Redis consumer group name. All worker instances join
	// this group so each stream entry is processed by exactly one worker.
	consumerGroup = "db-writers"

	// streamMaxLen caps the stream length (approximate trim — very efficient).
	// Acts as a buffer ring: old ACKed entries are evicted automatically.
	streamMaxLen = 10_000

	// workerBatchSize is the max number of entries read per XReadGroup call.
	workerBatchSize = 50
)

// NewRedisClient parses the REDIS_URL and returns a configured client.
func NewRedisClient(url string) (*redis.Client, error) {
	opt, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}
	return redis.NewClient(opt), nil
}
