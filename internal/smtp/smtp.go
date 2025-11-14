package smtp

import (
	"bytes"
	"embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"net/smtp"
	"time"

	"github.com/hiiamtrong/go-imap-bot/internal/config"
	"github.com/hiiamtrong/go-imap-bot/internal/models"
	"github.com/hiiamtrong/go-imap-bot/internal/vietqr"
	"github.com/hiiamtrong/go-imap-bot/pkg/currencypkg"
)

//go:embed templates/split_bill.html
var mailTemplates embed.FS

//go:embed templates/split_bill_angry.html
var mailTemplatesAngry embed.FS

type SMTPService struct {
	config        *config.Config
	auth          smtp.Auth
	template      *template.Template
	templateAngry *template.Template
	vietQR        *vietqr.VietQRService
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

	tmplAngry, err := template.ParseFS(mailTemplatesAngry, "templates/split_bill_angry.html")
	if err != nil {
		return nil, fmt.Errorf("error parsing email template: %v", err)
	}

	return &SMTPService{
		config:        cfg,
		auth:          auth,
		template:      tmpl,
		templateAngry: tmplAngry,
		vietQR:        vietQR,
	}, nil
}

func (s *SMTPService) SendBulkSplitReminders(user *models.User, splits []*models.TransactionSplit, fromUser string, hash string, mode string) error {
	fmt.Println("SendBulkSplitReminders", user, splits, fromUser, hash, mode)
	splitData := make([]SplitData, len(splits))
	totalAmount := int64(0)
	for i, split := range splits {
		createAt := split.TransactionCreatedAt
		if createAt.IsZero() {
			createAt = split.BillCreatedAt
		}

		splitData[i] = SplitData{
			Amount:          currencypkg.FormatCurrency(float64(split.Amount)),
			Reason:          split.Reason,
			CreatedAt:       createAt.Format("02/01/2006 15:04:05"),
			TotalBillAmount: currencypkg.FormatCurrency(float64(split.TotalBillAmount)),
		}
		totalAmount += split.Amount
	}

	qrCode, err := s.vietQR.GenerateSplitQR(totalAmount, hash)
	if err != nil {
		return fmt.Errorf("error generating QR code: %v", err)
	}

	data := BulkEmailData{
		FromUser:    fromUser,
		TotalAmount: currencypkg.FormatCurrency(float64(totalAmount)),
		Splits:      splitData,
		CreatedAt:   time.Now().Format("02/01/2006 15:04:05"),
		QRCode:      qrCode,
	}

	// Execute template
	var body bytes.Buffer
	if mode == "angry" {
		if err := s.templateAngry.Execute(&body, data); err != nil {
			return fmt.Errorf("error executing template: %v", err)
		}
	} else {
		if err := s.template.Execute(&body, data); err != nil {
			return fmt.Errorf("error executing template: %v", err)
		}
	}

	// Fix email headers format
	headers := map[string]string{
		"From":                      fmt.Sprintf("Bill Split Reminder <%s> to <%s>", s.config.SMTP.From, user.Name),
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

func (s *SMTPService) GetFromEmail() string {
	return s.config.SMTP.From
}
