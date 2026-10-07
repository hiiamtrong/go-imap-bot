package imapbot

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/hiiamtrong/go-imap-bot/internal/billsplit"
	"github.com/hiiamtrong/go-imap-bot/internal/config"
	"github.com/hiiamtrong/go-imap-bot/internal/database"
	"github.com/hiiamtrong/go-imap-bot/internal/llm"
	"github.com/hiiamtrong/go-imap-bot/internal/models"
	"github.com/hiiamtrong/go-imap-bot/internal/repository"
	_ "github.com/mattn/go-sqlite3"
)

const testChatID = 42

var repoRoot = func() string {
	wd, _ := os.Getwd()
	return filepath.Join(wd, "../..")
}()

type sentMessage struct {
	text   string
	markup string
}

type fixture struct {
	bot      *Bot
	db       *database.Database
	llmBill  *string
	llmCalls *int
	mu       *sync.Mutex
	sent     *[]sentMessage
}

func (f fixture) messages() []sentMessage {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]sentMessage(nil), *f.sent...)
}

func (f fixture) last(t *testing.T) sentMessage {
	t.Helper()
	msgs := f.messages()
	if len(msgs) == 0 {
		t.Fatal("bot sent no messages")
	}
	return msgs[len(msgs)-1]
}

func newFixture(t *testing.T, llmBill string) fixture {
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

	var mu sync.Mutex
	var sent []sentMessage
	tg := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		method := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
		switch method {
		case "getMe":
			fmt.Fprint(w, `{"ok":true,"result":{"id":1,"is_bot":true,"first_name":"bot","username":"bot"}}`)
		case "sendMessage":
			mu.Lock()
			sent = append(sent, sentMessage{text: r.Form.Get("text"), markup: r.Form.Get("reply_markup")})
			id := len(sent)
			mu.Unlock()
			fmt.Fprintf(w, `{"ok":true,"result":{"message_id":%d,"date":1,"chat":{"id":%d,"type":"private"}}}`, id, testChatID)
		default:
			fmt.Fprint(w, `{"ok":true,"result":true}`)
		}
	}))
	t.Cleanup(tg.Close)

	api, err := tgbotapi.NewBotAPIWithAPIEndpoint("token", tg.URL+"/bot%s/%s")
	if err != nil {
		t.Fatal(err)
	}

	f := fixture{db: db, llmBill: &llmBill, llmCalls: new(int), mu: &mu, sent: &sent}
	llmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*f.llmCalls++
		resp, _ := json.Marshal(map[string]any{
			"choices": []any{map[string]any{"message": map[string]string{"content": *f.llmBill}}},
		})
		w.Write(resp)
	}))
	t.Cleanup(llmSrv.Close)

	injector := NewBotInjector(
		db,
		repository.NewMailRepository(db),
		repository.NewTransactionRepository(db),
		repository.NewTagRepository(db),
		repository.NewUserRepository(db),
		repository.NewTelegramUserRepository(db),
		repository.NewTransactionSplitRepository(db),
		repository.NewSplitHashRepository(db),
		repository.NewAliasRepository(db),
		nil,
	)
	f.bot = &Bot{
		TelegramBot:    api,
		Context:        context.Background(),
		BotInjector:    injector,
		pendingActions: map[int]PendingAction{},
		selectedUsers:  map[int64]map[int64]bool{},
		llm:            llm.New(llmSrv.URL, "key", "model"),
	}

	if err := injector.TelegramUserRepository.Authorize(testChatID, "me", "me@example.com"); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f fixture) addUser(t *testing.T, name string) int64 {
	t.Helper()
	u := &models.User{Name: name, Email: strings.ToLower(strings.ReplaceAll(name, " ", "")) + "@example.com"}
	if err := f.bot.BotInjector.UserRepository.Create(u); err != nil {
		t.Fatal(err)
	}
	return u.ID
}

func (f fixture) addExpense(t *testing.T, amount int64, description string) int64 {
	t.Helper()
	tx, err := f.bot.createVirtualBill(amount, description)
	if err != nil {
		t.Fatal(err)
	}
	f.db.Conn.Exec("UPDATE transactions SET from_account = 'bank' WHERE id = ?", tx.ID)
	return tx.ID
}

