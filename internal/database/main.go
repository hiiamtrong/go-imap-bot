package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"github.com/hiiamtrong/go-imap-bot/internal/config"
)

type Database struct {
	Conn *sql.DB
}

func initDatabase(conn *sql.DB) error {
	// Read the SQL file
	sqlBytes, err := os.ReadFile("init/init.sql")
	if err != nil {
		return fmt.Errorf("failed to read init.sql: %v", err)
	}

	// Execute the SQL statements
	_, err = conn.Exec(string(sqlBytes))
	if err != nil {
		return fmt.Errorf("failed to execute init.sql: %v", err)
	}

	return nil
}

func GetDatabase(cfg *config.DatabaseConfig) (*Database, error) {
	// Enable WAL mode for better concurrent access
	// _busy_timeout: wait up to 30 seconds if database is locked
	// _txlock=immediate: acquire write lock immediately to prevent SQLITE_BUSY errors
	conn, err := sql.Open("sqlite3", cfg.DatabasePath+"?_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=30000&_txlock=immediate")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	if err := initDatabase(conn); err != nil {
		return nil, fmt.Errorf("failed to initialize database: %v", err)
	}

	fmt.Printf("Database initialized successfully\n")
	fmt.Printf("Database path: %v\n", cfg.DatabasePath)
	fmt.Printf("Database conn: %v\n", conn)
	return &Database{Conn: conn}, nil
}

func (db *Database) BeginTx(ctx context.Context) (*sql.Tx, error) {
	return db.Conn.BeginTx(ctx, nil)
}

func (db *Database) CommitTx(tx *sql.Tx) error {
	return tx.Commit()
}

func (db *Database) RollbackTx(tx *sql.Tx) error {
	return tx.Rollback()
}
