package amqp

import (
	"context"
	"encoding/json"
	"log"

	"order-service/internal/db"
	"order-service/internal/websocket"

	"github.com/rabbitmq/amqp091-go"
)

type OrderUpdate struct {
	OrderID int64  `json:"order_id"`
	Status  string `json:"status"`
}

type OrderUpdatesConsumer struct {
	conn *amqp091.Connection
	db   *db.DB
	hub  *websocket.Hub
}

func NewOrderUpdatesConsumer(conn *amqp091.Connection, db *db.DB, hub *websocket.Hub) (*OrderUpdatesConsumer, error) {
	return &OrderUpdatesConsumer{
		conn: conn,
		db:   db,
		hub:  hub,
	}, nil
}

func (c *OrderUpdatesConsumer) Start(ctx context.Context) error {
	ch, err := c.conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	_, err = ch.QueueDeclare("order_updates", true, false, false, false, nil)
	if err != nil {
		return err
	}

	msgs, err := ch.Consume("order_updates", "", false, false, false, false, nil)
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case msg := <-msgs:
			c.handleMessage(ctx, msg)
			msg.Ack(false)
		}
	}
}

func (c *OrderUpdatesConsumer) handleMessage(ctx context.Context, msg amqp091.Delivery) {
	var update OrderUpdate
	if err := json.Unmarshal(msg.Body, &update); err != nil {
		log.Printf("Invalid order update message: %v", err)
		return
	}

	var newStatus string
	switch update.Status {
	case "succeeded":
		newStatus = "PAID"
	case "failed":
		newStatus = "FAILED"
	default:
		log.Printf("Unknown payment status: %s", update.Status)
		return
	}

	err := c.db.UpdateOrderStatusIfPending(ctx, update.OrderID, newStatus)
	if err != nil {
		log.Printf("Failed to update order %d status to %s: %v", update.OrderID, newStatus, err)
		return
	}

	log.Printf("Order %d updated to status: %s", update.OrderID, newStatus)
	
	c.hub.BroadcastOrderUpdate(update.OrderID, newStatus)
}