type splitRow struct {
	userID    int64
	amount    int64
	completed bool
	reason    string
}

func (f fixture) splits(t *testing.T, txID int64) []splitRow {
	t.Helper()
	rows, err := f.db.Conn.Query("SELECT user_id, amount, completed, reason FROM transaction_splits WHERE transaction_id = ? ORDER BY id", txID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []splitRow
	for rows.Next() {
		var r splitRow
		if err := rows.Scan(&r.userID, &r.amount, &r.completed, &r.reason); err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	return out
}

func textMessage(text string) *tgbotapi.Message {
	return &tgbotapi.Message{Text: text, Chat: &tgbotapi.Chat{ID: testChatID}}
}

func draftIDFrom(t *testing.T, markup string) int64 {
	t.Helper()
	var id int64
	i := strings.Index(markup, "nlsplit_ok:")
	if i < 0 {
		t.Fatalf("no confirm button in markup %q", markup)
	}
	fmt.Sscanf(markup[i:], "nlsplit_ok:%d", &id)
	return id
}

func TestNaturalLanguageSplitMatchesExistingExpense(t *testing.T) {
	f := newFixture(t, `{"description":"Cơm trưa","total":160000,"people":[{"name":"Hà","amount":0},{"name":"Sơn","amount":0},{"name":"tôi","amount":0}]}`)
	f.bot.selfUserID = f.addUser(t, "Trọng")
	ha, son := f.addUser(t, "Thương Hà"), f.addUser(t, "son.ho")
	txID := f.addExpense(t, 160000, "VU XUAN TRONG chuyen tien")

	f.bot.handleNaturalLanguage(textMessage("chia 160k cơm trưa cho Hà, Sơn, tôi"))

	preview := f.last(t)
	for _, want := range []string{"Đề xuất chia bill", fmt.Sprintf("Giao dịch #%d", txID), "Thương Hà", "son.ho", "Trọng", "53,334"} {
		if !strings.Contains(preview.text, want) {
			t.Errorf("preview missing %q:\n%s", want, preview.text)
		}
	}
	if len(f.splits(t, txID)) != 0 {
		t.Fatal("splits were written before confirmation")
	}

	draftID := draftIDFrom(t, preview.markup)
	f.bot.confirmSplitDraft(testChatID, draftID)

	got := f.splits(t, txID)
	want := []splitRow{
		{ha, 53334, false, "Cơm trưa"},
		{son, 53333, false, "Cơm trưa"},
		{f.bot.selfUserID, 53333, true, "Cơm trưa"},
	}
	if len(got) != len(want) {
		t.Fatalf("splits = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("split %d = %+v, want %+v", i, got[i], want[i])
		}
	}

	before := len(f.messages())
	f.bot.confirmSplitDraft(testChatID, draftID)
	if len(f.splits(t, txID)) != 3 {
		t.Error("second confirm created duplicate splits")
	}
	if msgs := f.messages(); len(msgs) != before+1 || !strings.Contains(msgs[before].text, "hết hạn") {
		t.Errorf("second confirm reply = %+v", msgs[before:])
	}
}

func TestNaturalLanguageSplitCreatesVirtualBill(t *testing.T) {
	f := newFixture(t, `{"description":"Trà sữa","total":0,"people":[{"name":"Hà","amount":40000},{"name":"Sơn","amount":50000}]}`)
	f.addUser(t, "Thương Hà")
	f.addUser(t, "son.ho")

	f.bot.handleNaturalLanguage(textMessage("Hà 40k, Sơn 50k trà sữa"))
	preview := f.last(t)
	if !strings.Contains(preview.text, "Sẽ tạo bill mới") || !strings.Contains(preview.text, "90,000") {
		t.Fatalf("preview = %s", preview.text)
	}

	f.bot.confirmSplitDraft(testChatID, draftIDFrom(t, preview.markup))

	var txID, amount int64
	var from string
	err := f.db.Conn.QueryRow("SELECT id, amount, from_account FROM transactions WHERE description = 'Trà sữa'").Scan(&txID, &amount, &from)
	if err != nil || amount != 90000 || from != "Virtual Bill" {
		t.Fatalf("virtual bill = %d/%d/%q, err %v", txID, amount, from, err)
	}
	if n := len(f.splits(t, txID)); n != 2 {
		t.Errorf("splits = %d, want 2", n)
	}
}

func TestNaturalLanguageSplitRemembersAConfirmedAlias(t *testing.T) {
	f := newFixture(t, "{}")
	son := f.addUser(t, "son.ho")
	f.addUser(t, "Thương Hà")
	aliases := f.bot.BotInjector.AliasRepository

	*f.llmBill = fmt.Sprintf(`{"description":"Nước ép","total":100000,"people":[{"name":"Ki","amount":0,"user_id":%d},{"name":"Hà","amount":0,"user_id":0}]}`, son)
	f.bot.handleNaturalLanguage(textMessage("chia 100k nước ép, ki: son.ho, Hà"))
	preview := f.last(t)
	if !strings.Contains(preview.text, "son.ho (Ki)") {
		t.Fatalf("preview should show the pair it will remember:\n%s", preview.text)
	}
	if saved, _ := aliases.GetAll(); len(saved) != 0 {
		t.Fatalf("alias saved before confirmation: %v", saved)
	}

	f.bot.confirmSplitDraft(testChatID, draftIDFrom(t, preview.markup))
	if saved, _ := aliases.GetAll(); len(saved) != 1 || saved[billsplit.AliasKey("Ki")] != son {
		t.Fatalf("saved = %v, want Ki -> %d", saved, son)
	}

	*f.llmBill = `{"description":"Trà","total":60000,"people":[{"name":"Ki","amount":0,"user_id":0},{"name":"Hà","amount":0,"user_id":0}]}`
	f.bot.handleNaturalLanguage(textMessage("chia 60k trà cho Ki, Hà"))
	if next := f.last(t); !strings.Contains(next.text, "son.ho") || strings.Contains(next.text, "không tìm thấy") {
		t.Errorf("a remembered Ki should resolve to son.ho:\n%s", next.text)
	}
}

func TestNaturalLanguageSplitScalesListPrices(t *testing.T) {
	f := newFixture(t, `{"description":"Nước ép","total":100000,"prorate":true,"people":[{"name":"Hà","amount":70000,"user_id":0},{"name":"Sơn","amount":50000,"user_id":0}]}`)
	f.addUser(t, "Thương Hà")
	f.addUser(t, "son.ho")

	f.bot.handleNaturalLanguage(textMessage("chia 100k nước ép theo số tiền được giảm, Hà 70k, Sơn 50k"))
	preview := f.last(t)
	for _, want := range []string{"58,333", "41,667", "chia lại theo tổng bill"} {
		if !strings.Contains(preview.text, want) {
			t.Errorf("preview missing %q:\n%s", want, preview.text)
		}
	}
}

func TestNaturalLanguageSplitRejects(t *testing.T) {
	tests := []struct {
		name    string
		bill    string
		text    string
		users   []string
		wantMsg string
	}{
		{"ambiguous name", `{"description":"x","total":100000,"people":[{"name":"Linh","amount":0}]}`, "chia 100k cho Linh", []string{"D Linh", "linh.pham3"}, "nhiều người khớp"},
		{"unknown name", `{"description":"x","total":100000,"people":[{"name":"Zed","amount":0}]}`, "chia 100k cho Zed", []string{"Thương Hà"}, "không tìm thấy"},
		{"amounts do not add up", `{"description":"x","total":100000,"people":[{"name":"Hà","amount":10000}]}`, "Hà 10k tổng 100k", []string{"Thương Hà"}, "khác tổng bill"},
		{"missing transaction reference", `{"description":"x","total":100000,"people":[{"name":"Hà","amount":0}]}`, "chia #9999 cho Hà 100k", []string{"Thương Hà"}, "không tìm thấy giao dịch #9999"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t, tt.bill)
			for _, u := range tt.users {
				f.addUser(t, u)
			}
			f.bot.handleNaturalLanguage(textMessage(tt.text))

			reply := f.last(t)
			if !strings.Contains(reply.text, "Chưa thể chia bill") || !strings.Contains(reply.text, tt.wantMsg) {
				t.Errorf("reply = %q, want problem containing %q", reply.text, tt.wantMsg)
			}
			if strings.Contains(reply.markup, "nlsplit_ok") {
				t.Error("confirm button offered for a rejected plan")
			}
		})
	}
}

