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
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const (
	defaultPort    = "8082"
	contentTypeJSON = "application/json"
)

func main() {
	_ = godotenv.Load()

	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})

	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		dsn = "host=postgres user=postgres password=postgres dbname=antiplag sslmode=disable"
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect to db: %v", err)
	}

	if err := db.AutoMigrate(&Work{}); err != nil {
		log.Fatalf("failed to migrate db: %v", err)
	}

	service := NewAnalysisService(db, logger)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentTypeJSON)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","service":"file-analysis"}`))
	})

	r.Route("/works", func(r chi.Router) {
		r.Post("/", createWorkHandler(service, logger))
		r.Get("/{id}", getWorkHandler(service, logger))
		r.Get("/{id}/wordcloud", wordCloudHandler(service, logger))
	})

	addr := ":" + getEnv("PORT", defaultPort)
	logger.WithField("addr", addr).Info("starting File Analysis Service")
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


