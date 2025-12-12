package amqp

import (
	"context"
	"encoding/json"
	"errors"
	"log"

	"payments-service/internal/db"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/rabbitmq/amqp091-go"
)

type PaymentRequest struct {
	MessageID string  `json:"message_id"`
	OrderID   int64   `json:"order_id"`
	UserID    int64   `json:"user_id"`
	Amount    float64 `json:"amount"`
}

type PaymentResult struct {
	OrderID int64  `json:"order_id"`
	Status  string `json:"status"`
}

type Consumer struct {
	conn  *amqp091.Connection
	db    *db.DB
	pubCh *amqp091.Channel
}

func NewConsumer(conn *amqp091.Connection, db *db.DB) (*Consumer, error) {
	pubCh, err := conn.Channel()
	if err != nil {
		return nil, err
	}
	return &Consumer{
		conn:  conn,
		db:    db,
		pubCh: pubCh,
	}, nil
}

func (c *Consumer) Start(ctx context.Context) error {
	ch, err := c.conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	_, err = ch.QueueDeclare("payment_requests", true, false, false, false, nil)
	if err != nil {
		return err
	}

	msgs, err := ch.Consume("payment_requests", "", false, false, false, false, nil)
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case msg := <-msgs:
			if c.handleMessage(ctx, msg) {
				msg.Ack(false)
			} else {
				msg.Nack(false, true) 
			}
		}
	}
}

func (c *Consumer) handleMessage(ctx context.Context, msg amqp091.Delivery) bool {
	if len(msg.Body) == 0 {
		log.Printf("Received empty message, skipping")
		return true
	}

	var req PaymentRequest
	if err := json.Unmarshal(msg.Body, &req); err != nil {
		log.Printf("Invalid message format: %v, body: %s", err, string(msg.Body))
		return false
	}

	err := c.db.InsertInboxMessage(ctx, req.MessageID, msg.Body)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return true 
		}
		log.Printf("Failed to insert into inbox: %v", err)
		return false
	}

	success, err := c.db.DeductAtomic(ctx, req.UserID, req.OrderID, req.Amount)
	if err != nil {
		log.Printf("Deduction error: %v", err)
		return false
	}

	resultStatus := "failed"
	if success {
		resultStatus = "succeeded"
	}

	tx, err := c.db.BeginTx(ctx)
	if err != nil {
		log.Printf("Failed to start transaction: %v", err)
		return false
	}
	defer tx.Rollback(ctx)

	resultPayload := map[string]interface{}{
		"order_id": req.OrderID,
		"status":   resultStatus,
	}

	messageID := req.MessageID + "_result"
	if err := c.db.InsertOutboxTx(tx, messageID, resultPayload); err != nil {
		log.Printf("Failed to write to outbox: %v", err)
		return false
	}

	if err := tx.Commit(ctx); err != nil {
		log.Printf("Failed to commit transaction: %v", err)
		return false
	}

	return true
}

