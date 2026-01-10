package parser

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/emersion/go-message/mail"
	"github.com/hiiamtrong/go-imap-bot/internal/models"
)

// Test with raw HTML body (USD transaction simulation)
func TestVPBankParser_ParseUSD(t *testing.T) {
	// Simulated USD transaction HTML (based on VPBank email structure)
	// Amount field comes BEFORE description field (important for regex ordering)
	body := `
<h5 style="color:#ff0000"><b>- 20 USD</b></h5><p style="margin:0">Số tiền thay đổi / Changed Amount</p>
<h5 style="color:#333333"><b>VPB2601091234567890</b></h5><p style="margin:0">Nội dung / <span><em>Transaction Content</em></span></p>
<h5 style="color:#008000"><b>42,000,000 VND</b></h5><p style="margin:0">Hạn mức còn lại / Available Limit</p>
`

	parser := &VPBankParser{}
	result, err := parser.Parse(body)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if result.Amount != 20 {
		t.Errorf("Expected amount 20, got %d", result.Amount)
	}
	if result.Currency != "USD" {
		t.Errorf("Expected currency USD, got %s", result.Currency)
	}
	if result.Type != models.TransactionTypeSubtract {
		t.Errorf("Expected type Subtract, got %v", result.Type)
	}
	// Description should be the transaction code, not the amount
	if result.Description == "" {
		t.Error("Description is empty")
	} else {
		t.Logf("Description: %s", result.Description)
		// Should be "VPB2601091234567890 (20 USD)"
		if !strings.Contains(result.Description, "VPB2601091234567890") {
			t.Errorf("Expected description to contain transaction code VPB2601091234567890, got: %s", result.Description)
		}
	}
	if result.CurrentBalance != 42000000 {
		t.Errorf("Expected current balance 42000000, got %d", result.CurrentBalance)
	}
}

func TestVPBankParser_ParseMerchantName(t *testing.T) {
	// Read the new email template with merchant name as description
	data, err := os.ReadFile("templates/vpbank-1.eml")
	if err != nil {
		t.Fatalf("Failed to read vpbank-1.eml: %v", err)
	}

	// Parse the email to extract body (handles quoted-printable decoding)
	r, err := mail.CreateReader(strings.NewReader(string(data)))
	if err != nil {
		t.Fatalf("Failed to create mail reader: %v", err)
	}

	var body string
	for {
		p, err := r.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Failed to read part: %v", err)
		}

		switch p.Header.(type) {
		case *mail.InlineHeader:
			b, _ := io.ReadAll(p.Body)
			body = string(b)
		}
	}

	t.Logf("Decoded body length: %d", len(body))

	// Test the parser
	parser := &VPBankParser{}
	result, err := parser.Parse(body)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Verify results - this email has 6.45 USD
	if result.Amount != 645 {
		t.Errorf("Expected amount 645 (6.45 * 100), got %d", result.Amount)
	}
	if result.Currency != "USD" {
		t.Errorf("Expected currency USD, got %s", result.Currency)
	}
	if result.Type != models.TransactionTypeSubtract {
		t.Errorf("Expected type Subtract, got %v", result.Type)
	}
	// Description should be "GITHUB, INC." (merchant name)
	if result.Description == "" {
		t.Error("Description is empty - this is the bug we're debugging")
		// Print the relevant section for debugging
		if idx := strings.Index(body, "GITHUB"); idx != -1 {
			start := idx - 50
			if start < 0 {
				start = 0
			}
			end := idx + 200
			if end > len(body) {
				end = len(body)
			}
			t.Logf("Context around GITHUB:\n%s", body[start:end])
		}
	} else {
		t.Logf("Description: %s", result.Description)
		if !strings.Contains(result.Description, "GITHUB") {
			t.Errorf("Expected description to contain GITHUB, got: %s", result.Description)
		}
	}
	if result.CurrentBalance != 10026111 {
		t.Errorf("Expected current balance 10026111, got %d", result.CurrentBalance)
	}
}

func TestVPBankParser_Parse(t *testing.T) {
	// Read the email template
	data, err := os.ReadFile("templates/vpbank.eml")
	if err != nil {
		t.Fatalf("Failed to read vpbank.eml: %v", err)
	}

	// Parse the email to extract body (handles quoted-printable decoding)
	r, err := mail.CreateReader(strings.NewReader(string(data)))
	if err != nil {
		t.Fatalf("Failed to create mail reader: %v", err)
	}

	var body string
	for {
		p, err := r.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Failed to read part: %v", err)
		}

		switch p.Header.(type) {
		case *mail.InlineHeader:
			b, _ := io.ReadAll(p.Body)
			body = string(b)
		}
	}

	t.Logf("Decoded body length: %d", len(body))

	// Test the parser
	parser := &VPBankParser{}
	result, err := parser.Parse(body)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Verify results
	if result.Amount != 3536014 {
		t.Errorf("Expected amount 3536014, got %d", result.Amount)
	}
	if result.Currency != "VND" {
		t.Errorf("Expected currency VND, got %s", result.Currency)
	}
	if result.Description == "" {
		t.Error("Description is empty - this is the bug we're debugging")
		// Print the relevant section for debugging
		if idx := strings.Index(body, "VPB260109"); idx != -1 {
			start := idx - 50
			if start < 0 {
				start = 0
			}
			end := idx + 200
			if end > len(body) {
				end = len(body)
			}
			t.Logf("Context around VPB260109:\n%s", body[start:end])
		}
	} else {
		t.Logf("Description: %s", result.Description)
	}
	if result.CurrentBalance != 42000000 {
		t.Errorf("Expected current balance 42000000, got %d", result.CurrentBalance)
	}
}
