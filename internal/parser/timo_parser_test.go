package parser

import (
	"testing"

	"github.com/hiiamtrong/go-imap-bot/internal/models"
)

func TestTimoParser_English(t *testing.T) {
	tests := []struct {
		name, body, desc string
		amount, balance  int64
		typ              models.TransactionType
	}{
		{
			name:   "credit",
			body:   "<p><p>Your Spend Account has been credited 1,153,625 VND on 06/11/2026 18:16.\r\nYour current balance is: 1,153,658 VND.</p>\r\n<p>Transaction Description: MBVCB.1.2.Bill TRXJOLDVTYAONG.CT tu 1 NGUYEN PHUONG THIEN toi 0338613062 VU XUAN TRONG tai BVBANK TIMO.</p></p>",
			desc:   "MBVCB.1.2.Bill TRXJOLDVTYAONG.CT tu 1 NGUYEN PHUONG THIEN toi 0338613062 VU XUAN TRONG tai BVBANK TIMO",
			amount: 1153625, balance: 1153658, typ: models.TransactionTypeAdd,
		},
		{
			name:   "debit with capitalised balance",
			body:   "<p><p>Your Spend Account has been debited 295,548 VND on 29/10/2026 16:02.\r\nYour Current Balance is: 0 VND.</p>\r\n<p>Transaction Description: Sent by Vu Xuan Trong from my Timo.</p></p>",
			desc:   "Sent by Vu Xuan Trong from my Timo",
			amount: 295548, balance: 0, typ: models.TransactionTypeSubtract,
		},
		{
			name:   "vietnamese notice still works",
			body:   "Tài khoản của bạn đã tăng 50,000 VND vào 01/10/2026.\r\nSố dư hiện tại: 80,000 VND.\r\nMô tả: Bill TRXABC.",
			desc:   "Bill TRXABC",
			amount: 50000, balance: 80000, typ: models.TransactionTypeAdd,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := (&TimoParser{}).Parse(tt.body)
			if err != nil {
				t.Fatal(err)
			}
			if got.Amount != tt.amount || got.CurrentBalance != tt.balance || got.Type != tt.typ || got.Description != tt.desc {
				t.Errorf("got %+v", got)
			}
		})
	}
}
