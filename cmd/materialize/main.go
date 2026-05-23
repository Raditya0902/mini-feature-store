package main

import (
	"fmt"
	"os"

	"github.com/Raditya0902/mini-feature-store/internal/materialization"
	"github.com/Raditya0902/mini-feature-store/internal/offline"
	"github.com/Raditya0902/mini-feature-store/internal/online"
	"github.com/Raditya0902/mini-feature-store/internal/registry"
)

const (
	registryPath = "configs/feature_registry.yaml"
	rawDataPath  = "data/raw/taxi.csv"
	redisAddr    = "localhost:6379"
	parquetBase  = "."
)

func main() {
	reg, err := registry.Load(registryPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "materialize: loading registry: %v\n", err)
		os.Exit(1)
	}

	store := &offline.ParquetStore{BasePath: parquetBase}
	onlineStore := online.NewRedisStore(redisAddr)

	if err := materialization.Materialize(reg, store, onlineStore, rawDataPath); err != nil {
		fmt.Fprintf(os.Stderr, "materialize: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("materialization complete")
}
