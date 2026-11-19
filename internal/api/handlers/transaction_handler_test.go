package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/hiiamtrong/go-imap-bot/internal/api/dto"
	"github.com/hiiamtrong/go-imap-bot/internal/config"
	"github.com/hiiamtrong/go-imap-bot/internal/database"
	"github.com/hiiamtrong/go-imap-bot/internal/models"
	"github.com/hiiamtrong/go-imap-bot/internal/repository"
	"github.com/labstack/echo/v4"
)

func TestToTransactionDTOSuspectedGroupSpend(t *testing.T) {
	lunch := time.Date(2026, 9, 28, 12, 30, 0, 0, time.FixedZone("ICT", 7*3600))
	tests := []struct {
		name string
		tx   models.Transaction
		want bool
	}{
		{"open weekday lunch expense", models.Transaction{Type: "subtract", Amount: 160000, Description: "Cơm", Timestamp: lunch}, true},
		{"same expense once completed", models.Transaction{Type: "subtract", Amount: 160000, Description: "Cơm", Completed: true, Timestamp: lunch}, false},
		{"small expense", models.Transaction{Type: "subtract", Amount: 20000, Description: "Cà phê", Timestamp: lunch}, false},
		{"money in", models.Transaction{Type: "add", Amount: 160000, Description: "Cơm", Timestamp: lunch}, false},
	}
	h := &TransactionHandler{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := h.toTransactionDTO(&tt.tx).SuspectedGroupSpend; got != tt.want {
				t.Errorf("SuspectedGroupSpend = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetTransactionReturnsSourceMail(t *testing.T) {
	wd, _ := os.Getwd()
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(wd) })
	db, err := database.GetDatabase(&config.DatabaseConfig{DatabasePath: filepath.Join(t.TempDir(), "test.db")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Conn.Close() })

	mailRepo := repository.NewMailRepository(db)
	txRepo := repository.NewTransactionRepository(db)
	mail := &models.Mail{UID: 7, Subject: "Biến động số dư", From: "bank@x.vn", To: "me@x.com", Date: time.Unix(1743476285, 0), Body: "<b>- 10 USD</b>"}
	if err := mailRepo.Create(mail); err != nil {
		t.Fatal(err)
	}
	withBody := &models.Transaction{MailID: mail.ID, Amount: 10, Currency: "USD", From: mail.From, To: mail.To, Description: "x", Type: "subtract", Account: "9338613062", Counterparty: "MUJI (0071001345365)", Timestamp: mail.Date}
	if err := txRepo.Create(withBody); err != nil {
		t.Fatal(err)
	}
	orphan := &models.Transaction{MailID: 999, Amount: 1, Currency: "VND", From: "a", To: "b", Description: "y", Type: "add", Timestamp: mail.Date}
	if err := txRepo.Create(orphan); err != nil {
		t.Fatal(err)
	}

	h := NewTransactionHandler(txRepo, repository.NewTagRepository(db), repository.NewTransactionSplitRepository(db), repository.NewUserRepository(db), mailRepo)
	get := func(id int64) dto.TransactionResponse {
		e := echo.New()
		c := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), httptest.NewRecorder())
		c.SetParamNames("id")
		c.SetParamValues(strconv.FormatInt(id, 10))
		if err := h.GetTransaction(c); err != nil {
			t.Fatal(err)
		}
		rec := c.Response().Writer.(*httptest.ResponseRecorder)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d: %s", rec.Code, rec.Body)
		}
		var out struct{ Data dto.TransactionResponse }
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out.Data
	}

	got := get(withBody.ID)
	if got.From != "bank@x.vn" || got.To != "me@x.com" {
		t.Errorf("from/to = %q/%q", got.From, got.To)
	}
	if got.Account != "9338613062" || got.Counterparty != "MUJI (0071001345365)" {
		t.Errorf("account/counterparty = %q / %q", got.Account, got.Counterparty)
	}
	if got.Mail == nil || got.Mail.Subject != "Biến động số dư" || got.Mail.Body != "<b>- 10 USD</b>" {
		t.Errorf("mail = %+v", got.Mail)
	}
	if got := get(orphan.ID); got.Mail != nil || got.From != "a" {
		t.Errorf("transaction without a mail row: mail = %+v, from = %q", got.Mail, got.From)
	}
}
