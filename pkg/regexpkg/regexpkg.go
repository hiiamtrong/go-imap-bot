package regexpkg

import (
	"fmt"
	"regexp"
)

func ExtractHash(description string) (string, error) {
	re := regexp.MustCompile(`(trx|TRX)\w+(ong|ONG)`)
	matches := re.FindStringSubmatch(description)
	if len(matches) == 0 {
		return "", fmt.Errorf("no hash found in description")
	}
	return matches[0], nil
}
