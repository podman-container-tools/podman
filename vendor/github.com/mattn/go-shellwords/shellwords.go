package shellwords

import (
	"errors"
	"os"
	"strings"
	"unicode"
)

var (
	ParseEnv      bool = false
	ParseBacktick bool = false
	ParseComment  bool = false
)

var errInvalidCmdLine = errors.New("invalid command line string")

func isSpace(r rune) bool {
	switch r {
	case ' ', '\t', '\r', '\n':
		return true
	}
	return false
}

func isEnvNameRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

// parseEnvName parses the variable reference that follows a '$' at the start
// of s, either NAME or {NAME}. It returns the name and the number of bytes
// consumed, or ok == false if s does not start with a reference.
func parseEnvName(s string) (name string, n int, ok bool) {
	if strings.HasPrefix(s, "{") {
		end := strings.IndexFunc(s[1:], func(r rune) bool { return !isEnvNameRune(r) })
		if end < 0 || s[1+end] != '}' {
			return "", 0, false
		}
		return s[1 : 1+end], end + 2, true
	}
	end := strings.IndexFunc(s, func(r rune) bool { return !isEnvNameRune(r) })
	if end < 0 {
		end = len(s)
	}
	if end == 0 {
		return "", 0, false
	}
	return s[:end], end, true
}

type Parser struct {
	ParseEnv      bool
	ParseBacktick bool
	ParseComment  bool
	Position      int
	Dir           string
	excludedSep   []rune

	// If ParseEnv is true, use this for getenv.
	// If nil, use os.Getenv.
	Getenv func(string) string
}

func NewParser() *Parser {
	return &Parser{
		ParseEnv:      ParseEnv,
		ParseBacktick: ParseBacktick,
		ParseComment:  ParseComment,
		Position:      0,
		Dir:           "",
		excludedSep:   []rune{},
	}
}

type argType int

const (
	argNo argType = iota
	argSingle
	argQuoted
)

// isExcluded checks if separator should be ignored
func (p *Parser) isExcluded(r rune) bool {
	for _, v := range p.excludedSep {
		if v == r {
			return true
		}
	}
	return false
}

// SetExcludeSeparators indicates the parser to ignore provided separators when parsing
// example: parser.SetExcludeSeparators(';','\t')
func (p *Parser) SetExcludeSeparators(r ...rune) {
	p.excludedSep = r
}

// ExcludedSeparators returns excluded separators
func (p *Parser) ExcludedSeparators() []rune {
	return p.excludedSep
}

