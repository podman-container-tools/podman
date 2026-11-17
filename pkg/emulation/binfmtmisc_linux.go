//go:build !remote

package emulation

import (
	"bufio"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// registeredBinfmtMisc walks /proc/sys/fs/binfmt_misc and iterates through a
// list of known ELF header values to see if there's an emulator registered for
// them.  Returns the list of emulated targets (which may be empty), or an
// error if something unexpected happened.
func registeredBinfmtMisc() ([]string, error) {
	var registered []string
	globalEnabled := false
	err := filepath.WalkDir("/proc/sys/fs/binfmt_misc", func(path string, d fs.DirEntry, err error) error {
		if filepath.Base(path) == "register" { // skip this one
			return nil
		}
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil // skip the directory itself
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		if filepath.Base(path) == "status" {
			b, err := io.ReadAll(f)
			if err != nil {
				return err
			}
			status := strings.TrimSpace(string(b))
			switch status {
			case "disabled":
				globalEnabled = false
			case "enabled":
				globalEnabled = true
			default:
				return fmt.Errorf("unrecognized binfmt_misc status value %q in %q", status, path)
			}
			return nil
		}
		offset, magic, mask, err := parseBinfmtMisc(path, f)
		if err != nil {
			return err
		}
		if offset < 0 {
			return nil
		}
		for platform, headers := range getKnownELFPlatformHeaders() {
			for _, header := range headers {
				if magicMatch(header, offset, mask, magic) {
					registered = append(registered, platform)
					break
				}
			}
		}
		return nil
	})
	if !globalEnabled {
		return nil, nil
	}
	sort.Strings(registered)
	return registered, err
}

// magicMatch compares header, starting at the specified offset, masked with
// mask, against the magic value
func magicMatch(header []byte, offset int, mask, magic []byte) bool {
	mismatch := 0
	for i := offset; i < offset+len(magic); i++ {
		if i >= len(header) {
			break
		}
		m := magic[i-offset]
		if len(mask) > i-offset {
			m &= mask[i-offset]
		}
		if header[i] != m {
			// mismatch
			break
		}
		mismatch = i + 1
	}
	return mismatch >= offset+len(magic)
}

// parseBinfmtMisc parses a binfmt_misc registry entry.  It returns the offset,
// magic, and mask values, or an error if there was an error parsing the data.
// If the returned offset is negative, the entry was disabled or should be
// non-fatally ignored for some other reason.
func parseBinfmtMisc(path string, r io.Reader) (int, []byte, []byte, error) {
	offset := 0
	magicString, maskString := "", ""
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		text := scanner.Text()
		if strings.TrimSpace(text) == "" {
			continue
		}
		fields := strings.Fields(text)
		switch fields[0] {
		case "disabled":
			return -1, nil, nil, nil // we should ignore this specific one
		case "enabled": // keep scanning this entry
		case "interpreter": // good, but not something we need to record
		case "offset":
			if len(fields) != 2 {
				return -1, nil, nil, fmt.Errorf("invalid format for %q in %q", text, path)
			}
			offset64, err := strconv.ParseInt(fields[1], 10, 8)
			if err != nil {
				return -1, nil, nil, fmt.Errorf("invalid offset %q in %q", fields[1], path)
			}
			offset = int(offset64)
		case "magic":
			if len(fields) != 2 {
				return -1, nil, nil, fmt.Errorf("invalid format for %q in %q", text, path)
			}
			magicString = fields[1]
		case "mask":
			if len(fields) != 2 {
				return -1, nil, nil, fmt.Errorf("invalid format for %q in %q", text, path)
			}
			maskString = fields[1]
		case "flags", "flags:":
			if len(fields) != 2 {
				continue
			}
			if !strings.Contains(fields[1], "F") { // won't work in other mount namespaces, so ignore it
				return -1, nil, nil, nil
			}
		default:
			return -1, nil, nil, fmt.Errorf("unrecognized field %q in %q", fields[0], path)
		}
		continue
	}
	if err := scanner.Err(); err != nil {
		return -1, nil, nil, err
	}
	if magicString == "" || maskString == "" { // entry is missing some info we need here
		return -1, nil, nil, nil
	}
	magic, err := hex.DecodeString(magicString)
	if err != nil {
		return -1, nil, nil, fmt.Errorf("invalid magic %q in %q", magicString, path)
	}
	mask, err := hex.DecodeString(maskString)
	if err != nil {
		return -1, nil, nil, fmt.Errorf("invalid mask %q in %q", maskString, path)
	}
	return offset, magic, mask, nil
}
