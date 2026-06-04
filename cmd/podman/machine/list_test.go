//go:build amd64 || arm64

package machine

import (
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.podman.io/podman/v6/pkg/machine"
)

func Test_compareResponseByRunningAndLastUp(t *testing.T) {
	base := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)

	samples := []*machine.ListResponse{
		{Name: "a", Running: true, LastUp: base},
		{Name: "b", Running: true, LastUp: base.Add(time.Hour)},
		{Name: "c", Running: false, LastUp: base},
		{Name: "d", Running: false, LastUp: base.Add(time.Hour)},
		{Name: "e", Running: true, LastUp: base},
	}

	for _, a := range samples {
		for _, b := range samples {
			ab := compareResponseByRunningAndLastUp(a, b)
			ba := compareResponseByRunningAndLastUp(b, a)
			assert.Equal(t, ab, -ba,
				"antisymmetry violated for %s vs %s: compareResponseByRunningAndLastUp(a,b)=%d compareResponseByRunningAndLastUp(b,a)=%d",
				a.Name, b.Name, ab, ba)
		}
	}
}

func Test_sortMachines(t *testing.T) {
	base := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	t1 := base                    // oldest
	t2 := base.Add(1 * time.Hour) // middle
	t3 := base.Add(2 * time.Hour) // newest

	tests := []struct {
		name  string
		input []*machine.ListResponse
		want  []string
	}{
		{
			name: "running always before non-running regardless of time",
			input: []*machine.ListResponse{
				{Name: "stopped-newest", Running: false, LastUp: t3},
				{Name: "running-oldest", Running: true, LastUp: t1},
			},
			want: []string{"running-oldest", "stopped-newest"},
		},
		{
			name: "running group sorted by time descending",
			input: []*machine.ListResponse{
				{Name: "running-oldest", Running: true, LastUp: t1},
				{Name: "running-newest", Running: true, LastUp: t3},
				{Name: "running-middle", Running: true, LastUp: t2},
			},
			want: []string{"running-newest", "running-middle", "running-oldest"},
		},
		{
			name: "non-running group sorted by time descending",
			input: []*machine.ListResponse{
				{Name: "stopped-oldest", Running: false, LastUp: t1},
				{Name: "stopped-newest", Running: false, LastUp: t3},
				{Name: "stopped-middle", Running: false, LastUp: t2},
			},
			want: []string{"stopped-newest", "stopped-middle", "stopped-oldest"},
		},
		{
			name: "mixed: running first (by time desc), then non-running (by time desc)",
			input: []*machine.ListResponse{
				{Name: "stopped-middle", Running: false, LastUp: t2},
				{Name: "running-oldest", Running: true, LastUp: t1},
				{Name: "stopped-newest", Running: false, LastUp: t3},
				{Name: "running-newest", Running: true, LastUp: t3},
			},
			want: []string{"running-newest", "running-oldest", "stopped-newest", "stopped-middle"},
		},
	}

	getNames := func(l []*machine.ListResponse) []string {
		ns := make([]string, 0, len(l))
		for _, m := range l {
			ns = append(ns, m.Name)
		}
		return ns
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slices.SortFunc(tt.input, compareResponseByRunningAndLastUp)
			assert.Equal(t, tt.want, getNames(tt.input))
		})
	}
}
