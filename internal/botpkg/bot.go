package botpkg

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/hiiamtrong/imap-bot-go/internal/config"
	"github.com/hiiamtrong/imap-bot-go/internal/models"
	"github.com/hiiamtrong/imap-bot-go/pkg/currency"
)

type Bot struct {
	TelegramBot    *tgbotapi.BotAPI
	Context        context.Context
	BotInjector    *BotInjector
	updates        tgbotapi.UpdatesChannel
	pendingActions map[int]PendingAction
	lastMessageID  int
	selectedUsers  map[int64]map[int64]bool // map[transactionID]map[userID]selected
}

type PendingAction struct {
	Type          string
	TransactionID int64
	UserID        int64
}

func InitBot(cfg *config.Config, ctx context.Context, injector *BotInjector) *Bot {
	bot := &Bot{
		Context:        ctx,
		BotInjector:    injector,
		pendingActions: make(map[int]PendingAction),
		selectedUsers:  make(map[int64]map[int64]bool),
	}

	// Initialize Telegram bot
	telegramBot, err := tgbotapi.NewBotAPI(cfg.TelegramBot.Token)
	if err != nil {
		log.Fatalf("Failed to initialize Telegram bot: %v", err)
	}

	bot.TelegramBot = telegramBot

	// Set up bot commands
	commands := []tgbotapi.BotCommand{
		{
			Command:     "transactions",
			Description: "Xem giao dịch gần đây",
		},
		{
			Command:     "stats",
			Description: "Xem thống kê chi tiêu",
		},
		{
			Command:     "settings",
			Description: "Cài đặt tài khoản",
		},
		{
			Command:     "adduser",
			Description: "Thêm người dùng mới",
		},
		{
			Command:     "addtag",
			Description: "Thêm tag mới",
		},
	}

	// Set bot commands
	cmdConfig := tgbotapi.NewSetMyCommands(commands...)
	_, err = telegramBot.Request(cmdConfig)
	if err != nil {
		log.Printf("Error setting bot commands: %v", err)
	}

	// Set up updates channel
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.TelegramBot.GetUpdatesChan(u)
	bot.updates = updates

	// Start handling updates in a goroutine
	go bot.handleUpdates(ctx)

	return bot
}

func (b *Bot) handleUpdates(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case update := <-b.updates:
			if update.CallbackQuery != nil {
				// Handle callback queries (button clicks)
				b.handleCallbackQuery(update.CallbackQuery)
				continue
			}

			if update.Message == nil {
				continue
			}

			// Handle commands first
			if update.Message.IsCommand() {
				b.handleCommand(update.Message)
				continue
			}

			// Handle replies to previous messages
			if update.Message.ReplyToMessage != nil {
				replyToID := update.Message.ReplyToMessage.MessageID
				action, exists := b.pendingActions[replyToID]
				if !exists {
					continue
				}

				// Handle different types of pending actions
				switch action.Type {
				case "add_user":
					// Parse input format: "Name - Email"
					parts := strings.Split(update.Message.Text, "-")
					if len(parts) != 2 {
						msg := "❌ Định dạng không hợp lệ. Vui lòng nhập theo định dạng:\nTên - Email\nVí dụ: John Doe - john.doe@example.com\nHoặc gửi /cancel để hủy"
						b.handleAddUser(update.Message.Chat.ID, &msg)
						delete(b.pendingActions, replyToID)
						continue
					}

					name := strings.TrimSpace(parts[0])
					email := strings.TrimSpace(parts[1])

					if !strings.Contains(email, "@") {
						msg := "❌ Định dạng không hợp lệ. Vui lòng nhập theo định dạng:\nTên - Email\nVí dụ: John Doe - john.doe@example.com\nHoặc gửi /cancel để hủy"
						b.handleAddUser(update.Message.Chat.ID, &msg)
						delete(b.pendingActions, replyToID)
						continue
					}

					// Create new user
					_, err := b.BotInjector.Database.Conn.Exec(`
						INSERT INTO users (name, email, created_at)
						VALUES (?, ?, ?)
					`, name, email, time.Now())

					if err != nil {
						msg := fmt.Sprintf("❌ Lỗi khi thêm người dùng: %v\nVui lòng nhập theo định dạng:\nTên - Email\nVí dụ: John Doe - john.doe@example.com\nHoặc gửi /cancel để hủy", err)
						b.handleAddUser(update.Message.Chat.ID, &msg)
						delete(b.pendingActions, replyToID)
						continue
					} else {
						b.SendMessage(update.Message.Chat.ID, fmt.Sprintf("✅ Đã thêm người dùng '%s' với email '%s' thành công!", name, email))
						delete(b.pendingActions, replyToID)
					}

				case "new_tag":
					b.handleNewTagReply(update.Message.Chat.ID, action.TransactionID, update.Message.Text)
					delete(b.pendingActions, replyToID)

				case "register_email":
					email := strings.TrimSpace(update.Message.Text)
					if !strings.Contains(email, "@") {
						b.SendMessage(update.Message.Chat.ID, "Invalid email format. Please send a valid email address.")
						continue
					}

					err := b.authorizeUser(update.Message.Chat.ID, update.Message.From.UserName, email)
					if err != nil {
						log.Printf("Error authorizing user: %v", err)
						b.SendMessage(update.Message.Chat.ID, "Sorry, there was an error registering your email. Please try again later.")
						continue
					}

					b.SendMessage(update.Message.Chat.ID, fmt.Sprintf("Successfully registered with email: %s\n\n"+
						"Welcome! I'm your personal finance tracking bot. Here are the available commands:\n\n"+
						"/transactions - View recent transactions\n"+
						"/help - Show this help message", email))
					delete(b.pendingActions, replyToID)

				case "split_amount":
					amount, err := currency.ParseCurrency(update.Message.Text)
					if err != nil {
						b.SendMessage(update.Message.Chat.ID, "Số tiền không hợp lệ. Vui lòng thử lại.")
						continue
					}

					split := &models.TransactionSplit{
						TransactionID: action.TransactionID,
						UserID:        action.UserID,
						Amount:        int64(amount),
						CreatedAt:     time.Now(),
					}

					err = b.BotInjector.TransactionRepository.CreateSplit(split)
					if err != nil {
						log.Printf("Error creating split: %v", err)
						b.SendMessage(update.Message.Chat.ID, "Không thể lưu thông tin chia bill. Vui lòng thử lại.")
						continue
					}

					b.handleSplitBill(update.Message.Chat.ID, action.TransactionID)
					delete(b.pendingActions, replyToID)

				case "add_global_tag":
					_, err := b.BotInjector.TagRepository.Create(update.Message.Text)
					if err != nil {
						log.Printf("Error creating tag: %v", err)
						b.SendMessage(update.Message.Chat.ID, fmt.Sprintf("Không thể tạo tag mới. Vui lòng thử lại: %v", err.Error()))
					} else {
						b.SendMessage(update.Message.Chat.ID, fmt.Sprintf("✅ Đã thêm tag '%s' thành công!", update.Message.Text))
					}
					delete(b.pendingActions, replyToID)
				}
			}
		}
	}
}

