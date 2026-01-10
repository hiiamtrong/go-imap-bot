package parser

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/hiiamtrong/go-imap-bot/internal/models"
	"github.com/spf13/viper"
)

// VietcombankParser parses transaction emails from Vietcombank
type VietcombankParser struct{}

// getAllowedVietcombankAccount returns the allowed account from config
func getAllowedVietcombankAccount() string {
	return viper.GetString("VIETCOMBANK_ACCOUNT")
}


// Name returns the parser name
func (p *VietcombankParser) Name() string {
	return "Vietcombank"
}

// CanParse checks if this parser can handle the email
func (p *VietcombankParser) CanParse(from, subject string) bool {
	return containsIgnoreCase(from, "vietcombank.com.vn")
}

// Parse extracts transaction details from Vietcombank email body
// Vietcombank payment receipts are HTML tables with specific structure
func (p *VietcombankParser) Parse(body string) (*TransactionDetails, error) {
	// Extract debit account (Tài khoản nguồn) to filter by allowed account
	accountRegex := regexp.MustCompile(`(?s)<b>Tài khoản nguồn</b>.*?</td>\s*<td[^>]*>\s*(\d+)\s*</td>`)
	accountMatch := accountRegex.FindStringSubmatch(body)
	if len(accountMatch) < 2 {
		return nil, fmt.Errorf("could not find debit account in Vietcombank email body")
	}

	debitAccount := strings.TrimSpace(accountMatch[1])
	allowedAccount := getAllowedVietcombankAccount()
	if allowedAccount != "" && debitAccount != allowedAccount {
		return nil, fmt.Errorf("skipping Vietcombank email: account %s not in allowed list", debitAccount)
	}

	// Extract amount from HTML table
	// Pattern matches: <b>Số tiền</b>...<td...>167,000 VND</td>
	amountRegex := regexp.MustCompile(`(?s)<b>Số tiền</b>.*?</td>\s*<td[^>]*>\s*([\d,]+)\s*VND`)
	amountMatch := amountRegex.FindStringSubmatch(body)
	if len(amountMatch) < 2 {
		return nil, fmt.Errorf("could not find transaction amount in Vietcombank email body")
	}

	amount, err := parseAmount(amountMatch[1])
	if err != nil {
		return nil, fmt.Errorf("failed to parse amount: %v", err)
	}

	// Extract description from HTML table
	// Pattern matches: <b>Nội dung chuyển tiền</b>...<td...>description</td>
	descRegex := regexp.MustCompile(`(?s)<b>Nội dung chuyển tiền</b>.*?</td>\s*<td[^>]*>\s*([^<]+)\s*</td>`)
	descMatch := descRegex.FindStringSubmatch(body)

	var description string
	if len(descMatch) >= 2 {
		description = strings.TrimSpace(descMatch[1])
	} else {
		// Fallback: try alternative pattern or use default
		description = "Vietcombank transfer"
	}

	// Vietcombank payment receipts are always outgoing transfers (subtract)
	// and don't include current balance
	return &TransactionDetails{
		Amount:         amount,
		CurrentBalance: 0, // Not available in Vietcombank payment receipts
		Description:    description,
		Type:           models.TransactionTypeSubtract,
	}, nil
}
