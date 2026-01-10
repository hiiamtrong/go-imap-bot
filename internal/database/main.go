package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

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

func createMigrationsTable(conn *sql.DB) error {
	_, err := conn.Exec(`
		CREATE TABLE IF NOT EXISTS migrations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT UNIQUE NOT NULL,
			executed_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	return err
}

func isMigrationExecuted(conn *sql.DB, name string) (bool, error) {
	var count int
	err := conn.QueryRow("SELECT COUNT(*) FROM migrations WHERE name = ?", name).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func markMigrationExecuted(conn *sql.DB, name string) error {
	_, err := conn.Exec("INSERT INTO migrations (name) VALUES (?)", name)
	return err
}

func runMigrations(conn *sql.DB) error {
	// Create migrations table if it doesn't exist
	if err := createMigrationsTable(conn); err != nil {
		return fmt.Errorf("failed to create migrations table: %v", err)
	}

	// Find all migration files
	files, err := filepath.Glob("init/migrate_*.sql")
	if err != nil {
		return fmt.Errorf("failed to find migration files: %v", err)
	}

	// Sort files to ensure consistent execution order
	sort.Strings(files)

	for _, file := range files {
		migrationName := filepath.Base(file)

		// Check if migration was already executed
		executed, err := isMigrationExecuted(conn, migrationName)
		if err != nil {
			return fmt.Errorf("failed to check migration status for %s: %v", migrationName, err)
		}

		if executed {
			fmt.Printf("Migration %s already executed, skipping\n", migrationName)
			continue
		}

		// Read and execute migration
		sqlBytes, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %v", migrationName, err)
		}

		// Split by semicolon and execute each statement separately
		// This handles ALTER TABLE statements that may fail individually
		statements := strings.Split(string(sqlBytes), ";")
		for _, stmt := range statements {
			// Remove comment lines from statement
			lines := strings.Split(stmt, "\n")
			var cleanLines []string
			for _, line := range lines {
				trimmedLine := strings.TrimSpace(line)
				if trimmedLine != "" && !strings.HasPrefix(trimmedLine, "--") {
					cleanLines = append(cleanLines, line)
				}
			}
			stmt = strings.TrimSpace(strings.Join(cleanLines, "\n"))
			if stmt == "" {
				continue
			}

			_, err = conn.Exec(stmt)
			if err != nil {
				// For ALTER TABLE ADD COLUMN, ignore "duplicate column" errors
				if strings.Contains(stmt, "ALTER TABLE") && strings.Contains(stmt, "ADD COLUMN") {
					if strings.Contains(err.Error(), "duplicate column") {
						fmt.Printf("Column already exists, skipping: %s\n", stmt)
						continue
					}
				}
				// Log warning but continue for non-critical errors
				fmt.Printf("Warning: migration statement failed (may be expected): %v\n", err)
			}
		}

		// Mark migration as executed
		if err := markMigrationExecuted(conn, migrationName); err != nil {
			return fmt.Errorf("failed to mark migration %s as executed: %v", migrationName, err)
		}

		fmt.Printf("Migration %s executed successfully\n", migrationName)
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

	// Run migrations after initial schema setup
	if err := runMigrations(conn); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %v", err)
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
