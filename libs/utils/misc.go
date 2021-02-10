package utils

// SliceContainsString check if there exists an element exactly the string s
func SliceContainsString(arr []string, s string) bool {
	for _, ele := range arr {
		if ele == s {
			return true
		}
	}
	return false
}
