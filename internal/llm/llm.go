package llm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"math"
	"net/http"
	"slices"
	"strings"
	"time"
)

const maxMoney = 1e12

var imageTypes = []string{"image/jpeg", "image/png", "image/gif", "image/webp"}

const billSchema = `{"type":"object","properties":{"description":{"type":"string"},"total":{"type":"number"},"people":{"type":"array","items":{"type":"object","properties":{"name":{"type":"string"},"amount":{"type":"number"},"user_id":{"type":"integer"}},"required":["name","amount","user_id"],"additionalProperties":false}}},"required":["description","total","people"],"additionalProperties":false}`

const systemPrompt = `Bạn trích xuất thông tin chia bill từ tin nhắn tiếng Việt, ảnh (hóa đơn, ảnh chụp bảng hoặc đoạn chat) hoặc bảng CSV.
Chỉ trả về một JSON object, không giải thích:
{"description": string, "total": number, "people": [{"name": string, "amount": number}]}
Quy tắc:
- Tiền là VND, số nguyên: "50k"=50000, "1tr2"=1200000, "1.5tr"=1500000.
- description: tên món hoặc quán nếu người dùng nêu rõ; chuỗi rỗng nếu không nêu, không tự đặt. total: tổng bill, 0 nếu không nêu.
- people: mỗi người tham gia là một phần tử. name là tên của người đó đúng như in trên bill, ảnh, bảng hoặc đoạn chat, nếu không có bill thì đúng như người dùng viết, kể cả "tôi", "mình". amount là số tiền riêng của người đó, 0 nếu chia đều phần còn lại.
- user_id: id của người dùng đã biết khớp với người đó (danh sách người dùng và tên đã nhớ nằm cuối tin nhắn). Khớp theo tên trên bill, ảnh, bảng, chat và theo tên hoặc tài khoản trong tin nhắn, chấp nhận khác dấu, sai chính tả, viết tắt (ví dụ ảnh ghi "Hồngg Ngọc" mà tin nhắn ghi "Hồng Ngọc: ngoc.vu" thì là ngoc.vu). Tên đã nhớ là mặc định, trừ khi tin nhắn nói rõ khác. Trả 0 nếu không chắc chắn hoặc có nhiều người có thể khớp; không đoán.
- Với bảng: mỗi dòng là một người và số tiền của họ; total là dòng tổng nếu có.
- Không thêm người không được nhắc tới, không tự tính lại tổng.`

type Client struct {
	baseURL string
	apiKey  string
	model   string
	http    *http.Client
}

func New(baseURL, apiKey, model string) *Client {
	if apiKey == "" {
		return nil
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		model:   model,
		http:    &http.Client{Timeout: 90 * time.Second},
	}
}

// KnownUser is a user the model may match a person on the bill to. Account is
// the email without its domain, which is all the model needs.
type KnownUser struct {
	ID      int64
	Name    string
	Account string
}

type Input struct {
	Text   string
	Sheet  string
	Images [][]byte
	Users  []KnownUser
	// Aliases maps a remembered bill name (billsplit.AliasKey) to a user ID.
	Aliases map[string]int64
}

func (in Input) message() string {
	var b strings.Builder
	b.WriteString(in.Text)
	if in.Sheet != "" {
		b.WriteString("\n\nBảng CSV:\n" + in.Sheet)
	}
	if len(in.Users) > 0 {
		b.WriteString("\n\nNgười dùng đã biết (id | tên | tài khoản):\n")
		for _, u := range in.Users {
			fmt.Fprintf(&b, "%d | %s | %s\n", u.ID, u.Name, u.Account)
		}
	}
	if len(in.Aliases) > 0 {
		b.WriteString("\nTên đã nhớ (tên trên bill = id):\n")
		for _, key := range slices.Sorted(maps.Keys(in.Aliases)) {
			fmt.Fprintf(&b, "%s = %d\n", key, in.Aliases[key])
		}
	}
	return b.String()
}

type Person struct {
	Name   string
	Amount int64
	UserID int64 // the known user the model matched, 0 when it was not sure
}

type Bill struct {
	Description string
	Total       int64
	People      []Person
}

type rawBill struct {
	Description string  `json:"description"`
	Total       float64 `json:"total"`
	People      []struct {
		Name   string  `json:"name"`
		Amount float64 `json:"amount"`
		UserID float64 `json:"user_id"`
	} `json:"people"`
}

func (c *Client) ParseBill(ctx context.Context, in Input) (*Bill, error) {
	parts := []map[string]any{{"type": "text", "text": in.message()}}
	for _, img := range in.Images {
		mime := http.DetectContentType(img)
		if !slices.Contains(imageTypes, mime) {
			return nil, fmt.Errorf("unsupported image type %q", mime)
		}
		parts = append(parts, map[string]any{
			"type":      "image_url",
			"image_url": map[string]string{"url": "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(img)},
		})
	}

	body, err := json.Marshal(map[string]any{
		"model":       c.model,
		"temperature": 0,
		"response_format": map[string]any{
			"type":        "json_schema",
			"json_schema": map[string]any{"name": "bill", "schema": json.RawMessage(billSchema)},
		},
		"messages": []map[string]any{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": parts},
		},
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("llm status %d: %.200s", resp.StatusCode, respBody)
	}

	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, fmt.Errorf("decode llm response: %w", err)
	}
	if len(out.Choices) == 0 {
		return nil, fmt.Errorf("llm returned no choices")
	}

	return parseBill(out.Choices[0].Message.Content)
}

func parseBill(content string) (*Bill, error) {
	var raw rawBill
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		return nil, fmt.Errorf("decode bill: %w", err)
	}

	if raw.Total < 0 || raw.Total > maxMoney {
		return nil, fmt.Errorf("total %v out of range", raw.Total)
	}
	bill := &Bill{Description: strings.TrimSpace(raw.Description), Total: int64(math.Round(raw.Total))}
	for _, p := range raw.People {
		if p.Amount < 0 || p.Amount > maxMoney {
			return nil, fmt.Errorf("amount %v out of range", p.Amount)
		}
		if name := strings.TrimSpace(p.Name); name != "" {
			bill.People = append(bill.People, Person{Name: name, Amount: int64(math.Round(p.Amount)), UserID: int64(p.UserID)})
		}
	}
	return bill, nil
}
