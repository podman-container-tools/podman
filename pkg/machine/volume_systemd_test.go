package machine

import (
	"strings"
	"testing"
)

func TestCanonicalizeFCOSMountTarget(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// Known FCOS symlinks — must be rewritten
		{"/home/alice", "/var/home/alice"},
		{"/home/alice/projects", "/var/home/alice/projects"},
		{"/mnt/data", "/var/mnt/data"},
		{"/opt/myapp", "/var/opt/myapp"},
		{"/root", "/var/roothome"},
		{"/root/.config", "/var/roothome/.config"},
		{"/srv/www", "/var/srv/www"},
		// Exact match on a symlinked dir itself
		{"/home", "/var/home"},
		{"/mnt", "/var/mnt"},
		// Paths that do NOT start with a known symlink — unchanged
		{"/var/home/alice", "/var/home/alice"},
		{"/tmp/foo", "/tmp/foo"},
		{"/data/work", "/data/work"},
		{"/work", "/work"},
		// Prefix collision guard: /homes should NOT match /home
		{"/homes/alice", "/homes/alice"},
		{"/rootfs", "/rootfs"},
	}

	for _, tt := range tests {
		got := canonicalizeFCOSMountTarget(tt.input)
		if got != tt.expected {
			t.Errorf("canonicalizeFCOSMountTarget(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestGenerateSystemDFilesForVirtiofsmountsCanonicalPath(t *testing.T) {
	mounts := []VirtIoFs{
		NewVirtIoFsMount("/home/alice", "/home/alice", false),
		NewVirtIoFsMount("/data", "/data", false),
	}

	units, err := GenerateSystemDFilesForVirtiofsMounts(mounts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// First two units are the mount units; the rest are immutable-root helpers.
	mountUnits := units[:2]

	cases := []struct {
		wantName  string
		wantWhere string
	}{
		// /home/alice must be rewritten to /var/home/alice
		{"var-home-alice.mount", "/var/home/alice"},
		// /data has no FCOS symlink — stays as-is
		{"data.mount", "/data"},
	}

	for i, c := range cases {
		u := mountUnits[i]
		if u.Name != c.wantName {
			t.Errorf("unit[%d].Name = %q, want %q", i, u.Name, c.wantName)
		}
		if u.Contents == nil || !strings.Contains(*u.Contents, "Where="+c.wantWhere) {
			contents := ""
			if u.Contents != nil {
				contents = *u.Contents
			}
			t.Errorf("unit[%d] contents missing Where=%s:\n%s", i, c.wantWhere, contents)
		}
	}
}
