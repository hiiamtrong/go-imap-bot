package botpkg

import (
	"github.com/hiiamtrong/imap-bot-go/internal/database"
	"github.com/hiiamtrong/imap-bot-go/internal/repository"
	"github.com/hiiamtrong/imap-bot-go/pkg/smtppkg"
)

type BotInjector struct {
	Database                   *database.Database
	MailRepository             *repository.MailRepository
	TransactionRepository      *repository.TransactionRepository
	TagRepository              *repository.TagRepository
	UserRepository             *repository.UserRepository
	TelegramUserRepository     *repository.TelegramUserRepository
	TransactionSplitRepository *repository.TransactionSplitRepository
	SplitHashRepository        *repository.SplitHashRepository
	SMTP                       *smtppkg.SMTPService
}

func NewBotInjector(
	database *database.Database,
	mailRepository *repository.MailRepository,
	transactionRepository *repository.TransactionRepository,
	tagRepository *repository.TagRepository,
	userRepository *repository.UserRepository,
	telegramUserRepository *repository.TelegramUserRepository,
	transactionSplitRepository *repository.TransactionSplitRepository,
	splitHashRepository *repository.SplitHashRepository,
	smtp *smtppkg.SMTPService,
) *BotInjector {
	return &BotInjector{
		Database:                   database,
		MailRepository:             mailRepository,
		TransactionRepository:      transactionRepository,
		TagRepository:              tagRepository,
		UserRepository:             userRepository,
		TelegramUserRepository:     telegramUserRepository,
		TransactionSplitRepository: transactionSplitRepository,
		SplitHashRepository:        splitHashRepository,
		SMTP:                       smtp,
	}
}
