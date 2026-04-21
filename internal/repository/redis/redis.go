package redis

import (
	"context"

	"github.com/redis/go-redis/v9"
	"github.com/sushkomihail/metric-aggregation-service/internal/config"
)

type Client struct {
	rdb *redis.Client
}

func NewClient(ctx context.Context, config config.RedisConfig) (*Client, error) {
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
		return nil, err
	}

	return &Client{
		rdb: rdb,
	}, nil
}

func (c *Client) Close() error {
	return c.rdb.Close()
}

func (c *Client) HSet(ctx context.Context, key string, value interface{}) error {
	err := c.rdb.HSet(ctx, key, value).Err()
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) HGetAll(ctx context.Context, key string, dest interface{}) error {
	err := c.rdb.HGetAll(ctx, key).Scan(dest)
	if err != nil {
		return err
	}

	return nil
}
