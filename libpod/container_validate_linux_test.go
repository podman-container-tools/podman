//go:build !remote && linux

package libpod

import (
	"errors"
	"testing"

	spec "github.com/opencontainers/runtime-spec/specs-go"
	"go.podman.io/podman/v6/libpod/define"
)

func TestValidateNoCgroupsPIDNamespace(t *testing.T) {
	for _, tc := range []struct {
		name      string
		vm        bool
		noCgroups bool
		pid       string
		parent    string
		enabled   bool
		legacy    bool
		wantError bool
	}{
		{name: "private", noCgroups: true, pid: "private"},
		{name: "host requires VM", noCgroups: true, pid: "host", wantError: true},
		{name: "VM host", vm: true, noCgroups: true, pid: "host"},
		{name: "VM private", vm: true, noCgroups: true, pid: "private"},
		{name: "VM must support disabled", vm: true, pid: "host", wantError: true},
		{name: "private must support disabled", vm: true, pid: "private", wantError: true},
		{name: "VM enabled unchanged", vm: true, enabled: true, pid: "host"},
		{name: "VM cannot share container PID", vm: true, noCgroups: true, pid: "container", wantError: true},
		{name: "VM cannot join PID path", vm: true, noCgroups: true, pid: "path", wantError: true},
		{name: "VM cannot specify parent", vm: true, noCgroups: true, pid: "host", parent: "/parent", wantError: true},
		{name: "VM requires Linux config", vm: true, noCgroups: true, pid: "missing", wantError: true},
		{name: "VM host requires explicit disabled", vm: true, noCgroups: true, pid: "host", legacy: true, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config := &ContainerConfig{Spec: &spec.Spec{Process: &spec.Process{}, Linux: &spec.Linux{}}}
			config.Rootfs = "/test-rootfs"
			config.NoCgroups = !tc.enabled
			if !tc.enabled && !tc.legacy {
				config.CgroupsMode = "disabled"
			}
			config.CgroupParent = tc.parent
			switch tc.pid {
			case "private":
				config.Spec.Linux.Namespaces = []spec.LinuxNamespace{{Type: spec.PIDNamespace}}
			case "path":
				config.Spec.Linux.Namespaces = []spec.LinuxNamespace{{Type: spec.PIDNamespace, Path: "/proc/1/ns/pid"}}
			case "container":
				config.PIDNsCtr = "another-container"
			case "missing":
				config.Spec.Linux = nil
			}
			ctr := &Container{
				config: config,
				ociRuntime: &ConmonOCIRuntime{
					name: "test-runtime", supportsKVM: tc.vm, supportsNoCgroups: tc.noCgroups,
				},
			}
			err := ctr.validate()
			if tc.wantError {
				if !errors.Is(err, define.ErrInvalidArg) {
					t.Fatalf("expected invalid argument, got %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
		})
	}
}
