package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/Raditya0902/mini-feature-store/internal/online"
	"github.com/Raditya0902/mini-feature-store/internal/server"
)

const redisAddr = "localhost:6379"

func newTestStore(t *testing.T) *online.RedisStore {
	t.Helper()
	c := redis.NewClient(&redis.Options{Addr: redisAddr})
	defer c.Close()
	if err := c.Ping(context.Background()).Err(); err != nil {
		if os.Getenv("REQUIRE_REDIS") == "1" {
			t.Fatalf("Redis required at %s: %v", redisAddr, err)
		}
		t.Skipf("Redis unavailable at %s: %v", redisAddr, err)
	}
	return online.NewRedisStore(redisAddr)
}

func TestOnlineStore(t *testing.T) {
	store := newTestStore(t)

	t.Run("set and get round-trip", func(t *testing.T) {
		features := map[string]any{"avg_fare": float64(12.5), "zone": "city"}
		if err := store.Set("online-rt-e1", features); err != nil {
			t.Fatalf("Set: %v", err)
		}
		got, err := store.Get("online-rt-e1")
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if !reflect.DeepEqual(got, features) {
			t.Errorf("got %v, want %v", got, features)
		}
	})

	t.Run("get missing key", func(t *testing.T) {
		got, err := store.Get("online-never-set-xyzzy")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("expected empty map, got %v", got)
		}
	})

	t.Run("MGet mixed", func(t *testing.T) {
		features := map[string]any{"avg_fare": float64(18.0), "zone": "suburb"}
		if err := store.Set("online-mget-e1", features); err != nil {
			t.Fatalf("Set: %v", err)
		}
		if err := store.SetSnapshot("online-mget-e2", time.Unix(0, 0), features); err != nil {
			t.Fatalf("SetSnapshot: %v", err)
		}

		result, err := store.MGet([]string{"online-mget-e1", "online-mget-e2", "online-mget-e3-missing"})
		if err != nil {
			t.Fatalf("MGet: %v", err)
		}
		if !reflect.DeepEqual(result["online-mget-e1"], features) {
			t.Errorf("e1: got %v, want %v", result["online-mget-e1"], features)
		}
		if !reflect.DeepEqual(result["online-mget-e2"], features) {
			t.Errorf("e2: got %v, want %v", result["online-mget-e2"], features)
		}
		if result["online-mget-e3-missing"] != nil {
			t.Errorf("e3: expected nil, got %v", result["online-mget-e3-missing"])
		}
	})

	t.Run("overwrite", func(t *testing.T) {
		v1 := map[string]any{"avg_fare": float64(5.0)}
		v2 := map[string]any{"avg_fare": float64(99.0), "zone": "airport"}
		if err := store.Set("online-ow-e1", v1); err != nil {
			t.Fatalf("Set v1: %v", err)
		}
		if err := store.Set("online-ow-e1", v2); err != nil {
			t.Fatalf("Set v2: %v", err)
		}
		got, err := store.Get("online-ow-e1")
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if !reflect.DeepEqual(got, v2) {
			t.Errorf("got %v, want %v", got, v2)
		}
	})

	t.Run("online omits unavailable timestamps", func(t *testing.T) {
		legacy := map[string]any{"avg_fare": float64(12.5)}
		if err := store.Set("online-legacy", legacy); err != nil {
			t.Fatal(err)
		}
		mux := http.NewServeMux()
		server.NewServer(store, nil, nil).RegisterRoutes(mux)
		for entity, features := range map[string]map[string]any{
			"online-legacy":           legacy,
			"online-missing-snapshot": {},
		} {
			res := httptest.NewRecorder()
			mux.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/features/online?entity_id="+entity, nil))
			if res.Code != http.StatusOK {
				t.Fatalf("online request for %q: status %d, body %s", entity, res.Code, res.Body)
			}
			var got map[string]any
			if err := json.Unmarshal(res.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			want := map[string]any{"entity_id": entity, "features": features}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("got %v, want %v", got, want)
			}
		}
	})
}
