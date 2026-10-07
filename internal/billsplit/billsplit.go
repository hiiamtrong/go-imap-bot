package billsplit

import (
	"fmt"
	"math/big"
	"slices"
	"strings"
	"unicode"

	"github.com/hiiamtrong/go-imap-bot/internal/llm"
	"github.com/hiiamtrong/go-imap-bot/internal/models"
	"golang.org/x/text/unicode/norm"
)

// Alias is the bill's own name for the person when the model matched it to this
// user and the name would not find them by itself: offered for saving once the
// split is confirmed.
type Share struct {
	UserID int64
	Name   string
	Amount int64
	Alias  string
	// Covers lists the people whose share was folded into this one.
	Covers []string
}

func (s Share) Reason(description string) string {
	if len(s.Covers) == 0 {
		return description
	}
	return description + " (gồm phần của " + strings.Join(s.Covers, ", ") + ")"
}

type Plan struct {
	Total  int64
	Shares []Share
	// Prorated means the amounts were scaled to Total (see Prorate).
	Prorated bool
}

var (
	honorifics = strings.Fields("a c anh chi em ban bac co chu")
	selfWords  = strings.Fields("toi minh tao moi")
)

// KnownUsers is what the model needs to match a bill's names to users. Only the
// part of the email before "@" is sent.
func KnownUsers(users []*models.User) []llm.KnownUser {
	known := make([]llm.KnownUser, len(users))
	for i, u := range users {
		account, _, _ := strings.Cut(u.Email, "@")
		known[i] = llm.KnownUser{ID: u.ID, Name: u.Name, Account: account}
	}
	return known
}

// AliasKey makes "Hồngg Ngọc" and "hongg ngoc" the same alias.
func AliasKey(name string) string { return strings.Join(tokens(name), " ") }

// aliases maps AliasKey to a user ID: names the user already told us once.
func Resolve(people []llm.Person, total int64, users []*models.User, selfID int64, aliases map[string]int64) (Plan, []string) {
	var problems []string
	if len(people) == 0 {
		return Plan{}, []string{"không thấy người nào tham gia"}
	}

	shares := make([]Share, len(people))
	seen := map[int64]bool{}
	for i, p := range people {
		u, problem := findUser(p, users, selfID, aliases)
		if problem != "" {
			problems = append(problems, problem)
			continue
		}
		if seen[u.ID] {
			problems = append(problems, fmt.Sprintf("%q bị trùng với người đã liệt kê (%s)", p.Name, u.Name))
			continue
		}
		seen[u.ID] = true
		shares[i] = Share{UserID: u.ID, Name: u.Name, Amount: p.Amount}
		if u.ID == p.UserID && !isSelfWord(p.Name) {
			shares[i].Alias = aliasFor(p.Name, u, aliases)
		}
	}
	if len(problems) > 0 {
		return Plan{}, problems
	}

	var fixed int64
	var equal []int
	for i, p := range people {
		if p.Amount > 0 {
			fixed += p.Amount
		} else {
			equal = append(equal, i)
		}
	}

	switch {
	case len(equal) == 0 && total == 0:
		total = fixed
	case len(equal) == 0 && fixed != total:
		return Plan{}, []string{fmt.Sprintf("tổng các phần (%d) khác tổng bill (%d); nếu bill có giảm giá hoặc phí, hãy ghi thêm \"chia theo số tiền sau giảm giá\"", fixed, total)}
	case len(equal) > 0:
		if total == 0 {
			return Plan{}, []string{"thiếu tổng tiền để chia đều"}
		}
		rest := total - fixed
		if rest <= 0 {
			return Plan{}, []string{fmt.Sprintf("các phần đã nêu (%d) đã vượt tổng bill (%d)", fixed, total)}
		}
		n := int64(len(equal))
		if rest < n {
			return Plan{}, []string{fmt.Sprintf("còn lại %d không đủ chia cho %d người", rest, n)}
		}
		for j, i := range equal {
			shares[i].Amount = rest / n
			if j == 0 {
				shares[i].Amount += rest % n
			}
		}
	}

	return Plan{Total: total, Shares: shares}, nil
}

// explainsGap reports whether the bill's own fee and discount lines account for
// the exact difference between the item prices and the total, so the list
// prices can be scaled without the user having to ask.
func explainsGap(bill *llm.Bill, total int64) bool {
	adjustment := bill.AdjustmentSum()
	if adjustment == 0 {
		return false
	}
	var items int64
	for _, p := range bill.People {
		if p.Amount <= 0 {
			return false
		}
		items += p.Amount
	}
	return items+adjustment == total
}

