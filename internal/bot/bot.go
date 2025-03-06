package bot

import (
	"context"
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/hiiamtrong/imap-bot-go/internal/config"
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
}

func InitBot(config *config.Config, ctx context.Context, injector *BotInjector) *Bot {
	var err error
	bot, err := tgbotapi.NewBotAPI(config.TelegramBot.Token)
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = false

	log.Println("Bot is running...")

	return &Bot{
		TelegramBot: bot,
		BotInjector: injector,
	}
}

func (b *Bot) SendMessage(chatId int64, message string) {
	msg := tgbotapi.NewMessage(chatId, message)
	b.TelegramBot.Send(msg)
}
