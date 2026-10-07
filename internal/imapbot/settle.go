package imapbot

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/hiiamtrong/go-imap-bot/internal/models"
	"github.com/hiiamtrong/go-imap-bot/pkg/regexpkg"
)

// Banks often send two mails for one transfer, up to about a minute apart
// (45 pairs in the history, none with different amounts).
const duplicateWindow = 5 * time.Minute

// SettleSplitPayment closes the splits a transfer points to through the hash in
// its description once the transfer covers everything still owed on them.
//
// A transfer that falls short closes nothing, since nothing says which splits it
// was meant for. It is recorded as a negative split ("credit") on the transfer's
// own transaction, so the person's open total shrinks by what they already paid.
// Credits are closed together with the debts they were set against.
func (b *Bot) SettleSplitPayment(transaction *models.Transaction, recipientEmail string, tx *sql.Tx) {
	if transaction.Type != string(models.TransactionTypeAdd) || transaction.Amount <= 0 {
		return
	}
	hash, err := regexpkg.ExtractHash(transaction.Description)
	if err != nil {
		return
	}

	repo := b.BotInjector.TransactionSplitRepository
	splitIDs, err := b.BotInjector.SplitHashRepository.GetSplitIDs(hash)
	if err != nil {
		log.Printf("split hash not found (skipping split): %v", err)
		return
	}
	pending, err := repo.GetPendingByIDs(splitIDs, tx)
	if err != nil {
		log.Printf("failed to load pending splits: %v", err)
		return
	}
	if len(pending) == 0 {
		log.Printf("transaction %d repeats a payment for hash %s that is already settled", transaction.ID, hash)
		return
	}

	var userIDs []int64
	known := map[int64]bool{}
	open := map[int64]bool{}
	for _, split := range pending {
		open[split.ID] = true
		if !known[split.UserID] {
			known[split.UserID] = true
			userIDs = append(userIDs, split.UserID)
		}
	}

	// Checked before the amount: once a credit exists, a repeated mail would look
	// like it covers the remaining debt and close the whole bill.
	if repeated, err := b.repeatedPayment(transaction, userIDs, recipientEmail, tx); err != nil {
		log.Printf("failed to check for a repeated payment: %v", err)
	} else if repeated {
		return
	}

	credits, err := repo.GetPendingCreditsByUserIDs(userIDs, tx)
	if err != nil {
		log.Printf("failed to load pending credits: %v", err)
		return
	}
	for _, credit := range credits {
		if !open[credit.ID] {
			pending = append(pending, credit)
		}
	}

	var owed int64
	pendingIDs := make([]int64, len(pending))
	for i, split := range pending {
		owed += split.Amount
		pendingIDs[i] = split.ID
	}

	if transaction.Amount < owed {
		b.recordShortPayment(transaction, pending[0].UserID, owed, recipientEmail, tx)
		return
	}

	if err := repo.UpdateSplitStatus(pendingIDs, tx); err != nil {
		log.Printf("failed to update split status: %v", err)
		return
	}
	if err := b.NotifySplitBillComplete(recipientEmail, pendingIDs, transaction.ID, tx); err != nil {
		log.Printf("failed to notify split bill: %v", err)
	}
	if extra := transaction.Amount - owed; extra > 0 {
		text := fmt.Sprintf("ℹ️ Giao dịch #%d chuyển dư %s so với các khoản chia bill (%s).",
			transaction.ID, escapeMarkdown(formatVND(extra)), escapeMarkdown(formatVND(owed)))
		if err := b.notifyOwners(recipientEmail, tx, text); err != nil {
			log.Printf("failed to notify overpayment: %v", err)
		}
	}
}

func (b *Bot) repeatedPayment(transaction *models.Transaction, userIDs []int64, email string, tx *sql.Tx) (bool, error) {
	payments, err := b.BotInjector.TransactionSplitRepository.GetCreditPayments(userIDs, transaction.Amount, transaction.ID, tx)
	if err != nil {
		return false, err
	}
	for _, payment := range payments {
		gap := transaction.Timestamp.Sub(payment.At).Abs()
		if gap > duplicateWindow {
			continue
		}
		text := fmt.Sprintf("ℹ️ Giao dịch #%d giống giao dịch #%d (cùng số tiền %s, cách nhau %d giây) nên không ghi nhận thêm. "+
			"Nếu đây là lần chuyển thứ hai, hãy ghi nhận thủ công.",
			transaction.ID, payment.TransactionID, escapeMarkdown(formatVND(transaction.Amount)), int(gap.Seconds()))
		if err := b.notifyOwners(email, tx, text); err != nil {
			log.Printf("failed to notify repeated payment: %v", err)
		}
		return true, nil
	}
	return false, nil
}

func (b *Bot) recordShortPayment(transaction *models.Transaction, userID, owed int64, email string, tx *sql.Tx) {
	who := fmt.Sprintf("người dùng #%d", userID)
	if user, err := b.BotInjector.UserRepository.GetByID(userID); err == nil {
		who = user.Name
	}

	credit := &models.TransactionSplit{
		TransactionID: transaction.ID,
		UserID:        userID,
		Amount:        -transaction.Amount,
		Reason:        fmt.Sprintf("Đã nhận %s (giao dịch #%d)", formatVND(transaction.Amount), transaction.ID),
	}
	outcome := fmt.Sprintf("Đã ghi nhận khoản đã nhận, tổng còn nợ %s.", escapeMarkdown(formatVND(owed-transaction.Amount)))
	if err := b.BotInjector.TransactionSplitRepository.CreateTx(tx, credit); err != nil {
		log.Printf("failed to record the credit: %v", err)
		outcome = "Chưa ghi nhận được khoản đã nhận, bạn xử lý thủ công trên web."
	}

	text := fmt.Sprintf("⚠️ Giao dịch #%d nhận %s, chưa đủ cho các khoản chia bill của %s (còn nợ %s). %s",
		transaction.ID, escapeMarkdown(formatVND(transaction.Amount)), escapeMarkdown(who), escapeMarkdown(formatVND(owed)), outcome)
	if err := b.notifyOwners(email, tx, text); err != nil {
		log.Printf("failed to notify short payment: %v", err)
	}
}

func (b *Bot) notifyOwners(email string, tx *sql.Tx, text string) error {
	chatIDs, err := b.BotInjector.TelegramUserRepository.GetChatIDsByEmail(email, tx)
	if err != nil {
		return err
	}
	for _, chatID := range chatIDs {
		if err := b.SendMessage(chatID, text); err != nil {
			return err
		}
	}
	return nil
}
