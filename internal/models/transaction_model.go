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
	Completed      bool
	Timestamp      time.Time
	CreatedAt      time.Time
}