func (b *Bot) Send(msg tgbotapi.Chattable) (tgbotapi.Message, error) {
	switch m := msg.(type) {
	case tgbotapi.MessageConfig:
		m.ParseMode = "Markdown"
		return b.TelegramBot.Send(m)
	}

	return b.TelegramBot.Send(msg)
}

func (b *Bot) handleCallbackQuery(callback *tgbotapi.CallbackQuery) {
	// Store the message ID when handling callbacks
	b.lastMessageID = callback.Message.MessageID

	// Parse the callback data
	parts := strings.Split(callback.Data, ":")
	action := parts[0]

	// For actions that require transaction ID, ensure we have enough parts
	if len(parts) < 2 {
		return
	}

	// Parse transaction ID for transaction-specific actions
	transactionID, _ := strconv.ParseInt(parts[1], 10, 64)

	// Check if transaction is completed before processing any action
	var completed bool
	err := b.BotInjector.Database.Conn.QueryRow(`
		SELECT completed FROM transactions WHERE id = ?
	`, transactionID).Scan(&completed)
	if err != nil {
		log.Printf("[handleCallbackQuery] Error checking transaction completion status: %v", err)
		return
	}

	if completed && action != "back" {
		// Show notification that transaction is completed
		callbackConfig := tgbotapi.NewCallback(callback.ID, "⚠️ Giao dịch đã hoàn thành, không thể thực hiện thao tác này.")
		b.TelegramBot.Request(callbackConfig)
		return
	}

	// Answer the callback query to remove the loading indicator
	callbackConfig := tgbotapi.NewCallback(callback.ID, "")
	b.TelegramBot.Request(callbackConfig)

	// Handle menu actions that don't require transaction ID
	switch action {
	case "recent_transactions":
		b.handleTransactionsCommand(callback.Message.Chat.ID)
		return
	case "statistics":
		b.handleStatistics(callback.Message.Chat.ID)
		return
	case "settings":
		b.handleSettings(callback.Message.Chat.ID)
		return
	}

	// Handle transaction-specific actions
	switch action {
	case "tag":
		b.handleAddTag(callback.Message.Chat.ID, transactionID)
	case "select_tag":
		if len(parts) < 3 {
			return
		}
		tagID, _ := strconv.ParseInt(parts[2], 10, 64)
		b.handleSelectTag(callback.Message.Chat.ID, transactionID, tagID)
	case "new_tag":
		b.handleNewTag(callback.Message.Chat.ID, transactionID)
	case "back":
		b.handleBackToTransaction(callback.Message.Chat.ID, transactionID)
	case "back_to_split":
		b.handleSplitBill(callback.Message.Chat.ID, transactionID)
	case "split":
		b.handleSplitBill(callback.Message.Chat.ID, transactionID)
	case "complete":
		b.handleComplete(callback.Message.Chat.ID, transactionID)
	case "select_split_user":
		if len(parts) < 3 {
			return
		}
		userID, _ := strconv.ParseInt(parts[2], 10, 64)
		b.handleSelectSplitUser(callback.Message.Chat.ID, transactionID, userID)
	case "select_equal_split_user":
		if len(parts) < 3 {
			return
		}
		userID, _ := strconv.ParseInt(parts[2], 10, 64)
		b.handleSelectEqualSplitUser(callback.Message.Chat.ID, transactionID, userID)
	case "confirm_split":
		b.handleConfirmSplit(callback.Message.Chat.ID, transactionID)
	case "confirm_equal_split":
		b.handleConfirmEqualSplit(callback.Message.Chat.ID, transactionID)
	case "split_equally":
		b.handleSplitEqually(callback.Message.Chat.ID, transactionID)
	case "reset_split":
		b.handleResetSplit(callback.Message.Chat.ID, transactionID)
	}
}

