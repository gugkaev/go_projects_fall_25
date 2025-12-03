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
	defaultGatewayPort = "8080"
	maxUploadSizeMB    = 20
	maxUploadSizeB     = maxUploadSizeMB * 1024 * 1024
	contentTypeJSON    = "application/json"
)

func main() {
	_ = godotenv.Load()

	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentTypeJSON)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","service":"api-gateway"}`))
	})

	registerRoutes(r, logger)

	addr := ":" + getEnv("PORT", defaultGatewayPort)
	logger.WithField("addr", addr).Info("starting API Gateway")
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}


