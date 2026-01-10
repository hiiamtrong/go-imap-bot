package parser

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/hiiamtrong/go-imap-bot/internal/models"
)

// HSBCParser parses credit card transaction emails from HSBC
type HSBCParser struct{}

// Name returns the parser name
func (p *HSBCParser) Name() string {
	return "HSBC"
}

// CanParse checks if this parser can handle the email
func (p *HSBCParser) CanParse(from, subject string) bool {
	// Only parse emails from HSBC with the correct subject format
	// Must contain both "[TB/Alert]" and "Purchase transaction" (Giao dịch được thực hiện)
	return containsIgnoreCase(from, "notification.hsbc.com.hk") &&
		containsIgnoreCase(subject, "[TB/Alert]") &&
		(containsIgnoreCase(subject, "Purchase transaction") || containsIgnoreCase(subject, "Giao dịch được thực hiện"))
}

// Parse extracts transaction details from HSBC email body
func (p *HSBCParser) Parse(body string) (*TransactionDetails, error) {
	// Normalize HTML entities to plain text
	normalizedBody := strings.ReplaceAll(body, "&nbsp;", " ")
	normalizedBody = strings.ReplaceAll(normalizedBody, "&amp;", "&")

	// Pattern for Vietnamese: "số tiền 288,420 VND" or "số tiền 81.77 USD"
	// Supports decimals and various currencies (VND, USD, etc.)
	amountRegex := regexp.MustCompile(`số tiền\s*([\d,.]+)\s*([A-Z]{3})`)

	// Pattern for merchant: "tại Shopee" or "tại CLAUDE.AI SUBSCRIPTION" (before "vào ngày")
	// Captures everything after "tại " until " vào ngày"
	merchantRegex := regexp.MustCompile(`tại\s+(.+?)\s+vào ngày`)

	// Pattern for current balance: "Dư nợ hiện tại là X VND"
	balanceRegex := regexp.MustCompile(`Dư nợ hiện tại là\s*([\d,]+)\s*VND`)

	// Extract amount and currency
	amountMatch := amountRegex.FindStringSubmatch(normalizedBody)
	if amountMatch == nil {
		return nil, fmt.Errorf("failed to extract amount from HSBC email")
	}

	amountStr := amountMatch[1]
	currency := amountMatch[2]

	// Parse amount based on currency
	// For VND: dots and commas are thousand separators (288,420 or 288.420 -> 288420)
	// For foreign currencies: dot is decimal separator (20.00 -> store as original * 100 for cents)
	var amount int64
	var err error
	if currency == "VND" {
		amount, err = parseAmount(amountStr)
	} else {
		amount, err = parseForeignAmount(amountStr)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to parse HSBC amount: %v", err)
	}

	// Extract merchant for description
	description := ""
	merchantMatch := merchantRegex.FindStringSubmatch(normalizedBody)
	if merchantMatch != nil && len(merchantMatch) > 1 {
		description = "Purchase at " + strings.TrimSpace(merchantMatch[1])
		if currency != "VND" {
			description += fmt.Sprintf(" (%s %s)", amountStr, currency)
		}
	}

	// Extract current balance (debt) - always in VND
	var currentBalance int64
	balanceMatch := balanceRegex.FindStringSubmatch(normalizedBody)
	if balanceMatch != nil && len(balanceMatch) > 1 {
		currentBalance, _ = parseAmount(balanceMatch[1])
	}

	return &TransactionDetails{
		Amount:         amount,
		CurrentBalance: currentBalance,
		Description:    description,
		Type:           models.TransactionTypeSubtract, // Always subtract for credit card purchase
		Currency:       currency,
	}, nil
}
