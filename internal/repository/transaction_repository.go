package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/hiiamtrong/go-imap-bot/internal/database"
	"github.com/hiiamtrong/go-imap-bot/internal/models"
)

type TransactionRepository struct {
	db *database.Database
}

func NewTransactionRepository(db *database.Database) *TransactionRepository {
	return &TransactionRepository{db: db}
}

// Transaction executes a function within a database transaction
func (r *TransactionRepository) Transaction(fn func(tx *sql.Tx) error) error {
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

func (r *TransactionRepository) Create(transaction *models.Transaction) error {
	query := `
		INSERT INTO transactions (
			mail_id, amount, current_balance, type,
			from_account, to_account, description, 
			timestamp, created_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	result, err := r.db.Conn.Exec(
		query,
		transaction.MailID,
		transaction.Amount,
		transaction.CurrentBalance,
		transaction.Type,
		transaction.From,
		transaction.To,
		transaction.Description,
		transaction.Timestamp,
		time.Now(),
	)
	if err != nil {
		return fmt.Errorf("failed to insert transaction: %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %v", err)
	}

	transaction.ID = id
	return nil
}

func (r *TransactionRepository) CreateTx(tx *sql.Tx, transaction *models.Transaction) error {
	query := `
		INSERT INTO transactions (
			mail_id, amount, current_balance, type,
			from_account, to_account, description, 
			timestamp, created_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	result, err := tx.Exec(
		query,
		transaction.MailID,
		transaction.Amount,
		transaction.CurrentBalance,
		transaction.Type,
		transaction.From,
		transaction.To,
		transaction.Description,
		transaction.Timestamp,
		time.Now(),
	)
	if err != nil {
		return fmt.Errorf("failed to insert transaction: %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %v", err)
	}

	transaction.ID = id
	return nil
}

func (r *TransactionRepository) GetByMailID(mailID int64) ([]*models.Transaction, error) {
	query := `
		SELECT 
			id, mail_id, amount, current_balance, type,
			from_account, to_account, description,
			timestamp, created_at
		FROM transactions
		WHERE mail_id = ?
	`
	rows, err := r.db.Conn.Query(query, mailID)
	if err != nil {
		return nil, fmt.Errorf("failed to get transactions: %v", err)
	}
	defer rows.Close()

	var transactions []*models.Transaction
	for rows.Next() {
		t := &models.Transaction{}
		err := rows.Scan(
			&t.ID,
			&t.MailID,
			&t.Amount,
			&t.CurrentBalance,
			&t.Type,
			&t.From,
			&t.To,
			&t.Description,
			&t.Timestamp,
			&t.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan transaction: %v", err)
		}
		transactions = append(transactions, t)
	}

	return transactions, nil
}

func (r *TransactionRepository) GetByID(id int64) (*models.Transaction, error) {
	query := `
		SELECT 
			id, mail_id, amount, current_balance, type,
			from_account, to_account, description,
			timestamp, created_at
		FROM transactions
		WHERE id = ?
	`
	row := r.db.Conn.QueryRow(query, id)

	t := &models.Transaction{}
	var timestampStr string
	var createdAtStr string

	err := row.Scan(
		&t.ID,
		&t.MailID,
		&t.Amount,
		&t.CurrentBalance,
		&t.Type,
		&t.From,
		&t.To,
		&t.Description,
		&timestampStr,
		&createdAtStr,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to scan transaction: %v", err)
	}

	// Parse the timestamp string
	timestamp, err := time.Parse("2006-01-02 15:04:05-07:00", timestampStr)
	if err != nil {
		// Try alternative format if the first one fails
		timestamp, err = time.Parse(time.RFC3339, timestampStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse timestamp: %v", err)
		}
	}
	t.Timestamp = timestamp

	// Parse the created_at string - try both formats
	createdAt, err := time.Parse(time.RFC3339, createdAtStr)
	if err != nil {
		// Try the alternative format if RFC3339 fails
		createdAt, err = time.Parse("2006-01-02 15:04:05-07:00", createdAtStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse created_at: %v", err)
		}
	}
	t.CreatedAt = createdAt

	return t, nil
}

// Tag represents a transaction tag
type Tag struct {
	ID   int64
	Name string
}

// GetAllTags returns all available tags
func (r *TransactionRepository) GetAllTags() ([]Tag, error) {
	query := `SELECT id, name FROM tags ORDER BY name`
	rows, err := r.db.Conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get tags: %v", err)
	}
	defer rows.Close()

	var tags []Tag
	for rows.Next() {
		var tag Tag
		if err := rows.Scan(&tag.ID, &tag.Name); err != nil {
			return nil, fmt.Errorf("failed to scan tag: %v", err)
		}
		tags = append(tags, tag)
	}

	return tags, nil
}

// GetTagByID returns a tag by ID
func (r *TransactionRepository) GetTagByID(tagID int64) (*Tag, error) {
	query := `SELECT id, name FROM tags WHERE id = ?`
	var tag Tag
	err := r.db.Conn.QueryRow(query, tagID).Scan(&tag.ID, &tag.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to get tag: %v", err)
	}
	return &tag, nil
}

// AddTagToTransaction adds a tag to a transaction
func (r *TransactionRepository) AddTagToTransaction(transactionID, tagID int64) error {
	query := `INSERT INTO transaction_tags (transaction_id, tag_id) VALUES (?, ?)`
	_, err := r.db.Conn.Exec(query, transactionID, tagID)
	if err != nil {
		return fmt.Errorf("failed to add tag to transaction: %v", err)
	}
	return nil
}

// GetTagsForTransaction returns all tags for a transaction
func (r *TransactionRepository) GetTagsForTransaction(transactionID int64) ([]Tag, error) {
	query := `
		SELECT t.id, t.name 
		FROM tags t
		JOIN transaction_tags tt ON t.id = tt.tag_id
		WHERE tt.transaction_id = ?
		ORDER BY t.name
	`
	rows, err := r.db.Conn.Query(query, transactionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tags for transaction: %v", err)
	}
	defer rows.Close()

	var tags []Tag
	for rows.Next() {
		var tag Tag
		if err := rows.Scan(&tag.ID, &tag.Name); err != nil {
			return nil, fmt.Errorf("failed to scan tag: %v", err)
		}
		tags = append(tags, tag)
	}

	return tags, nil
}

// Add this after the existing GetByID method
func (r *TransactionRepository) GetSplitsForTransaction(transactionID int64) ([]*models.TransactionSplit, error) {
	query := `
		SELECT id, transaction_id, user_id, amount, created_at
		FROM transaction_splits
		WHERE transaction_id = ?
	`
	rows, err := r.db.Conn.Query(query, transactionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get splits: %v", err)
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
			return nil, fmt.Errorf("failed to scan split: %v", err)
		}
		splits = append(splits, split)
	}

	return splits, nil
}

