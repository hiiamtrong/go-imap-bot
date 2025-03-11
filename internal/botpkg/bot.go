package botpkg

import (
	"context"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/hiiamtrong/imap-bot-go/internal/config"
	"github.com/hiiamtrong/imap-bot-go/internal/database"
	"github.com/hiiamtrong/imap-bot-go/internal/models"
	"github.com/hiiamtrong/imap-bot-go/internal/repository"
	"github.com/hiiamtrong/imap-bot-go/pkg/currency"
)

type BotInjector struct {
	Database              *database.Database
	MailRepository        *repository.MailRepository
	TransactionRepository *repository.TransactionRepository
	TagRepository         *repository.TagRepository
}

func NewBotInjector(
	database *database.Database,
	mailRepository *repository.MailRepository,
	transactionRepository *repository.TransactionRepository,
	tagRepository *repository.TagRepository,
) *BotInjector {
	return &BotInjector{
		Database:              database,
		MailRepository:        mailRepository,
		TransactionRepository: transactionRepository,
		TagRepository:         tagRepository,
	}
}

type Bot struct {
	TelegramBot    *tgbotapi.BotAPI
	Context        context.Context
	BotInjector    *BotInjector
	updates        tgbotapi.UpdatesChannel
	pendingActions map[int]PendingAction
	lastMessageID  int
}

type PendingAction struct {
	Type          string
	TransactionID int64
}

func InitBot(cfg *config.Config, ctx context.Context, injector *BotInjector) *Bot {
	bot := &Bot{
		Context:        ctx,
		BotInjector:    injector,
		pendingActions: make(map[int]PendingAction),
	}

	// Initialize Telegram bot
	telegramBot, err := tgbotapi.NewBotAPI(cfg.TelegramBot.Token)
	if err != nil {
		log.Fatalf("Failed to initialize Telegram bot: %v", err)
	}

	bot.TelegramBot = telegramBot

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
			} else if update.Message != nil {
				// Handle text messages
				if update.Message.ReplyToMessage != nil {
					// This is a reply to a previous message
					replyToID := update.Message.ReplyToMessage.MessageID
					if action, exists := b.pendingActions[replyToID]; exists {
						// We have a pending action for this reply
						if action.Type == "new_tag" {
							// Handle new tag reply
							b.handleNewTagReply(update.Message.Chat.ID, action.TransactionID, update.Message.Text)
							delete(b.pendingActions, replyToID)
						} else if action.Type == "register_email" {
							// Handle email registration
							email := strings.TrimSpace(update.Message.Text)
							if !strings.Contains(email, "@") {
								b.SendMessage(update.Message.Chat.ID, "Invalid email format. Please send a valid email address.")
								return
							}

							err := b.authorizeUser(update.Message.Chat.ID, update.Message.From.UserName, email)
							if err != nil {
								log.Printf("Error authorizing user: %v", err)
								b.SendMessage(update.Message.Chat.ID, "Sorry, there was an error registering your email. Please try again later.")
								return
							}

							b.SendMessage(update.Message.Chat.ID, fmt.Sprintf("Successfully registered with email: %s\n\n"+
								"Welcome! I'm your personal finance tracking bot. Here are the available commands:\n\n"+
								"/transactions - View recent transactions\n"+
								"/help - Show this help message", email))
							delete(b.pendingActions, replyToID)
						}
					}
				} else if update.Message.IsCommand() {
					// Handle commands
					b.handleCommand(update.Message)
				}
			}
		}
	}
}

