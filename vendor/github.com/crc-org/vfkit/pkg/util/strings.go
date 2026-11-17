package util

import "strings"

func TrimQuotes(str string) string {
	if strings.HasPrefix(str, `"`) && strings.HasSuffix(str, `"`) {
		str = strings.Trim(str, `"`)
	}

	return str
}

func StringInSlice(st string, sl []string) bool {
	if sl == nil {
		return false
	}
	for _, s := range sl {
		if st == s {
			return true
		}
	}
	return false
}
