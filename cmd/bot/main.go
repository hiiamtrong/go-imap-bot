package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-message/mail"
	"github.com/hiiamtrong/go-imap-bot/internal/config"
	"github.com/hiiamtrong/go-imap-bot/internal/database"
	"github.com/hiiamtrong/go-imap-bot/internal/imapbot"
	"github.com/hiiamtrong/go-imap-bot/internal/models"
	"github.com/hiiamtrong/go-imap-bot/internal/parser"
	"github.com/hiiamtrong/go-imap-bot/internal/repository"
	"github.com/hiiamtrong/go-imap-bot/internal/s3"
	"github.com/hiiamtrong/go-imap-bot/internal/smtp"
	"github.com/hiiamtrong/go-imap-bot/internal/vietqr"
	"github.com/hiiamtrong/go-imap-bot/pkg/regexpkg"
	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/viper"
)

// getStartDate returns the configured start date for filtering emails
// Format: DD/MM/YYYY
func getStartDate() *time.Time {
	dateStr := viper.GetString("START_DATE")
	if dateStr == "" {
		return nil
	}
	t, err := time.Parse("02/01/2006", dateStr)
	if err != nil {
		log.Printf("Invalid START_DATE format: %v", err)
		return nil
	}
	return &t
}

func main() {
	cfg := config.NewConfig()
	// Setup sqlite
	db, err := database.GetDatabase(cfg.DatabaseConfig)
	if err != nil {
		log.Fatalf("Failed to get database: %v", err)
	}
	defer db.Conn.Close()

	s3Service, err := s3.NewS3Service(cfg)
	if err != nil {
		log.Fatalf("Failed to get S3 service: %v", err)
	}

	vietQR := vietqr.NewVietQRService(cfg, s3Service)

	smtp, err := smtp.NewSMTPService(cfg, vietQR)
	if err != nil {
		log.Fatalf("Failed to get SMTP service: %v", err)
	}

	mailRepo := repository.NewMailRepository(db)
	transactionRepo := repository.NewTransactionRepository(db)
	tagRepo := repository.NewTagRepository(db)
	userRepo := repository.NewUserRepository(db)
	telegramUserRepo := repository.NewTelegramUserRepository(db)
	transactionSplitRepo := repository.NewTransactionSplitRepository(db)
	splitHashRepo := repository.NewSplitHashRepository(db)

	botInjector := imapbot.NewBotInjector(
		db,
		mailRepo,
		transactionRepo,
		tagRepo,
		userRepo,
		telegramUserRepo,
		transactionSplitRepo,
		splitHashRepo,
		smtp,
	)
	bot := imapbot.InitBot(cfg, context.Background(), botInjector)

	// Connect to the IMAP server using TLS
	c, err := imapclient.DialTLS(cfg.MailConfig.Server, nil)
	if err != nil {
		log.Fatalf("Failed to connect to server: %v", err)
	}

	// Ensure we logout when done
	defer func() {
		cmd := c.Logout()
		if err := cmd.Wait(); err != nil {
			log.Printf("Failed to logout: %v", err)
		}

		fmt.Println("Logged out successfully.")
	}()

	cmd := c.Login(cfg.MailConfig.Username, cfg.MailConfig.Password)
	if err := cmd.Wait(); err != nil {
		log.Fatalf("Failed to login: %v", err)
	}
	fmt.Println("Logged in successfully.")

	for {
		runBot(cfg, c, bot)
		time.Sleep(1 * time.Second)
	}
}

func runBot(
	cfg *config.Config,
	c *imapclient.Client,
	bot *imapbot.Bot,
) {
	// Select the mailbox (e.g., INBOX)
	_, err := c.Select(cfg.MailConfig.Mailbox, nil).Wait()
	if err != nil {
		log.Fatalf("Failed to select mailbox: %v", err)
	}

	availableMails := searchEmails(c)
	nonProcessedMails, err := bot.BotInjector.MailRepository.GetNonProcessedMails(availableMails)
	if err != nil {
		log.Printf("failed to get non processed mails: %v", err)
	}

	if len(nonProcessedMails) == 0 {
		fmt.Println("No non processed mails")
		return
	}

	fmt.Printf("Non Processed Mails: %+v\n", nonProcessedMails)
	processEmails(c, bot, cfg, nonProcessedMails)
}