func (b *Bot) handleCallbackQuery(callback *tgbotapi.CallbackQuery) {
	// Store the message ID when handling callbacks
	b.lastMessageID = callback.Message.MessageID

	// Parse the callback data
	parts := strings.Split(callback.Data, ":")
	if len(parts) < 2 {
		return
	}

	action := parts[0]

	// Answer the callback query to remove the loading indicator
	callbackConfig := tgbotapi.NewCallback(callback.ID, "")
	b.TelegramBot.Request(callbackConfig)

	switch action {
	case "tag":
		// Show available tags
		transactionID, _ := strconv.ParseInt(parts[1], 10, 64)
		b.handleAddTag(callback.Message.Chat.ID, transactionID)

	case "select_tag":
		// Handle tag selection
		if len(parts) < 3 {
			return
		}
		transactionID, _ := strconv.ParseInt(parts[1], 10, 64)
		tagID, _ := strconv.ParseInt(parts[2], 10, 64)
		b.handleSelectTag(callback.Message.Chat.ID, transactionID, tagID)

	case "new_tag":
		// Handle new tag creation
		transactionID, _ := strconv.ParseInt(parts[1], 10, 64)
		b.handleNewTag(callback.Message.Chat.ID, transactionID)

	case "back":
		// Go back to main transaction view
		transactionID, _ := strconv.ParseInt(parts[1], 10, 64)
		b.handleBackToTransaction(callback.Message.Chat.ID, transactionID)

	case "split":
		transactionID, _ := strconv.ParseInt(parts[1], 10, 64)
		b.handleSplitBill(callback.Message.Chat.ID, transactionID)

	case "complete":
		transactionID, _ := strconv.ParseInt(parts[1], 10, 64)
		b.handleComplete(callback.Message.Chat.ID, transactionID)
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

	b.TelegramBot.Send(msg)
}

func (b *Bot) handleSplitBill(chatID int64, transactionID int64) {
	// Implement bill splitting functionality
	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("Chia bill cho giao dịch #%d", transactionID))
	b.TelegramBot.Send(msg)
}

func (b *Bot) handleComplete(chatID int64, transactionID int64) {
	// Implement completion functionality
	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("Đã hoàn thành giao dịch #%d", transactionID))
	b.TelegramBot.Send(msg)
}

func (b *Bot) SendMessage(chatId int64, message string) error {
	msg := tgbotapi.NewMessage(chatId, message)
	_, err := b.TelegramBot.Send(msg)
	return err
}

func (b *Bot) SendMessageWithButtons(chatId int64, message string, transactionID int64) error {
	// Create a custom keyboard with the requested buttons
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
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

	// Create message with keyboard
	msg := tgbotapi.NewMessage(chatId, message)
	msg.ReplyMarkup = keyboard

	_, err := b.TelegramBot.Send(msg)
	return err
}

func (b *Bot) handleSelectTag(chatID int64, transactionID int64, tagID int64) {
	// Apply the selected tag to the transaction
	err := b.BotInjector.TagRepository.AddToTransaction(transactionID, tagID)
	if err != nil {
		log.Printf("Error adding tag to transaction: %v", err)
		b.SendMessage(chatID, "Không thể thêm tag. Vui lòng thử lại sau.")
		return
	}

	// Get tag name
	tag, err := b.BotInjector.TagRepository.GetByID(tagID)
	if err != nil {
		log.Printf("Error getting tag: %v", err)
		b.SendMessage(chatID, "Đã thêm tag thành công.")
		return
	}

	// Send confirmation
	b.SendMessage(chatID, fmt.Sprintf("Đã thêm tag '%s' vào giao dịch #%d", tag.Name, transactionID))

	// Refresh transaction view
	b.handleBackToTransaction(chatID, transactionID)
}

