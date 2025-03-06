package repository

import (
	"fmt"
	"time"

	"github.com/hiiamtrong/imap-bot-go/internal/database"
	"github.com/hiiamtrong/imap-bot-go/internal/models"
)

type TransactionRepository struct {
	db *database.Database
}

func NewTransactionRepository(db *database.Database) *TransactionRepository {
	return &TransactionRepository{db: db}
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
	err := row.Scan(
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

	return t, nil
}

func (r *TransactionRepository) CreateSplit(split *models.TransactionSplit) error {
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
