package imapbot

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/hiiamtrong/go-imap-bot/internal/groupspend"
	"github.com/hiiamtrong/go-imap-bot/pkg/currencypkg"
)

const (
	digestHour = 20
)

func untilNext(now time.Time, hour int) time.Duration {
	n := now.In(groupspend.ICT)
	next := time.Date(n.Year(), n.Month(), n.Day(), hour, 0, 0, 0, groupspend.ICT)
	if !next.After(n) {
		next = next.AddDate(0, 0, 1)
	}
	return next.Sub(n)
}

func (b *Bot) runGroupSpendDigest(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(untilNext(time.Now(), digestHour)):
			b.sendGroupSpendDigest(ctx)
		}
	}
}

func (b *Bot) sendGroupSpendDigest(ctx context.Context) {
	recent, err := b.BotInjector.TransactionRepository.GetRecentTransactions(ctx, 100, 0, nil)
	if err != nil {
		log.Printf("Error getting transactions for digest: %v", err)
		return
	}

	var lines []string
	for _, t := range recent {
		if time.Since(t.Timestamp) > 72*time.Hour {
			break
		}
		if t.Completed || !groupspend.Suspect(t) {
			continue
		}
		splits, err := b.BotInjector.TransactionRepository.GetSplitsForTransaction(t.ID)
		if err != nil || len(splits) > 0 {
			continue
		}
		lines = append(lines, fmt.Sprintf("• #%d %s — %s (%s)",
			t.ID,
			escapeMarkdown(currencypkg.FormatCurrency(float64(t.Amount), "VND")),
			escapeMarkdown(t.Description),
			t.Timestamp.In(groupspend.ICT).Format("02/01 15:04")))
	}
	if len(lines) == 0 {
		return
	}

	hint := "Mở /transactions và bấm \"Chia bill\"."
	if b.llm != nil {
		hint = "Nhắn ví dụ: chia #ID cho Hà, Sơn, Tùng (hoặc gửi ảnh/link sheet). Không phải bill nhóm thì bấm \"Hoàn thành\" ở giao dịch."
	}
	message := "👥 *Chi tiêu nghi là hội nhóm, chưa chia bill*\n\n" + strings.Join(lines, "\n") + "\n\n" + escapeMarkdown(hint)

	chatIDs, err := b.BotInjector.TelegramUserRepository.GetAllChatIDs()
	if err != nil {
		log.Printf("Error getting chat IDs for digest: %v", err)
		return
	}
	for _, chatID := range chatIDs {
		if err := b.SendMessage(chatID, message); err != nil {
			log.Printf("Error sending digest to chat %d: %v", chatID, err)
		}
	}
}
