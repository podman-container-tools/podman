//go:build windows && !appengine
// +build windows,!appengine

package colorable

import (
	"bytes"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	stdsyscall "syscall"
	"unsafe"

	syscall "golang.org/x/sys/windows"
)

const (
	foregroundBlue      = 0x1
	foregroundGreen     = 0x2
	foregroundRed       = 0x4
	foregroundIntensity = 0x8
	foregroundMask      = (foregroundRed | foregroundBlue | foregroundGreen | foregroundIntensity)
	backgroundBlue      = 0x10
	backgroundGreen     = 0x20
	backgroundRed       = 0x40
	backgroundIntensity = 0x80
	backgroundMask      = (backgroundRed | backgroundBlue | backgroundGreen | backgroundIntensity)
	commonLvbUnderscore = 0x8000

	cENABLE_VIRTUAL_TERMINAL_PROCESSING = 0x4
)

const (
	genericRead  = 0x80000000
	genericWrite = 0x40000000
)

const (
	consoleTextmodeBuffer = 0x1
)

type wchar uint16
type short int16
type dword uint32
type word uint16

type coord struct {
	x short
	y short
}

// param packs the two signed 16-bit coordinates into the DWORD passed by value
// to console APIs, without reading past the four-byte coord on 64-bit Windows.
func (c coord) param() uintptr {
	return uintptr(uint16(c.x)) | uintptr(uint16(c.y))<<16
}

type smallRect struct {
	left   short
	top    short
	right  short
	bottom short
}

type consoleScreenBufferInfo struct {
	size              coord
	cursorPosition    coord
	attributes        word
	window            smallRect
	maximumWindowSize coord
}

type consoleCursorInfo struct {
	size    dword
	visible int32
}

var (
	kernel32                       = syscall.NewLazySystemDLL("kernel32.dll")
	procGetConsoleScreenBufferInfo = kernel32.NewProc("GetConsoleScreenBufferInfo")
	procSetConsoleTextAttribute    = kernel32.NewProc("SetConsoleTextAttribute")
	procSetConsoleCursorPosition   = kernel32.NewProc("SetConsoleCursorPosition")
	procFillConsoleOutputCharacter = kernel32.NewProc("FillConsoleOutputCharacterW")
	procFillConsoleOutputAttribute = kernel32.NewProc("FillConsoleOutputAttribute")
	procGetConsoleCursorInfo       = kernel32.NewProc("GetConsoleCursorInfo")
	procSetConsoleCursorInfo       = kernel32.NewProc("SetConsoleCursorInfo")
	procSetConsoleTitle            = kernel32.NewProc("SetConsoleTitleW")
	procGetConsoleMode             = kernel32.NewProc("GetConsoleMode")
	procSetConsoleMode             = kernel32.NewProc("SetConsoleMode")
	procCreateConsoleScreenBuffer  = kernel32.NewProc("CreateConsoleScreenBuffer")
)

// writer provides colorable Writer to the console
type writer struct {
	out       io.Writer
	handle    syscall.Handle
	althandle syscall.Handle
	oldattr   word
	curattr   word
	oldpos    coord
	rest      bytes.Buffer
	mutex     sync.Mutex
}

// NewColorable returns new instance of writer which handles escape sequence from File.
func NewColorable(file *os.File) io.Writer {
	if file == nil {
		panic("nil passed instead of *os.File to NewColorable()")
	}

	fd := file.Fd()
	var mode uint32
	if err := syscall.GetConsoleMode(syscall.Handle(fd), &mode); err == nil {
		if mode&cENABLE_VIRTUAL_TERMINAL_PROCESSING != 0 {
			return file
		}
		var csbi consoleScreenBufferInfo
		handle := syscall.Handle(fd)
		stdsyscall.SyscallN(procGetConsoleScreenBufferInfo.Addr(), uintptr(handle), uintptr(unsafe.Pointer(&csbi)))
		return &writer{out: file, handle: handle, oldattr: csbi.attributes, curattr: csbi.attributes, oldpos: coord{0, 0}}
	}
	return file
}

// NewColorableStdout returns new instance of writer which handles escape sequence for stdout.
func NewColorableStdout() io.Writer {
	return NewColorable(os.Stdout)
}

