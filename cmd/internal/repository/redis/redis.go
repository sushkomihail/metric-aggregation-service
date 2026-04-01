package redis

import (
	"context"
	"log"

	"github.com/redis/go-redis/v9"
)

type Client struct {
	rdb *redis.Client
}

func NewClient(address, password string, db int) *Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:     address,
		Password: password,
		DB:       db,
	})
	return &Client{
		rdb: rdb,
	}
}

func (c *Client) Close() error {
	return c.rdb.Close()
}

func (c *Client) IncrCounter(ctx context.Context, key string) {
	result, err := c.rdb.Incr(ctx, key).Result()
	if err != nil {
		log.Fatalf("failed to incr counter '%s': %v", key, err)
	}

	log.Printf("incr counter '%s' result: %d", key, result)
}