func (b *Bot) handleAddTag(chatID int64, transactionID int64) {
	// Get available tags from database
	tags, err := b.BotInjector.TagRepository.GetAllTags()
	if err != nil {
		log.Printf("Error getting tags: %v", err)
		b.SendMessage(chatID, "Không thể lấy danh sách tag. Vui lòng thử lại sau.")
		return
	}

	// Create keyboard with available tags
	var rows [][]tgbotapi.InlineKeyboardButton

	// Add tags in rows of 2 buttons each
	for i := 0; i < len(tags); i += 2 {
		var row []tgbotapi.InlineKeyboardButton

		// Add first tag in row
		row = append(row, tgbotapi.NewInlineKeyboardButtonData(
			tags[i].Name,
			fmt.Sprintf("select_tag:%d:%d", transactionID, tags[i].ID),
		))

		// Add second tag if available
		if i+1 < len(tags) {
			row = append(row, tgbotapi.NewInlineKeyboardButtonData(
				tags[i+1].Name,
				fmt.Sprintf("select_tag:%d:%d", transactionID, tags[i+1].ID),
			))
		}

		rows = append(rows, row)
	}

	// Add "New Tag" button at the bottom
	rows = append(rows, []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("+ Tạo tag mới", fmt.Sprintf("new_tag:%d", transactionID)),
	})

	// Add "Back" button
	rows = append(rows, []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("« Quay lại", fmt.Sprintf("back:%d", transactionID)),
	})

	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)

	// Send message with tag options
	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("Chọn tag cho giao dịch #%d:", transactionID))
	msg.ReplyMarkup = keyboard

	b.Send(msg)
}

func (b *Bot) handleSplitBill(chatID int64, transactionID int64) {
	// Get transaction details first
	transaction, err := b.BotInjector.TransactionRepository.GetByID(transactionID)
	if err != nil {
		log.Printf("Error getting transaction: %v", err)
		b.SendMessage(chatID, "Không thể lấy thông tin giao dịch.")
		return
	}

	// Get all users
	users, err := b.BotInjector.Database.Conn.Query(`
		SELECT id, name, email 
		FROM users 
		ORDER BY name
	`)
	if err != nil {
		log.Printf("Error getting users: %v", err)
		b.SendMessage(chatID, "Không thể lấy danh sách người dùng.")
		return
	}
	defer users.Close()

	// Create keyboard with user selection buttons
	var rows [][]tgbotapi.InlineKeyboardButton
	for users.Next() {
		var user models.User
		err := users.Scan(&user.ID, &user.Name, &user.Email)
		if err != nil {
			log.Printf("Error scanning user: %v", err)
			continue
		}

		// Create a button for each user
		row := tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				user.Name,
				fmt.Sprintf("select_split_user:%d:%d", transactionID, user.ID),
			),
		)
		rows = append(rows, row)
	}

	// Add "Split Equally", "Reset", "Confirm Split" and "Back" buttons at the bottom
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔄 Chia đều", fmt.Sprintf("split_equally:%d", transactionID)),
	))
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("⚠️ Reset", fmt.Sprintf("reset_split:%d", transactionID)),
	))
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("✅ Xác nhận chia bill", fmt.Sprintf("confirm_split:%d", transactionID)),
	))
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("« Quay lại", fmt.Sprintf("back:%d", transactionID)),
	))

	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)

	// Get existing splits
	splits, err := b.BotInjector.TransactionRepository.GetSplitsForTransaction(transactionID)
	if err != nil {
		log.Printf("Error getting splits: %v", err)
		splits = []*models.TransactionSplit{} // Use empty slice if error
	}

	// Format amount for display
	formattedAmount := currency.FormatCurrency(math.Abs(float64(transaction.Amount)))

	// Build split summary
	var splitSummary strings.Builder
	var totalSplit int64
	if len(splits) > 0 {
		splitSummary.WriteString("\n\nChia bill hiện tại:\n")
		for _, split := range splits {
			user, err := b.BotInjector.UserRepository.GetByID(split.UserID)
			if err != nil {
				continue
			}
			splitSummary.WriteString(fmt.Sprintf("- %s: %s\n",
				user.Name,
				currency.FormatCurrency(float64(split.Amount)),
			))
			totalSplit += split.Amount
		}
		splitSummary.WriteString(fmt.Sprintf("\nTổng đã chia: %s", currency.FormatCurrency(float64(totalSplit))))
		if totalSplit < transaction.Amount {
			splitSummary.WriteString(fmt.Sprintf("\nCòn lại: %s", currency.FormatCurrency(float64(transaction.Amount-totalSplit))))
		}
	}

	// Send message with user selection buttons
	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf(
		"💰 Chia bill cho giao dịch #%d\n"+
			"Số tiền: %s\n\n"+
			"Chọn người muốn chia bill:%s",
		transactionID,
		formattedAmount,
		splitSummary.String(),
	))
	msg.ReplyMarkup = keyboard
	b.Send(msg)
}

