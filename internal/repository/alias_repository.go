package repository

import "github.com/hiiamtrong/go-imap-bot/internal/database"

// AliasRepository remembers which user a display name on a bill belongs to
// (billsplit.AliasKey -> user ID), whichever app or chat the bill came from.
type AliasRepository struct {
	db *database.Database
}

func NewAliasRepository(db *database.Database) *AliasRepository {
	return &AliasRepository{db: db}
}

func (r *AliasRepository) GetAll() (map[string]int64, error) {
	rows, err := r.db.Conn.Query(`SELECT alias_key, user_id FROM user_aliases`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	aliases := map[string]int64{}
	for rows.Next() {
		var key string
		var userID int64
		if err := rows.Scan(&key, &userID); err != nil {
			return nil, err
		}
		aliases[key] = userID
	}
	return aliases, rows.Err()
}

// Save points key at userID, replacing an earlier mapping. An empty key (a name
// with no letters) is ignored.
func (r *AliasRepository) Save(key, alias string, userID int64) error {
	if key == "" {
		return nil
	}
	_, err := r.db.Conn.Exec(`
		INSERT INTO user_aliases (alias_key, alias, user_id) VALUES (?, ?, ?)
		ON CONFLICT(alias_key) DO UPDATE SET alias = excluded.alias, user_id = excluded.user_id`,
		key, alias, userID)
	return err
}
