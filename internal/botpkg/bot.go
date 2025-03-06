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
	"github.com/hiiamtrong/imap-bot-go/internal/models"
	"github.com/hiiamtrong/imap-bot-go/internal/repository"
)

type BotInjector struct {
	MailRepository        *repository.MailRepository
	TransactionRepository *repository.TransactionRepository
}

func NewBotInjector(
	mailRepository *repository.MailRepository,
	transactionRepository *repository.TransactionRepository,
) *BotInjector {
	return &BotInjector{
		MailRepository:        mailRepository,
		TransactionRepository: transactionRepository,
	}
}

type Bot struct {
	TelegramBot *tgbotapi.BotAPI
	BotInjector *BotInjector
	Updates     tgbotapi.UpdatesChannel
	userStates  map[int64]UserState
}

func InitBot(config *config.Config, ctx context.Context, injector *BotInjector) *Bot {
	var err error
	bot, err := tgbotapi.NewBotAPI(config.TelegramBot.Token)
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = false

	log.Println("Bot is running...")

	updates, err := bot.GetUpdatesChan(tgbotapi.UpdateConfig{
		Timeout: 60,
	})
	if err != nil {
		log.Fatalf("Error getting updates: %v", err)
	}

	bot.Updates = updates

	bot = &Bot{
		TelegramBot: bot,
		BotInjector: injector,
		Updates:     updates,
		userStates:  make(map[int64]UserState),
	}

	// Start handling callbacks in a separate goroutine
	go bot.HandleCallbacks()

	return bot
}

func (b *Bot) SendMessage(chatId int64, message string) error {
	msg := tgbotapi.NewMessage(chatId, message)
	_, err := b.TelegramBot.Send(msg)
	if err != nil {
		log.Printf("Error sending message: %v", err)
		return err
	}
	return nil
}

// InlineKeyboardButton represents a button in an inline keyboard
type InlineKeyboardButton struct {
	Text         string
	CallbackData string
}

// SendMessageWithInlineKeyboard sends a message with an inline keyboard
func (b *Bot) SendMessageWithInlineKeyboard(chatID int64, text string, keyboard [][]InlineKeyboardButton) error {
	inlineKeyboard := tgbotapi.InlineKeyboardMarkup{}

	// Convert our button format to Telegram's format
	for _, row := range keyboard {
		var tgRow []tgbotapi.InlineKeyboardButton
		for _, btn := range row {
			tgRow = append(tgRow, tgbotapi.NewInlineKeyboardButtonData(btn.Text, btn.CallbackData))
		}
		inlineKeyboard.InlineKeyboard = append(inlineKeyboard.InlineKeyboard, tgRow)
	}

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = inlineKeyboard

	_, err := b.TelegramBot.Send(msg)
	if err != nil {
		return fmt.Errorf("error sending message with keyboard: %v", err)
	}
	return nil
}

// HandleCallbacks processes callback queries from inline keyboards
func (b *Bot) HandleCallbacks() {
	for update := range b.Updates {
		if update.CallbackQuery != nil {
			// Extract the callback data
			callbackData := update.CallbackQuery.Data

			// Parse the callback data
			parts := strings.Split(callbackData, ":")
			if len(parts) < 2 {
				continue
			}

			action := parts[0]

			switch action {
			case "split_money":
				// Handle split money action
				if len(parts) < 2 {
					continue
				}

				transactionID, err := strconv.ParseInt(parts[1], 10, 64)
				if err != nil {
					continue
				}

				// Show split money dialog
				b.showSplitMoneyDialog(update.CallbackQuery.Message.Chat.ID, transactionID)

				// Answer the callback query to remove the loading indicator
				callback := tgbotapi.NewCallback(update.CallbackQuery.ID, "")
				b.TelegramBot.Request(callback)
			}
		}
	}
}

