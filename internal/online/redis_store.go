package online

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"
)

type RedisStore struct {
	client *redis.Client
}

func NewRedisStore(addr string) *RedisStore {
	return &RedisStore{
		client: redis.NewClient(&redis.Options{Addr: addr}),
	}
}

func featureKey(entityID string) string {
	return "features:" + entityID
}

func (s *RedisStore) Set(entityID string, features map[string]any) error {
	data, err := json.Marshal(features)
	if err != nil {
		return fmt.Errorf("encoding features: %w", err)
	}
	if err := s.client.Set(context.Background(), featureKey(entityID), data, 0).Err(); err != nil {
		return fmt.Errorf("redis SET: %w", err)
	}
	return nil
}

func (s *RedisStore) Get(entityID string) (map[string]any, error) {
	data, err := s.client.Get(context.Background(), featureKey(entityID)).Bytes()
	if errors.Is(err, redis.Nil) {
		return map[string]any{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("redis GET: %w", err)
	}
	var features map[string]any
	if err := json.Unmarshal(data, &features); err != nil {
		return nil, fmt.Errorf("decoding features: %w", err)
	}
	return features, nil
}

func (s *RedisStore) MGet(entityIDs []string) (map[string]map[string]any, error) {
	if len(entityIDs) == 0 {
		return map[string]map[string]any{}, nil
	}

	keys := make([]string, len(entityIDs))
	for i, id := range entityIDs {
		keys[i] = featureKey(id)
	}

	vals, err := s.client.MGet(context.Background(), keys...).Result()
	if err != nil {
		return nil, fmt.Errorf("redis MGET: %w", err)
	}

	result := make(map[string]map[string]any, len(entityIDs))
	for i, val := range vals {
		if val == nil {
			result[entityIDs[i]] = nil
			continue
		}
		var features map[string]any
		if err := json.Unmarshal([]byte(val.(string)), &features); err != nil {
			return nil, fmt.Errorf("decoding features for %q: %w", entityIDs[i], err)
		}
		result[entityIDs[i]] = features
	}
	return result, nil
}
