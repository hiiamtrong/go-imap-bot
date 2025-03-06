package database

import (
	"database/sql"
	"fmt"
	"os"

	"github.com/hiiamtrong/imap-bot-go/internal/config"
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
	conn, err := sql.Open("sqlite3", cfg.DatabasePath)
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
