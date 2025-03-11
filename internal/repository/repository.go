package repository

import (
	"database/sql"

	"github.com/hiiamtrong/imap-bot-go/internal/models"
)

// Repository defines common transaction methods for all repositories
type Repository interface {
	// Transaction executes a function within a database transaction
	Transaction(fn func(tx *sql.Tx) error) error
}

// MailRepository defines methods for mail operations
type IMailRepository interface {
	Repository
	Create(mail *models.Mail) error
	CreateTx(tx *sql.Tx, mail *models.Mail) error
	GetByUID(uid int64) (*models.Mail, error)
	GetNonProcessedMails(uids []int64) ([]int64, error)
}

// TransactionRepository defines methods for transaction operations
type ITransactionRepository interface {
	Repository
	Create(transaction *models.Transaction) error
	CreateTx(tx *sql.Tx, transaction *models.Transaction) error
	GetByMailID(mailID int64) ([]*models.Transaction, error)
	GetByID(id int64) (*models.Transaction, error)
}

// TransactionSplitRepository defines methods for transaction split operations
type ITransactionSplitRepository interface {
	Repository
	Create(split *models.TransactionSplit) error
	CreateTx(tx *sql.Tx, split *models.TransactionSplit) error
	GetByTransactionID(transactionID int64) ([]*models.TransactionSplit, error)
}

// UserRepository defines methods for user operations
type IUserRepository interface {
	Repository
	Create(user *models.User) error
	CreateTx(tx *sql.Tx, user *models.User) error
	GetByID(id int64) (*models.User, error)
}