func (p *Parser) Parse(line string) ([]string, error) {
	args := make([]string, 0, 1+len(line)/8)
	buf := make([]byte, 0, len(line))
	var escaped, doubleQuoted, singleQuoted, backQuote, dollarQuote, comment bool
	var backtick []byte

	pos := -1
	got := argNo
	// Whether the pending token contains any quoted or escaped character.
	tokenQuoted := false
	// Whether the pending token contains output of a command substitution
	// or an environment variable expansion.
	substituted := false
	// Whether the previous character was an unquoted, unescaped '$'.
	afterDollar := false
	// Byte offset in line up to which characters were already consumed.
	skip := 0

	getenv := p.Getenv
	if getenv == nil {
		getenv = os.Getenv
	}

	flush := func() {
		if got == argQuoted || (got != argNo && len(buf) > 0) {
			args = append(args, string(buf))
		}
		buf = buf[:0]
		got = argNo
		tokenQuoted = false
		substituted = false
	}

	i := -1
loop:
	for idx, r := range line {
		i++

		if idx < skip {
			continue
		}

		prevDollar := afterDollar
		afterDollar = false

		if comment {
			if r == '\n' {
				comment = false
				// Defensive only: a comment can begin just when got == argNo,
				// which is reached solely through flush (which clears this
				// flag), and nothing inside a comment sets it. Kept so the
				// invariant survives if either of those ever changes.
				tokenQuoted = false
			}
			continue
		}

		if escaped {
			escaped = false
			tokenQuoted = true
			if backQuote || dollarQuote {
				buf = append(buf, '\\')
				buf = append(buf, string(r)...)
				backtick = append(backtick, '\\')
				backtick = append(backtick, string(r)...)
				got = argSingle
				continue
			}
			if r == 't' {
				r = '\t'
			}
			if r == 'n' {
				r = '\n'
			}
			buf = append(buf, string(r)...)
			got = argSingle
			continue
		}

		if r == '\\' {
			if singleQuoted {
				buf = append(buf, '\\')
			} else {
				escaped = true
			}
			continue
		}

		if p.isExcluded(r) {
			got = argSingle
			buf = append(buf, string(r)...)
			if backQuote || dollarQuote {
				backtick = append(backtick, string(r)...)
			}
			continue
		}

		if isSpace(r) {
			if singleQuoted || doubleQuoted || backQuote || dollarQuote {
				buf = append(buf, byte(r))
				if backQuote || dollarQuote {
					backtick = append(backtick, byte(r))
				}
			} else {
				flush()
			}
			continue
		}

		switch r {
		case '`':
			if !singleQuoted && !doubleQuoted && !dollarQuote {
				if p.ParseBacktick {
					if backQuote {
						out, err := shellRun(string(backtick), p.Dir)
						if err != nil {
							return nil, err
						}
						buf = append(buf[:len(buf)-len(backtick)], out...)
						substituted = true
					}
					backtick = backtick[:0]
					backQuote = !backQuote
					continue
				}
				backtick = backtick[:0]
				backQuote = !backQuote
			}

		case ')':
			if !singleQuoted && !doubleQuoted && !backQuote {
				if p.ParseBacktick {
					// Security fix:
					// A bare ')' must never open dollarQuote state.
					// Preserve prior behavior by rejecting unmatched ')'
					// when command substitution parsing is enabled.
					if !dollarQuote {
						return nil, errInvalidCmdLine
					}

					out, err := shellRun(string(backtick), p.Dir)
					if err != nil {
						return nil, err
					}

					// Defensive guard: valid $(...) implies the buffer must contain
					// the "$(" prefix plus the collected command body.
					if len(buf) < len(backtick)+2 {
						return nil, errInvalidCmdLine
					}

					buf = append(buf[:len(buf)-len(backtick)-2], out...)
					substituted = true
					backtick = backtick[:0]
					dollarQuote = false
					continue
				}

				// Backtick parsing disabled:
				// A bare ')' is a syntax error, consistent with '(' handling.
				// Only close an already-open $(...) region.
				if !dollarQuote {
					return nil, errInvalidCmdLine
				}

				buf = append(buf, ')')
				backtick = backtick[:0]
				dollarQuote = false
				got = argSingle
				continue
			}

		case '(':
			if !singleQuoted && !doubleQuoted && !backQuote {
				if !dollarQuote && prevDollar {
					dollarQuote = true
					buf = append(buf, '(')
					continue
				} else {
					return nil, errInvalidCmdLine
				}
			}

		case '"':
			if !singleQuoted && !dollarQuote && !backQuote {
				if doubleQuoted {
					got = argQuoted
				}
				doubleQuoted = !doubleQuoted
				tokenQuoted = true
				continue
			}

		case '\'':
			if !doubleQuoted && !dollarQuote && !backQuote {
				if singleQuoted {
					got = argQuoted
				}
				singleQuoted = !singleQuoted
				tokenQuoted = true
				continue
			}

		case ';', '&', '|', '<', '>':
			if !(escaped || singleQuoted || doubleQuoted || backQuote || dollarQuote) {
				// A file descriptor number is only a redirect prefix while it
				// is unquoted; quoting makes it an ordinary argument. Output of
				// a command substitution is never one either, and its length
				// does not match the source text.
				if r == '>' && len(buf) > 0 && !tokenQuoted && !substituted {
					isDigits := true
					for _, c := range buf {
						if c < '0' || c > '9' {
							isDigits = false
							break
						}
					}
					if isDigits {
						i -= len(buf)
						got = argNo
					}
				}
				pos = i
				break loop
			}
		case '$':
			if p.ParseEnv && !singleQuoted && !backQuote && !dollarQuote {
				name, n, ok := parseEnvName(line[idx+1:])
				if !ok {
					break
				}
				skip = idx + 1 + n
				value := getenv(name)
				if doubleQuoted {
					buf = append(buf, value...)
				} else {
					// Split the value into fields, but never interpret its
					// contents as shell syntax.
					for _, c := range value {
						if isSpace(c) && !p.isExcluded(c) {
							flush()
							continue
						}
						buf = append(buf, string(c)...)
						got = argSingle
					}
				}
				substituted = true
				continue
			}

		case '#':
			if p.ParseComment && got == argNo && !substituted && !(escaped || singleQuoted || doubleQuoted || backQuote || dollarQuote) {
				comment = true
				continue loop
			}
		}

		got = argSingle
		buf = append(buf, string(r)...)
		if backQuote || dollarQuote {
			backtick = append(backtick, string(r)...)
		}
		afterDollar = r == '$' && !(singleQuoted || doubleQuoted || backQuote || dollarQuote)
	}

	flush()

	if escaped || singleQuoted || doubleQuoted || backQuote || dollarQuote {
		return nil, errInvalidCmdLine
	}

	p.Position = pos

	return args, nil
}

func (p *Parser) ParseWithEnvs(line string) (envs []string, args []string, err error) {
	_args, err := p.Parse(line)
	if err != nil {
		return nil, nil, err
	}
	envs = []string{}
	args = []string{}
	parsingEnv := true
	for _, arg := range _args {
		if parsingEnv && isEnv(arg) {
			envs = append(envs, arg)
		} else {
			if parsingEnv {
				parsingEnv = false
			}
			args = append(args, arg)
		}
	}
	return envs, args, nil
}

func isEnv(arg string) bool {
	i := strings.IndexByte(arg, '=')
	if i <= 0 {
		return false
	}
	for j := 0; j < i; j++ {
		c := arg[j]
		if c == '_' || ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z') || (j > 0 && '0' <= c && c <= '9') {
			continue
		}
		return false
	}
	return true
}

func Parse(line string) ([]string, error) {
	return NewParser().Parse(line)
}

func ParseWithEnvs(line string) (envs []string, args []string, err error) {
	return NewParser().ParseWithEnvs(line)
}
