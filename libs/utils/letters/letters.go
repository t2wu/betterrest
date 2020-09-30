package letters

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// CamelCaseToPascalCase turns camel case to pascal case (first letter capitalized)
func CamelCaseToPascalCase(str string) string {
	if len(str) == 0 {
		return str
	}
	return strings.ToUpper(string(str[0])) + str[1:]
}

func isStrLowerAtPosI(s string, i int) bool {
	b := s[i]
	r, _ := utf8.DecodeRune([]byte{b})
	return unicode.IsLower(r)
}

func isStrUpperAtPosI(s string, i int) bool {
	return !isStrLowerAtPosI(s, i)
}
