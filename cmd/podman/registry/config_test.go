package registry

import (
	"os"
	"slices"
	"testing"

	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/pkg/domain/entities"
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
