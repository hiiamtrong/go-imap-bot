package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func fakeServer(t *testing.T, status int, content string, gotBody *string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer key" {
			t.Errorf("authorization = %q", r.Header.Get("Authorization"))
		}
		b, _ := io.ReadAll(r.Body)
		*gotBody = string(b)

		w.WriteHeader(status)
		resp, _ := json.Marshal(map[string]any{
			"choices": []any{map[string]any{"message": map[string]string{"content": content}}},
		})
		w.Write(resp)
	}))
}

func TestNewDisabledWithoutKey(t *testing.T) {
	if New("http://x", "", "m") != nil {
		t.Fatal("client without api key must be disabled")
	}
}

func TestParseBill(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    Bill
		wantErr bool
	}{
		{
			name:    "plain json",
			content: `{"description":"Nước ép","total":252000,"people":[{"name":"Hà","amount":0},{"name":" Sơn ","amount":28000.4}]}`,
			want:    Bill{Description: "Nước ép", Total: 252000, People: []Person{{Name: "Hà"}, {Name: "Sơn", Amount: 28000}}},
		},
		{
			name:    "blank names are dropped",
			content: `{"description":"x","total":5,"people":[{"name":"","amount":10},{"name":"  ","amount":10},{"name":"B","amount":1}]}`,
			want:    Bill{Description: "x", Total: 5, People: []Person{{Name: "B", Amount: 1}}},
		},
		{
			name:    "user_id is optional and null means unknown",
			content: `{"description":"","total":0,"people":[{"name":"Ki","amount":0,"user_id":68},{"name":"Hà","amount":0,"user_id":null},{"name":"B","amount":0}]}`,
			want:    Bill{People: []Person{{Name: "Ki", UserID: 68}, {Name: "Hà"}, {Name: "B"}}},
		},
		{
			name:    "prorate is read, and defaults to false",
			content: `{"description":"","total":100,"prorate":true,"people":[{"name":"A","amount":50}]}`,
			want:    Bill{Total: 100, Prorate: true, People: []Person{{Name: "A", Amount: 50}}},
		},
		{name: "prose instead of json", content: "xin lỗi", wantErr: true},
		{name: "broken json", content: `{"total": }`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseBill(tt.content)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if got.Description != tt.want.Description || got.Total != tt.want.Total || len(got.People) != len(tt.want.People) {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
			for i := range got.People {
				if got.People[i] != tt.want.People[i] {
					t.Errorf("person %d = %+v, want %+v", i, got.People[i], tt.want.People[i])
				}
			}
		})
	}
}

func TestClientParseBillRequestShape(t *testing.T) {
	var body string
	srv := fakeServer(t, http.StatusOK, `{"description":"d","total":1000,"people":[{"name":"A","amount":0}]}`, &body)
	defer srv.Close()

	c := New(srv.URL+"/v1/", "key", "test-model")
	bill, err := c.ParseBill(context.Background(), Input{
		Text:   "chia 1k cho A",
		Sheet:  "ten,tien\nA,1000",
		Images: [][]byte{{0xff, 0xd8, 0xff, 0xe0}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if bill.Total != 1000 || len(bill.People) != 1 {
		t.Fatalf("bill = %+v", bill)
	}
	for _, want := range []string{`"model":"test-model"`, `"json_schema"`, `"name":"bill"`, `"additionalProperties":false`, `"adjustments"`, `"covers"`, "ten,tien", "data:image/jpeg;base64,/9j/4A=="} {
		if !strings.Contains(body, want) {
			t.Errorf("request body missing %q: %s", want, body)
		}
	}
}

func TestClientParseBillHTTPError(t *testing.T) {
	var body string
	srv := fakeServer(t, http.StatusForbidden, "blocked", &body)
	defer srv.Close()

	_, err := New(srv.URL+"/v1", "key", "m").ParseBill(context.Background(), Input{Text: "x"})
	if err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("err = %v, want status 403", err)
	}
}

func rawServer(status int, body string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		io.WriteString(w, body)
	}))
}

