package currency

import (
	"fmt"
	"strconv"
	"strings"
)

func FormatCurrency(amount float64) string {
	return fmt.Sprintf("%s₫", formatNumber(amount))
}

func formatNumber(n float64) string {
	in := strconv.FormatFloat(n, 'f', 0, 64)
	numOfDigits := len(in)
	if n < 0 {
		numOfDigits-- // First character is the - sign (not a digit)
	}
	numOfCommas := (numOfDigits - 1) / 3

	out := make([]byte, len(in)+numOfCommas)
	if n < 0 {
		in = in[1:]
		out[0] = '-'
	}

	for i, j, k := len(in)-1, len(out)-1, 0; ; i, j = i-1, j-1 {
		out[j] = in[i]
		if i == 0 {
			break
		}
		if k++; k == 3 {
			j--
			out[j] = ','
			k = 0
		}
	}
	return string(out)
}

func ParseCurrency(s string) (float64, error) {
	// Remove currency symbol and any whitespace
	s = strings.TrimSpace(strings.ReplaceAll(s, "₫", ""))
	// Remove commas
	s = strings.ReplaceAll(s, ",", "")
	// Parse as float
	return strconv.ParseFloat(s, 64)
}
