package models

import "time"

type TransactionType string

const (
	TransactionTypeAdd      TransactionType = "add"
	TransactionTypeSubtract TransactionType = "subtract"
)

type Transaction struct {
	ID             int64
	MailID         int64
	Amount         int64
	CurrentBalance int64
	From           string
	To             string
	Description    string
	Type           string
	Timestamp      time.Time
	CreatedAt      time.Time
}

type TransactionSplit struct {
	ID            int64
	TransactionID int64
	Name          string
	Amount        int64
	CreatedAt     time.Time
}
