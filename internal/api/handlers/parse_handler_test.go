package handlers

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/hiiamtrong/go-imap-bot/internal/api/dto"
	"github.com/hiiamtrong/go-imap-bot/internal/billsplit"
	"github.com/hiiamtrong/go-imap-bot/internal/config"
	"github.com/hiiamtrong/go-imap-bot/internal/database"
	"github.com/hiiamtrong/go-imap-bot/internal/llm"
	"github.com/hiiamtrong/go-imap-bot/internal/models"
	"github.com/hiiamtrong/go-imap-bot/internal/repository"
	"github.com/labstack/echo/v4"
	_ "github.com/mattn/go-sqlite3"
)

var repoRoot = func() string {
	wd, _ := os.Getwd()
	return filepath.Join(wd, "../../..")
}()

type parseEnv struct {
	handler   *ParseHandler
	db        *database.Database
	userRepo  *repository.UserRepository
	aliasRepo *repository.AliasRepository
	txRepo    *repository.TransactionRepository
	mailRepo  *repository.MailRepository
	llmBody   *string
	llmCalls  *int
}

func newParseEnv(t *testing.T, llmReply string, llmStatus int) parseEnv {
	t.Helper()

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

	userRepo := repository.NewUserRepository(db)
	for _, name := range []string{"Trọng", "Thương Hà", "son.ho", "D Linh", "linh.pham3"} {
		email := strings.ToLower(strings.NewReplacer(" ", "", ".", "").Replace(name)) + "@example.com"
		if err := userRepo.Create(&models.User{Name: name, Email: email}); err != nil {
			t.Fatal(err)
		}
	}

	env := parseEnv{
		db:        db,
		userRepo:  userRepo,
		aliasRepo: repository.NewAliasRepository(db),
		txRepo:    repository.NewTransactionRepository(db),
		mailRepo:  repository.NewMailRepository(db),
		llmBody:   new(string),
		llmCalls:  new(int),
	}
	llmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*env.llmCalls++
		buf := new(strings.Builder)
		b := make([]byte, 4096)
		for {
			n, err := r.Body.Read(b)
			buf.Write(b[:n])
			if err != nil {
				break
			}
		}
		*env.llmBody = buf.String()
		w.WriteHeader(llmStatus)
		resp, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": llmReply}}}})
		w.Write(resp)
	}))
	t.Cleanup(llmSrv.Close)

	env.handler = NewParseHandler(llm.New(llmSrv.URL, "key", "m"), userRepo, env.aliasRepo, env.txRepo, 1)
	return env
}

func (e parseEnv) addExpense(t *testing.T, amount int64, description string) int64 {
	t.Helper()
	mail := &models.Mail{Subject: "s", From: "f", To: "t"}
	if err := e.mailRepo.Create(mail); err != nil {
		t.Fatal(err)
	}
	tx := &models.Transaction{MailID: mail.ID, Amount: amount, Type: "subtract", Description: description}
	if err := e.txRepo.Create(tx); err != nil {
		t.Fatal(err)
	}
	return tx.ID
}

func call(t *testing.T, h *ParseHandler, req any) (int, dto.Response) {
	t.Helper()
	body, _ := json.Marshal(req)
	rec := httptest.NewRecorder()
	ctx := echo.New().NewContext(httptest.NewRequest(http.MethodPost, "/api/splits/parse", strings.NewReader(string(body))), rec)
	ctx.Request().Header.Set("Content-Type", "application/json")
	if err := h.ParseBill(ctx); err != nil {
		t.Fatal(err)
	}
	var resp dto.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response is not JSON: %s", rec.Body.String())
	}
	return rec.Code, resp
}