func (b *Bot) handleComplete(chatID int64, transactionID int64) {
	// Mark transaction as completed
	_, err := b.BotInjector.Database.Conn.Exec(`
		UPDATE transactions 
		SET completed = 1 
		WHERE id = ?
	`, transactionID)
	if err != nil {
		log.Printf("Error marking transaction as completed: %v", err)
		b.SendMessage(chatID, "Không thể hoàn thành giao dịch. Vui lòng thử lại.")
		return
	}

	// Send success message
	b.SendMessage(chatID, fmt.Sprintf("✅ Đã hoàn thành giao dịch #%d", transactionID))

	// Show the updated transaction view
	b.handleBackToTransaction(chatID, transactionID)
}

func (b *Bot) SendMessage(chatId int64, message string) error {
	msg := tgbotapi.NewMessage(chatId, message)
	_, err := b.Send(msg)
	return err
}

func (b *Bot) SendMessageWithButtons(chatId int64, message string, transactionID int64, tx *sql.Tx) error {
	var completed bool
	var err error
	if tx != nil {
		err = tx.QueryRow(`
			SELECT completed FROM transactions WHERE id = ?
		`, transactionID).Scan(&completed)
	} else {
		err = b.BotInjector.Database.Conn.QueryRow(`
			SELECT completed FROM transactions WHERE id = ?
		`, transactionID).Scan(&completed)
	}

	if err != nil {
		log.Printf("[SendMessageWithButtons] Error checking transaction completion status: %v", err)
		return err
	}

	var msg tgbotapi.MessageConfig

	if completed {
		// For completed transactions, show a message indicating it's completed
		message = fmt.Sprintf("%s\n\n✅ Giao dịch đã hoàn thành", message)
		msg = tgbotapi.NewMessage(chatId, message)
	} else {
		// Create a custom keyboard with the requested buttons
		msg = tgbotapi.NewMessage(chatId, message)
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("Thêm tag", fmt.Sprintf("tag:%d", transactionID)),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("Chia bill", fmt.Sprintf("split:%d", transactionID)),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("Hoàn thành", fmt.Sprintf("complete:%d", transactionID)),
			),
		)
	}
	// Create message with keyboard

	_, err = b.Send(msg)
	return err
}

func (b *Bot) handleSelectTag(chatID int64, transactionID int64, tagID int64) {
	// Apply the selected tag to the transaction
	err := b.BotInjector.TagRepository.AddToTransaction(transactionID, tagID)
	if err != nil {
		log.Printf("Error adding tag to transaction: %v", err)
		msg := tgbotapi.NewMessage(chatID, "Đã tạo tag mới nhưng không thể thêm vào giao dịch. Vui lòng thử lại.")
		b.Send(msg)

		// Show tag list again
		b.handleAddTag(chatID, transactionID)
		return
	}

	// Get tag name
	tag, err := b.BotInjector.TagRepository.GetByID(tagID)
	if err != nil {
		log.Printf("Error getting tag: %v", err)
		b.SendMessage(chatID, "Đã thêm tag thành công.")
		return
	}

	// Send success message
	successMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ Đã tạo và thêm tag '%s' vào giao dịch #%d thành công!", tag.Name, transactionID))
	b.Send(successMsg)

	// Small delay to ensure messages are sent in the correct order
	time.Sleep(time.Millisecond * 100)

	// Show the updated transaction
	b.handleBackToTransaction(chatID, transactionID)
}

func (b *Bot) handleNewTag(chatID int64, transactionID int64) {
	// First send a message to acknowledge the user's action
	msg := tgbotapi.NewMessage(chatID, "Bạn đã chọn tạo tag mới.")
	b.Send(msg)

	// Then send another message asking for the tag name with ForceReply
	replyMsg := tgbotapi.NewMessage(chatID, "Vui lòng nhập tên tag mới:")
	replyMsg.ReplyMarkup = &tgbotapi.ForceReply{
		ForceReply: true,
		Selective:  true,
	}

	sent, err := b.Send(replyMsg)
	if err != nil {
		log.Printf("Error sending message: %v", err)
		b.handleBackToTransaction(chatID, transactionID)
		return
	}

	// Store the pending action
	b.pendingActions[sent.MessageID] = PendingAction{
		Type:          "new_tag",
		TransactionID: transactionID,
	}
}

func (b *Bot) handleNewTagReply(chatID int64, transactionID int64, tagName string) {
	// Create the new tag
	tagID, err := b.BotInjector.TagRepository.Create(tagName)
	if err != nil {
		log.Printf("Error creating tag: %v", err)
		msg := tgbotapi.NewMessage(chatID, "Xin lỗi, không thể tạo tag mới. Vui lòng thử lại sau.")
		b.Send(msg)

		// Show tag list again
		b.handleAddTag(chatID, transactionID)
		return
	}

	// Add the tag to the transaction
	err = b.BotInjector.TagRepository.AddToTransaction(transactionID, tagID)
	if err != nil {
		log.Printf("Error adding tag to transaction: %v", err)
		msg := tgbotapi.NewMessage(chatID, "Đã tạo tag mới nhưng không thể thêm vào giao dịch. Vui lòng thử lại.")
		b.Send(msg)

		// Show tag list again
		b.handleAddTag(chatID, transactionID)
		return
	}

	// Send success message
	successMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ Đã tạo và thêm tag '%s' vào giao dịch #%d thành công!", tagName, transactionID))
	b.Send(successMsg)

	// Small delay to ensure messages are sent in the correct order
	time.Sleep(time.Millisecond * 100)

	// Show the updated transaction
	b.handleBackToTransaction(chatID, transactionID)
}

