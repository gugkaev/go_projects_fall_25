package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"order-service/internal/amqp"
	"order-service/internal/db"
	"order-service/internal/handlers"
	"order-service/internal/middleware"
	"order-service/internal/outbox"
	"order-service/internal/websocket"
	"strconv"
	"strings"

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
		port = "8080"
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

	wsHub := websocket.NewHub()
	go wsHub.Run()

	worker := outbox.NewOutboxWorker(database, amqpConn)
	go worker.Start(context.Background())

	orderConsumer, err := amqp.NewOrderUpdatesConsumer(amqpConn, database, wsHub)
	if err != nil {
		log.Fatal("Failed to create order updates consumer:", err)
	}
	go orderConsumer.Start(context.Background())

	mux := http.NewServeMux()
	mux.HandleFunc("POST /orders", middleware.CORS(handlers.CreateOrderHandler(database)))
	mux.HandleFunc("GET /orders", middleware.CORS(handlers.ListOrdersHandler(database)))
	mux.HandleFunc("GET /orders/{id}", middleware.CORS(handlers.GetOrderHandler(database)))
	mux.HandleFunc("OPTIONS /orders", middleware.CORS(func(w http.ResponseWriter, r *http.Request) {}))
	mux.HandleFunc("OPTIONS /orders/", middleware.CORS(func(w http.ResponseWriter, r *http.Request) {}))
	
	mux.HandleFunc("GET /ws/orders/{id}", func(w http.ResponseWriter, r *http.Request) {
		pathParts := strings.Split(r.URL.Path, "/")
		if len(pathParts) < 4 {
			http.Error(w, "Invalid path", http.StatusBadRequest)
			return
		}
		orderIDStr := pathParts[3]
		orderID, err := strconv.ParseInt(orderIDStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid order ID", http.StatusBadRequest)
			return
		}
		wsHub.HandleWebSocket(w, r, orderID)
	})

	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	done := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, syscall.SIGINT, syscall.SIGTERM)
		<-sigint

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("Server shutdown error: %v", err)
		}
		close(done)
	}()

	log.Printf("Orders Service listening on :%s", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal("Server failed:", err)
	}
	<-done
	log.Println("Orders Service stopped gracefully")
}
