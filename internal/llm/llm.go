package llm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"slices"
	"strings"
	"time"
)

const maxMoney = 1e12

var imageTypes = []string{"image/jpeg", "image/png", "image/gif", "image/webp"}

const billSchema = `{"type":"object","properties":{"description":{"type":"string"},"total":{"type":"number"},"people":{"type":"array","items":{"type":"object","properties":{"name":{"type":"string"},"amount":{"type":"number"},"alias":{"type":"string"}},"required":["name","amount","alias"],"additionalProperties":false}}},"required":["description","total","people"],"additionalProperties":false}`

const systemPrompt = `Bạn trích xuất thông tin chia bill từ tin nhắn tiếng Việt, ảnh (hóa đơn, ảnh chụp bảng hoặc đoạn chat) hoặc bảng CSV.
Chỉ trả về một JSON object, không giải thích:
{"description": string, "total": number, "people": [{"name": string, "amount": number}]}
Quy tắc:
- Tiền là VND, số nguyên: "50k"=50000, "1tr2"=1200000, "1.5tr"=1500000.
- description: tên món hoặc quán nếu người dùng nêu rõ; chuỗi rỗng nếu không nêu, không tự đặt. total: tổng bill, 0 nếu không nêu.
- people: mỗi người tham gia là một phần tử. name giữ nguyên cách viết, kể cả "tôi", "mình". amount là số tiền riêng của người đó, 0 nếu chia đều phần còn lại.
- alias: chỉ điền khi tin nhắn cho biết một tên hoặc tài khoản khác cho chính người đó, ví dụ "ki: son.ho" hoặc danh sách tài khoản theo đúng thứ tự người trong ảnh. Khi đó name là tên hoặc tài khoản trong tin nhắn ("son.ho") và alias là tên của người đó đúng như in trên bill, ảnh, bảng hoặc đoạn chat ("Ki"). Không có thì alias là chuỗi rỗng; không tự đoán ghép người.
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

type Input struct {
	Text   string
	Sheet  string
	Images [][]byte
}

type Person struct {
	Name   string
	Amount int64
	Alias  string
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
		Alias  string  `json:"alias"`
	} `json:"people"`
}

func (c *Client) ParseBill(ctx context.Context, in Input) (*Bill, error) {
	text := in.Text
	if in.Sheet != "" {
		text += "\n\nBảng CSV:\n" + in.Sheet
	}
	parts := []map[string]any{{"type": "text", "text": text}}
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
			bill.People = append(bill.People, Person{Name: name, Amount: int64(math.Round(p.Amount)), Alias: strings.TrimSpace(p.Alias)})
		}
	}
	return bill, nil
}
