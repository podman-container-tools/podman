//go:build !windows && !js && !appengine
// +build !windows,!js,!appengine

package runewidth

import (
	"os"
	"strings"
)

func mblen(charset string) int {
	switch charset {
	case "utf-8", "utf8":
		return 6
	case "jis":
		return 8
	case "eucjp":
		return 3
	case "euckr", "euccn", "sjis", "cp932", "cp51932", "cp936", "cp949", "cp950", "big5", "gbk", "gb2312":
		return 2
	}
	return 1
}

// localeCharset extracts the charset part of a locale name of the form
// "ll.CHARSET" or "ll_CC.CHARSET" (two- or three-letter language code,
// optional uppercase country code). It returns "" if locale does not have
// that shape.
func localeCharset(locale string) string {
	n := 0
	for n < len(locale) && locale[n] >= 'a' && locale[n] <= 'z' {
		n++
	}
	if n < 2 || n > 3 {
		return ""
	}
	rest := locale[n:]
	if len(rest) >= 3 && rest[0] == '_' &&
		rest[1] >= 'A' && rest[1] <= 'Z' && rest[2] >= 'A' && rest[2] <= 'Z' {
		rest = rest[3:]
	}
	if len(rest) >= 2 && rest[0] == '.' {
		return rest[1:]
	}
	return ""
}

func isEastAsian(locale string) bool {
	charset := strings.ToLower(locale)
	if cs := localeCharset(locale); cs != "" {
		charset = strings.ToLower(cs)
	}

	if strings.HasSuffix(charset, "@cjk_narrow") {
		return false
	}

	for pos, b := range []byte(charset) {
		if b == '@' {
			charset = charset[:pos]
			break
		}
	}
	max := mblen(charset)
	if max > 1 && (charset[0] != 'u' ||
		strings.HasPrefix(locale, "ja") ||
		strings.HasPrefix(locale, "ko") ||
		strings.HasPrefix(locale, "zh")) {
		return true
	}
	return false
}

// IsEastAsian return true if the current locale is CJK
func IsEastAsian() bool {
	locale := os.Getenv("LC_ALL")
	if locale == "" {
		locale = os.Getenv("LC_CTYPE")
	}
	if locale == "" {
		locale = os.Getenv("LANG")
	}

	// ignore C locale
	if locale == "POSIX" || locale == "C" {
		return false
	}
	if len(locale) > 1 && locale[0] == 'C' && (locale[1] == '.' || locale[1] == '-') {
		return false
	}

	return isEastAsian(locale)
}
