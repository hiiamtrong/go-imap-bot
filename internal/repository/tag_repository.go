package repository

import (
	"fmt"
	"time"

	"github.com/hiiamtrong/imap-bot-go/internal/database"
	"github.com/hiiamtrong/imap-bot-go/internal/models"
)

type TagRepository struct {
	db *database.Database
}

func NewTagRepository(db *database.Database) *TagRepository {
	return &TagRepository{db: db}
}

// GetAllTags returns all available tags
func (r *TagRepository) GetAllTags() ([]models.Tag, error) {
	query := `SELECT id, name, created_at FROM tags ORDER BY name`
	rows, err := r.db.Conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get tags: %v", err)
	}
	defer rows.Close()

	var tags []models.Tag
	for rows.Next() {
		var tag models.Tag
		if err := rows.Scan(&tag.ID, &tag.Name, &tag.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan tag: %v", err)
		}
		tags = append(tags, tag)
	}

	return tags, nil
}

// GetByID returns a tag by ID
func (r *TagRepository) GetByID(tagID int64) (*models.Tag, error) {
	query := `SELECT id, name, created_at FROM tags WHERE id = ?`
	var tag models.Tag
	err := r.db.Conn.QueryRow(query, tagID).Scan(&tag.ID, &tag.Name, &tag.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to get tag: %v", err)
	}
	return &tag, nil
}

// Create creates a new tag
func (r *TagRepository) Create(name string) (int64, error) {
	query := `INSERT INTO tags (name, created_at) VALUES (?, ?)`
	result, err := r.db.Conn.Exec(query, name, time.Now())
	if err != nil {
		return 0, fmt.Errorf("failed to create tag: %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get tag ID: %v", err)
	}

	return id, nil
}

// AddToTransaction adds a tag to a transaction
func (r *TagRepository) AddToTransaction(transactionID, tagID int64) error {
	query := `INSERT INTO transaction_tags (transaction_id, tag_id, created_at) VALUES (?, ?, ?)`
	_, err := r.db.Conn.Exec(query, transactionID, tagID, time.Now())
	if err != nil {
		return fmt.Errorf("failed to add tag to transaction: %v", err)
	}
	return nil
}

// GetForTransaction returns all tags for a transaction
func (r *TagRepository) GetForTransaction(transactionID int64) ([]models.Tag, error) {
	query := `
		SELECT t.id, t.name, t.created_at
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

	var tags []models.Tag
	for rows.Next() {
		var tag models.Tag
		if err := rows.Scan(&tag.ID, &tag.Name, &tag.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan tag: %v", err)
		}
		tags = append(tags, tag)
	}

	return tags, nil
}
