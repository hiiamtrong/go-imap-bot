package smtppkg

import (
	"bytes"
	"embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"net/smtp"
	"time"

	"github.com/hiiamtrong/imap-bot-go/internal/config"
	"github.com/hiiamtrong/imap-bot-go/internal/models"
	"github.com/hiiamtrong/imap-bot-go/internal/vietqr"
	"github.com/hiiamtrong/imap-bot-go/pkg/currency"
)

//go:embed templates/split_bill.html
var mailTemplates embed.FS

type SMTPService struct {
	config   *config.Config
	auth     smtp.Auth
	template *template.Template
	vietQR   *vietqr.VietQRService
}

type EmailData struct {
	FromUser    string
	Amount      string
	Reason      string
	Transaction string
	CreatedAt   string
}

type BulkEmailData struct {
	FromUser    string
	TotalAmount string
	Splits      []SplitData
	CreatedAt   string
	QRCode      string
}

type SplitData struct {
	Amount          string
	Reason          string
	CreatedAt       string
	TotalBillAmount string
}

func NewSMTPService(cfg *config.Config, vietQR *vietqr.VietQRService) (*SMTPService, error) {
	auth := smtp.PlainAuth("", cfg.SMTP.User, cfg.SMTP.Pass, cfg.SMTP.Host)

	// Parse template during initialization
	tmpl, err := template.ParseFS(mailTemplates, "templates/split_bill.html")
	if err != nil {
		return nil, fmt.Errorf("error parsing email template: %v", err)
	}

	return &SMTPService{
		config:   cfg,
		auth:     auth,
		template: tmpl,
		vietQR:   vietQR,
	}, nil
}

func (s *SMTPService) SendSplitReminder(toEmail string, split *models.TransactionSplit, fromUser string, amount string) error {
	// Prepare email data
	data := EmailData{
		FromUser:    fromUser,
		Amount:      amount,
		Reason:      split.Reason,
		Transaction: fmt.Sprintf("%d", split.TransactionID),
		CreatedAt:   split.CreatedAt.Format("02/01/2006 15:04:05"),
	}

	// Execute template
	var body bytes.Buffer
	if err := s.template.Execute(&body, data); err != nil {
		return fmt.Errorf("error executing template: %v", err)
	}

	// Email headers
	headers := make(map[string]string)
	headers["From"] = s.config.SMTP.From
	headers["To"] = toEmail
	headers["Subject"] = "Nhắc nhở thanh toán"
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "text/html; charset=UTF-8"

	// Build email message
	var message bytes.Buffer
	for k, v := range headers {
		message.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	message.WriteString("\r\n")
	message.Write(body.Bytes())

	// Send email
	err := smtp.SendMail(
		fmt.Sprintf("%s:%d", s.config.SMTP.Host, s.config.SMTP.Port),
		s.auth,
		s.config.SMTP.From,
		[]string{toEmail},
		message.Bytes(),
	)
	if err != nil {
		return fmt.Errorf("error sending email: %v", err)
	}

	return nil
}

func (s *SMTPService) SendBulkSplitReminders(user *models.User, splits []*models.TransactionSplit, fromUser string, hash string) error {
	splitData := make([]SplitData, len(splits))
	totalAmount := int64(0)
	for i, split := range splits {
		splitData[i] = SplitData{
			Amount:          currency.FormatCurrency(float64(split.Amount)),
			Reason:          split.Reason,
			CreatedAt:       split.CreatedAt.Format("02/01/2006 15:04:05"),
			TotalBillAmount: currency.FormatCurrency(float64(split.TotalBillAmount)),
		}
		totalAmount += split.Amount
	}

	qrCode, err := s.vietQR.GenerateSplitQR(totalAmount, hash)
	if err != nil {
		return fmt.Errorf("error generating QR code: %v", err)
	}

	data := BulkEmailData{
		FromUser:    fromUser,
		TotalAmount: currency.FormatCurrency(float64(totalAmount)),
		Splits:      splitData,
		CreatedAt:   time.Now().Format("02/01/2006 15:04:05"),
		QRCode:      qrCode,
	}

	// Execute template
	var body bytes.Buffer
	if err := s.template.Execute(&body, data); err != nil {
		return fmt.Errorf("error executing template: %v", err)
	}

	// Fix email headers format
	headers := map[string]string{
		"From":                      fmt.Sprintf("Bill Split Reminder <%s>", s.config.SMTP.From),
		"To":                        user.Email,
		"Subject":                   "=?utf-8?B?" + base64.StdEncoding.EncodeToString([]byte("Tổng hợp các khoản cần thanh toán")) + "?=",
		"MIME-Version":              "1.0",
		"Content-Type":              "text/html; charset=UTF-8",
		"Content-Transfer-Encoding": "base64",
	}

	var message bytes.Buffer
	for k, v := range headers {
		message.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	message.WriteString("\r\n")
	message.WriteString(base64.StdEncoding.EncodeToString(body.Bytes()))
	message.WriteString("\r\n")

	return smtp.SendMail(
		fmt.Sprintf("%s:%d", s.config.SMTP.Host, s.config.SMTP.Port),
		s.auth,
		s.config.SMTP.From,
		[]string{user.Email},
		message.Bytes(),
	)
}
