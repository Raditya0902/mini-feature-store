package online

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisStore struct {
	client *redis.Client
}

type Snapshot struct {
	Features         map[string]any `json:"features"`
	FeatureTimestamp *time.Time     `json:"feature_timestamp"`
}

// The prefix distinguishes snapshots from legacy JSON feature maps, whatever
// feature names those maps contain.
const snapshotPrefix = "snapshot:v1:"

func NewRedisStore(addr string) *RedisStore {
	return &RedisStore{
		client: redis.NewClient(&redis.Options{Addr: addr}),
	}
}

func featureKey(entityID string) string {
	return "features:" + entityID
}

func (s *RedisStore) Set(entityID string, features map[string]any) error {
	return s.set(entityID, "", features)
}

func (s *RedisStore) SetSnapshot(entityID string, timestamp time.Time, features map[string]any) error {
	timestamp = timestamp.UTC()
	return s.set(entityID, snapshotPrefix, Snapshot{Features: features, FeatureTimestamp: &timestamp})
}

func (s *RedisStore) set(entityID, prefix string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encoding features: %w", err)
	}
	if err := s.client.Set(context.Background(), featureKey(entityID), append([]byte(prefix), data...), 0).Err(); err != nil {
		return fmt.Errorf("redis SET: %w", err)
	}
	return nil
}

func (s *RedisStore) Get(entityID string) (map[string]any, error) {
	snapshot, err := s.GetSnapshot(entityID)
	return snapshot.Features, err
}

func (s *RedisStore) GetSnapshot(entityID string) (Snapshot, error) {
	data, err := s.client.Get(context.Background(), featureKey(entityID)).Bytes()
	if errors.Is(err, redis.Nil) {
		return Snapshot{Features: map[string]any{}}, nil
	}
	if err != nil {
		return Snapshot{}, fmt.Errorf("redis GET: %w", err)
	}
	snapshot, err := decodeSnapshot(data)
	if err != nil {
		return Snapshot{}, fmt.Errorf("decoding features: %w", err)
	}
	return snapshot, nil
}

func decodeSnapshot(data []byte) (Snapshot, error) {
	var snapshot Snapshot
	if value, ok := bytes.CutPrefix(data, []byte(snapshotPrefix)); ok {
		err := json.Unmarshal(value, &snapshot)
		return snapshot, err
	}
	err := json.Unmarshal(data, &snapshot.Features)
	return snapshot, err
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
		snapshot, err := decodeSnapshot([]byte(val.(string)))
		if err != nil {
			return nil, fmt.Errorf("decoding features for %q: %w", entityIDs[i], err)
		}
		result[entityIDs[i]] = snapshot.Features
	}
	return result, nil
}
