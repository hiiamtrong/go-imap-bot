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
}

// DefaultRegistry is the global parser registry with all bank parsers
var DefaultRegistry *ParserRegistry

func init() {
	DefaultRegistry = NewParserRegistry()
	DefaultRegistry.Register(&TimoParser{})
	DefaultRegistry.Register(&VietcombankParser{})
}

// ParseTransactionEmail parses a transaction email using the appropriate bank parser
func ParseTransactionEmail(from, subject, body string) (*TransactionDetails, error) {
	return DefaultRegistry.Parse(from, subject, body)
}

// parseAmount converts a string amount like "100,000" or "100.000" to int64 (100000)
func parseAmount(amount string) (int64, error) {
	// Remove commas, dots (as thousand separators) and spaces
	cleanAmount := strings.ReplaceAll(amount, ",", "")
	cleanAmount = strings.ReplaceAll(cleanAmount, ".", "")
	cleanAmount = strings.TrimSpace(cleanAmount)

	// Convert to int64
	return strconv.ParseInt(cleanAmount, 10, 64)
}
