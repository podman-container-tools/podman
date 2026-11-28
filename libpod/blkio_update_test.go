package libpod

import (
	stdjson "encoding/json"
	"testing"

	spec "github.com/opencontainers/runtime-spec/specs-go"
)

func TestMergeBlockIOUpdateClearsOnlyExplicitLists(t *testing.T) {
	device := spec.LinuxThrottleDevice{LinuxBlockIODevice: spec.LinuxBlockIODevice{Major: 8}, Rate: 100}
	for _, tt := range []struct {
		name     string
		request  string
		wantRead int
	}{
		{"omitted", "{}", 1},
		{"null", `{"throttleReadBpsDevice":null}`, 1},
		{"empty", `{"throttleReadBpsDevice":[]}`, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			saved := &spec.LinuxBlockIO{
				ThrottleReadBpsDevice:  []spec.LinuxThrottleDevice{device},
				ThrottleWriteBpsDevice: []spec.LinuxThrottleDevice{device},
			}
			var update spec.LinuxBlockIO
			if err := stdjson.Unmarshal([]byte(tt.request), &update); err != nil {
				t.Fatal(err)
			}
			mergeBlockIOUpdate(saved, &update)
			if len(saved.ThrottleReadBpsDevice) != tt.wantRead || len(saved.ThrottleWriteBpsDevice) != 1 {
				t.Fatalf("unexpected saved block I/O rules: %+v", saved)
			}
		})
	}
}

func TestBlockIORuntimeUpdateResetsRemovedDevices(t *testing.T) {
	first := spec.LinuxBlockIODevice{Major: 8, Minor: 0}
	second := spec.LinuxBlockIODevice{Major: 8, Minor: 16}
	weight := uint16(100)
	old := &spec.LinuxResources{BlockIO: &spec.LinuxBlockIO{
		WeightDevice: []spec.LinuxWeightDevice{{LinuxBlockIODevice: first, Weight: &weight}},
		ThrottleReadBpsDevice: []spec.LinuxThrottleDevice{
			{LinuxBlockIODevice: first, Rate: 100},
			{LinuxBlockIODevice: second, Rate: 300},
		},
		ThrottleWriteIOPSDevice: []spec.LinuxThrottleDevice{{LinuxBlockIODevice: first, Rate: 400}},
	}}
	saved := &spec.LinuxResources{
		BlockIO: &spec.LinuxBlockIO{
			ThrottleReadBpsDevice: []spec.LinuxThrottleDevice{{LinuxBlockIODevice: second, Rate: 200}},
		},
		Unified: map[string]string{"io.max": "8:32 wbps=100"},
	}
	live := blockIORuntimeUpdate(old, saved, true)
	if got, want := live.Unified["io.max"], "8:0 rbps=max\n8:0 wiops=max\n8:32 wbps=100"; got != want {
		t.Errorf("io.max = %q, want %q", got, want)
	}
	if got := live.Unified["io.bfq.weight"]; got != "8:0 default" {
		t.Errorf("io.bfq.weight = %q, want a default reset", got)
	}
	withoutBFQ := blockIORuntimeUpdate(old, saved, false)
	if _, ok := withoutBFQ.Unified["io.bfq.weight"]; ok {
		t.Errorf("unexpected BFQ reset: %v", withoutBFQ.Unified)
	}
	if withoutBFQ.Unified["io.max"] != live.Unified["io.max"] {
		t.Errorf("missing throttle resets without BFQ: %v", withoutBFQ.Unified)
	}
	if len(saved.Unified) != 1 || saved.Unified["io.max"] != "8:32 wbps=100" {
		t.Errorf("runtime resets leaked into saved resources: %v", saved.Unified)
	}
}
