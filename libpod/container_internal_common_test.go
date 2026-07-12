//go:build !remote

package libpod

import (
	"testing"

	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/stretchr/testify/assert"
)

func TestHasSELinuxContextOption(t *testing.T) {
	tests := []struct {
		name    string
		options []string
		want    bool
	}{
		// Positive cases — each SELinux context prefix should be detected
		{name: "context=", options: []string{"context=system_u:object_r:container_t:s0"}, want: true},
		{name: "fscontext=", options: []string{"fscontext=system_u:object_r:container_t:s0"}, want: true},
		{name: "defcontext=", options: []string{"defcontext=system_u:object_r:container_t:s0"}, want: true},
		{name: "rootcontext=", options: []string{"rootcontext=system_u:object_r:container_t:s0"}, want: true},

		// Context option mixed with other options
		{name: "context= mixed", options: []string{"size=64m", "context=system_u:object_r:tmp_t:s0", "mode=1777"}, want: true},
		{name: "fscontext= mixed", options: []string{"noexec", "fscontext=system_u:object_r:tmp_t:s0"}, want: true},

		// Negative cases — no SELinux context option
		{name: "nil slice", options: nil, want: false},
		{name: "empty slice", options: []string{}, want: false},
		{name: "no SELinux option", options: []string{"size=64m", "mode=1777", "noexec"}, want: false},
		{name: "single non-SELinux option", options: []string{"nosuid"}, want: false},

		// Edge cases — partial prefix matches should NOT count
		{name: "partial prefix context", options: []string{"context"}, want: false},
		{name: "partial prefix fscontext", options: []string{"fscontext"}, want: false},
		{name: "partial prefix defcontext", options: []string{"defcontext"}, want: false},
		{name: "partial prefix rootcontext", options: []string{"rootcontext"}, want: false},

		// Related but different options should NOT match
		{name: "context without equals", options: []string{"context"}, want: false},
		{name: "seclabel", options: []string{"seclabel"}, want: false},
		{name: "label=disabled", options: []string{"label:disabled"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasSELinuxContextOption(tt.options)
			if got != tt.want {
				t.Errorf("hasSELinuxContextOption(%v) = %v, want %v", tt.options, got, tt.want)
			}
		})
	}
}

func newTestGenerator(t *testing.T) *generate.Generator {
	t.Helper()
	g, err := generate.New("linux")
	if err != nil {
		t.Fatal(err)
	}
	return &g
}

func TestAddSELinuxMountOptionsNonTmpfs(t *testing.T) {
	label := "system_u:object_r:container_file_t:s0:c1,c2"

	t.Run("tmpfs with fscontext clears global label, adds context to bind", func(t *testing.T) {
		g := newTestGenerator(t)
		g.AddMount(spec.Mount{
			Type:        "tmpfs",
			Destination: "/tmp",
			Options:     []string{`fscontext="system_u:object_r:container_file_t:s0:c1,c2"`},
		})
		g.AddMount(spec.Mount{
			Type:        "bind",
			Source:      "/host/path",
			Destination: "/container/path",
			Options:     []string{"rbind", "rprivate"},
		})
		g.SetLinuxMountLabel(label)

		addSELinuxMountOptionsNonTmpfs(g, label)

		// Global MountLabel should be cleared
		assert.Equal(t, "", g.Config.Linux.MountLabel)

		// tmpfs should keep its fscontext=, not get context=
		tmpfs := g.Config.Mounts[0]
		for _, o := range tmpfs.Options {
			if o == `context="system_u:object_r:container_file_t:s0:c1,c2"` {
				t.Error("tmpfs mount should not have context=")
			}
		}

		// bind mount should get context=
		bind := g.Config.Mounts[1]
		hasCtx := false
		for _, o := range bind.Options {
			if o == `context="system_u:object_r:container_file_t:s0:c1,c2"` {
				hasCtx = true
			}
		}
		assert.True(t, hasCtx, "non-tmpfs mount should have context=")
	})

	t.Run("no tmpfs mounts, global label unchanged", func(t *testing.T) {
		g := newTestGenerator(t)
		g.SetLinuxMountLabel(label)

		addSELinuxMountOptionsNonTmpfs(g, label)

		assert.Equal(t, label, g.Config.Linux.MountLabel)
	})

	t.Run("tmpfs without SELinux option, global label unchanged", func(t *testing.T) {
		g := newTestGenerator(t)
		g.AddMount(spec.Mount{
			Type:        "tmpfs",
			Destination: "/tmp",
			Options:     []string{"size=64m"},
		})
		g.SetLinuxMountLabel(label)

		addSELinuxMountOptionsNonTmpfs(g, label)

		// No tmpfs with SELinux context, global label should stay
		assert.Equal(t, label, g.Config.Linux.MountLabel)
	})
}