// Handle commands
func (b *Bot) handleCommand(message *tgbotapi.Message) {
	switch message.Command() {
	case "cancel":
		// Clear any pending actions for this user's messages
		for k, v := range b.pendingActions {
			if v.Type == "add_user" {
				delete(b.pendingActions, k)
			}
		}
		b.SendMessage(message.Chat.ID, "✅ Đã hủy thao tác.")
		return
	case "start":
		// Check if user is authorized
		authorized, err := b.isUserAuthorized(message.Chat.ID)
		if err != nil {
			log.Printf("Error checking user authorization: %v", err)
			b.SendMessage(message.Chat.ID, "Sorry, there was an error processing your request.")
			return
		}

		if !authorized {
			// Ask user to provide their email
			msg := tgbotapi.NewMessage(message.Chat.ID, "Welcome! To get started, please send me your email address that receives bank notifications.")
			msg.ReplyMarkup = tgbotapi.ForceReply{
				ForceReply: true,
				Selective:  true,
			}
			sent, err := b.Send(msg)
			if err != nil {
				log.Printf("Error sending message: %v", err)
				return
			}

			// Store pending action for email registration
			b.pendingActions[sent.MessageID] = PendingAction{
				Type: "register_email",
			}
		} else {
			email, _ := b.getUserEmail(message.Chat.ID)
			b.SendMessage(message.Chat.ID, fmt.Sprintf("Xin chào! Email đăng ký của bạn: %s", email))
		}
	case "transactions":
		b.handleTransactionsCommand(message.Chat.ID)
	case "stats":
		b.handleStatistics(message.Chat.ID)
	case "settings":
		b.handleSettings(message.Chat.ID)
	case "adduser":
		b.handleAddUser(message.Chat.ID, nil)
	case "addtag":
		b.handleAddGlobalTag(message.Chat.ID)
	}
}

func (b *Bot) handleStatistics(chatID int64) {
	// Create keyboard with back button

	msg := tgbotapi.NewMessage(chatID,
		"📊 Tính năng thống kê đang được phát triển.\n\n"+
			"Sẽ sớm có các thống kê về:\n"+
			"- Chi tiêu theo tháng\n"+
			"- Chi tiêu theo tag\n"+
			"- So sánh các khoản chi\n"+
			"- Và nhiều thông tin hữu ích khác",
	)

	b.Send(msg)
}

func (b *Bot) handleSettings(chatID int64) {
	// Get current email
	email, err := b.getUserEmail(chatID)
	if err != nil {
		log.Printf("Error getting user email: %v", err)
		email = "Chưa đăng ký"
	}

	// Create keyboard with settings options
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✉️ Đổi email", "change_email"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf(
		"⚙️ Cài đặt\n\n"+
			"Email hiện tại: %s\n\n"+
			"Chọn cài đặt bạn muốn thay đổi:",
		email,
	))
	msg.ReplyMarkup = keyboard

	b.Send(msg)
}

func (b *Bot) isUserAuthorized(chatID int64) (bool, error) {
	query := `SELECT COUNT(*) FROM authorized_telegram_users WHERE chat_id = ?`
	var count int
	err := b.BotInjector.Database.Conn.QueryRow(query, chatID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check user authorization: %v", err)
	}
	return count > 0, nil
}

func (b *Bot) authorizeUser(chatID int64, username string, email string) error {
	query := `INSERT INTO authorized_telegram_users (chat_id, username, email) VALUES (?, ?, ?)`
	_, err := b.BotInjector.Database.Conn.Exec(query, chatID, username, email)
	if err != nil {
		return fmt.Errorf("failed to authorize user: %v", err)
	}
	return nil
}

func (b *Bot) getUserEmail(chatID int64) (string, error) {
	query := `SELECT email FROM authorized_telegram_users WHERE chat_id = ?`
	var email string
	err := b.BotInjector.Database.Conn.QueryRow(query, chatID).Scan(&email)
	if err != nil {
		return "", fmt.Errorf("failed to get user email: %v", err)
	}
	return email, nil
}

func (b *Bot) NotifyNewTransaction(transaction *models.Transaction, email string, tx *sql.Tx) error {
	// Get all authorized users with matching email
	query := `SELECT chat_id FROM authorized_telegram_users WHERE email = ?`
	var rows *sql.Rows
	var err error
	if tx != nil {
		rows, err = tx.Query(query, email)
	} else {
		rows, err = b.BotInjector.Database.Conn.Query(query, email)
	}
	if err != nil {
		return fmt.Errorf("failed to get authorized users: %v", err)
	}
	defer rows.Close()

	var chatIDs []int64
	for rows.Next() {
		var chatID int64
		if err := rows.Scan(&chatID); err != nil {
			log.Printf("Error scanning chat ID: %v", err)
			continue
		}
		chatIDs = append(chatIDs, chatID)
	}

	if len(chatIDs) == 0 {
		log.Printf("No users found with email: %s", email)
		return nil
	}

	// Send notification to each authorized user
	for _, chatID := range chatIDs {
		err := b.SendMessageWithButtons(chatID, formatNewTransactionMessage(transaction), transaction.ID, tx)
		if err != nil {
			log.Printf("Error sending notification to chat ID %d: %v", chatID, err)
		}
	}

	return nil
}

