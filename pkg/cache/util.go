package cache

import (
	"regexp"
	"strconv"
	"strings"
)

func ExtractLevelAndNickname(text string) (int, string) {
	r1 := strings.TrimPrefix(text, "Lv")
	r2 := strings.TrimPrefix(r1, "lv")
	r3 := strings.TrimPrefix(r2, "LV")
	r4 := strings.ReplaceAll(r3, " ", "")
	r5 := strings.ReplaceAll(r4, ".", "")

	re := regexp.MustCompile(`^(\d{1,3})(.*)`)
	match := re.FindStringSubmatch(r5)
	if len(match) > 0 {
		digits := match[1]
		nickname := match[2]
		for i := len(digits); i > 0; i-- {
			// try to convert the level digits and check if it's in the range
			levelNum, err := strconv.Atoi(digits[:i])
			if err == nil {
				if levelNum >= 85 && levelNum <= 200 {
					return levelNum, digits[i:] + nickname
				}
			}
		}
	}
	// no match
	return 0, text
}
