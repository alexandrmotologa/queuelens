package client

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// ScanKeys iterates through the Redis keyspace matching a pattern without blocking.
// It stops after collecting maxKeys or when the scan completes.
func ScanKeys(ctx context.Context, rdb redis.UniversalClient, pattern string, maxKeys int64) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var matched []string
	var cursor uint64
	var count int64 = 250

	for {
		keys, nextCursor, err := rdb.Scan(ctx, cursor, pattern, count).Result()
		if err != nil {
			return nil, fmt.Errorf("scan error for pattern %s: %w", pattern, err)
		}

		matched = append(matched, keys...)
		if maxKeys > 0 && int64(len(matched)) >= maxKeys {
			matched = matched[:maxKeys]
			break
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	return matched, nil
}
