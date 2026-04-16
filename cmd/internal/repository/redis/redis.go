package redis

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type Client struct {
	rdb *redis.Client
}

func NewClient(ctx context.Context, config Config) (*Client, error) {
	// TODO: read timeouts from .yml
	rdb := redis.NewClient(&redis.Options{
		Addr:         config.Addr,
		Password:     config.Password,
		DB:           config.DB,
		Username:     config.User,
		DialTimeout:  10 * time.Second,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
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
