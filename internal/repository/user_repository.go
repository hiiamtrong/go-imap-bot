package repository

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/hiiamtrong/go-imap-bot/internal/database"
	"github.com/hiiamtrong/go-imap-bot/internal/models"
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
	query := `SELECT id, name, email, is_whitelisted FROM users WHERE id = ?`
	user := &models.User{}
	err := r.db.Conn.QueryRow(query, id).Scan(&user.ID, &user.Name, &user.Email, &user.IsWhitelisted)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (r *UserRepository) GetAll() ([]*models.User, error) {
	query := `SELECT id, name, email, is_whitelisted FROM users`
	rows, err := r.db.Conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query users: %v", err)
	}
	defer rows.Close()

	users := []*models.User{}
	for rows.Next() {
		var user models.User
		if err := rows.Scan(&user.ID, &user.Name, &user.Email, &user.IsWhitelisted); err != nil {
			return nil, fmt.Errorf("failed to scan user: %v", err)
		}
		users = append(users, &user)
	}
	return users, nil
}

func (r *UserRepository) GetInIDs(ids []int64) ([]*models.User, error) {
	if len(ids) == 0 {
		return []*models.User{}, nil
	}

	// Create the correct number of placeholders for the IN clause
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}

	// Build the query with the correct number of placeholders
	query := fmt.Sprintf(
		"SELECT id, name, email, is_whitelisted FROM users WHERE id IN (%s)",
		strings.Join(placeholders, ","),
	)

	// Execute the query with the expanded arguments
	rows, err := r.db.Conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query users: %v", err)
	}
	defer rows.Close()

	users := []*models.User{}
	for rows.Next() {
		var user models.User
		if err := rows.Scan(&user.ID, &user.Name, &user.Email, &user.IsWhitelisted); err != nil {
			return nil, fmt.Errorf("failed to scan user: %v", err)
		}
		users = append(users, &user)
	}

	return users, nil
}

// Update updates a user
func (r *UserRepository) Update(user *models.User) error {
	query := `UPDATE users SET name = ?, email = ?, is_whitelisted = ? WHERE id = ?`
	_, err := r.db.Conn.Exec(query, user.Name, user.Email, user.IsWhitelisted, user.ID)
	if err != nil {
		return fmt.Errorf("failed to update user: %v", err)
	}
	return nil
}

// Delete deletes a user by ID
func (r *UserRepository) Delete(id int64) error {
	query := `DELETE FROM users WHERE id = ?`
	_, err := r.db.Conn.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete user: %v", err)
	}
	return nil
}

// UserFilters holds filter parameters for user queries
type UserFilters struct {
	Search    string // Search in name or email
	Whitelist *bool  // Filter by whitelist status (nil = all, true = whitelisted, false = not whitelisted)
}

// GetAllWithFilters returns users matching the given filters
func (r *UserRepository) GetAllWithFilters(filters UserFilters) ([]*models.User, error) {
	query := `SELECT id, name, email, is_whitelisted FROM users`
	args := []interface{}{}
	conditions := []string{}

	// Search in name or email
	if filters.Search != "" {
		conditions = append(conditions, "(name LIKE ? OR email LIKE ?)")
		searchPattern := "%" + filters.Search + "%"
		args = append(args, searchPattern, searchPattern)
	}

	// Filter by whitelist status
	if filters.Whitelist != nil {
		conditions = append(conditions, "is_whitelisted = ?")
		args = append(args, *filters.Whitelist)
	}

	// Add WHERE clause if there are conditions
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	query += " ORDER BY name"

	rows, err := r.db.Conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query users: %v", err)
	}
	defer rows.Close()

	users := []*models.User{}
	for rows.Next() {
		var user models.User
		if err := rows.Scan(&user.ID, &user.Name, &user.Email, &user.IsWhitelisted); err != nil {
			return nil, fmt.Errorf("failed to scan user: %v", err)
		}
		users = append(users, &user)
	}
	return users, nil
}
