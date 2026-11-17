package parser

import (
	"fmt"
	"strings"
	"unicode"
)

/* Functions to split/join, unescape/escape strings similar to Exec=... lines in unit files */

type SplitFlags = uint64

const (
	SplitRelax                  SplitFlags = 1 << iota // Allow unbalanced quote and eat up trailing backslash.
	SplitCUnescape                                     // Unescape known escape sequences.
	SplitUnescapeRelax                                 // Allow and keep unknown escape sequences, allow and keep trailing backslash.
	SplitUnescapeSeparators                            // Unescape separators (those specified, or whitespace by default).
	SplitKeepQuote                                     // Ignore separators in quoting with "" and ''.
	SplitUnquote                                       // Ignore separators in quoting with "" and '', and remove the quotes.
	SplitDontCoalesceSeparators                        // Don't treat multiple adjacent separators as one
	SplitRetainEscape                                  // Treat escape character '\' as any other character without special meaning
	SplitRetainSeparators                              // Do not advance the original string pointer past the separator(s) */
)

const WhitespaceSeparators = " \t\n\r"

func unoctchar(v byte) int {
	if v >= '0' && v <= '7' {
		return int(v - '0')
	}

	return -1
}

func unhexchar(v byte) int {
	if v >= '0' && v <= '9' {
		return int(v - '0')
	}

	if v >= 'a' && v <= 'f' {
		return int(v - 'a' + 10)
	}

	if v >= 'A' && v <= 'F' {
		return int(v - 'A' + 10)
	}

	return -1
}

func isValidUnicode(c uint32) bool {
	return c <= unicode.MaxRune
}

/* This is based on code from systemd (src/basic/escape.c), marked LGPL-2.1-or-later and is copyrighted by the systemd developers */

func cUnescapeOne(p string, acceptNul bool) (int, rune, bool) {
	count := 1
	eightBit := false
	var ret rune

	// Unescapes C style. Returns the unescaped character in ret.
	// Returns eightBit as true if the escaped sequence either fits in
	// one byte in UTF-8 or is a non-unicode literal byte and should
	// instead be copied directly.

	if len(p) < 1 {
		return -1, 0, false
	}

	switch p[0] {
	case 'a':
		ret = '\a'
	case 'b':
		ret = '\b'
	case 'f':
		ret = '\f'
	case 'n':
		ret = '\n'
	case 'r':
		ret = '\r'
	case 't':
		ret = '\t'
	case 'v':
		ret = '\v'
	case '\\':
		ret = '\\'
	case '"':
		ret = '"'
	case '\'':
		ret = '\''
	case 's':
		/* This is an extension of the XDG syntax files */
		ret = ' '
	case 'x':
		/* hexadecimal encoding */
		if len(p) < 3 {
			return -1, 0, false
		}

		a := unhexchar(p[1])
		if a < 0 {
			return -1, 0, false
		}

		b := unhexchar(p[2])
		if b < 0 {
			return -1, 0, false
		}

		/* Don't allow NUL bytes */
		if a == 0 && b == 0 && !acceptNul {
			return -1, 0, false
		}

		ret = rune((a << 4) | b)
		eightBit = true
		count = 3
	case 'u':
		/* C++11 style 16bit unicode */

		if len(p) < 5 {
			return -1, 0, false
		}

		var a [4]int
		for i := range 4 {
			a[i] = unhexchar(p[1+i])
			if a[i] < 0 {
				return -1, 0, false
			}
		}

		c := (uint32(a[0]) << 12) | (uint32(a[1]) << 8) | (uint32(a[2]) << 4) | uint32(a[3])

		/* Don't allow 0 chars */
		if c == 0 && !acceptNul {
			return -1, 0, false
		}

		ret = rune(c)
		count = 5
	case 'U':
		/* C++11 style 32bit unicode */

		if len(p) < 9 {
			return -1, 0, false
		}

		var a [8]int
		for i := range 8 {
			a[i] = unhexchar(p[1+i])
			if a[i] < 0 {
				return -10, 0, false
			}
		}

		c := (uint32(a[0]) << 28) | (uint32(a[1]) << 24) | (uint32(a[2]) << 20) | (uint32(a[3]) << 16) |
			(uint32(a[4]) << 12) | (uint32(a[5]) << 8) | (uint32(a[6]) << 4) | uint32(a[7])

		/* Don't allow 0 chars */
		if c == 0 && !acceptNul {
			return -1, 0, false
		}

		/* Don't allow invalid code points */
		if !isValidUnicode(c) {
			return -1, 0, false
		}

		ret = rune(c)
		count = 9
	case '0', '1', '2', '3', '4', '5', '6', '7':
		/* octal encoding */

		if len(p) < 3 {
			return -1, 0, false
		}

		a := unoctchar(p[0])
		if a < 0 {
			return -1, 0, false
		}

		b := unoctchar(p[1])
		if b < 0 {
			return -1, 0, false
		}

		c := unoctchar(p[2])
		if c < 0 {
			return -1, 0, false
		}

		/* don't allow NUL bytes */
		if a == 0 && b == 0 && c == 0 && !acceptNul {
			return -1, 0, false
		}

		/* Don't allow bytes above 255 */
		m := (uint32(a) << 6) | (uint32(b) << 3) | uint32(c)
		if m > 255 {
			return -1, 0, false
		}

		ret = rune(m)
		eightBit = true
		count = 3
	default:
		return -1, 0, false
	}

	return count, ret, eightBit
}

