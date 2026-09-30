package groupspend

import (
	"testing"
	"time"

	"github.com/hiiamtrong/go-imap-bot/internal/models"
)

func at(day, hour int) time.Time {
	return time.Date(2026, 9, day, hour, 30, 0, 0, ICT)
}

func TestSuspect(t *testing.T) {
	// 2026-09-28 is a Monday, 2026-09-27 a Sunday.
	tests := []struct {
		name string
		tx   models.Transaction
		want bool
	}{
		{"weekday lunch bill", models.Transaction{Type: "subtract", Amount: 160000, Currency: "VND", Description: "Cơm trưa", Timestamp: at(28, 12)}, true},
		{"empty currency counts as VND", models.Transaction{Type: "subtract", Amount: 160000, Description: "Phở vịt", Timestamp: at(28, 11)}, true},
		{"same instant in UTC still lunch in Vietnam", models.Transaction{Type: "subtract", Amount: 160000, Description: "Bún", Timestamp: at(28, 12).UTC()}, true},
		{"weekend", models.Transaction{Type: "subtract", Amount: 160000, Description: "Cơm", Timestamp: at(27, 12)}, false},
		{"evening", models.Transaction{Type: "subtract", Amount: 160000, Description: "Cơm", Timestamp: at(28, 19)}, false},
		{"too small", models.Transaction{Type: "subtract", Amount: 40000, Description: "Cà phê", Timestamp: at(28, 12)}, false},
		{"too large", models.Transaction{Type: "subtract", Amount: 5000000, Description: "Cơm", Timestamp: at(28, 12)}, false},
		{"money in", models.Transaction{Type: "add", Amount: 160000, Description: "Cơm", Timestamp: at(28, 12)}, false},
		{"foreign currency", models.Transaction{Type: "subtract", Amount: 160000, Currency: "USD", Description: "Lunch", Timestamp: at(28, 12)}, false},
		{"personal merchant", models.Transaction{Type: "subtract", Amount: 160000, Description: "Grab* A-9QUQGI3GWMG7AV", Timestamp: at(28, 12)}, false},
		{"virtual bill", models.Transaction{Type: "subtract", Amount: 160000, From: "Virtual Bill", Description: "Cơm", Timestamp: at(28, 12)}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Suspect(&tt.tx); got != tt.want {
				t.Errorf("Suspect = %v, want %v", got, tt.want)
			}
		})
	}
}
