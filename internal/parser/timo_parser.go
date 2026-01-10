package parser

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/hiiamtrong/go-imap-bot/internal/models"
)

// TimoParser parses transaction emails from Timo bank
type TimoParser struct{}

// Name returns the parser name
func (p *TimoParser) Name() string {
	return "Timo"
}

// CanParse checks if this parser can handle the email
func (p *TimoParser) CanParse(from, subject string) bool {
	return containsIgnoreCase(from, "timo.vn")
}

// Parse extracts transaction details from Timo email body
func (p *TimoParser) Parse(body string) (*TransactionDetails, error) {
	// Regular expressions for extracting information
	amountRegexSubtract := regexp.MustCompile(`giảm ([\d,.]+) VND`)
	amountRegexAdd := regexp.MustCompile(`tăng ([\d,.]+) VND`)
	balanceRegex := regexp.MustCompile(`Số dư hiện tại: ([\d,.]+) VND`)
	descRegex := regexp.MustCompile(`Mô tả: (.+)\.`)

	var amount int64
	var transactionType models.TransactionType

	// Try to match subtract first
	amountMatch := amountRegexSubtract.FindStringSubmatch(body)
	if len(amountMatch) >= 2 {
		parsedAmount, err := parseAmount(amountMatch[1])
		if err != nil {
			return nil, fmt.Errorf("failed to parse subtract amount: %v", err)
		}
		amount = parsedAmount
		transactionType = models.TransactionTypeSubtract
	} else {
		// Try to match add
		amountMatch = amountRegexAdd.FindStringSubmatch(body)
		if len(amountMatch) >= 2 {
			parsedAmount, err := parseAmount(amountMatch[1])
			if err != nil {
				return nil, fmt.Errorf("failed to parse add amount: %v", err)
			}
			amount = parsedAmount
			transactionType = models.TransactionTypeAdd
		} else {
			return nil, fmt.Errorf("could not find transaction amount in email body")
		}
	}

	// Extract current balance
	balanceMatch := balanceRegex.FindStringSubmatch(body)
	if len(balanceMatch) < 2 {
		return nil, fmt.Errorf("could not find current balance in email body")
	}
	balance, err := parseAmount(balanceMatch[1])
	if err != nil {
		return nil, fmt.Errorf("failed to parse balance: %v", err)
	}

	// Extract description
	descMatch := descRegex.FindStringSubmatch(body)
	if len(descMatch) < 2 {
		return nil, fmt.Errorf("could not find transaction description in email body")
	}
	description := strings.TrimSpace(descMatch[1])

	return &TransactionDetails{
		Amount:         amount,
		CurrentBalance: balance,
		Description:    description,
		Type:           transactionType,
	}, nil
}
