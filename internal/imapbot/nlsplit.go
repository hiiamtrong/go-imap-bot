package imapbot

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/hiiamtrong/go-imap-bot/internal/billsplit"
	"github.com/hiiamtrong/go-imap-bot/internal/groupspend"
	"github.com/hiiamtrong/go-imap-bot/internal/llm"
	"github.com/hiiamtrong/go-imap-bot/internal/models"
	"github.com/hiiamtrong/go-imap-bot/pkg/currencypkg"
)

const (
	draftTTL      = time.Hour
	maxPhotoBytes = 10 << 20
)

var (
	txRef          = regexp.MustCompile(`#(\d+)`)
	errTargetTaken = errors.New("target transaction is completed or already split")
)

type splitDraft struct {
	chatID      int64
	txID        int64
	description string
	plan        billsplit.Plan
	createdAt   time.Time
}

type draftStore struct {
	mu     sync.Mutex
	next   int64
	drafts map[int64]*splitDraft
}

func (s *draftStore) put(d *splitDraft) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.drafts == nil {
		s.drafts = make(map[int64]*splitDraft)
	}
	for id, old := range s.drafts {
		if time.Since(old.createdAt) > draftTTL {
			delete(s.drafts, id)
		}
	}
	s.next++
	s.drafts[s.next] = d
	return s.next
}

func (s *draftStore) take(id, chatID int64) *splitDraft {
	s.mu.Lock()
	defer s.mu.Unlock()

	d, ok := s.drafts[id]
	if !ok || d.chatID != chatID || time.Since(d.createdAt) > draftTTL {
		return nil
	}
	delete(s.drafts, id)
	return d
}

func (b *Bot) handleNaturalLanguage(msg *tgbotapi.Message) {
	if b.llm == nil {
		return
	}
	chatID := msg.Chat.ID
	if ok, err := b.isUserAuthorized(chatID); err != nil || !ok {
		return
	}

	text := msg.Text
	if text == "" {
		text = msg.Caption
	}
	sheetURL, hasSheet := billsplit.SheetExportURL(text)
	if len(msg.Photo) == 0 && !hasSheet && !strings.ContainsAny(text, "0123456789") {
		return
	}

	ctx, cancel := context.WithTimeout(b.Context, 2*time.Minute)
	defer cancel()

	in := llm.Input{Text: text}
	if hasSheet {
		csv, err := billsplit.FetchSheetCSV(ctx, sheetURL)
		if err != nil {
			b.SendMessage(chatID, "❌ "+escapeMarkdown(err.Error()))
			return
		}
		in.Sheet = csv
	}
	if len(msg.Photo) > 0 {
		img, err := b.downloadPhoto(ctx, msg.Photo[len(msg.Photo)-1].FileID)
		if err != nil {
			log.Printf("Error downloading photo: %v", err)
			b.SendMessage(chatID, "❌ Không tải được ảnh, vui lòng thử lại.")
			return
		}
		in.Images = [][]byte{img}
	}

	users, err := b.BotInjector.UserRepository.GetAll()
	if err != nil {
		log.Printf("Error getting users: %v", err)
		b.SendMessage(chatID, "Không thể lấy danh sách người dùng.")
		return
	}

	aliases, err := b.BotInjector.AliasRepository.GetAll()
	if err != nil {
		log.Printf("Error getting aliases: %v", err)
		b.SendMessage(chatID, "Không thể lấy danh sách người dùng.")
		return
	}

	in.Users, in.Aliases = billsplit.KnownUsers(users), aliases
	bill, err := b.llm.ParseBill(ctx, in)
	if err != nil {
		log.Printf("Error parsing bill with LLM: %v", err)
		b.SendMessage(chatID, "❌ Không phân tích được nội dung chia bill, vui lòng thử lại.")
		return
	}
	b.previewSplit(ctx, chatID, text, bill, users, aliases)
}

