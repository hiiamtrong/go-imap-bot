package imapbot

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/hiiamtrong/go-imap-bot/internal/models"
)

var transferTime = time.Date(2026, 10, 7, 0, 36, 0, 0, time.FixedZone("ICT", 7*3600))

type settleEnv struct {
	fixture
	user int64
	hash string
	ids  []int64
}

func newSettleEnv(t *testing.T, amounts ...int64) settleEnv {
	t.Helper()
	f := newFixture(t, "")
	env := settleEnv{fixture: f, user: f.addUser(t, "toan.tran2")}
	for _, amount := range amounts {
		txID := f.addExpense(t, amount, "Cơm")
		res, err := f.db.Conn.Exec("INSERT INTO transaction_splits (transaction_id, user_id, amount) VALUES (?, ?, ?)", txID, env.user, amount)
		if err != nil {
			t.Fatal(err)
		}
		id, _ := res.LastInsertId()
		env.ids = append(env.ids, id)
	}
	env.hash = env.newHash(t, env.ids)
	return env
}

func (e settleEnv) newHash(t *testing.T, ids []int64) string {
	t.Helper()
	hash, err := e.bot.BotInjector.SplitHashRepository.GenerateHash(ids)
	if err != nil {
		t.Fatal(err)
	}
	return hash
}

func (e settleEnv) transfer(t *testing.T, amount int64, txType string) *models.Transaction {
	return e.transferAt(t, amount, txType, transferTime)
}

