package dto

import "time"

// Response wrappers
type Response struct {
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Message string      `json:"message,omitempty"`
}

// Transaction DTOs
type TransactionResponse struct {
	ID          int64           `json:"id"`
	MailID      int64           `json:"mail_id"`
	Amount      int64           `json:"amount"`
	Balance     int64           `json:"balance"`
	Type        string          `json:"type"`
	Description string          `json:"description"`
	Completed   bool            `json:"completed"`
	Timestamp   time.Time       `json:"timestamp"`
	Tags        []TagResponse   `json:"tags,omitempty"`
	Splits      []SplitResponse `json:"splits,omitempty"`
}

type CreateVirtualBillRequest struct {
	Amount      int64  `json:"amount" validate:"required,gt=0"`
	Description string `json:"description" validate:"required"`
}

// User DTOs
type UserResponse struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Whitelist bool      `json:"whitelist"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateUserRequest struct {
	Name      string `json:"name" validate:"required"`
	Email     string `json:"email" validate:"required,email"`
	Whitelist bool   `json:"whitelist"`
}

type UpdateUserRequest struct {
	Name      string `json:"name"`
	Email     string `json:"email" validate:"omitempty,email"`
	Whitelist *bool  `json:"whitelist"`
}

// Tag DTOs
type TagResponse struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type CreateTagRequest struct {
	Name string `json:"name" validate:"required"`
}

// Split DTOs
type SplitResponse struct {
	ID            int64         `json:"id"`
	TransactionID int64         `json:"transaction_id"`
	UserID        int64         `json:"user_id"`
	Amount        int64         `json:"amount"`
	Reason        string        `json:"reason,omitempty"`
	Completed     bool          `json:"completed"`
	User          *UserResponse `json:"user,omitempty"`
	SplitHash     string        `json:"split_hash,omitempty"`
}

type CreateSplitRequest struct {
	TransactionID int64              `json:"transaction_id" validate:"required"`
	Users         []SplitUserRequest `json:"users" validate:"required,dive"`
}

type SplitUserRequest struct {
	UserID int64   `json:"user_id" validate:"required"`
	Amount float64 `json:"amount" validate:"required,gt=0"`
	Reason string  `json:"reason"`
}

// Reminder DTOs
type SendReminderRequest struct {
	SplitIDs []int64 `json:"split_ids" validate:"required,min=1"`
	Angry    bool    `json:"angry"`
}

// Statistics DTOs
type StatisticsResponse struct {
	TotalSpent       int64             `json:"totalSpent"`
	TotalReceived    int64             `json:"totalReceived"`
	Balance          int64             `json:"balance"`
	TransactionCount int               `json:"transactionCount"`
	PendingSplits    int               `json:"pendingSplits"`
	CompletedSplits  int               `json:"completedSplits"`
	SpendingByMonth  []MonthlySpending `json:"spendingByMonth,omitempty"`
	SpendingByTag    []TagSpending     `json:"spendingByTag,omitempty"`
}

type MonthlySpending struct {
	Month  string `json:"month"`
	Amount int64  `json:"amount"`
	Count  int    `json:"count"`
}

type TagSpending struct {
	Tag    string `json:"tag"`
	Amount int64  `json:"amount"`
	Count  int    `json:"count"`
}

// UserSplitSummary DTOs
type UserSplitSummary struct {
	User        UserResponse    `json:"user"`
	Splits      []SplitResponse `json:"splits"`
	TotalAmount int64           `json:"total_amount"`
	BillCount   int             `json:"bill_count"`
}
