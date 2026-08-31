package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
	"oms/internal/config"
	"oms/internal/models"
)

const (
	ProductListCacheKey = "oms:products:all"
	LockPrefix          = "oms:lock:product:"
)

type RedisClient struct {
	client *redis.Client
}

func NewRedisClient(cfg *config.Config) (*RedisClient, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisHost,
		Password: cfg.RedisPass,
		DB:       0,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to ping Redis at %s: %w", cfg.RedisHost, err)
	}

	log.Println("Successfully connected to Redis")
	return &RedisClient{client: rdb}, nil
}

func (r *RedisClient) GetCachedProducts(ctx context.Context) ([]models.Product, error) {
	val, err := r.client.Get(ctx, ProductListCacheKey).Result()
	if err != nil {
		return nil, err
	}

	var products []models.Product
	if err := json.Unmarshal([]byte(val), &products); err != nil {
		return nil, err
	}

	return products, nil
}

func (r *RedisClient) SetCachedProducts(ctx context.Context, products []models.Product, ttl time.Duration) error {
	data, err := json.Marshal(products)
	if err != nil {
		return err
	}

	return r.client.Set(ctx, ProductListCacheKey, data, ttl).Err()
}

func (r *RedisClient) InvalidateProductCache(ctx context.Context) error {
	return r.client.Del(ctx, ProductListCacheKey).Err()
}

// AcquireLock attempts to set a distributed mutex key in Redis for a specific product ID
func (r *RedisClient) AcquireLock(ctx context.Context, productID int, ttl time.Duration) (bool, error) {
	key := fmt.Sprintf("%s%d", LockPrefix, productID)
	return r.client.SetNX(ctx, key, "locked", ttl).Result()
}

// ReleaseLock releases the distributed lock
func (r *RedisClient) ReleaseLock(ctx context.Context, productID int) error {
	key := fmt.Sprintf("%s%d", LockPrefix, productID)
	return r.client.Del(ctx, key).Err()
}
