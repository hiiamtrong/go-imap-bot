package parser

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/hiiamtrong/go-imap-bot/internal/models"
)

// VPBankParser parses credit card balance change emails from VPBank
type VPBankParser struct{}

// Name returns the parser name
func (p *VPBankParser) Name() string {
	return "VPBank"
}

// CanParse checks if this parser can handle the email
func (p *VPBankParser) CanParse(from, subject string) bool {
	// Only parse emails from VPBank with the correct subject format
	return containsIgnoreCase(from, "care.vpb.com.vn") &&
		containsIgnoreCase(subject, "VPBank xin thong bao bien dong so du The tin dung")
}

// Parse extracts transaction details from VPBank email body
func (p *VPBankParser) Parse(body string) (*TransactionDetails, error) {
	// Pattern for amount with +/- prefix: "<b>+ 3,536,014 VND</b>" or "<b>- 10 USD</b>"
	// Supports multiple currencies and decimal amounts
	amountRegex := regexp.MustCompile(`<b>([+-])\s*([\d,.]+)\s*([A-Z]{3})</b>`)

	// Pattern for description/transaction content
	// Match: <b>GITHUB, INC.</b></h5><p...>Nội dung or Transaction Content
	// Or: <b>VPB2601091878690593</b></h5><p...>Nội dung or Transaction Content
	// Use (?s) for DOTALL mode to match across newlines
	// Allow optional whitespace between tags for robustness
	// Use [^<+-] as first character to exclude amount fields (which start with + or -)
	// Then [^<]* to capture rest of content (merchant name or transaction code)
	descRegex := regexp.MustCompile(`(?s)<b>([^<+-][^<]*)</b>\s*</h5>\s*<p[^>]*>.*?(?:dung|Transaction Content)`)

	// Pattern for available limit: "<b>42,000,000 VND</b></h5><p...>...Hạn mức còn lại"
	// Allow optional whitespace between tags for robustness
	limitRegex := regexp.MustCompile(`(?s)<b>([\d,]+)\s*VND</b>\s*</h5>\s*<p[^>]*>.*?(?:còn l|Available)`)

	// Extract amount, currency and type
	amountMatch := amountRegex.FindStringSubmatch(body)
	if amountMatch == nil {
		return nil, fmt.Errorf("failed to extract amount from VPBank email")
	}

	sign := amountMatch[1]
	amountStr := amountMatch[2]
	currency := amountMatch[3]

	amount, err := parseAmount(amountStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse VPBank amount: %v", err)
	}

	// Determine transaction type based on sign
	txType := models.TransactionTypeAdd
	if sign == "-" {
		txType = models.TransactionTypeSubtract
	}

	// Extract description (transaction content)
	description := ""
	descMatch := descRegex.FindStringSubmatch(body)
	if descMatch != nil && len(descMatch) > 1 {
		description = strings.TrimSpace(descMatch[1])
		if currency != "VND" {
			description += fmt.Sprintf(" (%s %s)", amountStr, currency)
		}
	}

	// Extract available limit (use as current balance reference)
	var currentBalance int64
	limitMatch := limitRegex.FindStringSubmatch(body)
	if limitMatch != nil && len(limitMatch) > 1 {
		currentBalance, _ = parseAmount(limitMatch[1])
	}

	return &TransactionDetails{
		Amount:         amount,
		CurrentBalance: currentBalance,
		Description:    description,
		Type:           txType,
		Currency:       currency,
	}, nil
}
