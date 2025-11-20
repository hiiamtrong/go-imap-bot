package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hiiamtrong/go-imap-bot/internal/database"
	"github.com/hiiamtrong/go-imap-bot/internal/models"
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
	INSERT INTO transaction_splits (transaction_id, user_id, amount, reason, created_at)
	VALUES (?, ?, ?, ?, ?)
`
	result, err := r.db.Conn.Exec(
		query,
		split.TransactionID,
		split.UserID,
		split.Amount,
		split.Reason,
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
			transaction_id, user_id, amount, reason, created_at
		)
		VALUES (?, ?, ?, ?, ?)
	`
	result, err := tx.Exec(
		query,
		split.TransactionID,
		split.UserID,
		split.Amount,
		split.Reason,
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

// UpdateByID updates a split by its ID with the given amount and reason
func (r *TransactionSplitRepository) UpdateByID(id int64, amount int64, reason string) error {
	_, err := r.db.Conn.Exec(`
		UPDATE transaction_splits
		SET amount = ?, reason = ?
		WHERE id = ?
	`, amount, reason, id)
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

// CompleteSplit marks a single split as completed without requiring a transaction
func (r *TransactionSplitRepository) CompleteSplit(splitID int64) error {
	query := `UPDATE transaction_splits SET completed = 1 WHERE id = ?`
	_, err := r.db.Conn.Exec(query, splitID)
	if err != nil {
		return fmt.Errorf("failed to complete split: %v", err)
	}
	return nil
}

// CompleteAllSplitsForUser marks all pending splits for a user as completed
func (r *TransactionSplitRepository) CompleteAllSplitsForUser(userID int64) error {
	query := `UPDATE transaction_splits SET completed = 1 WHERE user_id = ? AND completed = 0`
	_, err := r.db.Conn.Exec(query, userID)
	if err != nil {
		return fmt.Errorf("failed to complete splits for user: %v", err)
	}
	return nil
}

// GetByID gets a single split by ID
func (r *TransactionSplitRepository) GetByID(id int64) (*models.TransactionSplit, error) {
	query := `
		SELECT id, transaction_id, user_id, amount, reason, created_at, completed
		FROM transaction_splits
		WHERE id = ?
	`
	split := &models.TransactionSplit{}
	err := r.db.Conn.QueryRow(query, id).Scan(
		&split.ID,
		&split.TransactionID,
		&split.UserID,
		&split.Amount,
		&split.Reason,
		&split.CreatedAt,
		&split.Completed,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get split: %v", err)
	}
	return split, nil
}

func (r *TransactionSplitRepository) GetPendingSplits() ([]*models.TransactionSplit, error) {
	query := `
		SELECT 
			ts.id,
			ts.transaction_id,
			ts.user_id,
			ts.amount,
			ts.reason,
			ts.created_at,
			t.description as transaction_description
		FROM transaction_splits ts
		JOIN transactions t ON t.id = ts.transaction_id
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
		var transactionDescription string
		split := &models.TransactionSplit{}
		err := rows.Scan(
			&split.ID,
			&split.TransactionID,
			&split.UserID,
			&split.Amount,
			&split.Reason,
			&split.CreatedAt,
			&transactionDescription,
		)
		if err != nil {
			return nil, err
		}
		if split.Reason == "" {
			split.Reason = transactionDescription
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
			m.timestamp as bill_created_at,
			t.description as transaction_description
		FROM transaction_splits ts
		INNER JOIN transactions t ON t.id = ts.transaction_id
		INNER JOIN mails m ON m.id = t.mail_id
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
		var billCreatedAtStr string
		var transactionDescription string
		split := &models.TransactionSplit{}
		err := rows.Scan(
			&split.ID,
			&split.TransactionID,
			&split.UserID,
			&split.Amount,
			&split.Reason,
			&split.CreatedAt,
			&split.TotalBillAmount,
			&billCreatedAtStr,
			&transactionDescription,
		)
		if err != nil {
			return nil, err
		}

		// Try to parse as Unix timestamp first
		if timestamp, err := strconv.ParseInt(billCreatedAtStr, 10, 64); err == nil {
			split.BillCreatedAt = time.Unix(timestamp, 0)
		} else {
			// If not a Unix timestamp, try parsing as formatted time string
			billCreatedAt, err := time.Parse("2006-01-02 15:04:05-07:00", billCreatedAtStr)
			if err != nil {
				// Try alternative format if first parse fails
				billCreatedAt, err = time.Parse(time.RFC3339, billCreatedAtStr)
				if err != nil {
					return nil, fmt.Errorf("failed to parse timestamp: %v", err)
				}
			}
			split.BillCreatedAt = billCreatedAt
		}
		if split.Reason == "" {
			split.Reason = transactionDescription
		}
		splits = append(splits, split)
	}

	return splits, nil
}

func (r *TransactionSplitRepository) GetSplitsByIDs(splitIDs []int64) ([]*models.TransactionSplit, error) {
	placeholders := make([]string, len(splitIDs))
	args := make([]interface{}, len(splitIDs))
	for i, id := range splitIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	query := `
		SELECT 
			ts.id,
			ts.transaction_id,
			ts.user_id,
			ts.amount,
			ts.reason,
			ts.completed,
			ts.created_at,
			t.amount as total_bill_amount,
			t.created_at as transaction_created_at,
			m.timestamp as bill_created_at,
			t.description as transaction_description
		FROM transaction_splits ts
		INNER JOIN transactions t ON t.id = ts.transaction_id
		INNER JOIN mails m ON m.id = t.mail_id
		WHERE ts.id IN (%s)
	`
	query = fmt.Sprintf(query, strings.Join(placeholders, ","))

	rows, err := r.db.Conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var splits []*models.TransactionSplit
	for rows.Next() {
		var billCreatedAtStr string
		var transactionCreatedAtStr string
		var transactionDescription string
		split := &models.TransactionSplit{}
		err := rows.Scan(
			&split.ID,
			&split.TransactionID,
			&split.UserID,
			&split.Amount,
			&split.Reason,
			&split.Completed,
			&split.CreatedAt,
			&split.TotalBillAmount,
			&transactionCreatedAtStr,
			&billCreatedAtStr,
			&transactionDescription,
		)
		if err != nil {
			return nil, err
		}

		// Try to parse as Unix timestamp first
		if timestamp, err := strconv.ParseInt(billCreatedAtStr, 10, 64); err == nil {
			split.BillCreatedAt = time.Unix(timestamp, 0)
		} else {
			// If not a Unix timestamp, try parsing as formatted time string
			billCreatedAt, err := time.Parse("2006-01-02 15:04:05-07:00", billCreatedAtStr)
			if err != nil {
				// Try alternative format if first parse fails
				billCreatedAt, err = time.Parse(time.RFC3339, billCreatedAtStr)
				if err != nil {
					return nil, fmt.Errorf("failed to parse timestamp: %v", err)
				}
			}
			split.BillCreatedAt = billCreatedAt
		}

		// Try to parse as Unix timestamp first
		if timestamp, err := strconv.ParseInt(transactionCreatedAtStr, 10, 64); err == nil {
			split.TransactionCreatedAt = time.Unix(timestamp, 0)
		} else {
			// If not a Unix timestamp, try parsing as formatted time string
			transactionCreatedAt, err := time.Parse("2006-01-02 15:04:05-07:00", transactionCreatedAtStr)
			if err != nil {
				// Try alternative format if first parse fails
				transactionCreatedAt, err = time.Parse(time.RFC3339, transactionCreatedAtStr)
				if err != nil {
					return nil, fmt.Errorf("failed to parse timestamp: %v", err)
				}
			}
			split.TransactionCreatedAt = transactionCreatedAt
		}

		if split.Reason == "" {
			split.Reason = transactionDescription
		}
		splits = append(splits, split)
	}

	return splits, nil
}

// GetByIDs is an alias for GetSplitsByIDs for consistency
func (r *TransactionSplitRepository) GetByIDs(splitIDs []int64) ([]*models.TransactionSplit, error) {
	return r.GetSplitsByIDs(splitIDs)
}

// Delete deletes a transaction split by ID
func (r *TransactionSplitRepository) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM transaction_splits WHERE id = ?`
	_, err := r.db.Conn.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete split: %v", err)
	}
	return nil
}
