package outbox

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"order-service/internal/db"

	"github.com/rabbitmq/amqp091-go"
)

const (
	outboxPollInterval = 100 * time.Millisecond
	batchSize          = 10
	rabbitMQQueue      = "payment_requests"
)

type OutboxWorker struct {
	db       *db.DB
	amqpConn *amqp091.Connection
}

func NewOutboxWorker(db *db.DB, amqpConn *amqp091.Connection) *OutboxWorker {
	return &OutboxWorker{
		db:       db,
		amqpConn: amqpConn,
	}
}

func (w *OutboxWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(outboxPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.processBatch(ctx)
		}
	}
}

func (w *OutboxWorker) processBatch(ctx context.Context) {
	messages, err := w.db.GetUnprocessedOutboxMessages(ctx, batchSize)
	if err != nil {
		log.Printf("Failed to fetch outbox messages: %v", err)
		return
	}
	if len(messages) == 0 {
		return
	}

	ch, err := w.amqpConn.Channel()
	if err != nil {
		log.Printf("Failed to open RabbitMQ channel: %v", err)
		return
	}
	defer ch.Close()

	_, err = ch.QueueDeclare(rabbitMQQueue, true, false, false, false, nil)
	if err != nil {
		log.Printf("Failed to declare queue: %v", err)
		return
	}

	for _, msg := range messages {
		if msg.Payload == nil || len(msg.Payload) == 0 {
			log.Printf("Skipping message %d with empty payload", msg.ID)
			continue
		}

		body, err := json.Marshal(msg.Payload)
		if err != nil {
			log.Printf("Failed to marshal message payload %d: %v", msg.ID, err)
			continue
		}

		if len(body) == 0 {
			log.Printf("Skipping message %d with empty JSON body", msg.ID)
			continue
		}

		err = ch.PublishWithContext(
			ctx,
			"",
			rabbitMQQueue,
			false,
			false,
			amqp091.Publishing{
				DeliveryMode: amqp091.Persistent,
				ContentType:  "application/json",
				Body:         body,
			},
		)
		if err != nil {
			log.Printf("Failed to publish to RabbitMQ: %v", err)
			continue
		}

		if err := w.db.MarkOutboxMessageAsProcessed(ctx, msg.ID); err != nil {
			log.Printf("Failed to mark message %d as processed: %v", msg.ID, err)
		} else {
			log.Printf("Published and marked message %d as processed", msg.ID)
		}
	}
}
