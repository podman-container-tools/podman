//go:build !remote && (linux || freebsd)

package libpod

import "testing"

func TestParseConmonVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		output string
		ver    string
		major  int
		ok     bool
	}{
		{output: "conmon version 2.1.12\ncommit: abc\n", ver: "2.1.12", major: 2, ok: true},
		{output: "conmon version 3.0.0-dev\ncommit: unknown\n", ver: "3.0.0-dev", major: 3, ok: true},
		{output: "conmon version 3.1.0, commit: abc\n", ver: "3.1.0", major: 3, ok: true},
		{output: "conmon version 3.invalid\n", ok: false},
		{output: "conmon 2.1.12\n", ok: false},
	}
	for _, tt := range tests {
		ver, major, err := parseConmonVersion(tt.output)
		if tt.ok {
			if err != nil {
				t.Errorf("parseConmonVersion(%q) error = %v", tt.output, err)
				continue
			}
			if ver != tt.ver || major != tt.major {
				t.Errorf("parseConmonVersion(%q) = %q, %d; want %q, %d", tt.output, ver, major, tt.ver, tt.major)
			}
			continue
		}
		if err == nil {
			t.Errorf("parseConmonVersion(%q) = %q, %d; want error", tt.output, ver, major)
		}
	}
}
