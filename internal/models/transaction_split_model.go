package models

import "time"

type TransactionSplit struct {
	ID                   int64
	TransactionID        int64
	UserID               int64
	Amount               int64
	Reason               string
	Completed            bool
	CreatedAt            time.Time
	TotalBillAmount      int64
	TransactionCreatedAt time.Time
	BillCreatedAt        time.Time
}
