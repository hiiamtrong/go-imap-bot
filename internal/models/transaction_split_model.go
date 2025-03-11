package models

import "time"

type TransactionSplit struct {
	ID            int64
	TransactionID int64
	UserID        int64
	Amount        int64
	CreatedAt     time.Time
}
