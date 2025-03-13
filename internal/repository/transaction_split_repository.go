package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/hiiamtrong/imap-bot-go/internal/database"
	"github.com/hiiamtrong/imap-bot-go/internal/models"
)

type TransactionSplitRepository struct {
	db *database.Database
}

func NewTransactionSplitRepository(db *database.Database) *TransactionSplitRepository {
	return &TransactionSplitRepository{db: db}
}

// Transaction executes a function within a database transaction
func (r *TransactionSplitRepository) Transaction(fn func(tx *sql.Tx) error) error {
	tx, err := r.db.Conn.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}

	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p) // re-throw panic after rollback
		}
	}()

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("error rolling back transaction: %v (original error: %v)", rbErr, err)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error committing transaction: %v", err)
	}

	return nil
}

func (r *TransactionSplitRepository) Create(split *models.TransactionSplit) error {
	query := `
	INSERT INTO transaction_splits (transaction_id, user_id, amount, created_at)
	VALUES (?, ?, ?, ?)
`
	result, err := r.db.Conn.Exec(
		query,
		split.TransactionID,
		split.UserID,
		split.Amount,
		split.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create split: %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %v", err)
	}

	split.ID = id
	return nil
}

func (r *TransactionSplitRepository) CreateTx(tx *sql.Tx, split *models.TransactionSplit) error {
	query := `
		INSERT INTO transaction_splits (
			transaction_id, user_id, amount, created_at
		)
		VALUES (?, ?, ?, ?)
	`
	result, err := tx.Exec(
		query,
		split.TransactionID,
		split.UserID,
		split.Amount,
		time.Now(),
	)
	if err != nil {
		return fmt.Errorf("failed to insert transaction split: %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %v", err)
	}

	split.ID = id
	return nil
}

func (r *TransactionSplitRepository) GetByTransactionID(transactionID int64) ([]*models.TransactionSplit, error) {
	query := `
		SELECT id, transaction_id, user_id, amount, created_at
		FROM transaction_splits
		WHERE transaction_id = ?
	`
	rows, err := r.db.Conn.Query(query, transactionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction splits: %v", err)
	}
	defer rows.Close()

	var splits []*models.TransactionSplit
	for rows.Next() {
		split := &models.TransactionSplit{}
		err := rows.Scan(
			&split.ID,
			&split.TransactionID,
			&split.UserID,
			&split.Amount,
			&split.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan transaction split: %v", err)
		}
		splits = append(splits, split)
	}

	return splits, nil
}

func (r *TransactionSplitRepository) Reset(ctx context.Context, transactionID int64) error {
	query := `
		DELETE FROM transaction_splits WHERE transaction_id = ?
	`
	_, err := r.db.Conn.Exec(query, transactionID)
	if err != nil {
		return fmt.Errorf("failed to reset transaction splits: %v", err)
	}
	return nil
}

func (r *TransactionSplitRepository) Update(split *models.TransactionSplit) error {
	_, err := r.db.Conn.Exec(`
		UPDATE transaction_splits 
		SET reason = ? 
		WHERE transaction_id = ? AND user_id = ?
	`, split.Reason, split.TransactionID, split.UserID)
	if err != nil {
		return fmt.Errorf("failed to update split: %v", err)
	}
	return nil
}

func (r *TransactionSplitRepository) UpdateSplitStatus(splitIDs []int64, tx *sql.Tx) error {
	placeholders := make([]string, len(splitIDs))
	args := make([]interface{}, len(splitIDs))
	for i, id := range splitIDs {
		placeholders[i] = "?"
		args[i] = id
	}
	query := `
		UPDATE transaction_splits 
		SET completed = 1 
		WHERE id IN (%s)
	`
	query = fmt.Sprintf(query, strings.Join(placeholders, ","))

	_, err := tx.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to update split status: %v", err)
	}
	return nil
}

func (r *TransactionSplitRepository) GetPendingSplits() ([]*models.TransactionSplit, error) {
	query := `
		SELECT 
			ts.id,
			ts.transaction_id,
			ts.user_id,
			ts.amount,
			ts.reason,
			ts.created_at
		FROM transaction_splits ts
		WHERE ts.completed = 0
		ORDER BY ts.created_at DESC
	`

	rows, err := r.db.Conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var splits []*models.TransactionSplit
	for rows.Next() {
		split := &models.TransactionSplit{}
		err := rows.Scan(
			&split.ID,
			&split.TransactionID,
			&split.UserID,
			&split.Amount,
			&split.Reason,
			&split.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		splits = append(splits, split)
	}

	return splits, nil
}

func (r *TransactionSplitRepository) GetPendingSplitsByUserID(userID int64) ([]*models.TransactionSplit, error) {
	query := `
		SELECT 
			ts.id,
			ts.transaction_id,
			ts.user_id,
			ts.amount,
			ts.reason,
			ts.created_at,
			t.amount as total_bill_amount,
			t.timestamp as bill_created_at
		FROM transaction_splits ts
		INNER JOIN transactions t ON t.id = ts.transaction_id
		WHERE ts.completed = 0
		AND ts.user_id = ?
		ORDER BY ts.created_at DESC
	`

	rows, err := r.db.Conn.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var splits []*models.TransactionSplit
	for rows.Next() {
		var billCreatedAt int64
		split := &models.TransactionSplit{}
		err := rows.Scan(
			&split.ID,
			&split.TransactionID,
			&split.UserID,
			&split.Amount,
			&split.Reason,
			&split.CreatedAt,
			&split.TotalBillAmount,
			&billCreatedAt,
		)
		if err != nil {
			return nil, err
		}
		split.BillCreatedAt = time.Unix(billCreatedAt, 0)
		splits = append(splits, split)
	}

	return splits, nil
}
