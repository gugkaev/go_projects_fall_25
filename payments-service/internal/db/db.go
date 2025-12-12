package db

import (
	"context"
	"encoding/json"
	"errors"

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

func (d *DB) Close() {
	d.pool.Close()
}

func (d *DB) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return d.pool.Begin(ctx)
}

func (d *DB) CreateAccount(ctx context.Context, userID string) error {
	_, err := d.pool.Exec(ctx,
		"INSERT INTO accounts (user_id) VALUES ($1) ON CONFLICT (user_id) DO NOTHING",
		userID,
	)
	return err
}

func (d *DB) Deposit(ctx context.Context, userID int64, amount float64) error {
	if amount <= 0 {
		return errors.New("amount must be positive")
	}
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	
	var currentVersion int
	err = tx.QueryRow(ctx,
		"SELECT version FROM accounts WHERE user_id = $1 FOR UPDATE",
		userID,
	).Scan(&currentVersion)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("account not found")
		}
		return err
	}
	
	_, err = tx.Exec(ctx,
		"UPDATE accounts SET balance = balance + $1, version = version + 1 WHERE user_id = $2 AND version = $3",
		amount, userID, currentVersion,
	)
	if err != nil {
		return err
	}
	
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}

func (d *DB) GetBalance(ctx context.Context, userID int64) (float64, error) {
	var balance float64
	err := d.pool.QueryRow(ctx,
		"SELECT balance FROM accounts WHERE user_id = $1",
		userID,
	).Scan(&balance)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, errors.New("account not found")
		}
		return 0, err
	}
	return balance, nil
}

func (d *DB) InsertInboxMessage(ctx context.Context, messageID string, payload []byte) error {
	_, err := d.pool.Exec(ctx,
		"INSERT INTO inbox (message_id, payload) VALUES ($1, $2)",
		messageID, payload,
	)
	return err
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

func (d *DB) DeductAtomic(ctx context.Context, userID int64, orderID int64, amount float64) (bool, error) {
	if amount <= 0 {
		return false, errors.New("amount must be positive")
	}
	
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	
	var existingDeduction int64
	err = tx.QueryRow(ctx,
		"SELECT id FROM deductions WHERE order_id = $1",
		orderID,
	).Scan(&existingDeduction)
	if err == nil {
		tx.Rollback(ctx)
		return true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return false, err
	}
	
	var currentVersion int
	err = tx.QueryRow(ctx,
		"SELECT version FROM accounts WHERE user_id = $1 FOR UPDATE",
		userID,
	).Scan(&currentVersion)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, errors.New("account not found")
		}
		return false, err
	}
	
	result, err := tx.Exec(ctx,
		"UPDATE accounts SET balance = balance - $1, version = version + 1 WHERE user_id = $2 AND balance >= $1 AND version = $3",
		amount, userID, currentVersion,
	)
	if err != nil {
		return false, err
	}
	
	if result.RowsAffected() == 0 {
		tx.Rollback(ctx)
		return false, nil
	}
	
	_, err = tx.Exec(ctx,
		"INSERT INTO deductions (order_id, user_id, amount) VALUES ($1, $2, $3)",
		orderID, userID, amount,
	)
	if err != nil {
		return false, err
	}
	
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	
	return true, nil
}
