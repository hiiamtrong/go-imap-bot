package repository

import (
	"database/sql"
	"os"
	"testing"

	"github.com/hiiamtrong/go-imap-bot/internal/database"
	_ "github.com/mattn/go-sqlite3"
)

func TestMailGetByIDReturnsBody(t *testing.T) {
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
	// Mail 1 predates body storage (NULL body), mail 2 has one.
	if _, err := conn.Exec(`INSERT INTO mails (uid, subject, from_account, to_account, timestamp, body) VALUES (1, 'old', 'bank@x.vn', 'me@x.com', 1743476285, NULL), (2, 'new', 'bank@x.vn', 'me@x.com', 1743476285, 'So du: 1.000 VND')`); err != nil {
		t.Fatal(err)
	}

	r := NewMailRepository(&database.Database{Conn: conn})
	old, err := r.GetByID(1)
	if err != nil || old.Body != "" || old.Subject != "old" {
		t.Errorf("old mail = %+v, err %v", old, err)
	}
	cur, err := r.GetByID(2)
	if err != nil || cur.Body != "So du: 1.000 VND" || cur.From != "bank@x.vn" || cur.To != "me@x.com" {
		t.Errorf("new mail = %+v, err %v", cur, err)
	}
	if _, err := r.GetByID(99); err != sql.ErrNoRows {
		t.Errorf("missing mail err = %v, want sql.ErrNoRows", err)
	}
}
