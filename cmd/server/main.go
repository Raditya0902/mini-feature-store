package main

import (
	"log"
	"net/http"

	"github.com/Raditya0902/mini-feature-store/internal/offline"
	"github.com/Raditya0902/mini-feature-store/internal/online"
	"github.com/Raditya0902/mini-feature-store/internal/registry"
	"github.com/Raditya0902/mini-feature-store/internal/server"
)

const (
	registryPath = "configs/feature_registry.yaml"
	redisAddr    = "localhost:6379"
	parquetBase  = "."
	listenAddr   = ":8080"
)

func main() {
	reg, err := registry.Load(registryPath)
	if err != nil {
		log.Fatalf("loading registry: %v", err)
	}

	onlineStore := online.NewRedisStore(redisAddr)
	offlineStore := &offline.ParquetStore{BasePath: parquetBase}

	srv := server.NewServer(onlineStore, offlineStore, reg)
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	log.Printf("listening on %s", listenAddr)
	log.Fatal(http.ListenAndServe(listenAddr, mux))
}
