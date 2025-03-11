package models

import "time"

type TransactionSplit struct {
	ID            int64
	TransactionID int64
	Name          string
	Amount        int64
	CreatedAt     time.Time
}
