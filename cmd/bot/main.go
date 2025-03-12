package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-message/mail"
	"github.com/hiiamtrong/imap-bot-go/internal/botpkg"
	"github.com/hiiamtrong/imap-bot-go/internal/config"
	"github.com/hiiamtrong/imap-bot-go/internal/database"
	"github.com/hiiamtrong/imap-bot-go/internal/models"
	"github.com/hiiamtrong/imap-bot-go/internal/parser"
	"github.com/hiiamtrong/imap-bot-go/internal/repository"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	cfg := config.NewConfig()
	// Setup sqlite
	db, err := database.GetDatabase(cfg.DatabaseConfig)
	if err != nil {
		log.Fatalf("Failed to get database: %v", err)
	}
	defer db.Conn.Close()

	mailRepo := repository.NewMailRepository(db)
	transactionRepo := repository.NewTransactionRepository(db)
	tagRepo := repository.NewTagRepository(db)
	userRepo := repository.NewUserRepository(db)
	telegramUserRepo := repository.NewTelegramUserRepository(db)
	transactionSplitRepo := repository.NewTransactionSplitRepository(db)

	botInjector := botpkg.NewBotInjector(
		db,
		mailRepo,
		transactionRepo,
		tagRepo,
		userRepo,
		telegramUserRepo,
		transactionSplitRepo,
	)
	bot := botpkg.InitBot(cfg, context.Background(), botInjector)

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
	bot *botpkg.Bot,
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
	criteria := imap.SearchCriteria{
		Header: []imap.SearchCriteriaHeaderField{
			{
				Key:   "Subject",
				Value: "Thông báo thay đổi số dư tài khoản",
			},
			{
				Key:   "From",
				Value: "support@timo.vn",
			},
		},
	}

	searchCmd := c.Search(&criteria, nil)

	searchData, err := searchCmd.Wait()
	if err != nil {
		log.Fatalf("Failed to search: %v", err)
	}

	numSets := searchData.All.String()
	pair := strings.Split(numSets, ",")

	availableMails := make([]int64, 0)

	for _, p := range pair {
		numSet := strings.Split(p, ":")
		start, err := strconv.ParseUint(numSet[0], 10, 32)
		if err != nil {
			log.Fatalf("Failed to parse start: %v", err)
		}
		if len(numSet) > 1 {
			end, err := strconv.ParseUint(numSet[1], 10, 32)
			if err != nil {
				log.Fatalf("Failed to parse end: %v", err)
			}

			for i := start; i <= end; i++ {
				availableMails = append(availableMails, int64(i))
			}
		} else {
			availableMails = append(availableMails, int64(start))
		}
	}

	return availableMails
}

func processEmails(c *imapclient.Client, bot *botpkg.Bot, cfg *config.Config, nonProcessedMails []int64) {
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

		processEmail(msg, bot, cfg)
	}

	// Close the message body
	if err := fetchCmd.Close(); err != nil {
		log.Printf("FETCH command failed: %v", err)
	}
}

func processEmail(msg *imapclient.FetchMessageData, bot *botpkg.Bot, cfg *config.Config) {
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

			// Parse transaction details
			details, err := parser.ParseTransactionEmail(bodyText)
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
				transaction.Timestamp.Unix(),
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

	// Only proceed if we have a transaction
	if transaction == nil {
		log.Printf("no transaction found in email")
		return
	}

	// Get the first email address from the To field
	recipientEmail := strings.Split(newMail.To, ",")[0]

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
