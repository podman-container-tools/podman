//go:build !remote && (linux || freebsd)

package compat

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/moby/moby/api/types/container"
	spec "github.com/opencontainers/runtime-spec/specs-go"
)

func TestUpdateBlockIOPreservesRequestPresence(t *testing.T) {
	var options container.UpdateConfig
	err := json.Unmarshal([]byte(`{
		"BlkioWeight":200,
		"BlkioWeightDevice":[{"Path":"/dev/test","Weight":150}],
		"BlkioDeviceReadBps":[],
		"BlkioDeviceWriteBps":null,
		"BlkioDeviceReadIOps":[{"Path":"/dev/test","Rate":400}]
	}`), &options)
	if err != nil {
		t.Fatal(err)
	}
	update, err := updateBlockIO(&options, func(path string) (int64, int64, error) {
		if path != "/dev/test" {
			t.Fatalf("unexpected device: %q", path)
		}
		return 8, 32, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	numbers := spec.LinuxBlockIODevice{Major: 8, Minor: 32}
	if update.Weight == nil || *update.Weight != 200 || len(update.WeightDevice) != 1 ||
		update.WeightDevice[0].LinuxBlockIODevice != numbers || *update.WeightDevice[0].Weight != 150 {
		t.Fatalf("unexpected weight config: %+v", update)
	}
	if update.ThrottleReadBpsDevice == nil || len(update.ThrottleReadBpsDevice) != 0 ||
		update.ThrottleWriteBpsDevice != nil || update.ThrottleWriteIOPSDevice != nil {
		t.Fatalf("lost omitted/null/empty distinction: %+v", update)
	}
	if len(update.ThrottleReadIOPSDevice) != 1 ||
		update.ThrottleReadIOPSDevice[0] != (spec.LinuxThrottleDevice{LinuxBlockIODevice: numbers, Rate: 400}) {
		t.Fatalf("unexpected read IOPS config: %+v", update.ThrottleReadIOPSDevice)
	}
}

func TestUpdateBlockIOReturnsDeviceError(t *testing.T) {
	var options container.UpdateConfig
	if err := json.Unmarshal([]byte(`{"BlkioDeviceWriteIOps":[{"Path":"/tmp/file","Rate":400}]}`), &options); err != nil {
		t.Fatal(err)
	}
	notDevice := errors.New("not a block device")
	update, err := updateBlockIO(&options, func(string) (int64, int64, error) { return 0, 0, notDevice })
	if !errors.Is(err, notDevice) || update != nil {
		t.Fatalf("got update %v, error %v, want unresolved device error", update, err)
	}
}
