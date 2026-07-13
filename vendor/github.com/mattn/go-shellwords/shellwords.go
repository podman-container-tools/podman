package shellwords

import (
	"bytes"
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

func replaceEnv(getenv func(string) string, s string) string {
	if getenv == nil {
		getenv = os.Getenv
	}

	var buf bytes.Buffer
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		if r == '\\' {
			i++
			if i == len(rs) {
				break
			}
			buf.WriteRune(rs[i])
			continue
		} else if r == '$' {
			i++
			if i == len(rs) {
				buf.WriteRune(r)
				break
			}
			if rs[i] == 0x7b {
				i++
				p := i
				for ; i < len(rs); i++ {
					r = rs[i]
					if r == '\\' {
						i++
						if i == len(rs) {
							return s
						}
						continue
					}
					if r == 0x7d || (!unicode.IsLetter(r) && r != '_' && !unicode.IsDigit(r)) {
						break
					}
				}
				if r != 0x7d {
					return s
				}
				if i > p {
					buf.WriteString(getenv(string(rs[p:i])))
				}
			} else {
				p := i
				for ; i < len(rs); i++ {
					r := rs[i]
					if r == '\\' {
						i++
						if i == len(rs) {
							return s
						}
						continue
					}
					if !unicode.IsLetter(r) && r != '_' && !unicode.IsDigit(r) {
						break
					}
				}
				if i > p {
					buf.WriteString(getenv(string(rs[p:i])))
					i--
				} else {
					buf.WriteRune('$')
					i--
				}
			}
		} else {
			buf.WriteRune(r)
		}
	}
	return buf.String()
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

	flush := func() error {
		if got == argQuoted || (got != argNo && len(buf) > 0) {
			token := string(buf)
			if p.ParseEnv {
				if got == argSingle {
					parser := &Parser{ParseEnv: false, ParseBacktick: false, Position: 0, Dir: p.Dir}
					strs, err := parser.Parse(replaceEnv(p.Getenv, token))
					if err != nil {
						return err
					}
					args = append(args, strs...)
				} else {
					args = append(args, replaceEnv(p.Getenv, token))
				}
			} else {
				args = append(args, token)
			}
		}
		buf = buf[:0]
		got = argNo
		return nil
	}

	i := -1
loop:
	for _, r := range line {
		i++

		if comment {
			if r == '\n' {
				comment = false
			}
			continue
		}

		if escaped {
			escaped = false
			if backQuote || dollarQuote {
				buf = append(buf, '\\')
				buf = append(buf, string(r)...)
				backtick = append(backtick, '\\')
				backtick = append(backtick, string(r)...)
				got = argSingle
				continue
			}
			if p.ParseEnv && r == '$' {
				// Keep the backslash so replaceEnv treats the '$' as
				// literal instead of expanding it.
				buf = append(buf, '\\', '$')
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
			} else if err := flush(); err != nil {
				return nil, err
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
				if n := len(buf); !dollarQuote && n > 0 && buf[n-1] == '$' && !(n > 1 && buf[n-2] == '\\') {
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
				continue
			}

		case '\'':
			if !doubleQuoted && !dollarQuote && !backQuote {
				if singleQuoted {
					got = argQuoted
				}
				singleQuoted = !singleQuoted
				continue
			}

		case ';', '&', '|', '<', '>':
			if !(escaped || singleQuoted || doubleQuoted || backQuote || dollarQuote) {
				if r == '>' && len(buf) > 0 {
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
		case '#':
			if p.ParseComment && len(buf) == 0 && !(escaped || singleQuoted || doubleQuoted || backQuote || dollarQuote) {
				comment = true
				continue loop
			}
		}

		got = argSingle
		buf = append(buf, string(r)...)
		if backQuote || dollarQuote {
			backtick = append(backtick, string(r)...)
		}
	}

	if err := flush(); err != nil {
		return nil, err
	}

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
