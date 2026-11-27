package registry

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"

	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/rootless"
)

func unsetEnvForTest(t *testing.T, key string) {
	t.Helper()
	t.Setenv(key, "")
	if err := os.Unsetenv(key); err != nil {
		t.Fatal(err)
	}
}

func TestParseEarlyCLIOptions(t *testing.T) {
	unsetEnvForTest(t, "CONTAINER_HOST")
	unsetEnvForTest(t, "CONTAINER_CONNECTION")
	options := parseEarlyCLIOptions([]string{"podman", "--log-level=trace", "--module=one", "--module=two", "--connection=test", "version"})
	if options.parseErr != nil {
		t.Fatal(options.parseErr)
	}
	if options.logLevel != "trace" {
		t.Errorf("expected trace log level, got %q", options.logLevel)
	}
	if !slices.Equal(options.modules, []string{"one", "two"}) {
		t.Errorf("expected modules [one two], got %v", options.modules)
	}
	if !options.remote {
		t.Error("expected --connection to enable remote mode")
	}
}

func TestParseEarlyCLIOptionsRemote(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		env        string
		wantRemote bool
	}{
		{name: "context", args: []string{"podman", "--context=test", "version"}, wantRemote: true},
		{name: "host", args: []string{"podman", "--host=unix:///run/podman.sock", "version"}, wantRemote: true},
		{name: "url", args: []string{"podman", "--url=unix:///run/podman.sock", "version"}, wantRemote: true},
		{name: "remote false overrides host environment", args: []string{"podman", "--remote=false", "version"}, env: "CONTAINER_HOST"},
		{name: "remote false overrides connection environment", args: []string{"podman", "--remote=false", "version"}, env: "CONTAINER_CONNECTION"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			unsetEnvForTest(t, "CONTAINER_HOST")
			unsetEnvForTest(t, "CONTAINER_CONNECTION")
			if test.env != "" {
				t.Setenv(test.env, "test")
			}
			if remote := parseEarlyCLIOptions(test.args).remote; remote != test.wantRemote {
				t.Errorf("remote = %v, want %v", remote, test.wantRemote)
			}
		})
	}
}

func TestParseEarlyCLIOptionsCompletion(t *testing.T) {
	for _, completion := range []string{cobra.ShellCompRequestCmd, cobra.ShellCompNoDescRequestCmd} {
		t.Run(completion, func(t *testing.T) {
			options := parseEarlyCLIOptions([]string{"podman", completion, "--remote", "--module=test", "--log-level"})
			if !options.completion {
				t.Error("expected shell completion to be detected")
			}
			if !options.remote {
				t.Error("expected --remote to be parsed during completion")
			}
			if options.parseErr == nil {
				t.Error("expected the incomplete --log-level flag to produce a parse error")
			}
			if !slices.Equal(options.modules, []string{"test"}) {
				t.Errorf("expected parsed modules [test], got %v", options.modules)
			}
		})
	}
}

func TestIsRemotePodmanSh(t *testing.T) {
	if isRemote(entities.TunnelMode, []string{PodmanSh, "-c", "id"}) {
		t.Error("expected podmansh to remain local")
	}
}

func TestParseEarlyCLIOptionsErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "module missing value", args: []string{"podman", "--module"}},
		{name: "log level missing value", args: []string{"podman", "--log-level"}},
		{name: "invalid debug value", args: []string{"podman", "--debug=invalid"}},
		{name: "invalid remote value", args: []string{"podman", "--remote=invalid"}},
		{name: "connection missing value", args: []string{"podman", "--connection"}},
		{name: "context missing value", args: []string{"podman", "--context"}},
		{name: "host missing value", args: []string{"podman", "--host"}},
		{name: "url missing value", args: []string{"podman", "--url"}},
		{name: "invalid flag syntax", args: []string{"podman", "---"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := parseEarlyCLIOptions(test.args).parseErr; err == nil {
				t.Error("expected a parse error")
			}
		})
	}
}

func TestIsFilesystemError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{name: "nil error", err: nil, expected: false},
		{name: "EIO", err: syscall.EIO, expected: true},
		{name: "ENOENT", err: syscall.ENOENT, expected: true},
		{name: "ESTALE", err: syscall.ESTALE, expected: true},
		{name: "os.ErrNotExist", err: os.ErrNotExist, expected: true},
		{name: "os.ErrPermission", err: os.ErrPermission, expected: true},
		{name: "wrapped PathError", err: &os.PathError{Op: "open", Path: "/etc/storage.conf", Err: syscall.EIO}, expected: true},
		{name: "wrapped SyscallError", err: &os.SyscallError{Syscall: "openat", Err: syscall.ENOENT}, expected: true},
		{name: "string error input/output error", err: errors.New("open /dev/sdb1: input/output error"), expected: true},
		{name: "string error no such file", err: errors.New("open /etc/storage.conf: no such file or directory"), expected: true},
		{name: "generic toml error", err: errors.New("toml: expected key"), expected: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isFilesystemError(tc.err); got != tc.expected {
				t.Errorf("isFilesystemError(%v) = %v, want %v", tc.err, got, tc.expected)
			}
		})
	}
}

func TestEnrichConfigError_NonRootless(t *testing.T) {
	origErr := errors.New("test error")
	info := &rootless.RootlessNamespaceInfo{
		IsRootless: false,
	}

	res := enrichConfigErrorWithInfo(origErr, info)
	if res != origErr {
		t.Errorf("expected error to be untouched when non-rootless, got %v", res)
	}

	if nilRes := enrichConfigErrorWithInfo(nil, info); nilRes != nil {
		t.Errorf("expected nil error to return nil, got %v", nilRes)
	}
}

