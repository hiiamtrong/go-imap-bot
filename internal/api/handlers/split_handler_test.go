package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/hiiamtrong/go-imap-bot/internal/api/dto"
	"github.com/hiiamtrong/go-imap-bot/internal/billsplit"
	"github.com/hiiamtrong/go-imap-bot/internal/repository"
	"github.com/labstack/echo/v4"
)

func TestCreateSplitSavesConfirmedAliases(t *testing.T) {
	env := newParseEnv(t, "{}", http.StatusOK)
	h := NewSplitHandler(repository.NewTransactionSplitRepository(env.db), env.txRepo, env.userRepo, env.aliasRepo, nil, repository.NewSplitHashRepository(env.db))
	txID := env.addExpense(t, 100000, "Nước ép")
	son, ha, linh := userIDByName(t, env, "son.ho"), userIDByName(t, env, "Thương Hà"), userIDByName(t, env, "D Linh")

	body, _ := json.Marshal(dto.CreateSplitRequest{
		TransactionID: txID,
		Users:         []dto.SplitUserRequest{{UserID: son, Amount: 50000}, {UserID: ha, Amount: 50000}},
		Aliases: []dto.AliasRequest{
			{Alias: " Ki ", UserID: son},
			{Alias: "Hồngg Ngọc", UserID: ha},
			{Alias: "Dropped", UserID: linh}, // deselected before confirming
			{Alias: "???", UserID: son},      // no letters to key on
		},
	})
	rec := httptest.NewRecorder()
	ctx := echo.New().NewContext(httptest.NewRequest(http.MethodPost, "/api/splits", strings.NewReader(string(body))), rec)
	ctx.Request().Header.Set("Content-Type", "application/json")
	if err := h.CreateSplit(ctx); err != nil || rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, err = %v, body = %s", rec.Code, err, rec.Body)
	}

	got, err := env.aliasRepo.GetAll()
	want := map[string]int64{billsplit.AliasKey("Ki"): son, billsplit.AliasKey("Hồngg Ngọc"): ha}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("saved = %v (err %v), want %v", got, err, want)
	}
}
