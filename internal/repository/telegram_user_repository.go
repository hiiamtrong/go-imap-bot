package repository

import (
	"database/sql"
	"fmt"

	"github.com/hiiamtrong/imap-bot-go/internal/database"
	"github.com/hiiamtrong/imap-bot-go/internal/models"
)

type TelegramUserRepository struct {
	db *database.Database
}

func NewTelegramUserRepository(db *database.Database) *TelegramUserRepository {
	return &TelegramUserRepository{
		db: db,
	}
}

// Transaction executes a function within a database transaction
func (r *TelegramUserRepository) Transaction(fn func(tx *sql.Tx) error) error {
	tx, err := r.db.Conn.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("tx failed: %v, unable to rollback: %v", err, rbErr)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	return nil
}

func (r *TelegramUserRepository) IsAuthorized(chatID int64) (bool, error) {
	var count int
	err := r.db.Conn.QueryRow("SELECT COUNT(*) FROM authorized_telegram_users WHERE chat_id = ?", chatID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check user authorization: %v", err)
	}
	return count > 0, nil
}

func (r *TelegramUserRepository) Authorize(chatID int64, username string, email string) error {
	_, err := r.db.Conn.Exec("INSERT INTO authorized_telegram_users (chat_id, username, email) VALUES (?, ?, ?)",
		chatID, username, email)
	if err != nil {
		return fmt.Errorf("failed to authorize user: %v", err)
	}
	return nil
}

func (r *TelegramUserRepository) GetEmail(chatID int64) (string, error) {
	var email string
	err := r.db.Conn.QueryRow("SELECT email FROM authorized_telegram_users WHERE chat_id = ?", chatID).Scan(&email)
	if err != nil {
		return "", fmt.Errorf("failed to get user email: %v", err)
	}
	return email, nil
}

func (r *TelegramUserRepository) GetChatIDsByEmail(email string, tx *sql.Tx) ([]int64, error) {
	var rows *sql.Rows
	var err error

	if tx != nil {
		rows, err = tx.Query("SELECT chat_id FROM authorized_telegram_users WHERE email = ?", email)
	} else {
		rows, err = r.db.Conn.Query("SELECT chat_id FROM authorized_telegram_users WHERE email = ?", email)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get authorized users: %v", err)
	}
	defer rows.Close()

	var chatIDs []int64
	for rows.Next() {
		var chatID int64
		if err := rows.Scan(&chatID); err != nil {
			return nil, fmt.Errorf("error scanning chat ID: %v", err)
		}
		chatIDs = append(chatIDs, chatID)
	}

	return chatIDs, nil
}

func (r *TelegramUserRepository) GetByChatID(chatID int64) (*models.TelegramUser, error) {
	var user models.TelegramUser
	err := r.db.Conn.QueryRow("SELECT chat_id, username, email FROM authorized_telegram_users WHERE chat_id = ?", chatID).Scan(&user.ChatID, &user.Username, &user.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %v", err)
	}
	return &user, nil
}
