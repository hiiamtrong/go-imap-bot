package billsplit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hiiamtrong/go-imap-bot/internal/llm"
	"github.com/hiiamtrong/go-imap-bot/internal/models"
)

var users = []*models.User{
	{ID: 1, Name: "Tùng", Email: "tung.nguyen5@sotatek.com"},
	{ID: 4, Name: "Trọng", Email: "trong.vu@sotatek.com"},
	{ID: 9, Name: "Đức", Email: "duc.phung@sotatek.com"},
	{ID: 10, Name: "Hạnh Lê", Email: "lethithuhanh.txpt@gmail.com"},
	{ID: 11, Name: "hieu.tran7", Email: "hieu.nguyen@sotatek.com"},
	{ID: 5, Name: "A Phong"},
	{ID: 6, Name: "D Linh"},
	{ID: 15, Name: "linh.pham3"},
	{ID: 16, Name: "tam.hoang"},
	{ID: 31, Name: "tung.le2"},
	{ID: 32, Name: "toan.tran2"},
	{ID: 65, Name: "Thương Hà"},
	{ID: 68, Name: "son.ho"},
	{ID: 70, Name: "an.vu2@sotatek.com"},
}

func people(pairs ...any) []llm.Person {
	var out []llm.Person
	for i := 0; i < len(pairs); i += 2 {
		out = append(out, llm.Person{Name: pairs[i].(string), Amount: int64(pairs[i+1].(int))})
	}
	return out
}

func amounts(p Plan) map[int64]int64 {
	m := map[int64]int64{}
	for _, s := range p.Shares {
		m[s.UserID] = s.Amount
	}
	return m
}

func TestResolveNames(t *testing.T) {
	tests := []struct {
		name     string
		person   string
		wantID   int64
		wantProb string
	}{
		{"accent-free matches email-style name", "Toàn", 32, ""},
		{"first name inside full name", "Hà", 65, ""},
		{"diacritics ignored", "Son", 68, ""},
		{"exact beats partial", "Tùng", 1, ""},
		{"email local part", "an.vu", 70, ""},
		{"honorific dropped", "anh Phong", 5, ""},
		{"self word", "tôi", 4, ""},
		{"email local part matches when no name does", "trong.vu", 4, ""},
		{"full email", "trong.vu@sotatek.com", 4, ""},
		{"email with digits", "tung.nguyen5", 1, ""},
		{"email of an accented name", "duc.phung", 9, ""},
		{"a name match is never overridden by emails", "Hà", 65, ""},
		{"the email domain is never matched", "sotatek", 0, "không tìm thấy"},
		{"shared email token is ambiguous", "nguyen", 0, "nhiều người khớp"},
		{"ambiguous", "Linh", 0, "nhiều người khớp"},
		{"unknown", "Zed", 0, "không tìm thấy"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan, problems := Resolve(people(tt.person, 10000), 10000, users, 4, nil)
			if tt.wantProb != "" {
				if len(problems) != 1 || !strings.Contains(problems[0], tt.wantProb) {
					t.Fatalf("problems = %v, want one containing %q", problems, tt.wantProb)
				}
				return
			}
			if len(problems) != 0 {
				t.Fatalf("unexpected problems: %v", problems)
			}
			if plan.Shares[0].UserID != tt.wantID {
				t.Errorf("resolved to %d, want %d", plan.Shares[0].UserID, tt.wantID)
			}
		})
	}
}

func TestResolveSelfWithoutSelfID(t *testing.T) {
	_, problems := Resolve(people("mình", 0), 1000, users, 0, nil)
	if len(problems) != 1 || !strings.Contains(problems[0], "chưa biết bạn là ai") {
		t.Fatalf("problems = %v", problems)
	}
}

func TestResolveAmounts(t *testing.T) {
	t.Run("equal split gives remainder to first", func(t *testing.T) {
		plan, problems := Resolve(people("Trọng", 0, "Hà", 0, "Sơn", 0), 100000, users, 4, nil)
		if len(problems) != 0 {
			t.Fatal(problems)
		}
		got := amounts(plan)
		if got[4] != 33334 || got[65] != 33333 || got[68] != 33333 {
			t.Errorf("amounts = %v", got)
		}
	})

	t.Run("fixed amounts and equal remainder", func(t *testing.T) {
		plan, problems := Resolve(people("Trọng", 60000, "Hà", 0, "Sơn", 0), 160000, users, 4, nil)
		if len(problems) != 0 {
			t.Fatal(problems)
		}
		got := amounts(plan)
		if got[4] != 60000 || got[65] != 50000 || got[68] != 50000 {
			t.Errorf("amounts = %v", got)
		}
	})

	t.Run("all fixed without total takes their sum", func(t *testing.T) {
		plan, problems := Resolve(people("Hà", 40000, "Sơn", 50000), 0, users, 4, nil)
		if len(problems) != 0 || plan.Total != 90000 {
			t.Fatalf("total = %d, problems = %v", plan.Total, problems)
		}
	})

	problemCases := []struct {
		name   string
		people []llm.Person
		total  int64
		want   string
	}{
		{"fixed sum differs from total", people("Hà", 40000, "Sơn", 50000), 100000, "khác tổng bill"},
		{"fixed exceeds total", people("Hà", 90000, "Sơn", 0), 80000, "vượt tổng bill"},
		{"equal split without total", people("Hà", 0, "Sơn", 0), 0, "thiếu tổng tiền"},
		{"duplicate person", people("Hà", 0, "Thương Hà", 0), 100000, "trùng"},
		{"nobody", nil, 100000, "không thấy người nào"},
		{"remainder smaller than head count", people("Hà", 0, "Sơn", 0, "Trọng", 0), 2, "không đủ chia"},
		{"fixed shares leave nothing for equal ones", people("Hà", 100000, "Sơn", 0), 100000, "vượt tổng bill"},
	}
	for _, tt := range problemCases {
		t.Run(tt.name, func(t *testing.T) {
			_, problems := Resolve(tt.people, tt.total, users, 4, nil)
			if len(problems) != 1 || !strings.Contains(problems[0], tt.want) {
				t.Fatalf("problems = %v, want one containing %q", problems, tt.want)
			}
		})
	}
}

