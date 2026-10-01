package repository

import (
	"database/sql"
	"os"
	"reflect"
	"testing"

	"github.com/hiiamtrong/go-imap-bot/internal/database"
	_ "github.com/mattn/go-sqlite3"
)

func newAliasRepo(t *testing.T) *AliasRepository {
	t.Helper()
	conn, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	conn.SetMaxOpenConns(1) // every :memory: connection is its own database

	schema, err := os.ReadFile("../../init/init.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(string(schema)); err != nil {
		t.Fatal(err)
	}
	return NewAliasRepository(&database.Database{Conn: conn})
}

func TestAliasRepository(t *testing.T) {
	r := newAliasRepo(t)

	got, err := r.GetAll()
	if err != nil || len(got) != 0 {
		t.Fatalf("empty table: got %v, err %v", got, err)
	}

	for _, a := range []struct {
		key, alias string
		userID     int64
	}{
		{"ki", "Ki", 68},
		{"hongg ngoc", "Hồngg Ngọc", 4},
		{"ki", "Ki", 70}, // remapping replaces, it does not duplicate
		{"", "???", 1},   // nothing to key on
	} {
		if err := r.Save(a.key, a.alias, a.userID); err != nil {
			t.Fatalf("Save(%q): %v", a.key, err)
		}
	}

	got, err = r.GetAll()
	want := map[string]int64{"ki": 70, "hongg ngoc": 4}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("GetAll = %v (err %v), want %v", got, err, want)
	}
}