// showSplitMoneyDialog displays a dialog to select people to split money with
func (b *Bot) showSplitMoneyDialog(chatID int64, transactionID int64) error {
	// Get the transaction details
	transaction, err := b.BotInjector.TransactionRepository.GetByID(transactionID)
	if err != nil {
		return fmt.Errorf("failed to get transaction: %v", err)
	}

	// Create a message asking who to split with
	message := fmt.Sprintf(
		"Chia tiền %s cho ai?\nNhập tên người nhận và số tiền, mỗi người một dòng.\nVí dụ:\nNguyễn Văn A 50000\nTrần Thị B 30000",
		formatCurrency(math.Abs(float64(transaction.Amount))),
	)

	// Send the message
	msg := tgbotapi.NewMessage(chatID, message)

	// Set a force reply to make it easier for the user to respond
	msg.ReplyMarkup = tgbotapi.ForceReply{
		ForceReply: true,
		Selective:  false,
	}

	// Store the transaction ID in the user's state
	b.userStates[chatID] = UserState{
		State:         StateSplittingMoney,
		TransactionID: transactionID,
	}

	_, err = b.TelegramBot.Send(msg)
	if err != nil {
		return fmt.Errorf("error sending split money dialog: %v", err)
	}

	return nil
}

// UserState represents the current state of a user's interaction
type UserState struct {
	State         int
	TransactionID int64
}

// State constants
const (
	StateNone           = 0
	StateSplittingMoney = 1
)

func formatCurrency(amount float64) string {
	// Implement your formatting logic here
	return fmt.Sprintf("%.2f", amount)
}

// ProcessMessage processes incoming text messages
func (b *Bot) ProcessMessage(update tgbotapi.Update) {
	if update.Message == nil || update.Message.Text == "" {
		return
	}

	chatID := update.Message.Chat.ID

	// Check if the user is in a specific state
	state, exists := b.userStates[chatID]
	if exists {
		switch state.State {
		case StateSplittingMoney:
			b.handleSplitMoneyInput(chatID, update.Message.Text, state.TransactionID)
			return
		}
	}

	// Handle regular commands or messages
	// ...
}

// handleSplitMoneyInput processes the user's input for splitting money
func (b *Bot) handleSplitMoneyInput(chatID int64, text string, transactionID int64) {
	// Get the transaction
	transaction, err := b.BotInjector.TransactionRepository.GetByID(transactionID)
	if err != nil {
		b.SendMessage(chatID, "Không thể tìm thấy giao dịch. Vui lòng thử lại.")
		b.userStates[chatID] = UserState{State: StateNone}
		return
	}

	// Parse the input
	lines := strings.Split(text, "\n")
	splits := make([]SplitEntry, 0)
	totalSplitAmount := int64(0)

	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		// Last part is the amount, everything before is the name
		amountStr := parts[len(parts)-1]
		name := strings.Join(parts[:len(parts)-1], " ")

		amount, err := strconv.ParseInt(amountStr, 10, 64)
		if err != nil {
			continue
		}

		splits = append(splits, SplitEntry{
			Name:   name,
			Amount: amount,
		})

		totalSplitAmount += amount
	}

	// Validate the total split amount
	if totalSplitAmount > transaction.Amount {
		b.SendMessage(chatID, fmt.Sprintf(
			"Tổng số tiền chia (%s) lớn hơn số tiền giao dịch (%s). Vui lòng thử lại.",
			formatCurrency(float64(totalSplitAmount)),
			formatCurrency(float64(transaction.Amount)),
		))
		return
	}

	// Save the splits
	for _, split := range splits {
		err := b.BotInjector.TransactionRepository.CreateSplit(&models.TransactionSplit{
			TransactionID: transactionID,
			Name:          split.Name,
			Amount:        split.Amount,
			CreatedAt:     time.Now(),
		})
		if err != nil {
			b.SendMessage(chatID, fmt.Sprintf("Lỗi khi lưu chia tiền: %v", err))
			return
		}
	}

	// Reset user state
	b.userStates[chatID] = UserState{State: StateNone}

	// Send confirmation message
	message := "Đã chia tiền thành công:\n"
	for _, split := range splits {
		message += fmt.Sprintf("%s: %s\n", split.Name, formatCurrency(float64(split.Amount)))
	}

	if totalSplitAmount < transaction.Amount {
		remaining := transaction.Amount - totalSplitAmount
		message += fmt.Sprintf("\nCòn lại: %s", formatCurrency(float64(remaining)))
	}

	b.SendMessage(chatID, message)
}

// SplitEntry represents a single entry in a money split
type SplitEntry struct {
	Name   string
	Amount int64
}
