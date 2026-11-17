//go:build !remote

package emulation

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// parseBinfmtMisc parses a binfmt_misc registry entry.  It returns the offset,
// magic, and mask values, or an error if there was an error parsing the data.
// If the returned offset is negative, the entry was disabled or should be
// non-fatally ignored for some other reason.
func TestParseBinfmtMisc(t *testing.T) {
	vectors := []struct {
		platform, contents string
	}{
		{
			"linux/386",
			`
			enabled
			interpreter /usr/bin/qemu-i386-static
			flags: F
			offset 0
			magic 7f454c4601010100000000000000000002000300
			mask  fffffffffffefe00fffffffffffffffffeffffff
			`,
		},
		{
			"linux/amd64",
			`
			enabled
			interpreter /usr/bin/qemu-x86_64-static
			flags: F
			offset 0
			magic 7f454c4602010100000000000000000002003e00
			mask  fffffffffffefe00fffffffffffffffffeffffff
			`,
		},
		{
			"linux/arm",
			`
			enabled
			interpreter /usr/bin/qemu-arm-static
			flags: F
			offset 0
			magic 7f454c4601010100000000000000000002002800
			mask  ffffffffffffff00fffffffffffffffffeffffff
			`,
		},
		{
			"linux/arm64",
			`
			enabled
			interpreter /usr/bin/qemu-aarch64-static
			flags: F
			offset 0
			magic 7f454c460201010000000000000000000200b700
			mask  ffffffffffffff00fffffffffffffffffeffffff
			`,
		},
		{
			"linux/ppc64le",
			`
			enabled
			interpreter /usr/bin/qemu-ppc64le-static
			flags: F
			offset 0
			magic 7f454c4602010100000000000000000002001500
			mask  ffffffffffffff00fffffffffffffffffeffff00
			`,
		},
		{
			"linux/s390x",
			`
			enabled
			interpreter /usr/bin/qemu-s390x-static
			flags: F
			offset 0
			magic 7f454c4602020100000000000000000000020016
			mask  ffffffffffffff00fffffffffffffffffffeffff
			`,
		},
	}
	for i := range vectors {
		v := vectors[i]
		t.Run(v.platform, func(t *testing.T) {
			offset, magic, mask, err := parseBinfmtMisc(fmt.Sprintf("test vector %d", i), strings.NewReader(v.contents))
			require.NoError(t, err, "parseBinfmtMisc: %v", err)
			require.GreaterOrEqual(t, offset, 0, "%q shouldn't have been disabled", v.platform)
			headers := getKnownELFPlatformHeaders()[v.platform]
			matched := false
			for _, header := range headers {
				if magicMatch(header, offset, mask, magic) {
					matched = true
				}
			}
			assert.True(t, matched, "%q did not match an expected header match", v.platform)
		})
	}
}
