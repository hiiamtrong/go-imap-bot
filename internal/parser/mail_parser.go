package parser

import (
	"strconv"
	"strings"

	"github.com/hiiamtrong/go-imap-bot/internal/models"
)

// TransactionDetails holds parsed transaction information
type TransactionDetails struct {
	Amount         int64
	CurrentBalance int64
	Description    string
	Type           models.TransactionType
	Currency       string
}

// DefaultRegistry is the global parser registry with all bank parsers
var DefaultRegistry *ParserRegistry

func init() {
	DefaultRegistry = NewParserRegistry()
	DefaultRegistry.Register(&TimoParser{})
	DefaultRegistry.Register(&VietcombankParser{})
	DefaultRegistry.Register(&HSBCParser{})
	DefaultRegistry.Register(&VPBankParser{})
}

// ParseTransactionEmail parses a transaction email using the appropriate bank parser
func ParseTransactionEmail(from, subject, body string) (*TransactionDetails, error) {
	return DefaultRegistry.Parse(from, subject, body)
}

// CanParseEmail checks if any parser can handle the email based on from and subject
// Used for early filtering before saving mail to database
func CanParseEmail(from, subject string) bool {
	for _, p := range DefaultRegistry.parsers {
		if p.CanParse(from, subject) {
			return true
		}
	}
	return false
}

// parseAmount converts a string amount like "100,000" or "100.000" to int64 (100000)
// Used for VND where dots and commas are thousand separators
func parseAmount(amount string) (int64, error) {
	// Remove commas, dots (as thousand separators) and spaces
	cleanAmount := strings.ReplaceAll(amount, ",", "")
	cleanAmount = strings.ReplaceAll(cleanAmount, ".", "")
	cleanAmount = strings.TrimSpace(cleanAmount)

	// Convert to int64
	return strconv.ParseInt(cleanAmount, 10, 64)
}

// parseForeignAmount converts a foreign currency amount like "20.00" or "1,234.56" to int64
// For foreign currencies, dot is decimal separator, comma is thousand separator
// Returns the integer part only (e.g., "20.00" -> 20, "1,234.56" -> 1234)
func parseForeignAmount(amount string) (int64, error) {
	// Remove commas (thousand separators)
	cleanAmount := strings.ReplaceAll(amount, ",", "")
	cleanAmount = strings.TrimSpace(cleanAmount)

	// If there's a dot (decimal separator), take only the integer part
	if idx := strings.Index(cleanAmount, "."); idx != -1 {
		cleanAmount = cleanAmount[:idx]
	}

	// Convert to int64
	return strconv.ParseInt(cleanAmount, 10, 64)
}
