package gotag

import "strings"

func TagValueHasPrefix(tagVal, prefix string) bool {
	pairs := strings.Split(tagVal, ";")
	for _, pair := range pairs {
		if strings.HasPrefix(pair, prefix) {
			return true
		}
	}
	return false
}

func TagFieldByPrefix(tagVal, prefix string) string {
	pairs := strings.Split(tagVal, ";")
	for _, pair := range pairs {
		if strings.HasPrefix(pair, prefix) {
			return pair
		}
	}
	return ""
}