func formatNewTransactionMessage(t *models.Transaction) string {
	amountType := "Tăng"
	if t.Type == string(models.TransactionTypeSubtract) {
		amountType = "Giảm"
	}

	formattedAmount := currency.FormatCurrency(math.Abs(float64(t.Amount)))
	formattedBalance := currency.FormatCurrency(math.Abs(float64(t.CurrentBalance)))

	// Load Asia/Ho_Chi_Minh location
	location, _ := time.LoadLocation("Asia/Ho_Chi_Minh")

	return fmt.Sprintf(
		"🔔 *New Transaction #%d*\n\n"+
			"*Người gửi:* %s\n"+
			"*Số tiền:* %s %s\n"+
			"*Số dư:* %s\n"+
			"*Thời gian:* %s\n"+
			"*Mô tả:* %s",
		t.ID,
		t.From,
		amountType,
		formattedAmount,
		formattedBalance,
		t.Timestamp.In(location).Format("02/01/2006 15:04:05"),
		t.Description,
	)
}

func formatTransactionMessage(t *models.Transaction, tagsText string) string {
	amountType := "Tăng"
	if t.Type == string(models.TransactionTypeSubtract) {
		amountType = "Giảm"
	}

	formattedAmount := currency.FormatCurrency(math.Abs(float64(t.Amount)))
	formattedBalance := currency.FormatCurrency(math.Abs(float64(t.CurrentBalance)))

	location, _ := time.LoadLocation("Asia/Ho_Chi_Minh")

	return fmt.Sprintf(
		"🔔 *Transaction #%d*\n\n"+
			"*Người gửi:* %s\n"+
			"*Số tiền:* %s %s\n"+
			"*Số dư:* %s\n"+
			"*Thời gian:* %s\n"+
			"*Mô tả:* %s%s",
		t.ID,
		t.From,
		amountType,
		formattedAmount,
		formattedBalance,
		t.Timestamp.In(location).Format("02/01/2006 15:04:05"),
		t.Description,
		tagsText,
	)
}

func (b *Bot) handleTransactionsCommand(chatID int64) {
	// Get recent transactions from the database
	query := `
		SELECT id, amount, current_balance, type, from_account, description, timestamp
		FROM transactions
		ORDER BY timestamp DESC
		LIMIT 5
	`
	rows, err := b.BotInjector.Database.Conn.Query(query)
	if err != nil {
		log.Printf("Error querying transactions: %v", err)
		b.SendMessage(chatID, "Sorry, I couldn't retrieve your transactions right now.")
		return
	}
	defer rows.Close()

	var transactions []*models.Transaction
	for rows.Next() {
		t := &models.Transaction{}
		var timestamp int64
		err := rows.Scan(&t.ID, &t.Amount, &t.CurrentBalance, &t.Type, &t.From, &t.Description, &timestamp)
		if err != nil {
			log.Printf("Error scanning transaction: %v", err)
			continue
		}
		t.Timestamp = time.Unix(timestamp, 0)
		if err != nil {
			log.Printf("Error parsing timestamp: %v", err)
			continue
		}
		transactions = append(transactions, t)
	}

	if len(transactions) == 0 {
		b.SendMessage(chatID, "No recent transactions found.")
		return
	}

	// Send each transaction with its own set of buttons
	for _, t := range transactions {
		tags, _ := b.BotInjector.TagRepository.GetForTransaction(t.ID)
		tagsText := ""
		if len(tags) > 0 {
			tagNames := make([]string, len(tags))
			for i, tag := range tags {
				tagNames[i] = fmt.Sprintf("*%s*", tag.Name)
			}
			tagsText = "\nTags: " + strings.Join(tagNames, ", ")
		}

		message := formatTransactionMessage(t, tagsText)

		err := b.SendMessageWithButtons(chatID, message, t.ID, nil)
		if err != nil {
			log.Printf("Error sending transaction: %v", err)
		}
	}
}

func (b *Bot) handleBackToTransaction(chatID int64, transactionID int64) {
	// Get transaction details
	transaction, err := b.BotInjector.TransactionRepository.GetByID(transactionID)
	if err != nil {
		log.Printf("Error getting transaction: %v", err)
		b.SendMessage(chatID, "Không thể lấy thông tin giao dịch.")
		return
	}

	// Get tags for this transaction
	tags, _ := b.BotInjector.TagRepository.GetForTransaction(transactionID)
	tagsText := ""
	if len(tags) > 0 {
		tagNames := make([]string, len(tags))
		for i, tag := range tags {
			tagNames[i] = fmt.Sprintf("*%s*", tag.Name)
		}
		tagsText = "\nTags: " + strings.Join(tagNames, ", ")
	}

	// Format the message
	message := formatTransactionMessage(transaction, tagsText)

	err = b.SendMessageWithButtons(chatID, message, transactionID, nil)
	if err != nil {
		log.Printf("Error sending transaction: %v", err)
	}
}

