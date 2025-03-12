package models

import "time"

type Tag struct {
	ID        int64
	Name      string
	CreatedAt time.Time
}

type TransactionTag struct {
	TransactionID int64
	TagID         int64
	CreatedAt     time.Time
}