func parsed(t *testing.T, resp dto.Response) dto.ParseBillResponse {
	t.Helper()
	raw, _ := json.Marshal(resp.Data)
	var out dto.ParseBillResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

const equalSplit = `{"description":"Cơm trưa","total":160000,"people":[{"name":"Hà","amount":0},{"name":"Sơn","amount":0},{"name":"tôi","amount":0}]}`

func TestParseBillProposesShares(t *testing.T) {
	env := newParseEnv(t, equalSplit, http.StatusOK)

	code, resp := call(t, env.handler, dto.ParseBillRequest{Text: "chia 160k cơm cho Hà, Sơn, tôi"})
	if code != http.StatusOK {
		t.Fatalf("status = %d, error = %q", code, resp.Error)
	}
	got := parsed(t, resp)
	if got.Description != "Cơm trưa" || got.Total != 160000 || len(got.Shares) != 3 {
		t.Fatalf("got %+v", got)
	}
	want := map[string]int64{"Thương Hà": 53334, "son.ho": 53333, "Trọng": 53333}
	for _, s := range got.Shares {
		if want[s.Name] != s.Amount || s.UserID == 0 {
			t.Errorf("share %+v, want amount %d", s, want[s.Name])
		}
	}
}

func TestParseBillUsesTheTransactionTotal(t *testing.T) {
	reply := `{"description":"","total":0,"people":[{"name":"Hà","amount":0},{"name":"Sơn","amount":0}]}`
	env := newParseEnv(t, reply, http.StatusOK)
	id := env.addExpense(t, 100000, "VU XUAN TRONG chuyen tien")

	code, resp := call(t, env.handler, dto.ParseBillRequest{Text: "chia đều cho Hà, Sơn", TransactionID: id})
	if code != http.StatusOK {
		t.Fatalf("status = %d, error = %q", code, resp.Error)
	}
	if got := parsed(t, resp); got.Total != 100000 || got.Description != "VU XUAN TRONG chuyen tien" {
		t.Errorf("got %+v", got)
	}

	env2 := newParseEnv(t, `{"description":"x","total":90000,"people":[{"name":"Hà","amount":0}]}`, http.StatusOK)
	id2 := env2.addExpense(t, 100000, "Cơm")
	if code, resp := call(t, env2.handler, dto.ParseBillRequest{Text: "chia 90k", TransactionID: id2}); code != http.StatusUnprocessableEntity || !strings.Contains(resp.Error, "khác số tiền giao dịch") {
		t.Errorf("mismatch: status = %d, error = %q", code, resp.Error)
	}
}

func TestParseBillRejections(t *testing.T) {
	png := "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte("\x89PNG\r\n\x1a\n...."))
	tests := []struct {
		name  string
		reply string
		req   dto.ParseBillRequest
		code  int
		want  string
	}{
		{"ambiguous name", `{"description":"x","total":10000,"people":[{"name":"Linh","amount":0}]}`, dto.ParseBillRequest{Text: "chia 10k cho Linh"}, 422, "nhiều người khớp"},
		{"unknown name", `{"description":"x","total":10000,"people":[{"name":"Zed","amount":0}]}`, dto.ParseBillRequest{Text: "chia 10k cho Zed"}, 422, "không tìm thấy"},
		{"nobody found", `{"description":"x","total":10000,"people":[]}`, dto.ParseBillRequest{Text: "chia 10k"}, 422, "không thấy người nào"},
		{"nothing provided", equalSplit, dto.ParseBillRequest{}, 400, "Provide a message"},
		{"blank text", equalSplit, dto.ParseBillRequest{Text: "   "}, 400, "Provide a message"},
		{"too many images", equalSplit, dto.ParseBillRequest{Images: []string{png, png, png, png}}, 400, "At most 3"},
		{"bad base64", equalSplit, dto.ParseBillRequest{Images: []string{"data:image/png;base64,@@@"}}, 400, "not valid base64"},
		{"unknown transaction", equalSplit, dto.ParseBillRequest{Text: "x", TransactionID: 9999}, 404, "Transaction not found"},
		{"non-image bytes", equalSplit, dto.ParseBillRequest{Images: []string{base64.StdEncoding.EncodeToString([]byte("hello"))}}, 502, "Could not analyze"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newParseEnv(t, tt.reply, http.StatusOK)
			code, resp := call(t, env.handler, tt.req)
			if code != tt.code || !strings.Contains(resp.Error, tt.want) {
				t.Errorf("status = %d, error = %q; want %d containing %q", code, resp.Error, tt.code, tt.want)
			}
		})
	}
}

