package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
)

const (
	defaultPort      = "8081"
	defaultDataDir   = "/data/files"
	maxUploadSizeMB  = 20
	maxUploadSizeB   = maxUploadSizeMB * 1024 * 1024
	contentTypeJSON  = "application/json"
)

func main() {
	_ = godotenv.Load()

	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})

	dataDir := getEnv("DATA_DIR", defaultDataDir)
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		log.Fatalf("failed to create data dir: %v", err)
	}

	fs := NewFileStore(dataDir, logger)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentTypeJSON)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","service":"file-storage"}`))
	})

	r.Route("/files", func(r chi.Router) {
		r.Post("/", uploadHandler(fs, logger))
		r.Get("/{id}", downloadHandler(fs, logger))
	})

	addr := ":" + getEnv("PORT", defaultPort)
	logger.WithFields(logrus.Fields{
		"addr":    addr,
		"dataDir": dataDir,
	}).Info("starting File Storage Service")

	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("failed to start file-storage server: %v", err)
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}


