package redis

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/yuzvak/flashsale-service/internal/config"
)

type Connection struct {
	client *redis.Client
}

func NewConnection(cfg config.RedisConfig) (*Connection, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: 100, // Connection pool size
	})

	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return &Connection{
		client: client,
	}, nil
}

func (c *Connection) Close() error {
	return c.client.Close()
}

func (c *Connection) GetClient() *redis.Client {
	return c.client
}