func (b *Bot) handleSelectSplitUser(chatID int64, transactionID int64, userID int64) {
	// Get user details
	var user models.User
	err := b.BotInjector.Database.Conn.QueryRow(`
		SELECT id, name, email FROM users WHERE id = ?
	`, userID).Scan(&user.ID, &user.Name, &user.Email)
	if err != nil {
		log.Printf("Error getting user: %v", err)
		b.SendMessage(chatID, "Không thể lấy thông tin người dùng.")
		return
	}

	// Ask for split amount
	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf(
		"Nhập số tiền cho %s:\n"+
			"(Gửi số tiền hoặc nhấn /cancel để hủy)",
		user.Name,
	))
	msg.ReplyMarkup = tgbotapi.ForceReply{
		ForceReply: true,
		Selective:  true,
	}
	sent, err := b.Send(msg)
	if err != nil {
		log.Printf("Error sending message: %v", err)
		return
	}

	// Store pending action
	b.pendingActions[sent.MessageID] = PendingAction{
		Type:          "split_amount",
		TransactionID: transactionID,
		UserID:        userID,
	}
}

func (b *Bot) handleConfirmSplit(chatID int64, transactionID int64) {
	// Get transaction details
	transaction, err := b.BotInjector.TransactionRepository.GetByID(transactionID)
	if err != nil {
		log.Printf("Error getting transaction: %v", err)
		b.SendMessage(chatID, "Không thể lấy thông tin giao dịch.")
		return
	}

	// Get all splits for this transaction
	splits, err := b.BotInjector.TransactionRepository.GetSplitsForTransaction(transactionID)
	if err != nil {
		log.Printf("Error getting splits: %v", err)
		b.SendMessage(chatID, "Không thể lấy thông tin chia bill.")
		return
	}

	// Calculate total split amount
	var totalSplit int64
	for _, split := range splits {
		totalSplit += split.Amount
	}

	// Check if total split matches transaction amount
	if totalSplit != transaction.Amount {
		b.SendMessage(chatID, fmt.Sprintf(
			"⚠️ Tổng số tiền chia (%s) không khớp với số tiền giao dịch (%s).\n"+
				"Vui lòng kiểm tra lại.",
			currency.FormatCurrency(float64(totalSplit)),
			currency.FormatCurrency(float64(transaction.Amount)),
		))
		return
	}

	// Format confirmation message
	var message strings.Builder
	message.WriteString(fmt.Sprintf("✅ Đã chia bill cho giao dịch #%d:\n\n", transactionID))

	for _, split := range splits {
		user, err := b.BotInjector.UserRepository.GetByID(split.UserID)
		if err != nil {
			continue
		}
		message.WriteString(fmt.Sprintf(
			"- %s: %s\n",
			user.Name,
			currency.FormatCurrency(float64(split.Amount)),
		))
	}

	b.SendMessage(chatID, message.String())

	// Return to transaction view
	b.handleBackToTransaction(chatID, transactionID)
}

func (b *Bot) handleSplitEqually(chatID int64, transactionID int64) {
	// Get transaction details first
	transaction, err := b.BotInjector.TransactionRepository.GetByID(transactionID)
	if err != nil {
		log.Printf("Error getting transaction: %v", err)
		b.SendMessage(chatID, "Không thể lấy thông tin giao dịch.")
		return
	}

	// Get all users
	users, err := b.BotInjector.Database.Conn.Query(`
		SELECT id, name, email 
		FROM users 
		ORDER BY name
	`)
	if err != nil {
		log.Printf("Error getting users: %v", err)
		b.SendMessage(chatID, "Không thể lấy danh sách người dùng.")
		return
	}
	defer users.Close()

	// Create keyboard with user selection buttons
	var rows [][]tgbotapi.InlineKeyboardButton
	for users.Next() {
		var user models.User
		err := users.Scan(&user.ID, &user.Name, &user.Email)
		if err != nil {
			log.Printf("Error scanning user: %v", err)
			continue
		}

		// Create a button for each user
		row := tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				"☐ "+user.Name,
				fmt.Sprintf("select_equal_split_user:%d:%d", transactionID, user.ID),
			),
		)
		rows = append(rows, row)
	}

	// Add "Confirm" and "Back" buttons at the bottom
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("✅ Xác nhận chia đều", fmt.Sprintf("confirm_equal_split:%d", transactionID)),
	))
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("« Quay lại", fmt.Sprintf("back_to_split:%d", transactionID)),
	))

	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)

	// Format amount for display
	formattedAmount := currency.FormatCurrency(math.Abs(float64(transaction.Amount)))

	// Send message with user selection buttons
	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf(
		"💰 Chọn người muốn chia đều cho giao dịch #%d\n"+
			"Số tiền: %s\n\n"+
			"Chọn những người tham gia chia bill:",
		transactionID,
		formattedAmount,
	))
	msg.ReplyMarkup = keyboard
	b.Send(msg)
}