func (b *Bot) handleNewTag(chatID int64, transactionID int64) {
	// First send a message to acknowledge the user's action
	msg := tgbotapi.NewMessage(chatID, "Bạn đã chọn tạo tag mới.")
	b.TelegramBot.Send(msg)

	// Then send another message asking for the tag name with ForceReply
	replyMsg := tgbotapi.NewMessage(chatID, "Vui lòng nhập tên tag mới:")
	replyMsg.ReplyMarkup = &tgbotapi.ForceReply{
		ForceReply: true,
		Selective:  true,
	}

	sent, err := b.TelegramBot.Send(replyMsg)
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
		b.TelegramBot.Send(msg)

		// Show tag list again
		b.handleAddTag(chatID, transactionID)
		return
	}

	// Add the tag to the transaction
	err = b.BotInjector.TagRepository.AddToTransaction(transactionID, tagID)
	if err != nil {
		log.Printf("Error adding tag to transaction: %v", err)
		msg := tgbotapi.NewMessage(chatID, "Đã tạo tag mới nhưng không thể thêm vào giao dịch. Vui lòng thử lại.")
		b.TelegramBot.Send(msg)

		// Show tag list again
		b.handleAddTag(chatID, transactionID)
		return
	}

	// Send success message
	successMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ Đã tạo và thêm tag '%s' vào giao dịch #%d thành công!", tagName, transactionID))
	b.TelegramBot.Send(successMsg)

	// Small delay to ensure messages are sent in the correct order
	time.Sleep(time.Millisecond * 100)

	// Show the updated transaction
	b.handleBackToTransaction(chatID, transactionID)
}

// Function to ask for token address
func (b *Bot) askForTokenAddress(chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "Enter token address you would like to snipe:")

	// Use ForceReply to prompt the user to reply
	msg.ReplyMarkup = tgbotapi.ForceReply{
		ForceReply: true,
		Selective:  true,
	}

	sent, err := b.TelegramBot.Send(msg)
	if err != nil {
		log.Printf("Error sending message: %v", err)
		return
	}

	// Store the message ID to track the reply
	b.pendingActions[sent.MessageID] = PendingAction{
		Type: "token_address",
	}
}

// Handle commands
func (b *Bot) handleCommand(message *tgbotapi.Message) {
	switch message.Command() {
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
			sent, err := b.TelegramBot.Send(msg)
			if err != nil {
				log.Printf("Error sending message: %v", err)
				return
			}

			// Store pending action for email registration
			b.pendingActions[sent.MessageID] = PendingAction{
				Type: "register_email",
			}
		} else {
			// Get user's email
			email, err := b.getUserEmail(message.Chat.ID)
			if err != nil {
				log.Printf("Error getting user email: %v", err)
				b.SendMessage(message.Chat.ID, "Welcome back! Here are the available commands:\n\n"+
					"/transactions - View recent transactions\n"+
					"/help - Show this help message")
				return
			}
			b.SendMessage(message.Chat.ID, fmt.Sprintf("Welcome back! You are registered with email: %s\n\n"+
				"Available commands:\n"+
				"/transactions - View recent transactions\n"+
				"/help - Show this help message", email))
		}
	case "transactions":
		b.handleTransactionsCommand(message.Chat.ID)
	case "help":
		b.SendMessage(message.Chat.ID, "Available commands:\n\n"+
			"/transactions - View recent transactions\n"+
			"/help - Show this help message")
	default:
		b.SendMessage(message.Chat.ID, "Unknown command. Use /help to see available commands.")
	}
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

func (b *Bot) NotifyNewTransaction(transaction *models.Transaction, email string) error {
	// Get all authorized users with matching email
	query := `SELECT chat_id FROM authorized_telegram_users WHERE email = ?`
	rows, err := b.BotInjector.Database.Conn.Query(query, email)
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
		err := b.SendMessageWithButtons(chatID, formatTransactionMessage(transaction), transaction.ID)
		if err != nil {
			log.Printf("Error sending notification to chat ID %d: %v", chatID, err)
		}
	}

	return nil
}

