package currency

import "fmt"

func FormatCurrency(amount float64) string {
	// Convert to integer to avoid floating point precision issues
	intAmount := int64(amount)

	// Convert to string
	str := fmt.Sprintf("%d", intAmount)

	// Add thousand separators
	var result []rune
	length := len(str)
	for i, char := range str {
		result = append(result, char)
		if (length-i-1)%3 == 0 && i != length-1 {
			result = append(result, '.')
		}
	}

	return string(result) + " VND"
}
