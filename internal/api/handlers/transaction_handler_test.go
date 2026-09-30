package handlers

import (
	"testing"
	"time"

	"github.com/hiiamtrong/go-imap-bot/internal/models"
)

func TestToTransactionDTOSuspectedGroupSpend(t *testing.T) {
	lunch := time.Date(2026, 9, 28, 12, 30, 0, 0, time.FixedZone("ICT", 7*3600))
	tests := []struct {
		name string
		tx   models.Transaction
		want bool
	}{
		{"open weekday lunch expense", models.Transaction{Type: "subtract", Amount: 160000, Description: "Cơm", Timestamp: lunch}, true},
		{"same expense once completed", models.Transaction{Type: "subtract", Amount: 160000, Description: "Cơm", Completed: true, Timestamp: lunch}, false},
		{"small expense", models.Transaction{Type: "subtract", Amount: 20000, Description: "Cà phê", Timestamp: lunch}, false},
		{"money in", models.Transaction{Type: "add", Amount: 160000, Description: "Cơm", Timestamp: lunch}, false},
	}
	h := &TransactionHandler{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := h.toTransactionDTO(&tt.tx).SuspectedGroupSpend; got != tt.want {
				t.Errorf("SuspectedGroupSpend = %v, want %v", got, tt.want)
			}
		})
	}
}