func (b *Bot) previewSplit(ctx context.Context, chatID int64, text string, bill *llm.Bill, users []*models.User, aliases map[string]int64) {
	target, problem := b.pickTarget(ctx, text, bill.Total)
	if problem != "" {
		b.sendSplitProblems(chatID, []string{problem})
		return
	}

	plan, description, problems := billsplit.ForBill(bill, users, b.selfUserID, target, aliases)
	if len(problems) > 0 {
		b.sendSplitProblems(chatID, problems)
		return
	}

	draft := &splitDraft{chatID: chatID, description: description, plan: plan, createdAt: time.Now()}
	var source string
	if target != nil {
		draft.txID = target.ID
		source = fmt.Sprintf("Giao dịch #%d: %s (%s)", target.ID,
			escapeMarkdown(target.Description), target.Timestamp.In(groupspend.ICT).Format("02/01 15:04"))
	} else {
		source = "Sẽ tạo bill mới"
	}
	draftID := b.drafts.put(draft)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🤖 *Đề xuất chia bill*\n%s — %s\n%s\n\n",
		escapeMarkdown(description), escapeMarkdown(formatVND(plan.Total)), source))
	for _, s := range plan.Shares {
		label := s.Name
		if s.Alias != "" {
			label += " (" + s.Alias + ")"
		}
		if len(s.Covers) > 0 {
			label += " — gồm phần của " + strings.Join(s.Covers, ", ")
		}
		sb.WriteString(fmt.Sprintf("• %s: %s\n", escapeMarkdown(label), escapeMarkdown(formatVND(s.Amount))))
	}
	if plan.Prorated {
		sb.WriteString("\n_Các phần đã được chia lại theo tổng bill (sau giảm giá, phí)._\n")
	}

	msg := tgbotapi.NewMessage(chatID, sb.String())
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("✅ Xác nhận", fmt.Sprintf("nlsplit_ok:%d", draftID)),
		tgbotapi.NewInlineKeyboardButtonData("❌ Hủy", fmt.Sprintf("nlsplit_cancel:%d", draftID)),
	))
	b.Send(msg)
}

func (b *Bot) sendSplitProblems(chatID int64, problems []string) {
	var sb strings.Builder
	sb.WriteString("⚠️ Chưa thể chia bill:\n")
	for _, p := range problems {
		sb.WriteString("- " + escapeMarkdown(p) + "\n")
	}
	sb.WriteString("\nBạn chỉnh lại tin nhắn rồi gửi lại nhé.")
	b.SendMessage(chatID, sb.String())
}

// pickTarget returns the unsplit transaction the bill belongs to, or nil when a
// new virtual bill should be created. A non-empty problem stops the flow.
func (b *Bot) pickTarget(ctx context.Context, text string, total int64) (*models.Transaction, string) {
	txRepo := b.BotInjector.TransactionRepository

	if m := txRef.FindStringSubmatch(text); m != nil {
		id, _ := strconv.ParseInt(m[1], 10, 64)
		tx, err := txRepo.GetByID(id)
		if err != nil {
			return nil, fmt.Sprintf("không tìm thấy giao dịch #%d", id)
		}
		if tx.Completed {
			return nil, fmt.Sprintf("giao dịch #%d đã hoàn thành, không thể chia bill", id)
		}
		if splits, err := txRepo.GetSplitsForTransaction(id); err != nil || len(splits) > 0 {
			return nil, fmt.Sprintf("giao dịch #%d đã có chia bill, hãy Reset trước trong /transactions", id)
		}
		return tx, ""
	}

	recent, err := txRepo.GetRecentTransactions(ctx, 30, 0, nil)
	if err != nil {
		log.Printf("Error getting recent transactions: %v", err)
		return nil, ""
	}
	for _, tx := range recent {
		if tx.Completed || tx.Type != string(models.TransactionTypeSubtract) || time.Since(tx.Timestamp) > 7*24*time.Hour {
			continue
		}
		if total != 0 && tx.Amount != total {
			continue
		}
		if splits, err := txRepo.GetSplitsForTransaction(tx.ID); err == nil && len(splits) == 0 {
			return tx, ""
		}
	}
	return nil, ""
}