func TestEnrichConfigError_Rootless_FilesystemEIO(t *testing.T) {
	origErr := fmt.Errorf("open /home/test/.config/containers/storage.conf: %w", syscall.EIO)
	info := &rootless.RootlessNamespaceInfo{
		IsRootless:      true,
		HasJoinedNS:     true,
		MountNS:         "mnt:[4026531840]",
		UserNS:          "user:[4026531837]",
		PausePID:        98765,
		PausePIDRunning: true,
		JoinSource:      "pause.pid",
		PauseMountNS:    "mnt:[4026531840]",
	}

	enriched := enrichConfigErrorWithInfo(origErr, info)
	if enriched == nil {
		t.Fatal("expected non-nil enriched error")
	}

	if !errors.Is(enriched, syscall.EIO) {
		t.Errorf("expected error wrapping to preserve syscall.EIO")
	}

	msg := enriched.Error()
	expectedSubstrings := []string{
		"open /home/test/.config/containers/storage.conf: input/output error",
		"joined rootless mount namespace active",
		"mount ns: mnt:[4026531840]",
		"user ns: user:[4026531837]",
		"pause PID: 98765 (running)",
		"join source: pause.pid",
		"matches pause process mount ns",
		"podman system migrate",
		"stale mount view",
	}

	for _, sub := range expectedSubstrings {
		if !strings.Contains(msg, sub) {
			t.Errorf("expected error message to contain %q, but got: %s", sub, msg)
		}
	}
}

func TestEnrichConfigError_Rootless_FilesystemENOENT(t *testing.T) {
	origErr := fmt.Errorf("open /home/test/.config/containers/storage.conf: %w", os.ErrNotExist)
	info := &rootless.RootlessNamespaceInfo{
		IsRootless:      true,
		HasJoinedNS:     true,
		MountNS:         "mnt:[4026532000]",
		PausePID:        11111,
		PausePIDRunning: false,
		JoinSource:      "ns_handles",
	}

	enriched := enrichConfigErrorWithInfo(origErr, info)
	if enriched == nil {
		t.Fatal("expected non-nil enriched error")
	}

	if !errors.Is(enriched, os.ErrNotExist) {
		t.Errorf("expected error wrapping to preserve os.ErrNotExist")
	}

	msg := enriched.Error()
	expectedSubstrings := []string{
		"join source: ns_handles",
		"pause PID: 11111 (not running)",
		"podman system migrate",
	}

	for _, sub := range expectedSubstrings {
		if !strings.Contains(msg, sub) {
			t.Errorf("expected error message to contain %q, but got: %s", sub, msg)
		}
	}
}

func TestEnrichConfigError_Rootless_Unjoined(t *testing.T) {
	origErr := fmt.Errorf("open /home/test/.config/containers/storage.conf: %w", syscall.EACCES)
	info := &rootless.RootlessNamespaceInfo{
		IsRootless:  true,
		HasJoinedNS: false,
		MountNS:     "mnt:[4026531840]",
	}

	enriched := enrichConfigErrorWithInfo(origErr, info)
	if enriched == nil {
		t.Fatal("expected non-nil enriched error")
	}

	msg := enriched.Error()
	if !strings.Contains(msg, "rootless mode active (joined namespace: inactive)") {
		t.Errorf("expected unjoined rootless notice, got: %s", msg)
	}
	if !strings.Contains(msg, "podman system migrate") {
		t.Errorf("expected remediation hint on filesystem error, got: %s", msg)
	}
}

func TestEnrichConfigError_Rootless_NonFilesystemError(t *testing.T) {
	origErr := errors.New("toml: line 2: expected value")
	info := &rootless.RootlessNamespaceInfo{
		IsRootless:  true,
		HasJoinedNS: false,
		MountNS:     "mnt:[4026531840]",
	}

	enriched := enrichConfigErrorWithInfo(origErr, info)
	if enriched == nil {
		t.Fatal("expected non-nil enriched error")
	}

	msg := enriched.Error()
	if !strings.Contains(msg, "rootless mode active (joined namespace: inactive)") {
		t.Errorf("expected rootless namespace context, got: %s", msg)
	}
	if strings.Contains(msg, "podman system migrate") {
		t.Errorf("did not expect stale mount hint on non-filesystem syntax error, got: %s", msg)
	}
}

func TestEnrichConfigErrorWithStateDir(t *testing.T) {
	tempDir := t.TempDir()
	pauseFile := filepath.Join(tempDir, "pause.pid")
	if err := os.WriteFile(pauseFile, []byte("34567\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	nsFile := filepath.Join(tempDir, "ns_handles")
	if err := os.WriteFile(nsFile, []byte("handle_data"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("_CONTAINERS_ROOTLESS_UID", "1000")
	origErr := fmt.Errorf("open /storage.conf: %w", syscall.EIO)
	enriched := enrichConfigErrorWithStateDir(origErr, tempDir)
	if enriched == nil {
		t.Fatal("expected non-nil enriched error")
	}

	msg := enriched.Error()
	if !strings.Contains(msg, "pause PID: 34567") {
		t.Errorf("expected discovered pause PID in error message, got: %s", msg)
	}
	if !strings.Contains(msg, "podman system migrate") {
		t.Errorf("expected migration recommendation, got: %s", msg)
	}
}