func (r *TransactionRepository) Complete(ctx context.Context, transactionID int64) error {
	query := `UPDATE transactions SET completed = 1 WHERE id = ?`
	_, err := r.db.Conn.Exec(query, transactionID)
	if err != nil {
		return fmt.Errorf("failed to complete transaction: %v", err)
	}
	return nil
}

func (r *TransactionRepository) IsCompleted(ctx context.Context, transactionID int64) (bool, error) {
	query := `SELECT completed FROM transactions WHERE id = ?`
	var completed bool
	err := r.db.Conn.QueryRow(query, transactionID).Scan(&completed)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, fmt.Errorf("failed to check if transaction is completed: %v", err)
	}

	return completed, nil
}

func (r *TransactionRepository) GetRecentTransactions(ctx context.Context, limit int64, offset int64) ([]*models.Transaction, error) {
	query := `SELECT id, mail_id, amount, current_balance, type, from_account, to_account, description, timestamp, created_at FROM transactions 
	WHERE completed = 0
	ORDER BY timestamp DESC LIMIT ? OFFSET ?`
	rows, err := r.db.Conn.Query(query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get all transactions: %v", err)
	}
	defer rows.Close()

	var transactions []*models.Transaction
	for rows.Next() {
		t := &models.Transaction{}
		var timestampStr string
		err := rows.Scan(
			&t.ID,
			&t.MailID,
			&t.Amount,
			&t.CurrentBalance,
			&t.Type,
			&t.From,
			&t.To,
			&t.Description,
			&timestampStr,
			&t.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan transaction: %v", err)
		}

		// Parse the timestamp string
		timestamp, err := time.Parse("2006-01-02 15:04:05-07:00", timestampStr)
		if err != nil {
			// Try alternative format if the first one fails
			timestamp, err = time.Parse(time.RFC3339, timestampStr)
			if err != nil {
				return nil, fmt.Errorf("failed to parse timestamp: %v", err)
			}
		}
		t.Timestamp = timestamp

		transactions = append(transactions, t)
	}

	return transactions, nil
}

// RemoveTagFromTransaction removes a tag from a transaction
func (r *TransactionRepository) RemoveTagFromTransaction(transactionID, tagID int64) error {
	query := `DELETE FROM transaction_tags WHERE transaction_id = ? AND tag_id = ?`
	_, err := r.db.Conn.Exec(query, transactionID, tagID)
	if err != nil {
		return fmt.Errorf("failed to remove tag from transaction: %v", err)
	}
	return nil
}
