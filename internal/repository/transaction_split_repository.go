package repository

import (
	"database/sql"
	"fmt"
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
		INSERT INTO transaction_splits (
			transaction_id, name, amount, created_at
		)
		VALUES (?, ?, ?, ?)
	`
	result, err := r.db.Conn.Exec(
		query,
		split.TransactionID,
		split.Name,
		split.Amount,
		split.CreatedAt,
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

func (r *TransactionSplitRepository) CreateTx(tx *sql.Tx, split *models.TransactionSplit) error {
	query := `
		INSERT INTO transaction_splits (
			transaction_id, name, amount, created_at
		)
		VALUES (?, ?, ?, ?)
	`
	result, err := tx.Exec(
		query,
		split.TransactionID,
		split.Name,
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
		SELECT id, transaction_id, name, amount, created_at
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
			&split.Name,
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
