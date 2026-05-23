# Tasks: Phase 1 — Registry

## Setup
- [x] `go get gopkg.in/yaml.v3`

## Implementation
- [x] `internal/registry/model.go` — Entity, Feature, FeatureView, Registry structs + Duration type
- [x] `internal/registry/loader.go` — Load(path) function
- [x] `internal/registry/validator.go` — Validate(reg) function
- [x] `configs/feature_registry.yaml` — sample config (driver entity + driver_stats feature view)
- [x] `tests/registry_test.go` — TestLoad + TestValidate (table-driven)

## Verification
- [x] `go build ./internal/registry/...`
- [x] `go test ./tests/ -run "TestLoad|TestValidate" -v` — 15/15 subtests pass
- [ ] `go test ./...` — no regressions
