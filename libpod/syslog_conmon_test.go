//go:build !remote && (linux || freebsd)

package libpod

import (
	"slices"
	"testing"

	"go.podman.io/common/pkg/config"
	"go.podman.io/podman/v6/libpod/define"
)

func TestSyslogConmonArgs(t *testing.T) {
	t.Parallel()

	runtime := &Runtime{config: &config.Config{}}
	runtime.conmonVersionOnce.Do(func() {
		runtime.conmonVersion = "conmon version 3.0.0\n"
	})
	oci := &ConmonOCIRuntime{path: "/usr/bin/crun"}

	t.Run("address facility tag and label", func(t *testing.T) {
		args := conmonLogArgs(t, oci, runtime, define.SyslogLogging, "harbor", map[string]string{
			"syslog-address":  "tcp://127.0.0.1:1514",
			"syslog-facility": "local0",
		}, map[string]string{"FOO": "bar"})
		if !slices.Equal(flagValues(args, "--log-driver"), []string{define.SyslogLogging}) {
			t.Fatalf("--log-driver = %v", flagValues(args, "--log-driver"))
		}
		if slices.Contains(args, "-l") {
			t.Fatalf("syslog args include -l: %v", args)
		}
		if !slices.Equal(flagValues(args, "--log-driver-opt"), []string{
			"syslog-address=tcp://127.0.0.1:1514",
			"syslog-facility=local0",
		}) {
			t.Fatalf("--log-driver-opt = %v", flagValues(args, "--log-driver-opt"))
		}
		if !slices.Equal(flagValues(args, "--log-tag"), []string{"harbor"}) {
			t.Fatalf("--log-tag = %v", flagValues(args, "--log-tag"))
		}
		if !slices.Equal(flagValues(args, "--log-label"), []string{"FOO=bar"}) {
			t.Fatalf("--log-label = %v", flagValues(args, "--log-label"))
		}
	})

	t.Run("empty and unknown options are forwarded in order", func(t *testing.T) {
		args := conmonLogArgs(t, oci, runtime, define.SyslogLogging, "", map[string]string{
			"syslog-future-option": "example",
			"syslog-address":       "",
			"syslog-facility":      "",
			"tag":                  "harbor",
			"max-size":             "10mb",
		}, nil)
		if !slices.Equal(flagValues(args, "--log-driver-opt"), []string{
			"syslog-address=",
			"syslog-facility=",
			"syslog-future-option=example",
		}) {
			t.Fatalf("--log-driver-opt = %v", flagValues(args, "--log-driver-opt"))
		}
		if slices.Contains(args, "--log-tag") {
			t.Fatalf("unrelated tag stored in driver options was forwarded: %v", args)
		}
	})

	t.Run("log path is not forwarded", func(t *testing.T) {
		args, err := oci.sharedConmonArgs(&Container{
			runtime: runtime,
			config: &ContainerConfig{
				ID: "0123456789abcdef",
				ContainerMiscConfig: ContainerMiscConfig{
					CgroupManager: "cgroupfs",
				},
			},
		}, "0123456789abcdef", "/bundle", "/pid", "/var/log/ctr.log", "/exit", t.TempDir(), "", define.SyslogLogging, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		if slices.Contains(args, "-l") || slices.Contains(args, "--log-path") || slices.Contains(args, "/var/log/ctr.log") {
			t.Fatalf("log path was forwarded: %v", args)
		}
	})

	t.Run("journald stays on the v2 log path", func(t *testing.T) {
		args := conmonLogArgs(t, oci, runtime, define.JournaldLogging, "", nil, nil)
		if !slices.Equal(flagValues(args, "-l"), []string{define.JournaldLogging}) {
			t.Fatalf("-l = %v", flagValues(args, "-l"))
		}
		if slices.Contains(args, "--log-driver") {
			t.Fatalf("journald args include --log-driver: %v", args)
		}
	})
}

func conmonLogArgs(t *testing.T, oci *ConmonOCIRuntime, runtime *Runtime, driver, tag string, opts, labels map[string]string) []string {
	t.Helper()
	ctr := &Container{
		runtime: runtime,
		config: &ContainerConfig{
			ID: "0123456789abcdef",
			ContainerMiscConfig: ContainerMiscConfig{
				CgroupManager:    "cgroupfs",
				LogDriverOptions: opts,
			},
		},
	}
	args, err := oci.sharedConmonArgs(ctr, ctr.ID(), "/bundle", "/pid", "", "/exit", t.TempDir(), "", driver, tag, labels)
	if err != nil {
		t.Fatal(err)
	}
	return args
}

func flagValues(args []string, name string) []string {
	var values []string
	for i := 0; i < len(args)-1; i++ {
		if args[i] == name {
			values = append(values, args[i+1])
		}
	}
	return values
}