// NewColorableStderr returns new instance of writer which handles escape sequence for stderr.
func NewColorableStderr() io.Writer {
	return NewColorable(os.Stderr)
}

// `\033]0;TITLESTR\007`
func doTitleSequence(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, io.EOF
	}
	if data[0] != '0' && data[0] != '2' {
		return 1, nil
	}
	if len(data) < 2 {
		return 1, io.EOF
	}
	if data[1] != ';' {
		return 2, nil
	}
	end := bytes.IndexAny(data[2:], "\a\n")
	if end < 0 {
		return len(data), io.EOF
	}
	if end > 0 {
		title, err := syscall.UTF16PtrFromString(string(data[2 : 2+end]))
		if err == nil {
			stdsyscall.SyscallN(procSetConsoleTitle.Addr(), uintptr(unsafe.Pointer(title)))
		}
	}
	return end + 3, nil
}

// returns Atoi(s) unless s == "" in which case it returns def
func atoiWithDefault(s string, def int) (int, error) {
	if s == "" {
		return def, nil
	}
	return strconv.Atoi(s)
}

// splitParameters uses caller-provided storage for the usual short ANSI
// parameter lists, while still accepting arbitrarily long lists.
func splitParameters(s string, storage []string) []string {
	n := strings.Count(s, ";") + 1
	if n > cap(storage) {
		storage = make([]string, n)
	} else {
		storage = storage[:n]
	}
	for i := 0; i < n-1; i++ {
		end := strings.IndexByte(s, ';')
		storage[i] = s[:end]
		s = s[end+1:]
	}
	storage[n-1] = s
	return storage
}

// Write writes data on console
func (w *writer) Write(data []byte) (n int, err error) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	if w.rest.Len() == 0 && bytes.IndexByte(data, 0x1b) == -1 {
		w.out.Write(data)
		return len(data), nil
	}

	var csbi consoleScreenBufferInfo
	handle := w.handle

	input := data
	if w.rest.Len() > 0 {
		w.rest.Write(data)
		input = w.rest.Bytes()
		w.rest.Reset()
	}
	er := bytes.NewReader(input)