func TestClientParseBillBadResponses(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"body is not json", "<html>bad gateway</html>", "decode llm response"},
		{"empty body", "", "decode llm response"},
		{"no choices", `{"choices":[]}`, "llm returned no choices"},
		{"choices missing", `{}`, "llm returned no choices"},
		{"choices null", `{"choices":null}`, "llm returned no choices"},
		{"content empty", `{"choices":[{"message":{"content":""}}]}`, "decode bill"},
		{"content null", `{"choices":[{"message":{"content":null}}]}`, "decode bill"},
		{"response past 1 MiB is truncated", `{"choices":[{"message":{"content":"` + strings.Repeat("a", 1<<20) + `"}}]}`, "decode llm response"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := rawServer(http.StatusOK, tt.body)
			defer srv.Close()

			_, err := New(srv.URL, "key", "m").ParseBill(context.Background(), Input{Text: "x"})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestClientParseBillUnreachable(t *testing.T) {
	srv := rawServer(http.StatusOK, "")
	url := srv.URL
	srv.Close()

	if _, err := New(url, "key", "m").ParseBill(context.Background(), Input{Text: "x"}); err == nil {
		t.Fatal("expected error when the LLM endpoint is down")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	live := rawServer(http.StatusOK, "{}")
	defer live.Close()
	if _, err := New(live.URL, "key", "m").ParseBill(ctx, Input{Text: "x"}); err == nil {
		t.Fatal("expected error for a cancelled context")
	}
}

func TestClientParseBillEmptyInputStillSendsRequest(t *testing.T) {
	var body string
	srv := fakeServer(t, http.StatusOK, `{"description":"","total":0,"people":[]}`, &body)
	defer srv.Close()

	bill, err := New(srv.URL+"/v1", "key", "m").ParseBill(context.Background(), Input{})
	if err != nil {
		t.Fatal(err)
	}
	if bill.Description != "" || bill.Total != 0 || len(bill.People) != 0 {
		t.Errorf("bill = %+v, want zero value", bill)
	}
	if strings.Contains(body, "image_url") {
		t.Errorf("no images given but request has image_url: %s", body)
	}
}

func TestParseBillEmptyAndNullShapes(t *testing.T) {
	for _, content := range []string{`{}`, `null`, `{"people":null}`, `{"description":null,"total":null,"people":[]}`} {
		bill, err := parseBill(content)
		if err != nil || bill.Description != "" || bill.Total != 0 || len(bill.People) != 0 {
			t.Errorf("parseBill(%s) = %+v, %v; want zero bill, nil error", content, bill, err)
		}
	}

	if _, err := parseBill(""); err == nil {
		t.Error("empty content must be an error")
	}

	bill, err := parseBill(`{"people":[{"name":null,"amount":5},{"name":"A","amount":null}]}`)
	if err != nil || len(bill.People) != 1 || bill.People[0] != (Person{Name: "A"}) {
		t.Errorf("null name/amount: bill = %+v, err = %v", bill, err)
	}
}

func TestParseBillAmountLimit(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{"total at limit", `{"total":1000000000000,"people":[]}`, false},
		{"total just past limit", `{"total":1000000000001,"people":[]}`, true},
		{"person amount at limit", `{"people":[{"name":"A","amount":1000000000000}]}`, false},
		{"person amount just past limit", `{"people":[{"name":"A","amount":1000000000001}]}`, true},
		{"beyond int64", `{"total":1e30,"people":[]}`, true},
		{"negative total", `{"total":-1,"people":[]}`, true},
		{"negative person amount", `{"people":[{"name":"A","amount":-1}]}`, true},
		{"zero amounts mean equal share", `{"total":0,"people":[{"name":"A","amount":0}]}`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseBill(tt.content)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseBillImageTypes(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\n....")
	tests := []struct {
		name    string
		img     []byte
		want    string
		wantErr bool
	}{
		{"png is labelled png", png, "data:image/png;base64,", false},
		{"gif is labelled gif", []byte("GIF89a...."), "data:image/gif;base64,", false},
		{"text is rejected", []byte("not an image"), "", true},
		{"empty is rejected", nil, "", true},
		{"svg is rejected", []byte("<svg xmlns='http://www.w3.org/2000/svg'></svg>"), "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body string
			srv := fakeServer(t, http.StatusOK, `{"description":"d","total":1,"people":[]}`, &body)
			defer srv.Close()

			_, err := New(srv.URL+"/v1", "key", "m").ParseBill(context.Background(), Input{Text: "x", Images: [][]byte{tt.img}})
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && body != "" {
				t.Errorf("request was sent for a rejected image: %s", body)
			}
			if !tt.wantErr && !strings.Contains(body, tt.want) {
				t.Errorf("body lacks %q: %s", tt.want, body)
			}
		})
	}
}

func TestInputMessageListsKnownUsersAndAliases(t *testing.T) {
	got := Input{
		Text:    "chia 100k",
		Sheet:   "a,b",
		Users:   []KnownUser{{ID: 68, Name: "son.ho", Account: "son.ho"}, {ID: 4, Name: "Trọng", Account: "trong.vu"}},
		Aliases: map[string]int64{"ki": 68, "hongg ngoc": 4},
	}.message()

	for _, want := range []string{
		"chia 100k", "Bảng CSV:\na,b",
		"68 | son.ho | son.ho\n", "4 | Trọng | trong.vu\n",
		"hongg ngoc = 4\nki = 68\n", // sorted, so the request is stable
	} {
		if !strings.Contains(got, want) {
			t.Errorf("message missing %q:\n%s", want, got)
		}
	}
	if only := (Input{Text: "x"}).message(); only != "x" {
		t.Errorf("without users or aliases the message is just the text, got %q", only)
	}
}

func TestParseBillAdjustments(t *testing.T) {
	bill, err := parseBill(`{"description":"","total":511100,"prorate":false,"adjustments":[{"label":" Phí áp dụng ","amount":10000},{"label":"Giảm 10%","amount":-56900.4}],"people":[{"name":"A","amount":569000}]}`)
	if err != nil {
		t.Fatal(err)
	}
	want := []Adjustment{{"Phí áp dụng", 10000}, {"Giảm 10%", -56900}}
	if len(bill.Adjustments) != 2 || bill.Adjustments[0] != want[0] || bill.Adjustments[1] != want[1] {
		t.Fatalf("adjustments = %+v, want %+v", bill.Adjustments, want)
	}
	if bill.AdjustmentSum() != -46900 {
		t.Errorf("sum = %d, want -46900", bill.AdjustmentSum())
	}

	for _, content := range []string{`{"people":[]}`, `{"adjustments":null,"people":[]}`, `{"adjustments":[],"people":[]}`} {
		bill, err := parseBill(content)
		if err != nil || len(bill.Adjustments) != 0 || bill.AdjustmentSum() != 0 {
			t.Errorf("parseBill(%s) = %+v, %v; want no adjustments", content, bill, err)
		}
	}

	for _, content := range []string{
		`{"adjustments":[{"label":"x","amount":1000000000001}],"people":[]}`,
		`{"adjustments":[{"label":"x","amount":-1000000000001}],"people":[]}`,
	} {
		if _, err := parseBill(content); err == nil {
			t.Errorf("out-of-range adjustment accepted: %s", content)
		}
	}
}

func TestParseBillCovers(t *testing.T) {
	bill, err := parseBill(`{"covers":[{"name":" toan.tran ","user_id":32,"for":[{"name":"Trọng","user_id":4},{"name":" ","user_id":9},{"name":"son.ho","user_id":0}]},{"name":"","user_id":5,"for":[{"name":"X","user_id":1}]}],"people":[]}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(bill.Covers) != 1 {
		t.Fatalf("covers = %+v, want the nameless payer dropped", bill.Covers)
	}
	c := bill.Covers[0]
	if c.Payer != (Person{Name: "toan.tran", UserID: 32}) || len(c.For) != 2 || c.For[0] != (Person{Name: "Trọng", UserID: 4}) || c.For[1].Name != "son.ho" {
		t.Errorf("cover = %+v", c)
	}

	for _, content := range []string{`{"people":[]}`, `{"covers":null,"people":[]}`, `{"covers":[],"people":[]}`} {
		if bill, err := parseBill(content); err != nil || len(bill.Covers) != 0 {
			t.Errorf("parseBill(%s) = %+v, %v; want no covers", content, bill, err)
		}
	}
}