func (b *Bot) confirmSplitDraft(chatID, draftID int64) {
	d := b.drafts.take(draftID, chatID)
	if d == nil {
		b.SendMessage(chatID, "Đề xuất đã hết hạn hoặc đã được xử lý.")
		return
	}

	var txID int64
	err := b.BotInjector.TransactionSplitRepository.Transaction(func(tx *sql.Tx) error {
		txID = d.txID
		if txID == 0 {
			bill, err := b.createVirtualBillTx(tx, d.plan.Total, d.description)
			if err != nil {
				return err
			}
			txID = bill.ID
		} else {
			var completed bool
			var existing int
			err := tx.QueryRow(
				`SELECT completed, (SELECT COUNT(*) FROM transaction_splits WHERE transaction_id = ?) FROM transactions WHERE id = ?`,
				txID, txID,
			).Scan(&completed, &existing)
			if err != nil {
				return err
			}
			if completed || existing > 0 {
				return errTargetTaken
			}
		}

		for _, s := range d.plan.Shares {
			split := &models.TransactionSplit{TransactionID: txID, UserID: s.UserID, Amount: s.Amount, Reason: s.Reason(d.description)}
			if err := b.BotInjector.TransactionSplitRepository.CreateTx(tx, split); err != nil {
				return err
			}
			if b.selfUserID != 0 && s.UserID == b.selfUserID {
				if err := b.BotInjector.TransactionSplitRepository.UpdateSplitStatus([]int64{split.ID}, tx); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if errors.Is(err, errTargetTaken) {
		b.SendMessage(chatID, fmt.Sprintf("Giao dịch #%d đã hoàn thành hoặc vừa được chia bill ở nơi khác, đã hủy đề xuất.", d.txID))
		return
	}
	if err != nil {
		log.Printf("Error saving splits: %v", err)
		b.SendMessage(chatID, "Không thể lưu chia bill. Hãy gửi lại tin nhắn chia bill.")
		return
	}

	b.saveAliases(d.plan)
	b.SendMessage(chatID, fmt.Sprintf("✅ Đã chia bill cho giao dịch #%d (%d người)", txID, len(d.plan.Shares)))
	b.handleBackToTransaction(chatID, txID)
}

// saveAliases remembers the bill names the user just confirmed. A failure is
// logged, not reported: the split is already saved.
func (b *Bot) saveAliases(plan billsplit.Plan) {
	for _, s := range plan.Shares {
		if s.Alias == "" {
			continue
		}
		if err := b.BotInjector.AliasRepository.Save(billsplit.AliasKey(s.Alias), s.Alias, s.UserID); err != nil {
			log.Printf("Error saving alias %q: %v", s.Alias, err)
		}
	}
}

func (b *Bot) downloadPhoto(ctx context.Context, fileID string) ([]byte, error) {
	file, err := b.TelegramBot.GetFile(tgbotapi.FileConfig{FileID: fileID})
	if err != nil {
		return nil, stripURL(err)
	}

	return fetchImage(ctx, file.Link(b.TelegramBot.Token))
}

func fetchImage(ctx context.Context, fileURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		return nil, stripURL(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, stripURL(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download photo: status %d", resp.StatusCode)
	}
	img, err := io.ReadAll(io.LimitReader(resp.Body, maxPhotoBytes+1))
	if err != nil {
		return nil, err
	}
	if len(img) == 0 {
		return nil, fmt.Errorf("download photo: empty body")
	}
	if len(img) > maxPhotoBytes {
		return nil, fmt.Errorf("download photo: larger than %d bytes", maxPhotoBytes)
	}
	return img, nil
}

func stripURL(err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return urlErr.Err
	}
	return err
}

func formatVND(amount int64) string {
	return currencypkg.FormatCurrency(float64(amount), "VND")
}