func (e settleEnv) transferAt(t *testing.T, amount int64, txType string, at time.Time) *models.Transaction {
	t.Helper()
	mail := &models.Mail{Subject: "s", From: "f", To: "t", Date: at}
	if err := e.bot.BotInjector.MailRepository.Create(mail); err != nil {
		t.Fatal(err)
	}
	incoming := &models.Transaction{
		MailID:      mail.ID,
		Amount:      amount,
		Type:        txType,
		Timestamp:   at,
		CreatedAt:   at,
		Description: "MBVCB.1641.6280BFTV.Bill " + e.hash + ".CT tu 0021 TRAN VAN THANH chuyen tien",
	}
	if err := e.bot.BotInjector.TransactionRepository.Create(incoming); err != nil {
		t.Fatal(err)
	}

	tx, err := e.db.BeginTx(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	e.bot.SettleSplitPayment(incoming, "me@example.com", tx)
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	return incoming
}

func (e settleEnv) completed(t *testing.T) []bool {
	t.Helper()
	var out []bool
	for _, id := range e.ids {
		var done bool
		if err := e.db.Conn.QueryRow("SELECT completed FROM transaction_splits WHERE id = ?", id).Scan(&done); err != nil {
			t.Fatal(err)
		}
		out = append(out, done)
	}
	return out
}

func (e settleEnv) stillOwed(t *testing.T) int64 {
	t.Helper()
	var sum int64
	if err := e.db.Conn.QueryRow("SELECT COALESCE(SUM(amount), 0) FROM transaction_splits WHERE user_id = ? AND completed = 0", e.user).Scan(&sum); err != nil {
		t.Fatal(err)
	}
	return sum
}

type creditRow struct {
	txID      int64
	amount    int64
	completed bool
	reason    string
}

func (e settleEnv) credits(t *testing.T) []creditRow {
	t.Helper()
	rows, err := e.db.Conn.Query("SELECT transaction_id, amount, completed, reason FROM transaction_splits WHERE amount < 0 ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []creditRow
	for rows.Next() {
		var c creditRow
		if err := rows.Scan(&c.txID, &c.amount, &c.completed, &c.reason); err != nil {
			t.Fatal(err)
		}
		out = append(out, c)
	}
	return out
}

func allEqual(values []bool, want bool) bool {
	for _, v := range values {
		if v != want {
			return false
		}
	}
	return true
}

func TestShortTransferIsRecordedAsACreditNotAClosedBill(t *testing.T) {
	// Six splits owing 402,344 in total, paid with 100,000 under the same hash.
	env := newSettleEnv(t, 55000, 40000, 40000, 40000, 182344, 45000)

	incoming := env.transfer(t, 100000, "add")

	if !allEqual(env.completed(t), false) {
		t.Fatalf("a transfer of 100,000 closed splits owing 402,344: %v", env.completed(t))
	}
	credits := env.credits(t)
	if len(credits) != 1 || credits[0].txID != incoming.ID || credits[0].amount != -100000 || credits[0].completed {
		t.Fatalf("credits = %+v, want one open -100,000 on transaction %d", credits, incoming.ID)
	}
	if !strings.Contains(credits[0].reason, "Đã nhận 100,000") || !strings.Contains(credits[0].reason, "#") {
		t.Errorf("credit reason = %q", credits[0].reason)
	}
	if got := env.stillOwed(t); got != 302344 {
		t.Errorf("still owed = %d, want 302,344", got)
	}

	msgs := env.messages()
	if len(msgs) != 1 {
		t.Fatalf("messages = %+v, want one warning", msgs)
	}
	for _, want := range []string{"100,000", "402,344", "302,344", "toan.tran2", "ghi nhận"} {
		if !strings.Contains(msgs[0].text, want) {
			t.Errorf("warning lacks %q: %s", want, msgs[0].text)
		}
	}
	if strings.Contains(msgs[0].text, "đã được thanh toán") {
		t.Errorf("warning claims the bill was paid: %s", msgs[0].text)
	}
}

func TestReminderTotalShrinksByWhatWasPaid(t *testing.T) {
	env := newSettleEnv(t, 55000, 40000, 40000, 40000, 182344, 45000)
	env.transfer(t, 100000, "add")

	pending, err := env.bot.BotInjector.TransactionSplitRepository.GetPendingSplitsByUserID(env.user)
	if err != nil {
		t.Fatal(err)
	}
	var sum int64
	for _, s := range pending {
		sum += s.Amount
	}
	if len(pending) != 7 || sum != 302344 {
		t.Errorf("reminder lists %d rows adding up to %d, want 7 rows adding up to 302,344", len(pending), sum)
	}
}

func TestMailRepeatedForTheSameTransferDoesNotCloseTheBill(t *testing.T) {
	// 150,000 owed, 100,000 paid. Without the repeat check the second mail would
	// look like it covers the remaining 50,000 and close everything.
	env := newSettleEnv(t, 150000)
	env.transfer(t, 100000, "add")
	before := len(env.messages())

	for _, gap := range []time.Duration{0, 14 * time.Second, 4*time.Minute + 59*time.Second} {
		env.transferAt(t, 100000, "add", transferTime.Add(gap))
	}

	if !allEqual(env.completed(t), false) {
		t.Fatalf("a repeated mail closed the bill: %v", env.completed(t))
	}
	if credits := env.credits(t); len(credits) != 1 {
		t.Fatalf("credits = %+v, want the single original one", credits)
	}
	if got := env.stillOwed(t); got != 50000 {
		t.Errorf("still owed = %d, want 50,000", got)
	}
	msgs := env.messages()[before:]
	if len(msgs) != 3 || !strings.Contains(msgs[1].text, "cách nhau 14 giây") || !strings.Contains(msgs[0].text, "giống giao dịch") {
		t.Errorf("each repeat should say it was ignored: %+v", msgs)
	}
}

func TestSecondTransferOfTheSameAmountLaterIsAnotherPayment(t *testing.T) {
	env := newSettleEnv(t, 55000, 40000, 40000, 40000, 182344, 45000)

	env.transfer(t, 100000, "add")
	env.transferAt(t, 100000, "add", transferTime.Add(5*time.Minute+time.Second))

	if got := env.stillOwed(t); got != 202344 || len(env.credits(t)) != 2 {
		t.Errorf("still owed = %d with %d credits, want 202,344 with 2", got, len(env.credits(t)))
	}
	if !allEqual(env.completed(t), false) {
		t.Errorf("splits = %v", env.completed(t))
	}
}

func TestPayingTheRestWithTheOldHashClosesDebtsAndCredit(t *testing.T) {
	env := newSettleEnv(t, 55000, 40000, 40000, 40000, 182344, 45000)
	env.transfer(t, 100000, "add")

	env.transferAt(t, 302344, "add", transferTime.Add(48*time.Hour))

	if !allEqual(env.completed(t), true) {
		t.Fatalf("debts = %v, want all closed", env.completed(t))
	}
	if credits := env.credits(t); len(credits) != 1 || !credits[0].completed {
		t.Errorf("credits = %+v, want the credit closed with the debts", credits)
	}
	if got := env.stillOwed(t); got != 0 {
		t.Errorf("still owed = %d, want 0", got)
	}
	last := env.messages()[len(env.messages())-1]
	if !strings.Contains(last.text, "đã được thanh toán") {
		t.Errorf("last message = %q", last.text)
	}
}

func TestPayingTheRestWithANewReminderHash(t *testing.T) {
	env := newSettleEnv(t, 55000, 40000, 40000, 40000, 182344, 45000)
	env.transfer(t, 100000, "add")

	pending, err := env.bot.BotInjector.TransactionSplitRepository.GetPendingSplitsByUserID(env.user)
	if err != nil {
		t.Fatal(err)
	}
	var ids []int64
	for _, s := range pending {
		ids = append(ids, s.ID)
	}
	env.hash = env.newHash(t, ids)

	env.transferAt(t, 302344, "add", transferTime.Add(48*time.Hour))

	if got := env.stillOwed(t); got != 0 {
		t.Errorf("still owed = %d, want 0", got)
	}
}

func TestPaymentThatStillFallsShortAfterACreditAddsAnother(t *testing.T) {
	env := newSettleEnv(t, 55000, 40000, 40000, 40000, 182344, 45000)
	env.transfer(t, 100000, "add")

	env.transferAt(t, 200000, "add", transferTime.Add(time.Hour))

	if got := env.stillOwed(t); got != 102344 || len(env.credits(t)) != 2 {
		t.Errorf("still owed = %d with %d credits, want 102,344 with 2", got, len(env.credits(t)))
	}
}

func TestTransferThatCoversTheDebtClosesEverySplit(t *testing.T) {
	env := newSettleEnv(t, 55000, 45000)

	env.transfer(t, 100000, "add")

	if !allEqual(env.completed(t), true) || len(env.credits(t)) != 0 {
		t.Fatalf("splits = %v, credits = %+v", env.completed(t), env.credits(t))
	}
	msgs := env.messages()
	if len(msgs) != 1 || !strings.Contains(msgs[0].text, "đã được thanh toán trong giao dịch #") {
		t.Fatalf("messages = %+v", msgs)
	}
}

func TestTransferOneDongShortIsRecordedAndOneDongOverIsAccepted(t *testing.T) {
	short := newSettleEnv(t, 55000, 45000)
	short.transfer(t, 99999, "add")
	if !allEqual(short.completed(t), false) || short.stillOwed(t) != 1 {
		t.Errorf("99,999 against 100,000: closed = %v, still owed = %d", short.completed(t), short.stillOwed(t))
	}

	over := newSettleEnv(t, 55000, 45000)
	over.transfer(t, 100001, "add")
	if !allEqual(over.completed(t), true) {
		t.Fatalf("100,001 against 100,000 left splits open: %v", over.completed(t))
	}
	msgs := over.messages()
	if len(msgs) != 2 || !strings.Contains(msgs[1].text, "chuyển dư 1₫") {
		t.Errorf("messages = %+v, want the payment notice then a surplus note", msgs)
	}
}

func TestOnlySplitsStillOwedCountTowardsThePayment(t *testing.T) {
	env := newSettleEnv(t, 55000, 45000)
	if _, err := env.db.Conn.Exec("UPDATE transaction_splits SET completed = 1 WHERE id = ?", env.ids[0]); err != nil {
		t.Fatal(err)
	}

	env.transfer(t, 45000, "add")

	if !allEqual(env.completed(t), true) {
		t.Fatalf("paying what was still owed left splits open: %v", env.completed(t))
	}
}

func TestRepeatedMailAfterTheBillWasClosedIsIgnored(t *testing.T) {
	env := newSettleEnv(t, 55000, 45000)

	env.transfer(t, 100000, "add")
	before := len(env.messages())
	env.transfer(t, 100000, "add")

	if got := len(env.messages()); got != before {
		t.Errorf("a duplicate mail produced %d extra message(s)", got-before)
	}
	if !allEqual(env.completed(t), true) || len(env.credits(t)) != 0 {
		t.Errorf("splits = %v, credits = %+v", env.completed(t), env.credits(t))
	}
}

func TestOtherPeoplesCreditsAreLeftAlone(t *testing.T) {
	env := newSettleEnv(t, 55000, 45000)
	other := env.addUser(t, "son.ho")
	txID := env.addExpense(t, 20000, "Phở")
	if _, err := env.db.Conn.Exec("INSERT INTO transaction_splits (transaction_id, user_id, amount) VALUES (?, ?, -20000)", txID, other); err != nil {
		t.Fatal(err)
	}

	env.transfer(t, 100000, "add")

	if !allEqual(env.completed(t), true) {
		t.Fatalf("splits = %v", env.completed(t))
	}
	var open bool
	if err := env.db.Conn.QueryRow("SELECT completed = 0 FROM transaction_splits WHERE user_id = ? AND amount < 0", other).Scan(&open); err != nil || !open {
		t.Errorf("another person's credit was closed (open = %v, err = %v)", open, err)
	}
}

func TestTransfersThatAreNotPaymentsAreIgnored(t *testing.T) {
	t.Run("money going out", func(t *testing.T) {
		env := newSettleEnv(t, 55000, 45000)
		env.transfer(t, 100000, "subtract")
		if !allEqual(env.completed(t), false) || len(env.messages()) != 0 || len(env.credits(t)) != 0 {
			t.Errorf("splits = %v, messages = %+v, credits = %+v", env.completed(t), env.messages(), env.credits(t))
		}
	})

	t.Run("unknown hash", func(t *testing.T) {
		env := newSettleEnv(t, 55000)
		env.hash = "TRXZZZZZZZZONG"
		env.transfer(t, 55000, "add")
		if !allEqual(env.completed(t), false) || len(env.messages()) != 0 {
			t.Errorf("splits = %v, messages = %+v", env.completed(t), env.messages())
		}
	})

	t.Run("no hash in the description", func(t *testing.T) {
		env := newSettleEnv(t, 55000)
		env.hash = "tien com"
		env.transfer(t, 55000, "add")
		if !allEqual(env.completed(t), false) || len(env.messages()) != 0 {
			t.Errorf("splits = %v, messages = %+v", env.completed(t), env.messages())
		}
	})
}