func TestNaturalLanguageSplitAlreadySplitTransaction(t *testing.T) {
	f := newFixture(t, `{"description":"x","total":0,"people":[{"name":"Hà","amount":0}]}`)
	ha := f.addUser(t, "Thương Hà")
	txID := f.addExpense(t, 100000, "Cơm")
	f.db.Conn.Exec("INSERT INTO transaction_splits (transaction_id, user_id, amount) VALUES (?, ?, 100000)", txID, ha)

	f.bot.handleNaturalLanguage(textMessage(fmt.Sprintf("chia #%d cho Hà", txID)))
	if reply := f.last(t); !strings.Contains(reply.text, "đã có chia bill") {
		t.Errorf("reply = %q", reply.text)
	}
}

func TestNaturalLanguageIgnoresUnauthorizedAndPlainChat(t *testing.T) {
	f := newFixture(t, `{"description":"x","total":1,"people":[]}`)

	stranger := textMessage("chia 100k cho Hà")
	stranger.Chat.ID = 999
	f.bot.handleNaturalLanguage(stranger)
	f.bot.handleNaturalLanguage(textMessage("chào bot"))

	if *f.llmCalls != 0 || len(f.messages()) != 0 {
		t.Errorf("llm calls = %d, messages = %d; want none", *f.llmCalls, len(f.messages()))
	}
}

