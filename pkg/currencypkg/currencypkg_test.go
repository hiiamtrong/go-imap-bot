package currencypkg

import (
	"testing"
)

func TestFormatCurrency(t *testing.T) {
	tests := []struct {
		name     string
		amount   float64
		expected string
	}{
		{
			name:     "Format zero",
			amount:   0,
			expected: "0₫",
		},
		{
			name:     "Format small amount",
			amount:   1000,
			expected: "1,000₫",
		},
		{
			name:     "Format large amount",
			amount:   1000000,
			expected: "1,000,000₫",
		},
		{
			name:     "Format negative amount",
			amount:   -50000,
			expected: "-50,000₫",
		},
		{
			name:     "Format very large amount",
			amount:   1234567890,
			expected: "1,234,567,890₫",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatCurrency(tt.amount)
			if result != tt.expected {
				t.Errorf("FormatCurrency(%f) = %s, want %s", tt.amount, result, tt.expected)
			}
		})
	}
}

func TestParseCurrency(t *testing.T) {
	tests := []struct {
		name           string
		currencyString string
		expected       float64
		expectError    bool
	}{
		{
			name:           "Parse simple number",
			currencyString: "1000",
			expected:       1000,
			expectError:    false,
		},
		{
			name:           "Parse formatted currency",
			currencyString: "1,000₫",
			expected:       1000,
			expectError:    false,
		},
		{
			name:           "Parse large formatted currency",
			currencyString: "1,234,567₫",
			expected:       1234567,
			expectError:    false,
		},
		{
			name:           "Parse currency with spaces",
			currencyString: " 50,000 ₫ ",
			expected:       50000,
			expectError:    false,
		},
		{
			name:           "Parse invalid currency",
			currencyString: "abc",
			expected:       0,
			expectError:    true,
		},
		{
			name:           "Parse empty string",
			currencyString: "",
			expected:       0,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseCurrency(tt.currencyString)

			if tt.expectError {
				if err == nil {
					t.Errorf("ParseCurrency(%s) expected error, got nil", tt.currencyString)
				}
			} else {
				if err != nil {
					t.Errorf("ParseCurrency(%s) unexpected error: %v", tt.currencyString, err)
				}
				if result != tt.expected {
					t.Errorf("ParseCurrency(%s) = %f, want %f", tt.currencyString, result, tt.expected)
				}
			}
		})
	}
}

func TestFormatAndParseCurrency(t *testing.T) {
	// Test that formatting and then parsing returns the original value
	amounts := []float64{0, 1000, 5000, 100000, 1234567, 9876543210}

	for _, amount := range amounts {
		formatted := FormatCurrency(amount)
		parsed, err := ParseCurrency(formatted)
		if err != nil {
			t.Errorf("ParseCurrency(FormatCurrency(%f)) unexpected error: %v", amount, err)
		}

		if parsed != amount {
			t.Errorf("ParseCurrency(FormatCurrency(%f)) = %f, want %f", amount, parsed, amount)
		}
	}
}