func TestParseBillForwardsImages(t *testing.T) {
	env := newParseEnv(t, equalSplit, http.StatusOK)
	raw := base64.StdEncoding.EncodeToString([]byte("\x89PNG\r\n\x1a\n...."))

	for _, image := range []string{"data:image/png;base64," + raw, raw} {
		*env.llmBody = ""
		if code, resp := call(t, env.handler, dto.ParseBillRequest{Text: "chia", Images: []string{image}}); code != http.StatusOK {
			t.Fatalf("status = %d, error = %q", code, resp.Error)
		}
		if !strings.Contains(*env.llmBody, "data:image/png;base64,"+raw) {
			t.Errorf("image was not forwarded to the model for input %.30q", image)
		}
	}
}

func TestParseBillUpstreamFailureDoesNotLeak(t *testing.T) {
	env := newParseEnv(t, "secret upstream detail", http.StatusInternalServerError)

	code, resp := call(t, env.handler, dto.ParseBillRequest{Text: "chia 10k cho Hà"})
	if code != http.StatusBadGateway || strings.Contains(resp.Error, "secret") {
		t.Errorf("status = %d, error = %q", code, resp.Error)
	}
}

func TestParseBillDisabledWithoutLLM(t *testing.T) {
	env := newParseEnv(t, equalSplit, http.StatusOK)
	env.handler.llm = nil

	if code, resp := call(t, env.handler, dto.ParseBillRequest{Text: "chia 10k"}); code != http.StatusServiceUnavailable || !strings.Contains(resp.Error, "not configured") {
		t.Errorf("status = %d, error = %q", code, resp.Error)
	}
	if *env.llmCalls != 0 {
		t.Error("model was called although the feature is disabled")
	}
}

func userIDByName(t *testing.T, env parseEnv, name string) int64 {
	t.Helper()
	users, err := env.userRepo.GetAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range users {
		if u.Name == name {
			return u.ID
		}
	}
	t.Fatalf("no user %q", name)
	return 0
}

func TestParseBillOffersTheModelsMatchForSaving(t *testing.T) {
	// son.ho is the third user newParseEnv creates.
	reply := `{"description":"","total":100000,"people":[{"name":"Ki","amount":0,"user_id":3},{"name":"Hà","amount":0,"user_id":0}]}`
	env := newParseEnv(t, reply, http.StatusOK)

	code, resp := call(t, env.handler, dto.ParseBillRequest{Text: "ki: son.ho, Hà"})
	if code != http.StatusOK {
		t.Fatalf("status = %d, error = %q", code, resp.Error)
	}
	aliases := map[string]string{}
	for _, s := range parsed(t, resp).Shares {
		aliases[s.Name] = s.Alias
	}
	// Hà was matched by token matching, not by the model, so nothing is offered.
	if want := map[string]string{"son.ho": "Ki", "Thương Hà": ""}; !reflect.DeepEqual(aliases, want) {
		t.Errorf("aliases = %v, want %v", aliases, want)
	}
	if saved, _ := env.aliasRepo.GetAll(); len(saved) != 0 {
		t.Errorf("parsing alone must not save anything, saved %v", saved)
	}
	for _, want := range []string{"Người dùng đã biết", "3 | son.ho | sonho", "2 | Thương Hà | thươnghà"} {
		if !strings.Contains(*env.llmBody, want) {
			t.Errorf("the model was not given %q: %s", want, *env.llmBody)
		}
	}
}

func TestParseBillUsesARememberedAlias(t *testing.T) {
	reply := `{"description":"","total":100000,"people":[{"name":"Ki","amount":0,"user_id":0},{"name":"Hà","amount":0,"user_id":0}]}`
	env := newParseEnv(t, reply, http.StatusOK)
	if err := env.aliasRepo.Save(billsplit.AliasKey("Ki"), "Ki", userIDByName(t, env, "son.ho")); err != nil {
		t.Fatal(err)
	}

	code, resp := call(t, env.handler, dto.ParseBillRequest{Text: "Ki, Hà"})
	if code != http.StatusOK {
		t.Fatalf("status = %d, error = %q", code, resp.Error)
	}
	names := map[string]bool{}
	for _, s := range parsed(t, resp).Shares {
		names[s.Name] = true
		if s.Alias != "" {
			t.Errorf("a remembered alias is not offered again: %+v", s)
		}
	}
	if !names["son.ho"] || !names["Thương Hà"] {
		t.Errorf("names = %v", names)
	}
	if !strings.Contains(*env.llmBody, "ki = 3") {
		t.Errorf("the model was not told the remembered name: %s", *env.llmBody)
	}
}