func TestSheetExportURL(t *testing.T) {
	tests := []struct {
		text string
		want string
		ok   bool
	}{
		{"chia theo https://docs.google.com/spreadsheets/d/AbC-1_x/edit?usp=sharing nhé", "https://docs.google.com/spreadsheets/d/AbC-1_x/export?format=csv", true},
		{"https://docs.google.com/spreadsheets/d/AbC/edit#gid=123456", "https://docs.google.com/spreadsheets/d/AbC/export?format=csv&gid=123456", true},
		{"https://evil.example/spreadsheets/d/AbC/edit", "", false},
		{"https://docs.google.com.evil.example/spreadsheets/d/AbC", "", false},
		{"không có link", "", false},
	}
	for _, tt := range tests {
		got, ok := SheetExportURL(tt.text)
		if got != tt.want || ok != tt.ok {
			t.Errorf("SheetExportURL(%q) = %q, %v; want %q, %v", tt.text, got, ok, tt.want, tt.ok)
		}
	}
}

func TestFetchCSV(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/csv":
			w.Header().Set("Content-Type", "text/csv; charset=utf-8")
			w.Write([]byte("ten,tien\nHà,40000\n"))
		case "/big":
			w.Header().Set("Content-Type", "text/csv")
			w.Write([]byte(strings.Repeat("a", maxSheetBytes+10)))
		default:
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte("<html>login</html>"))
		}
	}))
	defer srv.Close()

	got, err := fetchCSV(context.Background(), srv.Client(), srv.URL+"/csv")
	if err != nil || got != "ten,tien\nHà,40000\n" {
		t.Fatalf("csv = %q, err = %v", got, err)
	}
	if _, err := fetchCSV(context.Background(), srv.Client(), srv.URL+"/login"); err == nil || !strings.Contains(err.Error(), "chia sẻ") {
		t.Errorf("html response err = %v, want sharing hint", err)
	}
	if _, err := fetchCSV(context.Background(), srv.Client(), srv.URL+"/big"); err == nil || !strings.Contains(err.Error(), "quá lớn") {
		t.Errorf("oversized err = %v", err)
	}
}

func TestSheetRedirectPolicy(t *testing.T) {
	check := sheetClient.CheckRedirect
	req := func(u string) *http.Request { r, _ := http.NewRequest("GET", u, nil); return r }

	if err := check(req("https://doc-0g-sheets.googleusercontent.com/export/x"), nil); err != nil {
		t.Errorf("googleusercontent redirect rejected: %v", err)
	}
	for _, bad := range []string{"http://docs.google.com/x", "https://169.254.169.254/x", "https://google.com.evil.example/x"} {
		if err := check(req(bad), nil); err == nil {
			t.Errorf("redirect to %s allowed", bad)
		}
	}
}

func TestForBill(t *testing.T) {
	tx := &models.Transaction{ID: 7, Amount: 100000, Description: "VU XUAN TRONG chuyen tien"}
	equal := func(names ...string) []llm.Person {
		var out []llm.Person
		for _, n := range names {
			out = append(out, llm.Person{Name: n})
		}
		return out
	}

	t.Run("total and description default from the target transaction", func(t *testing.T) {
		plan, desc, problems := ForBill(&llm.Bill{People: equal("Hà", "Sơn")}, users, 4, tx, nil)
		if len(problems) != 0 || plan.Total != 100000 || desc != "VU XUAN TRONG chuyen tien" {
			t.Fatalf("plan = %+v, desc = %q, problems = %v", plan, desc, problems)
		}
	})

	t.Run("stated values win over the target", func(t *testing.T) {
		_, desc, problems := ForBill(&llm.Bill{Description: "Cơm", Total: 100000, People: equal("Hà")}, users, 4, tx, nil)
		if len(problems) != 0 || desc != "Cơm" {
			t.Fatalf("desc = %q, problems = %v", desc, problems)
		}
	})

	t.Run("stated total that differs from the target is rejected", func(t *testing.T) {
		_, _, problems := ForBill(&llm.Bill{Total: 90000, People: equal("Hà")}, users, 4, tx, nil)
		if len(problems) != 1 || !strings.Contains(problems[0], "khác số tiền giao dịch #7") {
			t.Fatalf("problems = %v", problems)
		}
	})

	t.Run("without a target a generic description is used", func(t *testing.T) {
		_, desc, problems := ForBill(&llm.Bill{Total: 50000, People: equal("Hà")}, users, 4, nil, nil)
		if len(problems) != 0 || desc != "Chia bill" {
			t.Fatalf("desc = %q, problems = %v", desc, problems)
		}
	})
}

