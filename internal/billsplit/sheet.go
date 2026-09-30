package billsplit

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const maxSheetBytes = 200 << 10

var (
	sheetLink = regexp.MustCompile(`https://docs\.google\.com/spreadsheets/d/([A-Za-z0-9_-]+)\S*`)
	sheetGID  = regexp.MustCompile(`gid=(\d+)`)

	sheetClient = &http.Client{
		Timeout: 20 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			host := req.URL.Hostname()
			if len(via) >= 5 || req.URL.Scheme != "https" ||
				!(strings.HasSuffix(host, ".google.com") || strings.HasSuffix(host, ".googleusercontent.com")) {
				return fmt.Errorf("redirect to %s not allowed", host)
			}
			return nil
		},
	}
)

func SheetExportURL(text string) (string, bool) {
	m := sheetLink.FindStringSubmatch(text)
	if m == nil {
		return "", false
	}
	u := "https://docs.google.com/spreadsheets/d/" + m[1] + "/export?format=csv"
	if g := sheetGID.FindStringSubmatch(m[0]); g != nil {
		u += "&gid=" + g[1]
	}
	return u, true
}

func FetchSheetCSV(ctx context.Context, exportURL string) (string, error) {
	return fetchCSV(ctx, sheetClient, exportURL)
}

func fetchCSV(ctx context.Context, client *http.Client, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/csv") {
		return "", errors.New("không đọc được sheet, hãy bật chia sẻ \"bất kỳ ai có đường liên kết\" ở quyền xem")
	}

	b, err := io.ReadAll(io.LimitReader(resp.Body, maxSheetBytes+1))
	if err != nil {
		return "", err
	}
	if len(b) > maxSheetBytes {
		return "", errors.New("sheet quá lớn, hãy chỉ giữ phần bảng chia bill")
	}
	return string(b), nil
}