func TestUntilNext(t *testing.T) {
	ict := time.FixedZone("ICT", 7*3600)
	tests := []struct {
		now  time.Time
		want time.Duration
	}{
		{time.Date(2026, 9, 30, 10, 0, 0, 0, ict), 10 * time.Hour},
		{time.Date(2026, 9, 30, 20, 0, 0, 0, ict), 24 * time.Hour},
		{time.Date(2026, 9, 30, 23, 30, 0, 0, ict), 20*time.Hour + 30*time.Minute},
		{time.Date(2026, 9, 30, 10, 0, 0, 0, ict).UTC(), 10 * time.Hour},
	}
	for _, tt := range tests {
		if got := untilNext(tt.now, 20); got != tt.want {
			t.Errorf("untilNext(%v) = %v, want %v", tt.now, got, tt.want)
		}
	}
}

func mondayNoon(day int) time.Time {
	return time.Date(2026, 9, day, 12, 30, 0, 0, time.FixedZone("ICT", 7*3600))
}

func TestNewTransactionSuspectHint(t *testing.T) {
	f := newFixture(t, "")
	create := func(amount int64, at time.Time) *models.Transaction {
		tx := &models.Transaction{MailID: 1, Amount: amount, Type: "subtract", Currency: "VND", Description: "Cơm", From: "bank", To: "me@example.com", Timestamp: at, CreatedAt: time.Now()}
		if err := f.bot.BotInjector.TransactionRepository.Create(tx); err != nil {
			t.Fatal(err)
		}
		return tx
	}

	suspect := create(160000, mondayNoon(28))
	if err := f.bot.NotifyNewTransaction(suspect, "me@example.com", nil); err != nil {
		t.Fatal(err)
	}
	if got := f.last(t).text; !strings.Contains(got, "Nghi là chi tiêu nhóm") || !strings.Contains(got, fmt.Sprintf("chia #%d", suspect.ID)) {
		t.Errorf("suspect notification lacks hint: %s", got)
	}

	plain := create(20000, mondayNoon(28))
	if err := f.bot.NotifyNewTransaction(plain, "me@example.com", nil); err != nil {
		t.Fatal(err)
	}
	if got := f.last(t).text; strings.Contains(got, "Nghi là chi tiêu nhóm") {
		t.Errorf("small transaction flagged: %s", got)
	}
}

