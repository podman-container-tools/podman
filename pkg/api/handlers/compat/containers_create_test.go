//go:build !remote && (linux || freebsd)

package compat

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.podman.io/common/pkg/config"
	"go.podman.io/podman/v6/pkg/api/handlers"
)

func TestCLIOptsInit(t *testing.T) {
	enabled, disabled := true, false
	tests := []struct {
		name          string
		defaultInit   bool
		requestedInit *bool
		want          *bool
	}{
		{
			name:        "default false, omitted",
			defaultInit: false,
			want:        nil,
		},
		{
			name:          "default false, explicit true",
			defaultInit:   false,
			requestedInit: &enabled,
			want:          &enabled,
		},
		{
			name:          "default false, explicit false",
			defaultInit:   false,
			requestedInit: &disabled,
			want:          &disabled,
		},
		{
			name:        "default true, omitted",
			defaultInit: true,
			want:        nil,
		},
		{
			name:          "default true, explicit true",
			defaultInit:   true,
			requestedInit: &enabled,
			want:          &enabled,
		},
		{
			name:          "default true, explicit false",
			defaultInit:   true,
			requestedInit: &disabled,
			want:          &disabled,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rtc := &config.Config{}
			rtc.Containers.Init = test.defaultInit

			cc := handlers.CreateContainerConfig{}
			cc.HostConfig.Init = test.requestedInit

			opts, _, err := cliOpts(cc, rtc)
			require.NoError(t, err)
			assert.Equal(t, test.want, opts.Init)
		})
	}
}

func TestExtractInlineSeccompProfile(t *testing.T) {
	inlineProfile := `{"defaultAction":"SCMP_ACT_ALLOW"}`

	tests := []struct {
		name            string
		opts            []string
		expectedProfile string
		expectedRest    []string
	}{
		{
			name:            "no security opts",
			opts:            nil,
			expectedProfile: "",
			expectedRest:    nil,
		},
		{
			name:            "path stays untouched",
			opts:            []string{"seccomp=/some/path.json"},
			expectedProfile: "",
			expectedRest:    []string{"seccomp=/some/path.json"},
		},
		{
			name:            "unconfined stays untouched",
			opts:            []string{"seccomp=unconfined"},
			expectedProfile: "",
			expectedRest:    []string{"seccomp=unconfined"},
		},
		{
			name:            "inline json is extracted",
			opts:            []string{"seccomp=" + inlineProfile},
			expectedProfile: inlineProfile,
			expectedRest:    []string{},
		},
		{
			name:            "inline json with leading whitespace",
			opts:            []string{"seccomp= \n\t" + inlineProfile},
			expectedProfile: " \n\t" + inlineProfile,
			expectedRest:    []string{},
		},
		{
			name:            "deprecated colon separator",
			opts:            []string{"seccomp:" + inlineProfile},
			expectedProfile: inlineProfile,
			expectedRest:    []string{},
		},
		{
			name:            "other options are preserved",
			opts:            []string{"label=disable", "seccomp=" + inlineProfile, "no-new-privileges"},
			expectedProfile: inlineProfile,
			expectedRest:    []string{"label=disable", "no-new-privileges"},
		},
		{
			name:            "duplicate, last inline wins",
			opts:            []string{"seccomp=/some/path.json", "seccomp=" + inlineProfile},
			expectedProfile: inlineProfile,
			expectedRest:    []string{},
		},
		{
			name:            "duplicate, last path wins",
			opts:            []string{"seccomp=" + inlineProfile, "seccomp=/some/path.json"},
			expectedProfile: "",
			expectedRest:    []string{"seccomp=/some/path.json"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			profile, rest := extractInlineSeccompProfile(test.opts)
			assert.Equal(t, test.expectedProfile, profile)
			assert.Equal(t, test.expectedRest, rest)
		})
	}
}
