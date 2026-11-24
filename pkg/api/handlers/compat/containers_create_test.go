//go:build !remote && (linux || freebsd)

package compat

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/moby/moby/api/types/network"
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

func TestHandlePortBindingsBC(t *testing.T) {
	const warn152 = "Following container port(s) have an empty list of port-bindings: 53/udp, 8080/tcp. Such bindings will be discarded in a future version."

	tests := []struct {
		name         string
		version      string // empty means no version in the URL
		portBindings network.PortMap
		wantWarning  string
		want         network.PortMap
	}{
		{
			name:         "nil map",
			version:      "1.52",
			portBindings: nil,
			want:         nil,
		},
		{
			name:    "no empty bindings",
			version: "1.52",
			portBindings: network.PortMap{
				network.MustParsePort("80/tcp"): {{HostPort: "8000"}},
			},
			want: network.PortMap{
				network.MustParsePort("80/tcp"): {{HostPort: "8000"}},
			},
		},
		{
			name:    "empty and nil bindings are backfilled with a warning",
			version: "1.52",
			portBindings: network.PortMap{
				network.MustParsePort("8080/tcp"): {},
				network.MustParsePort("53/udp"):   nil,
				network.MustParsePort("80/tcp"):   {{HostPort: "8000"}},
			},
			wantWarning: warn152,
			want: network.PortMap{
				network.MustParsePort("8080/tcp"): {{}},
				network.MustParsePort("53/udp"):   {{}},
				network.MustParsePort("80/tcp"):   {{HostPort: "8000"}},
			},
		},
		{
			name:    "no version is treated as latest",
			version: "",
			portBindings: network.PortMap{
				network.MustParsePort("8080/tcp"): {},
				network.MustParsePort("53/udp"):   nil,
			},
			wantWarning: warn152,
			want: network.PortMap{
				network.MustParsePort("8080/tcp"): {{}},
				network.MustParsePort("53/udp"):   {{}},
			},
		},
		{
			name:    "older API is backfilled without a warning",
			version: "1.51",
			portBindings: network.PortMap{
				network.MustParsePort("8080/tcp"): {},
			},
			want: network.PortMap{
				network.MustParsePort("8080/tcp"): {{}},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/containers/create", nil)
			if test.version != "" {
				r = mux.SetURLVars(r, map[string]string{"version": test.version})
			}
			assert.Equal(t, test.wantWarning, handlePortBindingsBC(r, test.portBindings))
			assert.Equal(t, test.want, test.portBindings)
		})
	}
}
