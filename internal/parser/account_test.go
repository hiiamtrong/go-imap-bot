package parser

import (
	"io"
	"os"
	"testing"

	"github.com/emersion/go-message/mail"
)

func templateBody(t *testing.T, name string) string {
	t.Helper()
	f, err := os.Open("templates/" + name)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r, err := mail.CreateReader(f)
	if err != nil {
		t.Fatal(err)
	}
	var body string
	for {
		p, err := r.NextPart()
		if err == io.EOF {
			return body
		}
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := p.Header.(*mail.InlineHeader); ok {
			b, _ := io.ReadAll(p.Body)
			body = string(b)
		}
	}
}

func TestParsersExtractAccountAndCounterparty(t *testing.T) {
	tests := []struct {
		template     string
		parser       BankParser
		account      string
		counterparty string
	}{
		{"vcb-1.eml", &VietcombankParser{}, "9338613062", "CTTNHH MUJI RRETAIL (0071001345365)"},
		{"hsbc.eml", &HSBCParser{}, "X6194", ""},
		{"vpbank.eml", &VPBankParser{}, "*4118", ""},
		{"vpbank-1.eml", &VPBankParser{}, "*4118", ""},
		{"timo.eml", &TimoParser{}, "Timo Spend Account", ""},
	}
	for _, tt := range tests {
		t.Run(tt.template, func(t *testing.T) {
			got, err := tt.parser.Parse(templateBody(t, tt.template))
			if err != nil {
				t.Fatal(err)
			}
			if got.Account != tt.account || got.Counterparty != tt.counterparty {
				t.Errorf("account/counterparty = %q / %q, want %q / %q", got.Account, got.Counterparty, tt.account, tt.counterparty)
			}
		})
	}
}