func (b *Bot) handleSelectEqualSplitUser(chatID int64, transactionID int64, userID int64) {
	// Initialize the map for this transaction if it doesn't exist
	if _, exists := b.selectedUsers[transactionID]; !exists {
		b.selectedUsers[transactionID] = make(map[int64]bool)
	}

	// Toggle the selection
	b.selectedUsers[transactionID][userID] = !b.selectedUsers[transactionID][userID]

	// Get all users to rebuild the keyboard
	users, err := b.BotInjector.Database.Conn.Query(`
		SELECT id, name, email 
		FROM users 
		ORDER BY name
	`)
	if err != nil {
		log.Printf("Error getting users: %v", err)
		b.SendMessage(chatID, "Không thể lấy danh sách người dùng.")
		return
	}
	defer users.Close()

	// Create keyboard with user selection buttons
	var rows [][]tgbotapi.InlineKeyboardButton
	for users.Next() {
		var user models.User
		err := users.Scan(&user.ID, &user.Name, &user.Email)
		if err != nil {
			log.Printf("Error scanning user: %v", err)
			continue
		}

		// Create a button for each user
		prefix := "☐"
		if b.selectedUsers[transactionID][user.ID] {
			prefix = "☑"
		}
		row := tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				prefix+" "+user.Name,
				fmt.Sprintf("select_equal_split_user:%d:%d", transactionID, user.ID),
			),
		)
		rows = append(rows, row)
	}

	// Add "Confirm" and "Back" buttons at the bottom
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("✅ Xác nhận chia đều", fmt.Sprintf("confirm_equal_split:%d", transactionID)),
	))
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("« Quay lại", fmt.Sprintf("back_to_split:%d", transactionID)),
	))

	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)

	// Update the message with the new keyboard
	edit := tgbotapi.NewEditMessageReplyMarkup(chatID, b.lastMessageID, keyboard)
	_, err = b.Send(edit)
	if err != nil {
		log.Printf("Error sending message: %v", err)
	}
}

func (b *Bot) handleConfirmEqualSplit(chatID int64, transactionID int64) {
	// Get transaction details
	transaction, err := b.BotInjector.TransactionRepository.GetByID(transactionID)
	if err != nil {
		log.Printf("Error getting transaction: %v", err)
		b.SendMessage(chatID, "Không thể lấy thông tin giao dịch.")
		return
	}

	// Get selected users
	selectedUsers := b.selectedUsers[transactionID]
	var selectedUserIDs []int64
	for userID, selected := range selectedUsers {
		if selected {
			selectedUserIDs = append(selectedUserIDs, userID)
		}
	}

	if len(selectedUserIDs) == 0 {
		b.SendMessage(chatID, "Vui lòng chọn ít nhất một người để chia bill.")
		return
	}

	// Calculate equal split amount
	splitAmount := transaction.Amount / int64(len(selectedUserIDs))
	remainder := transaction.Amount % int64(len(selectedUserIDs))

	// Delete existing splits
	_, err = b.BotInjector.Database.Conn.Exec(`DELETE FROM transaction_splits WHERE transaction_id = ?`, transactionID)
	if err != nil {
		log.Printf("Error deleting existing splits: %v", err)
		b.SendMessage(chatID, "Không thể xóa thông tin chia bill cũ.")
		return
	}

	// Create new splits
	for i, userID := range selectedUserIDs {
		amount := splitAmount
		if i == 0 {
			// Add remainder to first user to avoid losing money due to rounding
			amount += remainder
		}

		split := &models.TransactionSplit{
			TransactionID: transactionID,
			UserID:        userID,
			Amount:        amount,
			CreatedAt:     time.Now(),
		}

		err = b.BotInjector.TransactionRepository.CreateSplit(split)
		if err != nil {
			log.Printf("Error creating split for user %d: %v", userID, err)
			continue
		}
	}

	// Clean up the selected users map
	delete(b.selectedUsers, transactionID)

	// Show updated split view
	b.handleSplitBill(chatID, transactionID)
}

func (b *Bot) handleResetSplit(chatID int64, transactionID int64) {
	// Delete all splits for this transaction
	_, err := b.BotInjector.Database.Conn.Exec(`DELETE FROM transaction_splits WHERE transaction_id = ?`, transactionID)
	if err != nil {
		log.Printf("Error resetting splits: %v", err)
		b.SendMessage(chatID, "Không thể xóa thông tin chia bill.")
		return
	}

	// Show success message
	b.SendMessage(chatID, "Đã xóa toàn bộ thông tin chia bill.")

	// Show updated split view
	b.handleSplitBill(chatID, transactionID)
}

func (b *Bot) handleAddUser(chatID int64, alternativeMessage *string) {
	// Send message with ForceReply
	msg := tgbotapi.NewMessage(chatID, "Vui lòng nhập thông tin người dùng mới theo định dạng:\nTên - Email\nVí dụ: John Doe - john.doe@example.com\nHoặc gửi /cancel để hủy")
	if alternativeMessage != nil {
		msg.Text = *alternativeMessage
	}
	msg.ReplyMarkup = &tgbotapi.ForceReply{
		ForceReply: true,
		Selective:  true,
	}
	sent, err := b.Send(msg)
	if err != nil {
		log.Printf("Error sending message: %v", err)
		return
	}

	// Store pending action
	b.pendingActions[sent.MessageID] = PendingAction{
		Type: "add_user",
	}
}

func (b *Bot) handleAddGlobalTag(chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "Vui lòng nhập tên tag mới:")
	msg.ReplyMarkup = tgbotapi.ForceReply{
		ForceReply: true,
		Selective:  true,
	}
	sent, err := b.Send(msg)
	if err != nil {
		log.Printf("Error sending message: %v", err)
		return
	}

	b.pendingActions[sent.MessageID] = PendingAction{
		Type: "add_global_tag",
	}
}