/* This is based on code from systemd (src/basic/extract-workd.c), marked LGPL-2.1-or-later and is copyrighted by the systemd developers */

// Returns: word, remaining, more-words, error
func extractFirstWord(in string, separators string, flags SplitFlags) (string, string, bool, error) {
	var s strings.Builder
	var quote byte     // 0 or ' or "
	backslash := false // whether we've just seen a backslash

	// The string handling in this function is a bit weird, using
	// 0 bytes to mark end-of-string. This is because it is a direct
	// conversion of the C in systemd, and w want to ensure
	// exactly the same behaviour of some complex code

	p := 0
	end := len(in)
	var c byte

	nextChar := func() byte {
		p++
		if p >= end {
			return 0
		}
		return in[p]
	}

	/* Bail early if called after last value or with no input */
	if len(in) == 0 {
		goto finish
	}

	// Parses the first word of a string, and returns it and the
	// remainder. Removes all quotes in the process. When parsing
	// fails (because of an uneven number of quotes or similar),
	// the rest is at the first invalid character. */

loop1:
	for c = in[0]; ; c = nextChar() {
		switch {
		case c == 0:
			goto finishForceTerminate
		case strings.ContainsRune(separators, rune(c)):
			if flags&SplitDontCoalesceSeparators != 0 {
				if flags&SplitRetainSeparators == 0 {
					p++
				}
				goto finishForceNext
			}
		default:
			// We found a non-blank character, so we will always
			// want to return a string (even if it is empty),
			// allocate it here.
			break loop1
		}
	}

	for ; ; c = nextChar() {
		switch {
		case backslash:
			if c == 0 {
				if flags&SplitUnescapeRelax != 0 &&
					(quote == 0 || flags&SplitRelax != 0) {
					// If we find an unquoted trailing backslash and we're in
					// SplitUnescapeRelax mode, keep it verbatim in the
					// output.
					//
					// Unbalanced quotes will only be allowed in SplitRelax
					// mode, SplitUnescapeRelax mode does not allow them.
					s.WriteString("\\")
					goto finishForceTerminate
				}
				if flags&SplitRelax != 0 {
					goto finishForceTerminate
				}
				return "", "", false, fmt.Errorf("unbalanced escape")
			}

			if flags&(SplitCUnescape|SplitUnescapeSeparators) != 0 {
				r := -1
				var u rune

				if flags&SplitCUnescape != 0 {
					r, u, _ = cUnescapeOne(in[p:], false)
				}

				switch {
				case r > 0:
					p += r - 1
					s.WriteRune(u)
				case (flags&SplitUnescapeSeparators != 0) &&
					(strings.ContainsRune(separators, rune(c)) || c == '\\'):
					/* An escaped separator char or the escape char itself */
					s.WriteByte(c)
				case flags&SplitUnescapeRelax != 0:
					s.WriteByte('\\')
					s.WriteByte(c)
				default:
					return "", "", false, fmt.Errorf("unsupported escape char")
				}
			} else {
				s.WriteByte(c)
			}

			backslash = false
		case quote != 0:
			/* inside either single or double quotes */
		quoteloop:
			for ; ; c = nextChar() {
				switch {
				case c == 0:
					if flags&SplitRelax != 0 {
						goto finishForceTerminate
					}
					return "", "", false, fmt.Errorf("unbalanced quotes")
				case c == quote:
					/* found the end quote */
					quote = 0
					if flags&SplitUnquote != 0 {
						break quoteloop
					}
				case c == '\\' && (flags&SplitRetainEscape == 0):
					backslash = true
					break quoteloop
				}

				s.WriteByte(c)

				if quote == 0 {
					break quoteloop
				}
			}
		default:
		nonquoteloop:
			for ; ; c = nextChar() {
				switch {
				case c == 0:
					goto finishForceTerminate
				case (c == '\'' || c == '"') && (flags&(SplitKeepQuote|SplitUnquote) != 0):
					quote = c
					if flags&SplitUnquote != 0 {
						break nonquoteloop
					}
				case c == '\\' && (flags&SplitRetainEscape == 0):
					backslash = true
					break nonquoteloop
				case strings.ContainsRune(separators, rune(c)):
					if flags&SplitDontCoalesceSeparators != 0 {
						if flags&SplitRetainSeparators == 0 {
							p++
						}
						goto finishForceNext
					}

					if flags&SplitRetainSeparators == 0 {
						/* Skip additional coalesced separators. */
						for ; ; c = nextChar() {
							if c == 0 {
								goto finishForceTerminate
							}
							if !strings.ContainsRune(separators, rune(c)) {
								break
							}
						}
					}
					goto finish
				}

				s.WriteByte(c)

				if quote != 0 {
					break nonquoteloop
				}
			}
		}
	}

finishForceTerminate:
	p = end

finish:
	if s.Len() == 0 {
		return "", "", false, nil
	}

finishForceNext:
	return s.String(), in[p:], true, nil
}

