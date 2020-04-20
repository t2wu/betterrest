package letters

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// PascalCaseToSnakeCase turns Pascal case to snake case, but if xxxID then xxx_id
// This is so we can find the entry in the database
func PascalCaseToSnakeCase(str string) string {
	var newstr strings.Builder
	newstr.WriteString(strings.ToLower(string(str[0])))
	i := 1
	for i < len(str) {
		if isStrLowerAtPosI(str, i) {
			newstr.WriteString(string(str[i]))
			i = i + 1
		} else {
			byteArray := make([]byte, 0)
			byteArray = append(byteArray, str[i])
			var j int
			for j = i + 1; len(str) > j && isStrUpperAtPosI(str, j); j++ {
				byteArray = append(byteArray, str[j])
			}
			newstr.WriteString("_")
			newstr.WriteString(strings.ToLower(string(byteArray)))
			i = j
		}
	}
	return newstr.String()
}

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
