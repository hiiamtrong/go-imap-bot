package repository

import (
	"context"
	"database/sql"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/hiiamtrong/go-imap-bot/internal/database"
	"github.com/hiiamtrong/go-imap-bot/internal/models"
	_ "github.com/mattn/go-sqlite3"
)

func TestParseTimestamp(t *testing.T) {
	want := time.Date(2026, 9, 26, 12, 21, 30, 0, time.FixedZone("", 7*3600))
	tests := []struct {
		name    string
		in      string
		want    time.Time
		wantErr bool
	}{
		{"legacy unix seconds", "1743476285", time.Unix(1743476285, 0), false},
		{"go layout with offset", "2026-09-26 12:21:30+07:00", want, false},
		{"rfc3339", "2026-09-26T12:21:30+07:00", want, false},
		{"empty", "", time.Time{}, true},
		{"garbage", "yesterday", time.Time{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTimestamp(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && !got.Equal(tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCompleteManyAndCompletedFilter(t *testing.T) {
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
	for i := 1; i <= 3; i++ {
		if _, err := conn.Exec(`INSERT INTO transactions (type, mail_id, amount, current_balance, from_account, to_account, description, timestamp) VALUES ('subtract', 1, 1, 1, 'a', 'b', 'x', '2026-09-26T12:21:30+07:00')`); err != nil {
			t.Fatal(err)
		}
	}

	r := NewTransactionRepository(&database.Database{Conn: conn})
	ctx := context.Background()
	if err := r.CompleteMany(ctx, []int64{1, 3}); err != nil {
		t.Fatal(err)
	}
	if err := r.CompleteMany(ctx, nil); err != nil {
		t.Fatal(err)
	}

	ids := func(completed *bool) []int64 {
		got, err := r.GetRecentTransactionsWithFilters(ctx, 10, 0, TransactionFilters{Completed: completed})
		if err != nil {
			t.Fatal(err)
		}
		var out []int64
		for _, tx := range got {
			out = append(out, tx.ID)
		}
		slices.Sort(out)
		return out
	}
	yes, no := true, false
	if got := ids(&no); !slices.Equal(got, []int64{2}) {
		t.Errorf("pending = %v", got)
	}
	if got := ids(&yes); !slices.Equal(got, []int64{1, 3}) {
		t.Errorf("completed = %v", got)
	}
	if got := ids(nil); !slices.Equal(got, []int64{1, 2, 3}) {
		t.Errorf("all = %v", got)
	}
}

func TestAccountAndCounterpartyRoundTrip(t *testing.T) {
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

	r := NewTransactionRepository(&database.Database{Conn: conn})
	tx := &models.Transaction{MailID: 1, Amount: 1, Currency: "VND", Type: "subtract", Description: "x", Account: "9338613062", Counterparty: "MUJI (0071001345365)", Timestamp: time.Unix(1743476285, 0)}
	if err := r.Create(tx); err != nil {
		t.Fatal(err)
	}

	byID, err := r.GetByID(tx.ID)
	if err != nil {
		t.Fatal(err)
	}
	listed, err := r.GetRecentTransactionsWithFilters(context.Background(), 10, 0, TransactionFilters{})
	if err != nil || len(listed) != 1 {
		t.Fatalf("list = %v, err %v", listed, err)
	}
	for name, got := range map[string]*models.Transaction{"GetByID": byID, "list": listed[0]} {
		if got.Account != "9338613062" || got.Counterparty != "MUJI (0071001345365)" {
			t.Errorf("%s: account/counterparty = %q / %q", name, got.Account, got.Counterparty)
		}
	}
}

func TestPaginationIsStableWhenTimestampsTie(t *testing.T) {
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
	for i := 0; i < 5; i++ {
		if _, err := conn.Exec(`INSERT INTO transactions (type, mail_id, amount, current_balance, from_account, to_account, description, timestamp) VALUES ('subtract', 1, 1, 1, 'a', 'b', 'x', '2026-09-26T12:21:30+07:00')`); err != nil {
			t.Fatal(err)
		}
	}

	r := NewTransactionRepository(&database.Database{Conn: conn})
	var got []int64
	for offset := int64(0); offset < 5; offset += 2 {
		page, err := r.GetRecentTransactionsWithFilters(context.Background(), 2, offset, TransactionFilters{})
		if err != nil {
			t.Fatal(err)
		}
		for _, tx := range page {
			got = append(got, tx.ID)
		}
	}
	if want := []int64{5, 4, 3, 2, 1}; !slices.Equal(got, want) {
		t.Errorf("pages = %v, want %v", got, want)
	}
}