func TestGroupSpendDigest(t *testing.T) {
	f := newFixture(t, "")
	ha := f.addUser(t, "Thương Hà")

	ict := time.FixedZone("ICT", 7*3600)
	lunch := time.Now().In(ict)
	for {
		lunch = time.Date(lunch.Year(), lunch.Month(), lunch.Day(), 12, 30, 0, 0, ict)
		if wd := lunch.Weekday(); lunch.Before(time.Now()) && wd != time.Saturday && wd != time.Sunday {
			break
		}
		lunch = lunch.AddDate(0, 0, -1)
	}

	insert := func(amount int64, description string, completed bool) int64 {
		tx := &models.Transaction{MailID: 1, Amount: amount, Type: "subtract", Currency: "VND", Description: description, From: "bank", To: "me@example.com", Timestamp: lunch, CreatedAt: time.Now()}
		if err := f.bot.BotInjector.TransactionRepository.Create(tx); err != nil {
			t.Fatal(err)
		}
		if completed {
			f.db.Conn.Exec("UPDATE transactions SET completed = 1 WHERE id = ?", tx.ID)
		}
		return tx.ID
	}
	open := insert(150000, "Bún chả", false)
	insert(160000, "Đã xong", true)
	split := insert(170000, "Đã chia", false)
	f.db.Conn.Exec("INSERT INTO transaction_splits (transaction_id, user_id, amount) VALUES (?, ?, 170000)", split, ha)

	f.bot.sendGroupSpendDigest(context.Background())

	msgs := f.messages()
	if len(msgs) != 1 {
		t.Fatalf("digest messages = %d, want 1", len(msgs))
	}
	got := msgs[0].text
	if !strings.Contains(got, fmt.Sprintf("#%d", open)) || strings.Contains(got, "Đã xong") || strings.Contains(got, "Đã chia") {
		t.Errorf("digest = %s", got)
	}
}

func imageServer(status int, size int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		w.Write(make([]byte, size))
	}))
}