func splitStringAppend(appendTo []string, s string, separators string, flags SplitFlags) ([]string, error) {
	orig := appendTo
	for {
		word, remaining, moreWords, err := extractFirstWord(s, separators, flags)
		if err != nil {
			return orig, err
		}

		if !moreWords {
			break
		}
		appendTo = append(appendTo, word)
		s = remaining
	}
	return appendTo, nil
}

func splitString(s string, separators string, flags SplitFlags) ([]string, error) {
	return splitStringAppend(make([]string, 0), s, separators, flags)
}

func charNeedEscape(c rune, isPath bool) bool {
	if c > 128 {
		return false /* unicode is ok */
	}

	pathRune := (isPath && c == '-') ||
		(isPath && c == '/')

	return unicode.IsSpace(c) ||
		unicode.IsControl(c) ||
		pathRune ||
		c == '"' ||
		c == '\'' ||
		c == '\\'
}

func wordNeedEscape(word string) bool {
	for _, c := range word {
		if charNeedEscape(c, false) {
			return true
		}
	}
	return false
}

func appendEscapeWord(escaped *strings.Builder, word string) {
	escaped.WriteRune('"')
	escapeString(escaped, word, false)
	escaped.WriteRune('"')
}

func escapeString(escaped *strings.Builder, word string, isPath bool) {
	for i, c := range word {
		if charNeedEscape(c, isPath) {
			switch c {
			case '\a':
				escaped.WriteString("\\a")
			case '\b':
				escaped.WriteString("\\b")
			case '\n':
				escaped.WriteString("\\n")
			case '\r':
				escaped.WriteString("\\r")
			case '\t':
				escaped.WriteString("\\t")
			case '\v':
				escaped.WriteString("\\v")
			case '\f':
				escaped.WriteString("\\f")
			case '\\':
				escaped.WriteString("\\\\")
			case ' ':
				escaped.WriteString("\\x20")
			case '"':
				escaped.WriteString("\\\"")
			case '\'':
				escaped.WriteString("'")
			case '/':
				if isPath && i != 0 {
					escaped.WriteString("-")
				}
			default:
				fmt.Fprintf(escaped, "\\x%.2x", c)
			}
		} else {
			escaped.WriteRune(c)
		}
	}
}

func escapeWords(words []string) string {
	var escaped strings.Builder

	for i, word := range words {
		if i != 0 {
			escaped.WriteString(" ")
		}
		if wordNeedEscape(word) {
			appendEscapeWord(&escaped, word)
		} else {
			escaped.WriteString(word)
		}
	}
	return escaped.String()
}
