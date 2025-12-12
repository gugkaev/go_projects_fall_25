package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"payments-service/internal/amqp"
	"payments-service/internal/db"
	"payments-service/internal/handlers"
	"payments-service/internal/middleware"
	"payments-service/internal/outbox"

	"github.com/rabbitmq/amqp091-go"
)

func main() {
	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		log.Fatal("DB_URL is required")
	}
	rabbitURL := os.Getenv("RABBITMQ_URL")
	if rabbitURL == "" {
		log.Fatal("RABBITMQ_URL is required")
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	database, err := db.Connect(dbURL)
	if err != nil {
		log.Fatal("Failed to connect to DB:", err)
	}
	defer database.Close()

	amqpConn, err := amqp091.Dial(rabbitURL)
	if err != nil {
		log.Fatal("Failed to connect to RabbitMQ:", err)
	}
	defer amqpConn.Close()

	paymentConsumer, err := amqp.NewConsumer(amqpConn, database)
	if err != nil {
		log.Fatal("Failed to create payment consumer:", err)
	}
	go paymentConsumer.Start(context.Background())

	outboxWorker := outbox.NewOutboxWorker(database, amqpConn)
	go outboxWorker.Start(context.Background())

	mux := http.NewServeMux()
	mux.HandleFunc("POST /accounts", middleware.CORS(handlers.CreateAccountHandler(database)))
	mux.HandleFunc("POST /accounts/{user_id}/topup", middleware.CORS(handlers.TopupHandler(database)))
	mux.HandleFunc("GET /accounts/{user_id}/balance", middleware.CORS(handlers.GetBalanceHandler(database)))
	mux.HandleFunc("OPTIONS /accounts", middleware.CORS(func(w http.ResponseWriter, r *http.Request) {}))
	mux.HandleFunc("OPTIONS /accounts/", middleware.CORS(func(w http.ResponseWriter, r *http.Request) {}))

	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	done := make(chan bool, 1)
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, syscall.SIGINT, syscall.SIGTERM)
		<-sigint

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(ctx)
		done <- true
	}()

	log.Printf("Payments Service listening on :%s", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal("Server failed:", err)
	}
	<-done
	log.Println("Payments Service stopped")
}