func TestFetchImage(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		size    int
		wantErr string
	}{
		{"ok", http.StatusOK, 1024, ""},
		{"exactly at size limit", http.StatusOK, maxPhotoBytes, ""},
		{"one byte past size limit", http.StatusOK, maxPhotoBytes + 1, "larger than"},
		{"empty body", http.StatusOK, 0, "empty body"},
		{"not found", http.StatusNotFound, 10, "status 404"},
		{"server error", http.StatusInternalServerError, 10, "status 500"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := imageServer(tt.status, tt.size)
			defer srv.Close()

			img, err := fetchImage(context.Background(), srv.URL)
			if tt.wantErr == "" {
				if err != nil || len(img) != tt.size {
					t.Fatalf("len = %d, err = %v", len(img), err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestFetchImageDependencyFailures(t *testing.T) {
	srv := imageServer(http.StatusOK, 10)
	url := srv.URL
	srv.Close()
	if _, err := fetchImage(context.Background(), url); err == nil {
		t.Error("expected connection error for a down server")
	}

	live := imageServer(http.StatusOK, 10)
	defer live.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := fetchImage(ctx, live.URL); err == nil {
		t.Error("expected error for a cancelled context")
	}

	if _, err := fetchImage(context.Background(), "://bad url"); err == nil {
		t.Error("expected error for a malformed url")
	}
}

func TestNaturalLanguageLLMFailures(t *testing.T) {
	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer down.Close()

	tests := []struct {
		name string
		bill string
		llm  *llm.Client
		want string
	}{
		{"endpoint returns 500", "", llm.New(down.URL, "key", "m"), "Không phân tích được"},
		{"model answers in prose", "xin lỗi, tôi không hiểu", nil, "Không phân tích được"},
		{"model finds nobody", `{"description":"x","total":100000,"people":[]}`, nil, "không thấy người nào"},
		{"model returns absurd amount", `{"description":"x","total":1e30,"people":[{"name":"Hà","amount":0}]}`, nil, "Không phân tích được"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t, tt.bill)
			f.addUser(t, "Thương Hà")
			if tt.llm != nil {
				f.bot.llm = tt.llm
			}

			f.bot.handleNaturalLanguage(textMessage("chia 100k cho Hà"))

			reply := f.last(t)
			if !strings.Contains(reply.text, tt.want) || strings.Contains(reply.markup, "nlsplit_ok") {
				t.Errorf("reply = %q / %q, want %q and no confirm button", reply.text, reply.markup, tt.want)
			}
			var n int
			f.db.Conn.QueryRow("SELECT COUNT(*) FROM transaction_splits").Scan(&n)
			if n != 0 {
				t.Errorf("splits written after failure: %d", n)
			}
		})
	}
}

func TestNaturalLanguagePhotoDownloadFails(t *testing.T) {
	f := newFixture(t, `{"description":"x","total":1,"people":[]}`)

	msg := textMessage("")
	msg.Caption = "chia bill"
	msg.Photo = []tgbotapi.PhotoSize{{FileID: "abc"}}
	f.bot.handleNaturalLanguage(msg)

	if got := f.last(t).text; !strings.Contains(got, "Không tải được ảnh") {
		t.Errorf("reply = %q", got)
	}
	if *f.llmCalls != 0 {
		t.Errorf("LLM called %d times although the photo could not be downloaded", *f.llmCalls)
	}
}

func TestNaturalLanguageEmptyText(t *testing.T) {
	f := newFixture(t, `{"description":"x","total":1,"people":[]}`)

	f.bot.handleNaturalLanguage(textMessage(""))
	f.bot.handleNaturalLanguage(textMessage("   "))

	if *f.llmCalls != 0 || len(f.messages()) != 0 {
		t.Errorf("llm calls = %d, messages = %d; want none", *f.llmCalls, len(f.messages()))
	}
}

func TestFetchImageErrorsDoNotLeakURL(t *testing.T) {
	srv := imageServer(http.StatusOK, 10)
	secretURL := srv.URL + "/file/botSECRET-TOKEN/photo.jpg"
	srv.Close()

	_, err := fetchImage(context.Background(), secretURL)
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "SECRET-TOKEN") {
		t.Errorf("error leaks the request URL: %v", err)
	}

	_, err = fetchImage(context.Background(), "://botSECRET-TOKEN")
	if err == nil || strings.Contains(err.Error(), "SECRET-TOKEN") {
		t.Errorf("malformed url error leaks the token: %v", err)
	}
}

func TestConfirmRefusesWhenTargetChangedAfterPreview(t *testing.T) {
	previewThen := func(t *testing.T, f fixture, mutate func(txID, userID int64)) (txID int64, draftID int64) {
		t.Helper()
		user := f.addUser(t, "Thương Hà")
		txID = f.addExpense(t, 100000, "Cơm")
		f.bot.handleNaturalLanguage(textMessage(fmt.Sprintf("chia #%d cho Hà 100k", txID)))
		draftID = draftIDFrom(t, f.last(t).markup)
		mutate(txID, user)
		return txID, draftID
	}
	const bill = `{"description":"Cơm","total":100000,"people":[{"name":"Hà","amount":0}]}`

	t.Run("completed after preview", func(t *testing.T) {
		f := newFixture(t, bill)
		txID, draftID := previewThen(t, f, func(txID, _ int64) {
			f.db.Conn.Exec("UPDATE transactions SET completed = 1 WHERE id = ?", txID)
		})
		f.bot.confirmSplitDraft(testChatID, draftID)

		if n := len(f.splits(t, txID)); n != 0 {
			t.Errorf("splits written to a completed transaction: %d", n)
		}
		if got := f.last(t).text; !strings.Contains(got, "đã hoàn thành") {
			t.Errorf("reply = %q", got)
		}
	})

	t.Run("split elsewhere after preview", func(t *testing.T) {
		f := newFixture(t, bill)
		txID, draftID := previewThen(t, f, func(txID, userID int64) {
			f.db.Conn.Exec("INSERT INTO transaction_splits (transaction_id, user_id, amount) VALUES (?, ?, 100000)", txID, userID)
		})
		f.bot.confirmSplitDraft(testChatID, draftID)

		if n := len(f.splits(t, txID)); n != 1 {
			t.Errorf("splits = %d, want the single pre-existing one", n)
		}
	})
}

func TestCompletedTransactionIsNeverATarget(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{"explicit #id", "chia #%d cho Hà", "đã hoàn thành"},
		{"auto match by amount", "chia 100k cơm cho Hà", "Sẽ tạo bill mới"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t, `{"description":"Cơm","total":100000,"people":[{"name":"Hà","amount":0}]}`)
			f.addUser(t, "Thương Hà")
			txID := f.addExpense(t, 100000, "Cơm")
			f.db.Conn.Exec("UPDATE transactions SET completed = 1 WHERE id = ?", txID)

			text := tt.text
			if strings.Contains(text, "%d") {
				text = fmt.Sprintf(text, txID)
			}
			f.bot.handleNaturalLanguage(textMessage(text))

			got := f.last(t)
			if !strings.Contains(got.text, tt.want) || strings.Contains(got.text, fmt.Sprintf("Giao dịch #%d:", txID)) {
				t.Errorf("reply = %q", got.text)
			}
		})
	}
}

