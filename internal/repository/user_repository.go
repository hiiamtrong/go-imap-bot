package repository

import (
	"database/sql"
	"fmt"

	"github.com/hiiamtrong/imap-bot-go/internal/database"
	"github.com/hiiamtrong/imap-bot-go/internal/models"
)

type UserRepository struct {
	db *database.Database
}

func NewUserRepository(db *database.Database) *UserRepository {
	return &UserRepository{db: db}
}

// Transaction executes a function within a database transaction
func (r *UserRepository) Transaction(fn func(tx *sql.Tx) error) error {
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

func (r *UserRepository) Create(user *models.User) error {
	query := `INSERT INTO users (name, email) VALUES (?, ?)`
	result, err := r.db.Conn.Exec(query, user.Name, user.Email)
	if err != nil {
		return fmt.Errorf("failed to insert user: %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %v", err)
	}

	user.ID = id
	return nil
}

func (r *UserRepository) CreateTx(tx *sql.Tx, user *models.User) error {
	query := `INSERT INTO users (name, email) VALUES (?, ?)`
	result, err := tx.Exec(query, user.Name, user.Email)
	if err != nil {
		return fmt.Errorf("failed to insert user: %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %v", err)
	}

	user.ID = id
	return nil
}

func (r *UserRepository) GetByID(id int64) (*models.User, error) {
	query := `SELECT id, name, email FROM users WHERE id = ?`
	user := &models.User{}
	err := r.db.Conn.QueryRow(query, id).Scan(&user.ID, &user.Name, &user.Email)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %v", err)
	}
	return user, nil
}

func (r *UserRepository) GetAll() ([]*models.User, error) {
	query := `SELECT id, name, email FROM users`
	rows, err := r.db.Conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query users: %v", err)
	}
	defer rows.Close()

	users := []*models.User{}
	for rows.Next() {
		var user models.User
		if err := rows.Scan(&user.ID, &user.Name, &user.Email); err != nil {
			return nil, fmt.Errorf("failed to scan user: %v", err)
		}
		users = append(users, &user)
	}
	return users, nil
}
