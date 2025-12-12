package db

import (
	"context"
	"encoding/json"
	"errors"
	"order-service/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
	pool *pgxpool.Pool
}

func Connect(connStr string) (*DB, error) {
	pool, err := pgxpool.New(context.Background(), connStr)
	if err != nil {
		return nil, err
	}
	return &DB{pool: pool}, nil
}

func (d *DB) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return d.pool.Begin(ctx)
}

func (d *DB) InsertOrderTx(tx pgx.Tx, userID string, amount float64) (int64, error) {
	var id int64
	err := tx.QueryRow(
		context.Background(),
		"INSERT INTO orders (user_id, amount, status) VALUES ($1, $2, 'PENDING') RETURNING id",
		userID, amount,
	).Scan(&id)
	return id, err
}

func (d *DB) InsertOutboxTx(tx pgx.Tx, messageID string, payload map[string]interface{}) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = tx.Exec(
		context.Background(),
		"INSERT INTO outbox (message_id, payload) VALUES ($1, $2)",
		messageID, jsonData,
	)
	return err
}

func (d *DB) GetOrder(ctx context.Context, orderID int64) (*models.Order, error) {
	var order models.Order
	err := d.pool.QueryRow(
		ctx,
		"SELECT id, user_id, amount, status, created_at FROM orders WHERE id = $1",
		orderID,
	).Scan(&order.ID, &order.UserID, &order.Amount, &order.Status, &order.Created)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("order not found")
		}
		return nil, err
	}
	return &order, nil
}

func (d *DB) ListOrders(ctx context.Context, userID string) ([]models.Order, error) {
	rows, err := d.pool.Query(
		ctx,
		"SELECT id, user_id, amount, status, created_at FROM orders WHERE user_id = $1 ORDER BY created_at DESC",
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []models.Order
	for rows.Next() {
		var order models.Order
		err := rows.Scan(&order.ID, &order.UserID, &order.Amount, &order.Status, &order.Created)
		if err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}
	if orders == nil {
		orders = []models.Order{}
	}
	return orders, nil
}

type OutboxMessage struct {
	ID        int64
	MessageID string
	Payload   map[string]interface{}
}

func (d *DB) GetUnprocessedOutboxMessages(ctx context.Context, limit int) ([]OutboxMessage, error) {
	rows, err := d.pool.Query(
		ctx,
		"SELECT id, message_id, payload FROM outbox WHERE processed = false ORDER BY id LIMIT $1",
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []OutboxMessage
	for rows.Next() {
		var msg OutboxMessage
		var payloadJSON []byte
		if err := rows.Scan(&msg.ID, &msg.MessageID, &payloadJSON); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(payloadJSON, &msg.Payload); err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}
	return messages, nil
}

func (d *DB) MarkOutboxMessageAsProcessed(ctx context.Context, id int64) error {
	_, err := d.pool.Exec(ctx, "UPDATE outbox SET processed = true WHERE id = $1", id)
	return err
}

func (d *DB) UpdateOrderStatusIfPending(ctx context.Context, orderID int64, newStatus string) error {
	_, err := d.pool.Exec(ctx,
		"UPDATE orders SET status = $1 WHERE id = $2 AND status = 'PENDING'",
		newStatus, orderID,
	)
	return err
}

func (d *DB) Close() {
	d.pool.Close()
}
