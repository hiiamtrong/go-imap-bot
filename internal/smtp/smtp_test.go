package smtp

import (
	"testing"
	"time"

	"github.com/hiiamtrong/go-imap-bot/internal/models"
)

// Split ids follow the order the splits were made, not the order of the bills (seen on prod:
// the 04/11 bill was split after the 05/11 and 06/11 ones). Order and shown time are the mail's
// arrival time, in Vietnam time, whatever zone the mail carried.
func TestBuildSplitDataOrdersByMailTime(t *testing.T) {
	zone := time.FixedZone("", -8*3600)
	mailAt := func(d, h int) time.Time { return time.Date(2026, 11, d, h, 0, 0, 0, zone) }
	processed := time.Date(2026, 11, 7, 9, 0, 0, 0, time.UTC) // bot ran later; must not matter
	splits := []*models.TransactionSplit{
		{ID: 1, Reason: "06/11", Amount: 40000, BillCreatedAt: mailAt(6, 10), TransactionCreatedAt: processed},
		{ID: 2, Reason: "05/11", Amount: 54166, BillCreatedAt: mailAt(5, 10), TransactionCreatedAt: processed.Add(time.Hour)},
		{ID: 3, Reason: "04/11 no mail time", Amount: 269000, TransactionCreatedAt: time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)},
	}

	rows, total := buildSplitData(splits)

	var got []string
	for _, r := range rows {
		got = append(got, r.Reason)
	}
	if want := []string{"04/11 no mail time", "05/11", "06/11"}; got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Errorf("rows = %v, want %v", got, want)
	}
	if rows[1].CreatedAt != "06/11/2026 01:00:00" { // 05/11 10:00 at -08:00 is 06/11 01:00 in Vietnam
		t.Errorf("date = %q", rows[1].CreatedAt)
	}
	if total != 40000+54166+269000 {
		t.Errorf("total = %d", total)
	}
	if splits[0].ID != 1 {
		t.Error("caller's slice was reordered")
	}
}
