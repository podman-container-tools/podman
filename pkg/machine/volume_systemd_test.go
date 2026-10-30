package machine

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
		assert.Equal(t, tt.expected, got, "input: %q", tt.input)
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
		assert.Equal(t, c.wantName, u.Name, "unit[%d].Name", i)
		if assert.NotNil(t, u.Contents, "unit[%d].Contents", i) {
			assert.Contains(t, *u.Contents, "Where="+c.wantWhere, "unit[%d] missing Where=", i)
		}
	}
}

func TestGenerateSystemDFilesForVirtiofsMountsReadOnly(t *testing.T) {
	tests := []struct {
		name         string
		mounts       []VirtIoFs
		wantReadOnly []bool
	}{
		{
			name: "read-only volume mount specifies ro option",
			mounts: []VirtIoFs{
				NewVirtIoFsMount("/host/ro-dir", "/guest/ro-dir", true),
			},
			wantReadOnly: []bool{true},
		},
		{
			name: "read-write volume mount omits ro option",
			mounts: []VirtIoFs{
				NewVirtIoFsMount("/host/rw-dir", "/guest/rw-dir", false),
			},
			wantReadOnly: []bool{false},
		},
		{
			name: "mixed read-only and read-write mounts correctly reflect options",
			mounts: []VirtIoFs{
				NewVirtIoFsMount("/host/ro-dir", "/guest/ro-dir", true),
				NewVirtIoFsMount("/host/rw-dir", "/guest/rw-dir", false),
			},
			wantReadOnly: []bool{true, false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			units, err := GenerateSystemDFilesForVirtiofsMounts(tt.mounts)
			assert.NoError(t, err)

			mountUnits := units[:len(tt.mounts)]
			for i, isReadOnly := range tt.wantReadOnly {
				u := mountUnits[i]
				assert.NotNil(t, u.Contents)
				if isReadOnly {
					assert.Contains(t, *u.Contents, "Options=context=\"system_u:object_r:nfs_t:s0\",ro")
				} else {
					assert.Contains(t, *u.Contents, "Options=context=\"system_u:object_r:nfs_t:s0\"")
					assert.NotContains(t, *u.Contents, ",ro")
				}
			}
		})
	}
}
