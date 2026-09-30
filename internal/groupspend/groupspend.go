package groupspend

import (
	"regexp"
	"time"

	"github.com/hiiamtrong/go-imap-bot/internal/models"
)

const (
	minAmount = 80_000
	maxAmount = 700_000
)

var (
	ICT      = time.FixedZone("ICT", 7*3600)
	personal = regexp.MustCompile(`(?i)grab|shopee|speepay|tiktok|9pay|;rut;|purchase at|retail|transfer from|money pot|hu chi tieu|recurring|contribution|winmart|aeon`)
)

func Suspect(t *models.Transaction) bool {
	if t.Type != string(models.TransactionTypeSubtract) || t.From == "Virtual Bill" {
		return false
	}
	if t.Currency != "" && t.Currency != "VND" {
		return false
	}
	if t.Amount < minAmount || t.Amount > maxAmount || personal.MatchString(t.Description) {
		return false
	}

	local := t.Timestamp.In(ICT)
	if wd := local.Weekday(); wd == time.Saturday || wd == time.Sunday {
		return false
	}
	return local.Hour() >= 10 && local.Hour() <= 14
}
