package repository

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/hiiamtrong/go-imap-bot/internal/database"
	_ "github.com/mattn/go-sqlite3"
)

func TestRenameReason(t *testing.T) {
	conn, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	conn.SetMaxOpenConns(1)
	schema, err := os.ReadFile("../../init/init.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(string(schema)); err != nil {
		t.Fatal(err)
	}

	for _, row := range []struct {
		tx     int64
		reason string
	}{
		{1, "Cơm tối"},
		{1, "Cơm tối (gồm phần của Hà, Sơn)"},
		{1, "Cơm tối nay ngon"}, // typed by the user, only starts with the old name
		{1, "Tiền nhà"},
		{2, "Cơm tối"}, // another transaction
	} {
		if _, err := conn.Exec(`INSERT INTO transaction_splits (transaction_id, user_id, amount, reason) VALUES (?, 1, 1, ?)`, row.tx, row.reason); err != nil {
			t.Fatal(err)
		}
	}

	r := NewTransactionSplitRepository(&database.Database{Conn: conn})
	if err := r.RenameReason(context.Background(), 1, "Cơm tối", "Cơm trưa"); err != nil {
		t.Fatal(err)
	}

	rows, err := conn.Query(`SELECT reason FROM transaction_splits ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	want := []string{"Cơm trưa", "Cơm trưa (gồm phần của Hà, Sơn)", "Cơm tối nay ngon", "Tiền nhà", "Cơm tối"}
	for i := 0; rows.Next(); i++ {
		var got string
		if err := rows.Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != want[i] {
			t.Errorf("row %d reason = %q, want %q", i, got, want[i])
		}
	}
}
