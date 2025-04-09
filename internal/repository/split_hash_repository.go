package repository

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"math/big"
	"strings"

	"github.com/hiiamtrong/go-imap-bot/internal/database"
)

type SplitHashRepository struct {
	db *database.Database
}

func NewSplitHashRepository(db *database.Database) *SplitHashRepository {
	return &SplitHashRepository{
		db: db,
	}
}

// CreateTable creates the split_hashes table if it doesn't exist
func (r *SplitHashRepository) CreateTable() error {
	query := `CREATE TABLE IF NOT EXISTS split_hashes (
		hash TEXT PRIMARY KEY,
		split_ids TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`
	_, err := r.db.Conn.Exec(query)
	return err
}

// GenerateHash generates a unique hash for a set of split IDs
func (r *SplitHashRepository) GenerateHash(splitIDs []int64) (string, error) {
	const letters = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	const hashLength = 8

	// Generate random hash using only letters
	bytes := make([]byte, hashLength)
	for i := range bytes {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		if err != nil {
			return "", fmt.Errorf("failed to generate random number: %v", err)
		}
		bytes[i] = letters[n.Int64()]
	}
	hash := string(bytes)
	hash = strings.ToUpper(fmt.Sprintf("trx%song", hash))

	// Convert split IDs to comma-separated string
	idStrs := make([]string, len(splitIDs))
	for i, id := range splitIDs {
		idStrs[i] = fmt.Sprintf("%d", id)
	}
	splitIDsStr := strings.Join(idStrs, ",")

	// Store in database
	query := `INSERT INTO split_hashes (hash, split_ids) VALUES (?, ?)`
	_, err := r.db.Conn.Exec(query, hash, splitIDsStr)
	if err != nil {
		return "", fmt.Errorf("failed to store hash: %v", err)
	}

	return hash, nil
}

// GetSplitIDs retrieves split IDs associated with a hash
func (r *SplitHashRepository) GetSplitIDs(hash string) ([]int64, error) {
	var splitIDsStr string
	query := `SELECT split_ids FROM split_hashes WHERE hash = ?`
	err := r.db.Conn.QueryRow(query, strings.ToUpper(hash)).Scan(&splitIDsStr)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("hash not found: %s", hash)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get split IDs: %v", err)
	}

	// Parse comma-separated string back to []int64
	idStrs := strings.Split(splitIDsStr, ",")
	ids := make([]int64, len(idStrs))
	for i, str := range idStrs {
		var id int64
		_, err := fmt.Sscanf(str, "%d", &id)
		if err != nil {
			return nil, fmt.Errorf("invalid split ID in hash: %v", err)
		}
		ids[i] = id
	}

	return ids, nil
}

// CleanupOldHashes removes hashes older than the specified days
func (r *SplitHashRepository) CleanupOldHashes(days int) error {
	query := `DELETE FROM split_hashes WHERE created_at < datetime('now', '-? days')`
	_, err := r.db.Conn.Exec(query, days)
	return err
}