func searchEmails(c *imapclient.Client) []int64 {
	availableMails := make([]int64, 0)

	// Search criteria for different banks
	bankCriteria := []imap.SearchCriteria{
		// Timo bank
		{
			Header: []imap.SearchCriteriaHeaderField{
				{Key: "Subject", Value: "Thông báo thay đổi số dư tài khoản"},
				{Key: "From", Value: "support@timo.vn"},
			},
		},
		// Vietcombank
		{
			Header: []imap.SearchCriteriaHeaderField{
				{Key: "Subject", Value: "Biên lai chuyển tiền"},
				{Key: "From", Value: "info.vietcombank.com.vn"},
			},
		},
	}

	for _, criteria := range bankCriteria {
		searchCmd := c.Search(&criteria, nil)
		searchData, err := searchCmd.Wait()
		if err != nil {
			log.Printf("Failed to search for criteria: %v", err)
			continue
		}

		numSets := searchData.All.String()
		if numSets == "" {
			continue
		}

		pair := strings.Split(numSets, ",")
		for _, p := range pair {
			numSet := strings.Split(p, ":")
			start, err := strconv.ParseUint(numSet[0], 10, 32)
			if err != nil {
				log.Printf("Failed to parse start: %v", err)
				continue
			}
			if len(numSet) > 1 {
				end, err := strconv.ParseUint(numSet[1], 10, 32)
				if err != nil {
					log.Printf("Failed to parse end: %v", err)
					continue
				}
				for i := start; i <= end; i++ {
					availableMails = append(availableMails, int64(i))
				}
			} else {
				availableMails = append(availableMails, int64(start))
			}
		}
	}

	return availableMails
}

func processEmails(c *imapclient.Client, bot *imapbot.Bot, cfg *config.Config, nonProcessedMails []int64) {
	seqSet := imap.SeqSet{}
	for _, mail := range nonProcessedMails {
		seqSet.AddNum(uint32(mail))
	}

	fmt.Printf("Seq Set: %+v\n", seqSet)
	fetchCmd := c.Fetch(seqSet, &imap.FetchOptions{
		Envelope:    true,
		UID:         true,
		BodySection: []*imap.FetchItemBodySection{{}},
	})

	for {
		msg := fetchCmd.Next()
		if msg == nil {
			break
		}

		processEmail(msg, bot)
	}

	// Close the message body
	if err := fetchCmd.Close(); err != nil {
		log.Printf("FETCH command failed: %v", err)
	}
}