func formatTransactionMessage(t *models.Transaction) string {
	amountType := "Tăng"
	if t.Type == string(models.TransactionTypeSubtract) {
		amountType = "Giảm"
	}

	formattedAmount := currency.FormatCurrency(math.Abs(float64(t.Amount)))
	formattedBalance := currency.FormatCurrency(math.Abs(float64(t.CurrentBalance)))

	// Load Asia/Ho_Chi_Minh location
	location, _ := time.LoadLocation("Asia/Ho_Chi_Minh")

	return fmt.Sprintf(
		"🔔 New Transaction!\n\n"+
			"Người gửi: %s\n"+
			"Số tiền: %s %s\n"+
			"Số dư: %s\n"+
			"Thời gian: %s\n"+
			"Mô tả: %s",
		t.From,
		amountType,
		formattedAmount,
		formattedBalance,
		t.Timestamp.In(location).Format("02/01/2006 15:04:05"),
		t.Description,
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
		var timestampStr string
		err := rows.Scan(&t.ID, &t.Amount, &t.CurrentBalance, &t.Type, &t.From, &t.Description, &timestampStr)
		if err != nil {
			log.Printf("Error scanning transaction: %v", err)
			continue
		}

		// Parse the timestamp string
		timestamp, err := time.Parse("2006-01-02 15:04:05-07:00", timestampStr)
		if err != nil {
			log.Printf("Error parsing timestamp: %v", err)
			continue
		}
		t.Timestamp = timestamp
		transactions = append(transactions, t)
	}

	if len(transactions) == 0 {
		b.SendMessage(chatID, "No recent transactions found.")
		return
	}

	// Send each transaction with its own set of buttons
	for _, t := range transactions {
		amountType := "Tăng"
		if t.Type == string(models.TransactionTypeSubtract) {
			amountType = "Giảm"
		}

		formattedAmount := currency.FormatCurrency(math.Abs(float64(t.Amount)))
		formattedBalance := currency.FormatCurrency(math.Abs(float64(t.CurrentBalance)))

		// Load Asia/Ho_Chi_Minh location
		location, _ := time.LoadLocation("Asia/Ho_Chi_Minh")

		// Get tags for this transaction
		tags, _ := b.BotInjector.TagRepository.GetForTransaction(t.ID)
		tagsText := ""
		if len(tags) > 0 {
			tagNames := make([]string, len(tags))
			for i, tag := range tags {
				tagNames[i] = tag.Name
			}
			tagsText = "\nTags: " + strings.Join(tagNames, ", ")
		}

		message := fmt.Sprintf(
			"Người gửi: %s\n"+
				"Số tiền: %s %s\n"+
				"Số dư: %s\n"+
				"Thời gian: %s\n"+
				"Mô tả: %s%s",
			t.From,
			amountType,
			formattedAmount,
			formattedBalance,
			t.Timestamp.In(location).Format("02/01/2006 15:04:05"),
			t.Description,
			tagsText,
		)

		b.SendMessageWithButtons(chatID, message, t.ID)
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

	// Format and send transaction with buttons
	// This is similar to sendMessageTransaction but for an existing transaction
	amountType := "Tăng"
	if transaction.Type == string(models.TransactionTypeSubtract) {
		amountType = "Giảm"
	}

	formattedAmount := currency.FormatCurrency(math.Abs(float64(transaction.Amount)))
	formattedBalance := currency.FormatCurrency(math.Abs(float64(transaction.CurrentBalance)))

	// Load Asia/Ho_Chi_Minh location
	location, _ := time.LoadLocation("Asia/Ho_Chi_Minh")

	// Get tags for this transaction
	tags, _ := b.BotInjector.TagRepository.GetForTransaction(transactionID)
	tagsText := ""
	if len(tags) > 0 {
		tagNames := make([]string, len(tags))
		for i, tag := range tags {
			tagNames[i] = tag.Name
		}
		tagsText = "\nTags: " + strings.Join(tagNames, ", ")
	}

	// Format the message
	message := fmt.Sprintf(
		"Người gửi: %s\n"+
			"Số tiền: %s %s\n"+
			"Số dư: %s\n"+
			"Thời gian: %s\n"+
			"Mô tả: %s%s",
		transaction.From,
		amountType,
		formattedAmount,
		formattedBalance,
		transaction.Timestamp.In(location).Format("02/01/2006 15:04:05"),
		transaction.Description,
		tagsText,
	)

	b.SendMessageWithButtons(chatID, message, transactionID)
}