// applyCovers folds the share of every covered person into their payer's, so
// the payer ends up with one split for all of it and the total is unchanged.
func applyCovers(plan Plan, covers []llm.Cover, users []*models.User, selfID int64, aliases map[string]int64) (Plan, []string) {
	if len(covers) == 0 {
		return plan, nil
	}

	type resolved struct {
		payer   *models.User
		targets []*models.User
	}
	var all []resolved
	var problems []string
	payers := map[int64]bool{}
	for _, c := range covers {
		payer, problem := findUser(c.Payer, users, selfID, aliases)
		if problem != "" {
			problems = append(problems, problem)
			continue
		}
		r := resolved{payer: payer}
		for _, t := range c.For {
			target, problem := findUser(t, users, selfID, aliases)
			if problem != "" {
				problems = append(problems, problem)
			} else if target.ID != payer.ID {
				r.targets = append(r.targets, target)
			}
		}
		payers[payer.ID] = true
		all = append(all, r)
	}
	if len(problems) > 0 {
		return Plan{}, problems
	}

	inPlan := map[int64]*Share{}
	for i := range plan.Shares {
		inPlan[plan.Shares[i].UserID] = &plan.Shares[i]
	}
	covered := map[int64]bool{}
	moved := map[int64]int64{}
	names := map[int64][]string{}
	var order []*models.User
	for _, r := range all {
		if _, seen := moved[r.payer.ID]; !seen {
			order = append(order, r.payer)
			moved[r.payer.ID] = 0
		}
		for _, target := range r.targets {
			switch share := inPlan[target.ID]; {
			case payers[target.ID]:
				problems = append(problems, fmt.Sprintf("%s vừa chịu tiền giúp người khác vừa được người khác chịu giúp", target.Name))
			case covered[target.ID]:
				problems = append(problems, fmt.Sprintf("%s được chịu tiền giúp hai lần", target.Name))
			case share == nil:
				problems = append(problems, fmt.Sprintf("%s không có trong bill nên không thể được chịu tiền giúp", target.Name))
			default:
				covered[target.ID] = true
				moved[r.payer.ID] += share.Amount
				names[r.payer.ID] = append(names[r.payer.ID], target.Name)
			}
		}
	}
	if len(problems) > 0 {
		return Plan{}, problems
	}

	shares := make([]Share, 0, len(plan.Shares))
	for _, s := range plan.Shares {
		if covered[s.UserID] {
			continue
		}
		s.Amount += moved[s.UserID]
		s.Covers = names[s.UserID]
		shares = append(shares, s)
	}
	for _, payer := range order {
		if inPlan[payer.ID] == nil && moved[payer.ID] > 0 {
			shares = append(shares, Share{UserID: payer.ID, Name: payer.Name, Amount: moved[payer.ID], Covers: names[payer.ID]})
		}
	}
	plan.Shares = shares
	return plan, nil
}

func ForBill(bill *llm.Bill, users []*models.User, selfID int64, target *models.Transaction, aliases map[string]int64) (Plan, string, []string) {
	total, description := bill.Total, bill.Description
	if target != nil {
		if total == 0 {
			total = target.Amount
		}
		if description == "" {
			description = target.Description
		}
	}
	if description == "" {
		description = "Chia bill"
	}

	people, prorated := bill.People, false
	if bill.Prorate || explainsGap(bill, total) {
		people, prorated = Prorate(people, total)
	}

	plan, problems := Resolve(people, total, users, selfID, aliases)
	if len(problems) == 0 {
		plan, problems = applyCovers(plan, bill.Covers, users, selfID, aliases)
	}
	plan.Prorated = prorated && len(problems) == 0
	if target != nil && len(problems) == 0 && plan.Total != target.Amount {
		problems = append(problems, fmt.Sprintf("tổng chia (%d) khác số tiền giao dịch #%d (%d)", plan.Total, target.ID, target.Amount))
	}
	return plan, description, problems
}