func TestResolveSheetWithEmailStyleNames(t *testing.T) {
	plan, problems := Resolve(people("trong.vu", 4500, "hanh.le", 4500), 9000, users, 4, nil)
	if len(problems) != 0 {
		t.Fatalf("problems = %v", problems)
	}
	got := amounts(plan)
	if len(got) != 2 || got[4] != 4500 || got[10] != 4500 || plan.Total != 9000 {
		t.Errorf("plan = %+v", plan)
	}
}

func TestAliasKey(t *testing.T) {
	for in, want := range map[string]string{
		"Hồngg Ngọc": "hongg ngoc",
		"  KI ":      "ki",
		"Anh Tùng":   "tung",
		"":           "",
	} {
		if got := AliasKey(in); got != want {
			t.Errorf("AliasKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestResolveAliases(t *testing.T) {
	// The fixture users: 4 Trọng, 68 son.ho, 16 tam.hoang, 65 Thương Hà.
	one := func(p llm.Person, aliases map[string]int64) (Share, []string) {
		plan, problems := Resolve([]llm.Person{p}, 10000, users, 4, aliases)
		if len(problems) > 0 {
			return Share{}, problems
		}
		return plan.Shares[0], nil
	}

	t.Run("the model's match is used and its bill name is offered for saving", func(t *testing.T) {
		s, problems := one(llm.Person{Name: "Hồngg Ngọc", UserID: 4}, nil)
		if len(problems) > 0 || s.UserID != 4 || s.Alias != "Hồngg Ngọc" {
			t.Fatalf("share = %+v, problems = %v", s, problems)
		}
	})

	t.Run("a bill name that already identifies the user needs no alias", func(t *testing.T) {
		for _, name := range []string{"trong.vu", "Trọng", "trong.vu@sotatek.com"} {
			if s, problems := one(llm.Person{Name: name, UserID: 4}, nil); len(problems) > 0 || s.Alias != "" {
				t.Errorf("%q: share = %+v, problems = %v", name, s, problems)
			}
		}
	})

	t.Run("the model's match beats a remembered name, so a correction sticks", func(t *testing.T) {
		s, problems := one(llm.Person{Name: "Ki", UserID: 4}, map[string]int64{"ki": 68})
		if len(problems) > 0 || s.UserID != 4 || s.Alias != "Ki" {
			t.Fatalf("share = %+v, problems = %v", s, problems)
		}
	})

	t.Run("a remembered name resolves when the model is unsure, and is not offered again", func(t *testing.T) {
		s, problems := one(llm.Person{Name: "Ki"}, map[string]int64{"ki": 68})
		if len(problems) > 0 || s.UserID != 68 || s.Alias != "" {
			t.Fatalf("share = %+v, problems = %v", s, problems)
		}
	})

	t.Run("a remembered name beats a fuzzy match", func(t *testing.T) {
		s, problems := one(llm.Person{Name: "Hà"}, map[string]int64{"ha": 16})
		if len(problems) > 0 || s.UserID != 16 {
			t.Fatalf("share = %+v, problems = %v", s, problems)
		}
	})

	t.Run("an id the model invented falls back to matching the name", func(t *testing.T) {
		s, problems := one(llm.Person{Name: "Trọng", UserID: 999}, nil)
		if len(problems) > 0 || s.UserID != 4 || s.Alias != "" {
			t.Fatalf("share = %+v, problems = %v", s, problems)
		}
		if _, problems := one(llm.Person{Name: "nobody", UserID: 999}, nil); len(problems) == 0 {
			t.Fatal("want a not-found problem")
		}
	})

	t.Run("a remembered name pointing at a deleted user falls back too", func(t *testing.T) {
		s, problems := one(llm.Person{Name: "Trọng"}, map[string]int64{"trong": 999})
		if len(problems) > 0 || s.UserID != 4 {
			t.Fatalf("share = %+v, problems = %v", s, problems)
		}
	})

	t.Run("a first person word is never offered as an alias", func(t *testing.T) {
		s, problems := one(llm.Person{Name: "tôi", UserID: 4}, nil)
		if len(problems) > 0 || s.UserID != 4 || s.Alias != "" {
			t.Fatalf("share = %+v, problems = %v", s, problems)
		}
	})

	t.Run("a name guessed by token matching is never offered as an alias", func(t *testing.T) {
		s, problems := one(llm.Person{Name: "Hà"}, nil)
		if len(problems) > 0 || s.UserID != 65 || s.Alias != "" {
			t.Fatalf("share = %+v, problems = %v", s, problems)
		}
	})
}
