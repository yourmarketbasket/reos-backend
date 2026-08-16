package store

import (
	"context"
	"fmt"
	"os"

	"github.com/redis/go-redis/v9"
)

type RedisClient struct {
	Client *redis.Client
	ctx    context.Context
}

var Redis *RedisClient

func InitRedis() {
	redisURI := os.Getenv("REDIS_URI")
	if redisURI == "" {
		redisURI = "redis://localhost:6379"
	}

	opt, err := redis.ParseURL(redisURI)
	if err != nil {
		fmt.Printf("Failed to parse Redis URI '%s': %v. Operating without Redis.\n", redisURI, err)
		return
	}

	client := redis.NewClient(opt)
	ctx := context.Background()

	// Ping connection
	if err := client.Ping(ctx).Err(); err != nil {
		fmt.Printf("Failed to connect to Redis at %s: %v. Operating without Redis.\n", redisURI, err)
		return
	}

	Redis = &RedisClient{
		Client: client,
		ctx:    ctx,
	}
	fmt.Printf("Successfully connected to Redis at: %s!\n", redisURI)
}

func (r *RedisClient) Publish(channel string, message string) error {
	if r == nil || r.Client == nil {
		return fmt.Errorf("redis client uninitialized")
	}
	return r.Client.Publish(r.ctx, channel, message).Err()
}

func (r *RedisClient) Subscribe(channel string) *redis.PubSub {
	if r == nil || r.Client == nil {
		return nil
	}
	return r.Client.Subscribe(r.ctx, channel)
}