loop:
	for er.Len() > 0 {
		remaining := input[len(input)-er.Len():]
		escape := bytes.IndexByte(remaining, 0x1b)
		if escape < 0 {
			w.out.Write(remaining)
			break loop
		}
		if escape > 0 {
			if n, err := w.out.Write(remaining[:escape]); err != nil || n != escape {
				break loop
			}
		}
		er.Seek(int64(escape+1), io.SeekCurrent)
		c2, err := er.ReadByte()
		if err != nil {
			break loop
		}

		switch c2 {
		case '>':
			continue
		case ']':
			if bytes.IndexByte(remaining[escape+2:], 0x07) == -1 {
				w.rest.Write(remaining[escape:])
				break loop
			}
			consumed, err := doTitleSequence(remaining[escape+2:])
			er.Seek(int64(consumed), io.SeekCurrent)
			if err != nil {
				w.rest.Write(remaining[escape:])
				break loop
			}
			continue
		// https://github.com/mattn/go-colorable/issues/27
		case '7':
			stdsyscall.SyscallN(procGetConsoleScreenBufferInfo.Addr(), uintptr(handle), uintptr(unsafe.Pointer(&csbi)))
			w.oldpos = csbi.cursorPosition
			continue
		case '8':
			stdsyscall.SyscallN(procSetConsoleCursorPosition.Addr(), uintptr(handle), w.oldpos.param())
			continue
		case 0x5b:
			// execute part after switch
		default:
			continue
		}

		// Keep complete sequences in the input slice. Only an unfinished
		// sequence needs to be copied and retained for the next Write.
		params := remaining[escape+2:]
		var buf []byte
		var m byte
		for i, c := range params {
			if ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z') || c == '@' {
				m = c
				buf = params[:i]
				er.Seek(int64(i+1), io.SeekCurrent)
				break
			}
		}
		if m == 0 {
			w.rest.Write(remaining[escape:])
			break loop
		}

		switch m {
		case 'A':
			n, err = atoiWithDefault(string(buf), 1)
			if err != nil {
				continue
			}
			stdsyscall.SyscallN(procGetConsoleScreenBufferInfo.Addr(), uintptr(handle), uintptr(unsafe.Pointer(&csbi)))
			csbi.cursorPosition.y -= short(n)
			stdsyscall.SyscallN(procSetConsoleCursorPosition.Addr(), uintptr(handle), csbi.cursorPosition.param())
		case 'B':
			n, err = atoiWithDefault(string(buf), 1)
			if err != nil {
				continue
			}
			stdsyscall.SyscallN(procGetConsoleScreenBufferInfo.Addr(), uintptr(handle), uintptr(unsafe.Pointer(&csbi)))
			csbi.cursorPosition.y += short(n)
			stdsyscall.SyscallN(procSetConsoleCursorPosition.Addr(), uintptr(handle), csbi.cursorPosition.param())
		case 'C':
			n, err = atoiWithDefault(string(buf), 1)
			if err != nil {
				continue
			}
			stdsyscall.SyscallN(procGetConsoleScreenBufferInfo.Addr(), uintptr(handle), uintptr(unsafe.Pointer(&csbi)))
			csbi.cursorPosition.x += short(n)
			stdsyscall.SyscallN(procSetConsoleCursorPosition.Addr(), uintptr(handle), csbi.cursorPosition.param())
		case 'D':
			n, err = atoiWithDefault(string(buf), 1)
			if err != nil {
				continue
			}
			stdsyscall.SyscallN(procGetConsoleScreenBufferInfo.Addr(), uintptr(handle), uintptr(unsafe.Pointer(&csbi)))
			csbi.cursorPosition.x -= short(n)
			if csbi.cursorPosition.x < 0 {
				csbi.cursorPosition.x = 0
			}
			stdsyscall.SyscallN(procSetConsoleCursorPosition.Addr(), uintptr(handle), csbi.cursorPosition.param())
		case 'E':
			n, err = atoiWithDefault(string(buf), 1)
			if err != nil {
				continue
			}
			stdsyscall.SyscallN(procGetConsoleScreenBufferInfo.Addr(), uintptr(handle), uintptr(unsafe.Pointer(&csbi)))
			csbi.cursorPosition.x = 0
			csbi.cursorPosition.y += short(n)
			stdsyscall.SyscallN(procSetConsoleCursorPosition.Addr(), uintptr(handle), csbi.cursorPosition.param())
		case 'F':
			n, err = atoiWithDefault(string(buf), 1)
			if err != nil {
				continue
			}
			stdsyscall.SyscallN(procGetConsoleScreenBufferInfo.Addr(), uintptr(handle), uintptr(unsafe.Pointer(&csbi)))
			csbi.cursorPosition.x = 0
			csbi.cursorPosition.y -= short(n)
			stdsyscall.SyscallN(procSetConsoleCursorPosition.Addr(), uintptr(handle), csbi.cursorPosition.param())
		case 'G':
			n, err = strconv.Atoi(string(buf))
			if err != nil {
				continue
			}
			if n < 1 {
				n = 1
			}
			stdsyscall.SyscallN(procGetConsoleScreenBufferInfo.Addr(), uintptr(handle), uintptr(unsafe.Pointer(&csbi)))
			csbi.cursorPosition.x = short(n - 1)
			stdsyscall.SyscallN(procSetConsoleCursorPosition.Addr(), uintptr(handle), csbi.cursorPosition.param())
		case 'H', 'f':
			if len(buf) > 0 {
				var storage [8]string
				token := splitParameters(string(buf), storage[:])
				switch len(token) {
				case 1:
					n1, err := strconv.Atoi(token[0])
					if err != nil {
						continue
					}
					stdsyscall.SyscallN(procGetConsoleScreenBufferInfo.Addr(), uintptr(handle), uintptr(unsafe.Pointer(&csbi)))
					csbi.cursorPosition.y = short(n1 - 1)
				case 2:
					// Both coordinates are absolute; no console query is needed.
					n1, err := strconv.Atoi(token[0])
					if err != nil {
						continue
					}
					n2, err := strconv.Atoi(token[1])
					if err != nil {
						continue
					}
					csbi.cursorPosition.x = short(n2 - 1)
					csbi.cursorPosition.y = short(n1 - 1)
				default:
					stdsyscall.SyscallN(procGetConsoleScreenBufferInfo.Addr(), uintptr(handle), uintptr(unsafe.Pointer(&csbi)))
				}
			} else {
				stdsyscall.SyscallN(procGetConsoleScreenBufferInfo.Addr(), uintptr(handle), uintptr(unsafe.Pointer(&csbi)))
				csbi.cursorPosition.y = 0
			}
			stdsyscall.SyscallN(procSetConsoleCursorPosition.Addr(), uintptr(handle), csbi.cursorPosition.param())
		case 'J':
			n := 0
			if len(buf) > 0 {
				n, err = strconv.Atoi(string(buf))
				if err != nil {
					continue
				}
			}
			var count, written dword
			var cursor coord
			stdsyscall.SyscallN(procGetConsoleScreenBufferInfo.Addr(), uintptr(handle), uintptr(unsafe.Pointer(&csbi)))
			switch n {
			case 0:
				cursor = coord{x: csbi.cursorPosition.x, y: csbi.cursorPosition.y}
				count = dword(csbi.size.x) - dword(csbi.cursorPosition.x) + dword(csbi.size.y-csbi.cursorPosition.y)*dword(csbi.size.x)
			case 1:
				cursor = coord{x: csbi.window.left, y: csbi.window.top}
				count = dword(csbi.size.x) - dword(csbi.cursorPosition.x) + dword(csbi.window.top-csbi.cursorPosition.y)*dword(csbi.size.x)
			case 2:
				cursor = coord{x: csbi.window.left, y: csbi.window.top}
				count = dword(csbi.size.x) - dword(csbi.cursorPosition.x) + dword(csbi.size.y-csbi.cursorPosition.y)*dword(csbi.size.x)
			}
			stdsyscall.SyscallN(procFillConsoleOutputCharacter.Addr(), uintptr(handle), uintptr(' '), uintptr(count), cursor.param(), uintptr(unsafe.Pointer(&written)))
			stdsyscall.SyscallN(procFillConsoleOutputAttribute.Addr(), uintptr(handle), uintptr(csbi.attributes), uintptr(count), cursor.param(), uintptr(unsafe.Pointer(&written)))
		case 'K':
			n := 0
			if len(buf) > 0 {
				n, err = strconv.Atoi(string(buf))
				if err != nil {
					continue
				}
			}
			stdsyscall.SyscallN(procGetConsoleScreenBufferInfo.Addr(), uintptr(handle), uintptr(unsafe.Pointer(&csbi)))
			var cursor coord
			var count, written dword
			switch n {
			case 0:
				cursor = coord{x: csbi.cursorPosition.x, y: csbi.cursorPosition.y}
				count = dword(csbi.size.x - csbi.cursorPosition.x)
			case 1:
				cursor = coord{x: csbi.window.left, y: csbi.cursorPosition.y}
				count = dword(csbi.size.x - csbi.cursorPosition.x)
			case 2:
				cursor = coord{x: csbi.window.left, y: csbi.cursorPosition.y}
				count = dword(csbi.size.x)
			}
			stdsyscall.SyscallN(procFillConsoleOutputCharacter.Addr(), uintptr(handle), uintptr(' '), uintptr(count), cursor.param(), uintptr(unsafe.Pointer(&written)))
			stdsyscall.SyscallN(procFillConsoleOutputAttribute.Addr(), uintptr(handle), uintptr(csbi.attributes), uintptr(count), cursor.param(), uintptr(unsafe.Pointer(&written)))
		case 'X':
			n := 0
			if len(buf) > 0 {
				n, err = strconv.Atoi(string(buf))
				if err != nil {
					continue
				}
			}
			stdsyscall.SyscallN(procGetConsoleScreenBufferInfo.Addr(), uintptr(handle), uintptr(unsafe.Pointer(&csbi)))
			var cursor coord
			var written dword
			cursor = coord{x: csbi.cursorPosition.x, y: csbi.cursorPosition.y}
			stdsyscall.SyscallN(procFillConsoleOutputCharacter.Addr(), uintptr(handle), uintptr(' '), uintptr(n), cursor.param(), uintptr(unsafe.Pointer(&written)))
			stdsyscall.SyscallN(procFillConsoleOutputAttribute.Addr(), uintptr(handle), uintptr(csbi.attributes), uintptr(n), cursor.param(), uintptr(unsafe.Pointer(&written)))
		case 'm':
			attr := w.curattr
			cs := string(buf)
			if cs == "" {
				if w.curattr != w.oldattr {
					stdsyscall.SyscallN(procSetConsoleTextAttribute.Addr(), uintptr(handle), uintptr(w.oldattr))
					w.curattr = w.oldattr
				}
				continue
			}
			var storage [8]string
			token := splitParameters(cs, storage[:])
			for i := 0; i < len(token); i++ {
				ns := token[i]
				if n, err = strconv.Atoi(ns); err == nil {
					switch {
					case n == 0 || n == 100:
						attr = w.oldattr
					case n == 4:
						attr |= commonLvbUnderscore
					case (1 <= n && n <= 3) || n == 5:
						attr |= foregroundIntensity
					case n == 7 || n == 27:
						attr =
							(attr &^ (foregroundMask | backgroundMask)) |
								((attr & foregroundMask) << 4) |
								((attr & backgroundMask) >> 4)
					case n == 22:
						attr &^= foregroundIntensity
					case n == 24:
						attr &^= commonLvbUnderscore
					case 30 <= n && n <= 37:
						attr &= backgroundMask
						if (n-30)&1 != 0 {
							attr |= foregroundRed
						}
						if (n-30)&2 != 0 {
							attr |= foregroundGreen
						}
						if (n-30)&4 != 0 {
							attr |= foregroundBlue
						}
					case n == 38: // set foreground color.
						if i < len(token)-2 && (token[i+1] == "5" || token[i+1] == "05") {
							if n256, err := strconv.Atoi(token[i+2]); err == nil {

								attr &= backgroundMask
								attr |= n256foreAttr[n256%len(n256foreAttr)]
								i += 2
							}
						} else if len(token) == 5 && token[i+1] == "2" {
							var r, g, b int
							r, _ = strconv.Atoi(token[i+2])
							g, _ = strconv.Atoi(token[i+3])
							b, _ = strconv.Atoi(token[i+4])
							i += 4
							if r > 127 {
								attr |= foregroundRed
							}
							if g > 127 {
								attr |= foregroundGreen
							}
							if b > 127 {
								attr |= foregroundBlue
							}
						} else {
							attr = attr & (w.oldattr & backgroundMask)
						}
					case n == 39: // reset foreground color.
						attr &= backgroundMask
						attr |= w.oldattr & foregroundMask
					case 40 <= n && n <= 47:
						attr &= foregroundMask
						if (n-40)&1 != 0 {
							attr |= backgroundRed
						}
						if (n-40)&2 != 0 {
							attr |= backgroundGreen
						}
						if (n-40)&4 != 0 {
							attr |= backgroundBlue
						}
					case n == 48: // set background color.
						if i < len(token)-2 && token[i+1] == "5" {
							if n256, err := strconv.Atoi(token[i+2]); err == nil {

								attr &= foregroundMask
								attr |= (n256foreAttr[n256%len(n256foreAttr)] << 4)
								i += 2
							}
						} else if len(token) == 5 && token[i+1] == "2" {
							var r, g, b int
							r, _ = strconv.Atoi(token[i+2])
							g, _ = strconv.Atoi(token[i+3])
							b, _ = strconv.Atoi(token[i+4])
							i += 4
							if r > 127 {
								attr |= backgroundRed
							}
							if g > 127 {
								attr |= backgroundGreen
							}
							if b > 127 {
								attr |= backgroundBlue
							}
						} else {
							attr = attr & (w.oldattr & foregroundMask)
						}
					case n == 49: // reset foreground color.
						attr &= foregroundMask
						attr |= w.oldattr & backgroundMask
					case 90 <= n && n <= 97:
						attr = (attr & backgroundMask)
						attr |= foregroundIntensity
						if (n-90)&1 != 0 {
							attr |= foregroundRed
						}
						if (n-90)&2 != 0 {
							attr |= foregroundGreen
						}
						if (n-90)&4 != 0 {
							attr |= foregroundBlue
						}
					case 100 <= n && n <= 107:
						attr = (attr & foregroundMask)
						attr |= backgroundIntensity
						if (n-100)&1 != 0 {
							attr |= backgroundRed
						}
						if (n-100)&2 != 0 {
							attr |= backgroundGreen
						}
						if (n-100)&4 != 0 {
							attr |= backgroundBlue
						}
					}
				}
			}
			if attr != w.curattr {
				stdsyscall.SyscallN(procSetConsoleTextAttribute.Addr(), uintptr(handle), uintptr(attr))
				w.curattr = attr
			}
		case 'h':
			var ci consoleCursorInfo
			cs := string(buf)
			if cs == "5>" {
				stdsyscall.SyscallN(procGetConsoleCursorInfo.Addr(), uintptr(handle), uintptr(unsafe.Pointer(&ci)))
				ci.visible = 0
				stdsyscall.SyscallN(procSetConsoleCursorInfo.Addr(), uintptr(handle), uintptr(unsafe.Pointer(&ci)))
			} else if cs == "?25" {
				stdsyscall.SyscallN(procGetConsoleCursorInfo.Addr(), uintptr(handle), uintptr(unsafe.Pointer(&ci)))
				ci.visible = 1
				stdsyscall.SyscallN(procSetConsoleCursorInfo.Addr(), uintptr(handle), uintptr(unsafe.Pointer(&ci)))
			} else if cs == "?1049" {
				if w.althandle == 0 {
					h, _, _ := stdsyscall.SyscallN(procCreateConsoleScreenBuffer.Addr(), uintptr(genericRead|genericWrite), 0, 0, uintptr(consoleTextmodeBuffer), 0, 0)
					w.althandle = syscall.Handle(h)
					if w.althandle != 0 {
						handle = w.althandle
						stdsyscall.SyscallN(procGetConsoleScreenBufferInfo.Addr(), uintptr(handle), uintptr(unsafe.Pointer(&csbi)))
						w.curattr = csbi.attributes
					}
				}
			}
		case 'l':
			var ci consoleCursorInfo
			cs := string(buf)
			if cs == "5>" {
				stdsyscall.SyscallN(procGetConsoleCursorInfo.Addr(), uintptr(handle), uintptr(unsafe.Pointer(&ci)))
				ci.visible = 1
				stdsyscall.SyscallN(procSetConsoleCursorInfo.Addr(), uintptr(handle), uintptr(unsafe.Pointer(&ci)))
			} else if cs == "?25" {
				stdsyscall.SyscallN(procGetConsoleCursorInfo.Addr(), uintptr(handle), uintptr(unsafe.Pointer(&ci)))
				ci.visible = 0
				stdsyscall.SyscallN(procSetConsoleCursorInfo.Addr(), uintptr(handle), uintptr(unsafe.Pointer(&ci)))
			} else if cs == "?1049" {
				if w.althandle != 0 {
					syscall.CloseHandle(w.althandle)
					w.althandle = 0
					handle = w.handle
					stdsyscall.SyscallN(procGetConsoleScreenBufferInfo.Addr(), uintptr(handle), uintptr(unsafe.Pointer(&csbi)))
					w.curattr = csbi.attributes
				}
			}
		case 's':
			stdsyscall.SyscallN(procGetConsoleScreenBufferInfo.Addr(), uintptr(handle), uintptr(unsafe.Pointer(&csbi)))
			w.oldpos = csbi.cursorPosition
		case 'u':
			stdsyscall.SyscallN(procSetConsoleCursorPosition.Addr(), uintptr(handle), w.oldpos.param())
		}
	}

	return len(data), nil
}

// EnableColorsStdout enable colors if possible.
func EnableColorsStdout(enabled *bool) func() {
	var mode uint32
	h := os.Stdout.Fd()
	if r, _, _ := stdsyscall.SyscallN(procGetConsoleMode.Addr(), h, uintptr(unsafe.Pointer(&mode))); r != 0 {
		if r, _, _ = stdsyscall.SyscallN(procSetConsoleMode.Addr(), h, uintptr(mode|cENABLE_VIRTUAL_TERMINAL_PROCESSING)); r != 0 {
			if enabled != nil {
				*enabled = true
			}
			return func() {
				stdsyscall.SyscallN(procSetConsoleMode.Addr(), h, uintptr(mode))
			}
		}
	}
	if enabled != nil {
		*enabled = true
	}
	return func() {}
}