// Prorate scales every amount so they add up to total, handing the leftover
// units to the largest remainders so nothing is lost to rounding. It applies
// only when everyone has an amount and the sum differs from total; otherwise
// the people come back untouched and false is returned.
func Prorate(people []llm.Person, total int64) ([]llm.Person, bool) {
	var sum int64
	for _, p := range people {
		if p.Amount <= 0 {
			return people, false
		}
		sum += p.Amount
	}
	if len(people) == 0 || total <= 0 || sum == total {
		return people, false
	}

	scaled := make([]llm.Person, len(people))
	copy(scaled, people)
	rest := make([]*big.Int, len(people))
	var given int64
	for i, p := range people {
		num := new(big.Int).Mul(big.NewInt(p.Amount), big.NewInt(total))
		q, r := new(big.Int).QuoRem(num, big.NewInt(sum), new(big.Int))
		scaled[i].Amount, rest[i] = q.Int64(), r
		given += scaled[i].Amount
	}

	order := make([]int, len(people))
	for i := range order {
		order[i] = i
	}
	slices.SortStableFunc(order, func(a, b int) int { return rest[b].Cmp(rest[a]) })
	for _, i := range order[:total-given] {
		scaled[i].Amount++
	}
	return scaled, true
}

func findUser(p llm.Person, users []*models.User, selfID int64, aliases map[string]int64) (*models.User, string) {
	name := p.Name
	q := tokens(name)
	if len(q) == 0 {
		return nil, fmt.Sprintf("%q không phải tên hợp lệ", name)
	}

	if isSelfWord(name) {
		if u := userByID(users, selfID); u != nil {
			return u, ""
		}
		return nil, fmt.Sprintf("%q: chưa biết bạn là ai, hãy ghi tên thay vì %q", name, name)
	}

	// The model matched the name against the known users itself, so its answer
	// comes first. An id it invented, or one whose user was deleted, and a
	// remembered name pointing at a deleted user all fall through.
	if u := userByID(users, p.UserID); u != nil {
		return u, ""
	}
	if u := userByID(users, aliases[AliasKey(name)]); u != nil {
		return u, ""
	}

	best := bestMatches(q, users, func(u *models.User) string { return u.Name })
	if len(best) == 0 {
		best = bestMatches(q, users, func(u *models.User) string { return u.Email })
	}

	switch len(best) {
	case 0:
		return nil, fmt.Sprintf("%q: không tìm thấy người dùng", name)
	case 1:
		return best[0], ""
	}

	names := make([]string, 0, 4)
	for _, u := range best[:min(len(best), 4)] {
		names = append(names, u.Name)
	}
	return nil, fmt.Sprintf("%q: nhiều người khớp (%s), hãy ghi rõ hơn", name, strings.Join(names, ", "))
}

func isSelfWord(name string) bool {
	q := tokens(name)
	return len(q) == 1 && slices.Contains(selfWords, q[0])
}

func userByID(users []*models.User, id int64) *models.User {
	if i := slices.IndexFunc(users, func(u *models.User) bool { return u.ID == id }); i >= 0 {
		return users[i]
	}
	return nil
}

// aliasFor is the bill name worth remembering for u: one that is not already
// remembered and would not find u by name or email on its own.
func aliasFor(name string, u *models.User, aliases map[string]int64) string {
	key := AliasKey(name)
	if key == "" || aliases[key] == u.ID || key == AliasKey(u.Name) || key == AliasKey(u.Email) {
		return ""
	}
	return strings.TrimSpace(name)
}

func bestMatches(q []string, users []*models.User, field func(*models.User) string) []*models.User {
	var best []*models.User
	bestScore := 0
	for _, u := range users {
		s := score(q, tokens(field(u)))
		switch {
		case s > bestScore:
			best, bestScore = []*models.User{u}, s
		case s == bestScore && s > 0:
			best = append(best, u)
		}
	}
	return best
}

func score(q, u []string) int {
	set := make(map[string]bool, len(u))
	for _, t := range u {
		set[t] = true
	}
	for _, t := range q {
		if !set[t] {
			return 0
		}
	}
	if len(q) == len(u) {
		return 1000
	}
	return 500 - len(u)
}

func tokens(s string) []string {
	if i := strings.IndexByte(s, '@'); i >= 0 {
		s = s[:i]
	}
	s = strings.NewReplacer("đ", "d", "Đ", "d").Replace(s)
	var b strings.Builder
	for _, r := range norm.NFD.String(strings.ToLower(s)) {
		switch {
		case unicode.Is(unicode.Mn, r):
		case unicode.IsLetter(r):
			b.WriteRune(r)
		default:
			b.WriteByte(' ')
		}
	}

	all := strings.Fields(b.String())
	kept := all[:0:0]
	for _, t := range all {
		if !slices.Contains(honorifics, t) {
			kept = append(kept, t)
		}
	}
	if len(kept) == 0 {
		return all
	}
	return kept
}
