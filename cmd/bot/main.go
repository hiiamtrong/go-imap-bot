package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-message/mail"
	"github.com/hiiamtrong/imap-bot-go/internal/bot"
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

	botInjector := bot.NewBotInjector(mailRepo, transactionRepo)
	bot := bot.InitBot(cfg, context.Background(), botInjector)

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
	bot *bot.Bot,
) {
	// Select the mailbox (e.g., INBOX)
	_, err := c.Select(cfg.MailConfig.Mailbox, nil).Wait()
	if err != nil {
		log.Fatalf("Failed to select mailbox: %v", err)
	}
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

	nonProcessedMails, err := bot.BotInjector.MailRepository.GetNonProcessedMails(availableMails)
	if err != nil {
		log.Printf("failed to get non processed mails: %v", err)
	}

	if len(nonProcessedMails) == 0 {
		fmt.Println("No non processed mails")
		return
	}

	fmt.Printf("Non Processed Mails: %+v\n", nonProcessedMails)

	seqSet := imap.SeqSet{}
	for _, mail := range nonProcessedMails {
		seqSet.AddNum(uint32(mail))
	}

	fmt.Printf("Seq Set: %+v\n", seqSet)
	// Use searchData.All directly as it implements imap.NumSet
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
			continue
		}

		newMail := &models.Mail{
			UID: int64(msg.SeqNum),
		}

		// Read the message via the go-message library
		mr, err := mail.CreateReader(bodySection.Literal)
		if err != nil {
			log.Printf("failed to create mail reader: %v", err)
			continue
		}

		// Print a few header fields
		h := mr.Header
		if date, err := h.Date(); err != nil {
			log.Printf("failed to parse Date header field: %v", err)
			continue
		} else {
			newMail.Date = date
			log.Printf("Date: %v", date)
		}
		if to, err := h.AddressList("To"); err != nil {
			log.Printf("failed to parse To header field: %v", err)
			continue
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
			continue
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
			continue
		} else {
			newMail.Subject = subject
			log.Printf("Subject: %v", subject)
		}

		err = bot.BotInjector.MailRepository.Create(newMail)
		if err != nil {
			log.Printf("failed to create mail: %v", err)
			continue
		}

		fmt.Printf("New Mail: %+v\n", newMail)

		transaction := &models.Transaction{
			MailID: newMail.ID,
		}

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

				transaction.Amount = details.Amount
				transaction.CurrentBalance = details.CurrentBalance
				transaction.Description = details.Description
				transaction.Type = string(details.Type)
				transaction.Timestamp = newMail.Date
				transaction.CreatedAt = time.Now()
				transaction.From = newMail.From
				transaction.To = newMail.To

				err = bot.BotInjector.TransactionRepository.Create(transaction)
				if err != nil {
					log.Printf("failed to create transaction: %v", err)
					continue
				}

				SendMessageTransaction(bot, cfg.TelegramBot.ChatID, transaction)
			}
		}

		// Move to next message at the end of the loop
		fmt.Printf("Moving to next message\n")
	}

	// Close the message body
	if err := fetchCmd.Close(); err != nil {
		log.Printf("FETCH command failed: %v", err)
	}
}

func SendMessageTransaction(bot *bot.Bot, chatId int64, transaction *models.Transaction) {
	// Format amount with VND and determine if it's increasing or decreasing
	amountType := "Tăng"
	if transaction.Type == string(models.TransactionTypeSubtract) {
		amountType = "Giảm"
	}

	formattedAmount := formatCurrency(math.Abs(float64(transaction.Amount)))
	formattedBalance := formatCurrency(math.Abs(float64(transaction.CurrentBalance)))
	// Load Asia/Ho_Chi_Minh location
	location, _ := time.LoadLocation("Asia/Ho_Chi_Minh")

	// Format the message with timezone-adjusted timestamp
	message := fmt.Sprintf(
		"Người gửi: %s\n"+
			"Số tiền: %s %s\n"+
			"Số dư: %s\n"+
			"Thời gian: %s\n"+
			"Mô tả: %s",
		transaction.From,
		amountType,
		formattedAmount,
		formattedBalance,
		transaction.Timestamp.In(location).Format("02/01/2006 15:04:05"),
		transaction.Description,
	)

	// Send the formatted message
	bot.SendMessage(chatId, message)
}

func formatCurrency(amount float64) string {
	// Convert to integer to avoid floating point precision issues
	intAmount := int64(amount)

	// Convert to string
	str := fmt.Sprintf("%d", intAmount)

	// Add thousand separators
	var result []rune
	length := len(str)
	for i, char := range str {
		result = append(result, char)
		if (length-i-1)%3 == 0 && i != length-1 {
			result = append(result, '.')
		}
	}

	return string(result) + " VND"
}