func TestVirtualBillRolledBackWhenSplitInsertFails(t *testing.T) {
	f := newFixture(t, `{"description":"Trà sữa","total":0,"people":[{"name":"Hà","amount":40000}]}`)
	f.addUser(t, "Thương Hà")
	f.bot.handleNaturalLanguage(textMessage("Hà 40k trà sữa"))
	draftID := draftIDFrom(t, f.last(t).markup)

	if _, err := f.db.Conn.Exec(`CREATE TRIGGER reject_split BEFORE INSERT ON transaction_splits BEGIN SELECT RAISE(ABORT, 'boom'); END`); err != nil {
		t.Fatal(err)
	}
	f.bot.confirmSplitDraft(testChatID, draftID)

	var virtualBills, virtualMails int
	f.db.Conn.QueryRow("SELECT COUNT(*) FROM transactions WHERE from_account = 'Virtual Bill'").Scan(&virtualBills)
	f.db.Conn.QueryRow("SELECT COUNT(*) FROM mails WHERE subject = 'Virtual Bill'").Scan(&virtualMails)
	if virtualBills != 0 || virtualMails != 0 {
		t.Errorf("orphans left after failure: %d transactions, %d mails", virtualBills, virtualMails)
	}
	if got := f.last(t).text; !strings.Contains(got, "Không thể lưu chia bill") {
		t.Errorf("reply = %q", got)
	}
}

func TestNaturalLanguageSplitFoldsCoveredSharesIntoThePayer(t *testing.T) {
	f := newFixture(t, `{"description":"Cơm","total":0,"prorate":false,"adjustments":[],"covers":[{"name":"Linh","user_id":0,"for":[{"name":"Hà","user_id":0}]}],"people":[{"name":"Hà","amount":40000,"user_id":0},{"name":"Sơn","amount":50000,"user_id":0}]}`)
	f.addUser(t, "Thương Hà")
	son, linh := f.addUser(t, "son.ho"), f.addUser(t, "D Linh")

	f.bot.handleNaturalLanguage(textMessage("Hà 40k, Sơn 50k, Linh chịu tiền cho Hà"))
	preview := f.last(t)
	if !strings.Contains(preview.text, "D Linh — gồm phần của Thương Hà: 40,000") || strings.Contains(preview.text, "• Thương Hà:") {
		t.Fatalf("preview = %s", preview.text)
	}

	f.bot.confirmSplitDraft(testChatID, draftIDFrom(t, preview.markup))

	var txID int64
	if err := f.db.Conn.QueryRow("SELECT id FROM transactions WHERE description = 'Cơm'").Scan(&txID); err != nil {
		t.Fatal(err)
	}
	got := f.splits(t, txID)
	want := []splitRow{{son, 50000, false, "Cơm"}, {linh, 40000, false, "Cơm (gồm phần của Thương Hà)"}}
	if len(got) != len(want) {
		t.Fatalf("splits = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("split %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}