func processEmail(msg *imapclient.FetchMessageData, bot *imapbot.Bot) {
	var bodySection imapclient.FetchItemDataBodySection
	ok := false
	for {
		item := msg.Next()
		if item == nil {
			break
		}
		bodySection, ok = item.(imapclient.FetchItemDataBodySection)
		if ok {
			break
		}
	}
	if !ok {
		log.Printf("FETCH command did not return body section")
		return
	}

	newMail := &models.Mail{
		UID: int64(msg.SeqNum),
	}

	// Read the message via the go-message library
	mr, err := mail.CreateReader(bodySection.Literal)
	if err != nil {
		log.Printf("failed to create mail reader: %v", err)
		return
	}

	if !parseMailHeaders(mr, newMail) {
		return
	}

	// Check if email is before start date
	skipTransaction := false
	if startDate := getStartDate(); startDate != nil {
		if newMail.Date.Before(*startDate) {
			log.Printf("Email before start date, saving mail record but skipping transaction: date %v < %v", newMail.Date, *startDate)
			skipTransaction = true
		}
	}

	// Get database connection

	// Start transaction
	tx, err := bot.BotInjector.Database.BeginTx(context.Background())
	if err != nil {
		log.Printf("failed to begin transaction: %v", err)
		return
	}

	// Ensure rollback on error
	defer func() {
		if tx != nil {
			tx.Rollback()
		}
	}()

	// Create mail record
	query := `
		INSERT INTO mails (uid, subject, from_account, to_account, timestamp)
		VALUES (?, ?, ?, ?, ?)
	`
	result, err := tx.Exec(query, newMail.UID, newMail.Subject, newMail.From, newMail.To, newMail.Date.Unix())
	if err != nil {
		log.Printf("failed to create mail: %v", err)
		return
	}

	id, err := result.LastInsertId()
	if err != nil {
		log.Printf("failed to get last insert id: %v", err)
		return
	}
	newMail.ID = id

	fmt.Printf("New Mail: %+v\n", newMail)

	// If email is before start date, just save mail record and skip transaction processing
	if skipTransaction {
		if err = tx.Commit(); err != nil {
			log.Printf("failed to commit mail record: %v", err)
			return
		}
		tx = nil
		fmt.Printf("Saved mail record (skipped transaction) for UID: %d\n", newMail.UID)
		return
	}

	var transaction *models.Transaction

	// Process the message's parts
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		} else if err != nil {
			log.Printf("failed to read message part: %v", err)
			continue
		}

		switch p.Header.(type) {
		case *mail.InlineHeader:
			b, _ := io.ReadAll(p.Body)
			bodyText := string(b)

			// Parse transaction details (pass from and subject for bank detection)
			details, err := parser.ParseTransactionEmail(newMail.From, newMail.Subject, bodyText)
			if err != nil {
				log.Printf("Failed to parse transaction details: %v", err)
				continue
			}

			transaction = &models.Transaction{
				MailID:         newMail.ID,
				Amount:         details.Amount,
				CurrentBalance: details.CurrentBalance,
				Description:    details.Description,
				Type:           string(details.Type),
				Timestamp:      newMail.Date,
				CreatedAt:      time.Now(),
				From:           newMail.From,
				To:             newMail.To,
			}

			// Create transaction within transaction
			query := `
				INSERT INTO transactions (
					mail_id, amount, current_balance, type,
					from_account, to_account, description, 
					timestamp, created_at
				)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			`
			result, err := tx.Exec(
				query,
				transaction.MailID,
				transaction.Amount,
				transaction.CurrentBalance,
				transaction.Type,
				transaction.From,
				transaction.To,
				transaction.Description,
				transaction.Timestamp,
				time.Now(),
			)
			if err != nil {
				log.Printf("failed to create transaction: %v", err)
				return
			}

			id, err := result.LastInsertId()
			if err != nil {
				log.Printf("failed to get last insert id: %v", err)
				return
			}
			transaction.ID = id
		}
	}

	// If no transaction found (parsing failed or filtered), still save mail record
	if transaction == nil {
		log.Printf("no transaction found in email, saving mail record only")
		if err = tx.Commit(); err != nil {
			log.Printf("failed to commit mail record: %v", err)
			return
		}
		tx = nil
		fmt.Printf("Saved mail record (no transaction) for UID: %d\n", newMail.UID)
		return
	}

	// Get the first email address from the To field
	recipientEmail := strings.Split(newMail.To, ",")[0]

	// Check if transaction is a bill split (optional - don't block on errors)
	if matched, _ := regexp.MatchString(`(trx|TRX)\w+(ong|ONG)`, transaction.Description); matched {
		hash, err := regexpkg.ExtractHash(transaction.Description)
		if err != nil {
			log.Printf("failed to extract hash (skipping split): %v", err)
		} else {
			splitIDs, err := bot.BotInjector.SplitHashRepository.GetSplitIDs(hash)
			if err != nil {
				log.Printf("split hash not found (skipping split): %v", err)
			} else {
				err = bot.BotInjector.TransactionSplitRepository.UpdateSplitStatus(splitIDs, tx)
				if err != nil {
					log.Printf("failed to update split status: %v", err)
				} else {
					err = bot.NotifySplitBillComplete(recipientEmail, splitIDs, transaction.ID, tx)
					if err != nil {
						log.Printf("failed to notify split bill: %v", err)
					}
				}
			}
		}
	}

	err = bot.NotifyNewTransaction(transaction, recipientEmail, tx)
	if err != nil {
		log.Printf("failed to notify users: %v", err)
		return
	}

	// Store the transaction in the database first
	if err = tx.Commit(); err != nil {
		log.Printf("failed to commit transaction: %v", err)
		return
	}

	// Set tx to nil so the deferred rollback won't try to roll back a committed transaction
	tx = nil

	fmt.Printf("Successfully processed email with UID: %d\n", newMail.UID)
}

func parseMailHeaders(mr *mail.Reader, newMail *models.Mail) bool {
	h := mr.Header
	if date, err := h.Date(); err != nil {
		log.Printf("failed to parse Date header field: %v", err)
		return false
	} else {
		newMail.Date = date
		log.Printf("Date: %v", date)
	}

	if to, err := h.AddressList("To"); err != nil {
		log.Printf("failed to parse To header field: %v", err)
		return false
	} else {
		toAddresses := make([]string, len(to))
		for i, addr := range to {
			toAddresses[i] = addr.Address
		}
		newMail.To = strings.Join(toAddresses, ",")
		log.Printf("To: %v", toAddresses)
	}

	if from, err := h.AddressList("From"); err != nil {
		log.Printf("failed to parse From header field: %v", err)
		return false
	} else {
		fromAddresses := make([]string, len(from))
		for i, addr := range from {
			fromAddresses[i] = addr.Address
		}
		newMail.From = strings.Join(fromAddresses, ",")
		log.Printf("From: %v", fromAddresses)
	}

	if subject, err := h.Text("Subject"); err != nil {
		log.Printf("failed to parse Subject header field: %v", err)
		return false
	} else {
		newMail.Subject = subject
		log.Printf("Subject: %v", subject)
	}

	return true
}
