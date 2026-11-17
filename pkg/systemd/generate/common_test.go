//go:build !remote && (linux || freebsd)

package generate

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFilterPodFlags(t *testing.T) {
	tests := []struct {
		input    []string
		output   []string
		argCount int
	}{
		{
			[]string{"podman", "pod", "create"},
			[]string{"podman", "pod", "create"},
			0,
		},
		{
			[]string{"podman", "pod", "create", "--name", "foo"},
			[]string{"podman", "pod", "create", "--name", "foo"},
			0,
		},
		{
			[]string{"podman", "pod", "create", "--pod-id-file", "foo"},
			[]string{"podman", "pod", "create"},
			0,
		},
		{
			[]string{"podman", "pod", "create", "--pod-id-file=foo"},
			[]string{"podman", "pod", "create"},
			0,
		},
		{
			[]string{"podman", "pod", "create", "--pod-id-file", "foo", "--infra-conmon-pidfile", "foo"},
			[]string{"podman", "pod", "create"},
			0,
		},
		{
			[]string{"podman", "pod", "create", "--pod-id-file", "foo", "--infra-conmon-pidfile=foo"},
			[]string{"podman", "pod", "create"},
			0,
		},
		{
			[]string{"podman", "run", "--pod", "foo"},
			[]string{"podman", "run"},
			0,
		},
		{
			[]string{"podman", "run", "--pod=foo"},
			[]string{"podman", "run"},
			0,
		},
		{
			[]string{"podman", "run", "--pod=foo", "fedora", "podman", "run", "--pod=test", "alpine"},
			[]string{"podman", "run", "fedora", "podman", "run", "--pod=test", "alpine"},
			5,
		},
		{
			[]string{"podman", "run", "--pod", "foo", "fedora", "podman", "run", "--pod", "test", "alpine"},
			[]string{"podman", "run", "fedora", "podman", "run", "--pod", "test", "alpine"},
			6,
		},
		{
			[]string{"podman", "run", "--pod-id-file=foo", "fedora", "podman", "run", "--pod-id-file=test", "alpine"},
			[]string{"podman", "run", "fedora", "podman", "run", "--pod-id-file=test", "alpine"},
			5,
		},
		{
			[]string{"podman", "run", "--pod-id-file", "foo", "fedora", "podman", "run", "--pod-id-file", "test", "alpine"},
			[]string{"podman", "run", "fedora", "podman", "run", "--pod-id-file", "test", "alpine"},
			6,
		},
	}

	for _, test := range tests {
		processed := filterPodFlags(test.input, test.argCount)
		assert.Equal(t, test.output, processed)
	}
}

func TestFilterCommonContainerFlags(t *testing.T) {
	tests := []struct {
		input    []string
		output   []string
		argCount int
	}{
		{
			[]string{"podman", "run", "alpine"},
			[]string{"podman", "run", "alpine"},
			1,
		},
		{
			[]string{"podman", "run", "--conmon-pidfile", "foo", "alpine"},
			[]string{"podman", "run", "--conmon-pidfile", "foo", "alpine"},
			1,
		},
		{
			[]string{"podman", "run", "--conmon-pidfile=foo", "alpine"},
			[]string{"podman", "run", "--conmon-pidfile=foo", "alpine"},
			1,
		},
		{
			[]string{"podman", "run", "--cidfile", "foo", "alpine"},
			[]string{"podman", "run", "alpine"},
			1,
		},
		{
			[]string{"podman", "run", "--cidfile=foo", "alpine"},
			[]string{"podman", "run", "alpine"},
			1,
		},
		{
			[]string{"podman", "run", "--cgroups", "foo", "alpine"},
			[]string{"podman", "run", "alpine"},
			1,
		},
		{
			[]string{"podman", "run", "--cgroups=foo", "--restart=foo", "alpine"},
			[]string{"podman", "run", "alpine"},
			1,
		},
		{
			[]string{"podman", "run", "--cgroups=foo", "--rm", "--restart", "foo", "alpine"},
			[]string{"podman", "run", "alpine"},
			1,
		},
		{
			[]string{"podman", "run", "--cgroups", "--rm=bogus", "alpine", "--cgroups", "foo", "--conmon-pidfile", "foo", "--cidfile", "foo", "--rm"},
			[]string{"podman", "run", "alpine", "--cgroups", "foo", "--conmon-pidfile", "foo", "--cidfile", "foo", "--rm"},
			7,
		},
	}

	for _, test := range tests {
		processed := filterCommonContainerFlags(test.input, test.argCount)
		assert.Equal(t, test.output, processed)
	}
}

func TestEscapeSystemdArguments(t *testing.T) {
	tests := []struct {
		input  []string
		output []string
	}{
		{
			[]string{"foo", "bar=\"arg\""},
			[]string{"foo", "\"bar=\\\"arg\\\"\""},
		},
		{
			[]string{"foo", "bar=\"arg with space\""},
			[]string{"foo", "\"bar=\\\"arg with space\\\"\""},
		},
		{
			[]string{"foo", "bar=\"arg with\ttab\""},
			[]string{"foo", "\"bar=\\\"arg with\\ttab\\\"\""},
		},
		{
			[]string{"$"},
			[]string{"$$"},
		},
		{
			[]string{"foo", "command with dollar sign $"},
			[]string{"foo", "\"command with dollar sign $$\""},
		},
		{
			[]string{"foo", "command with two dollar signs $$"},
			[]string{"foo", "\"command with two dollar signs $$$$\""},
		},
		{
			[]string{"%"},
			[]string{"%%"},
		},
		{
			[]string{"foo", "command with percent sign %"},
			[]string{"foo", "\"command with percent sign %%\""},
		},
		{
			[]string{"foo", "command with two percent signs %%"},
			[]string{"foo", "\"command with two percent signs %%%%\""},
		},
		{
			[]string{`\`},
			[]string{`\\`},
		},
		{
			[]string{"foo", `command with backslash \`},
			[]string{"foo", `"command with backslash \\"`},
		},
		{
			[]string{"foo", `command with two backslashes \\`},
			[]string{"foo", `"command with two backslashes \\\\"`},
		},
		{
			[]string{"podman", "create", "--entrypoint", "foo"},
			[]string{"podman", "create", "--entrypoint", "foo"},
		},
		{
			[]string{"podman", "create", "--entrypoint=foo"},
			[]string{"podman", "create", "--entrypoint=foo"},
		},
		{
			[]string{"podman", "create", "--entrypoint", "[\"foo\"]"},
			[]string{"podman", "create", "--entrypoint", "\"[\\\"foo\\\"]\""},
		},
		{
			[]string{"podman", "create", "--entrypoint", "[\"sh\", \"-c\", \"date '+%s'\"]"},
			[]string{"podman", "create", "--entrypoint", "\"[\\\"sh\\\", \\\"-c\\\", \\\"date '+%%s'\\\"]\""},
		},
	}

	for _, test := range tests {
		quoted := escapeSystemdArguments(test.input)
		assert.Equal(t, test.output, quoted)
	}
}
