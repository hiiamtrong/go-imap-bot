package botpkg

import (
	"github.com/hiiamtrong/imap-bot-go/internal/database"
	"github.com/hiiamtrong/imap-bot-go/internal/repository"
)

type BotInjector struct {
	Database              *database.Database
	MailRepository        *repository.MailRepository
	TransactionRepository *repository.TransactionRepository
	TagRepository         *repository.TagRepository
	UserRepository        *repository.UserRepository
}

func NewBotInjector(
	database *database.Database,
	mailRepository *repository.MailRepository,
	transactionRepository *repository.TransactionRepository,
	tagRepository *repository.TagRepository,
	userRepository *repository.UserRepository,
) *BotInjector {
	return &BotInjector{
		Database:              database,
		MailRepository:        mailRepository,
		TransactionRepository: transactionRepository,
		TagRepository:         tagRepository,
		UserRepository:        userRepository,
	}
}
