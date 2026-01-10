package parser

import (
	"fmt"
	"strings"
)

// BankParser interface for parsing transaction emails from different banks
type BankParser interface {
	Name() string
	CanParse(from, subject string) bool
	Parse(body string) (*TransactionDetails, error)
}

// ParserRegistry manages multiple bank parsers
type ParserRegistry struct {
	parsers []BankParser
}

// NewParserRegistry creates a new parser registry
func NewParserRegistry() *ParserRegistry {
	return &ParserRegistry{parsers: make([]BankParser, 0)}
}

// Register adds a parser to the registry
func (r *ParserRegistry) Register(p BankParser) {
	r.parsers = append(r.parsers, p)
}

// Parse finds the appropriate parser and parses the email
func (r *ParserRegistry) Parse(from, subject, body string) (*TransactionDetails, error) {
	for _, p := range r.parsers {
		if p.CanParse(from, subject) {
			return p.Parse(body)
		}
	}
	return nil, fmt.Errorf("no parser found for email from: %s, subject: %s", from, subject)
}

// containsIgnoreCase checks if s contains substr (case-insensitive)
func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
