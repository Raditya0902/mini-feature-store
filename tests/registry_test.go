package tests

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Raditya0902/mini-feature-store/internal/registry"
)

func writeTempYAML(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "registry-*.yaml")
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	defer f.Close()
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}
	return f.Name()
}

func TestLoad(t *testing.T) {
	cases := []struct {
		name    string
		setup   func(t *testing.T) string
		wantErr string
	}{
		{
			name:    "valid sample config",
			setup:   func(t *testing.T) string { return "../configs/feature_registry.yaml" },
			wantErr: "",
		},
		{
			name:    "file not found",
			setup:   func(t *testing.T) string { return "testdata/nonexistent.yaml" },
			wantErr: "reading registry",
		},
		{
			name: "malformed yaml",
			setup: func(t *testing.T) string {
				return writeTempYAML(t, ": bad yaml")
			},
			wantErr: "parsing registry",
		},
		{
			name: "invalid ttl",
			setup: func(t *testing.T) string {
				return writeTempYAML(t, `
entities:
  - name: driver
    join_key: driver_id
feature_views:
  - name: driver_stats
    entity: driver
    source: data/driver_stats.parquet
    ttl: notaduration
    features:
      - name: trip_count
        dtype: int64
`)
			},
			wantErr: "invalid duration",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := tc.setup(t)
			reg, err := registry.Load(path)

			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("expected error containing %q, got %q", tc.wantErr, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if reg == nil {
				t.Fatal("expected non-nil registry")
			}
			if len(reg.Entities) == 0 || reg.Entities[0].Name != "driver" {
				t.Fatalf("expected entity name %q, got %v", "driver", reg.Entities)
			}
		})
	}
}

func validRegistry() *registry.Registry {
	return &registry.Registry{
		Entities: []registry.Entity{
			{Name: "driver", JoinKey: "driver_id"},
		},
		FeatureViews: []registry.FeatureView{
			{
				Name:   "driver_stats",
				Entity: "driver",
				Source: "data/driver_stats.parquet",
				TTL:    registry.Duration{Duration: 24 * time.Hour},
				Features: []registry.Feature{
					{Name: "trip_count", Dtype: "int64"},
				},
			},
		},
	}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*registry.Registry)
		wantErr string
	}{
		{
			name:    "valid registry",
			mutate:  func(r *registry.Registry) {},
			wantErr: "",
		},
		{
			name:    "empty entity name",
			mutate:  func(r *registry.Registry) { r.Entities[0].Name = "" },
			wantErr: "entity name",
		},
		{
			name:    "empty join_key",
			mutate:  func(r *registry.Registry) { r.Entities[0].JoinKey = "" },
			wantErr: "join_key",
		},
		{
			name: "duplicate entity names",
			mutate: func(r *registry.Registry) {
				r.Entities = append(r.Entities, registry.Entity{Name: "driver", JoinKey: "driver_id"})
			},
			wantErr: "duplicate entity",
		},
		{
			name:    "empty feature view name",
			mutate:  func(r *registry.Registry) { r.FeatureViews[0].Name = "" },
			wantErr: "feature view name",
		},
		{
			name:    "empty entity ref",
			mutate:  func(r *registry.Registry) { r.FeatureViews[0].Entity = "" },
			wantErr: "entity is required",
		},
		{
			name:    "empty source",
			mutate:  func(r *registry.Registry) { r.FeatureViews[0].Source = "" },
			wantErr: "source",
		},
		{
			name:    "zero ttl",
			mutate:  func(r *registry.Registry) { r.FeatureViews[0].TTL = registry.Duration{} },
			wantErr: "ttl",
		},
		{
			name:    "empty features list",
			mutate:  func(r *registry.Registry) { r.FeatureViews[0].Features = nil },
			wantErr: "features",
		},
		{
			name: "duplicate feature view names",
			mutate: func(r *registry.Registry) {
				r.FeatureViews = append(r.FeatureViews, r.FeatureViews[0])
			},
			wantErr: "duplicate feature view",
		},
		{
			name:    "unknown entity ref",
			mutate:  func(r *registry.Registry) { r.FeatureViews[0].Entity = "ghost" },
			wantErr: "unknown entity",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reg := validRegistry()
			tc.mutate(reg)

			err := registry.Validate(reg)

			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("expected error containing %q, got %q", tc.wantErr, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
