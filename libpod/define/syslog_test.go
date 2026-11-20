package define

import (
	"maps"
	"strings"
	"testing"
)

func TestNormalizeSyslogOptionKeys(t *testing.T) {
	t.Parallel()

	opts := map[string]string{"Syslog-Address": "tcp://127.0.0.1:514", "tag": "harbor"}
	if err := NormalizeSyslogOptionKeys(opts); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"syslog-address": "tcp://127.0.0.1:514", "tag": "harbor"}
	if !maps.Equal(opts, want) {
		t.Fatalf("normalized options = %v, want %v", opts, want)
	}
	if err := NormalizeSyslogOptionKeys(nil); err != nil {
		t.Fatal(err)
	}

	duplicates := []map[string]string{
		{"Syslog-Address": "tcp://server-a:514", "syslog-address": "tcp://server-b:514"},
		{"Syslog-Facility": "daemon", "syslog-facility": "daemon"},
	}
	for _, dup := range duplicates {
		before := maps.Clone(dup)
		err := NormalizeSyslogOptionKeys(dup)
		if err == nil || !strings.Contains(err.Error(), "duplicate log opt") {
			t.Fatalf("NormalizeSyslogOptionKeys(%v) = %v", before, err)
		}
		if !maps.Equal(dup, before) {
			t.Fatalf("duplicate input was mutated: got %v want %v", dup, before)
		}
	}
}
