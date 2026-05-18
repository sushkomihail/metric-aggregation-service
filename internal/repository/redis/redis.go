package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sushkomihail/metric-aggregation-service/internal/config"
	"github.com/sushkomihail/metric-aggregation-service/internal/logger"
	"github.com/sushkomihail/metric-aggregation-service/pkg/metrics"
)

type Client struct {
	rdb *redis.Client
	log *logger.Logger
}

func NewClient(ctx context.Context, config config.RedisConfig, log *logger.Logger) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:         config.Addr,
		Password:     config.Password,
		Username:     config.User,
		DB:           config.DB,
		MaxRetries:   config.MaxRetries,
		DialTimeout:  config.DialTimeout,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
	})

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return &Client{
		rdb: rdb,
		log: log,
	}, nil
}

func (c *Client) Close() error {
	return c.rdb.Close()
}

func (c *Client) HSet(ctx context.Context, key string, value interface{}, expireTime time.Duration) error {
	err := c.rdb.HSet(ctx, key, value).Err()
	if err != nil {
		return fmt.Errorf("failed to set hash field: %w", err)
	}

	if err = c.rdb.Expire(ctx, key, expireTime).Err(); err != nil {
		return fmt.Errorf("failed to set expiry: %w", err)
	}

	return nil
}

func (c *Client) SendMemoryInfo(ctx context.Context, interval time.Duration) error {
	timer := time.NewTicker(interval)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context canceled: %w", ctx.Err())
		case <-timer.C:
			res, err := c.rdb.Info(ctx, "memory").Result()
			if err != nil {
				c.log.Error("Failed to get memory info", "error", err)
				continue
			}

			metrics.ObserveRedisUsedMemory(res)
		}
	}
}

func (c *Client) HGetAll(ctx context.Context, key string, dest interface{}) error {
	err := c.rdb.HGetAll(ctx, key).Scan(dest)
	if err != nil {
		return fmt.Errorf("failed to get hash: %w", err)
	}

	return nil
}

func (c *Client) ZAddBatch(ctx context.Context, key string, members []interface{}) error {
	pipe := c.rdb.Pipeline()

	for _, member := range members {
		bytes, err := json.Marshal(member)
		if err != nil {
			return fmt.Errorf("failed to marshal: %w", err)
		}

		score := float64(time.Now().Unix())

		pipe.ZAdd(ctx, key, redis.Z{
			Score:  score,
			Member: bytes,
		})
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("pipeline failed: %w", err)
	}

	return nil
}

func (c *Client) ZAddWithUnixScore(ctx context.Context, key string, member interface{}, expireTime time.Duration) error {
	bytes, err := json.Marshal(member)
	if err != nil {
		return fmt.Errorf("failed to marshal member: %w", err)
	}

	err = c.rdb.ZAdd(ctx, key, redis.Z{
		Score:  float64(time.Now().Unix()),
		Member: bytes,
	}).Err()
	if err != nil {
		return fmt.Errorf("failed to add member: %w", err)
	}

	if err = c.rdb.Expire(ctx, key, expireTime).Err(); err != nil {
		return fmt.Errorf("failed to set expiry: %w", err)
	}

	return nil
}

func (c *Client) ZRangeByUnixScore(ctx context.Context, key string, start, end time.Time) ([]string, error) {
	results, err := c.rdb.ZRangeArgs(ctx, redis.ZRangeArgs{
		Key:     key,
		Start:   start.Unix(),
		Stop:    end.Unix(),
		ByScore: true,
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to zrange: %w", err)
	}

	return results, nil
}
