package models

import "time"

type Order struct {
	ID      int64     `json:"id"`
	UserID  string    `json:"user_id"`
	Amount  float64   `json:"amount"`
	Status  string    `json:"status"`
	Created time.Time `json:"created"`
}
