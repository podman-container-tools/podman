//go:build !remote && (linux || freebsd)

package compat

import (
	"testing"

	"github.com/moby/moby/api/types/mount"
	"github.com/stretchr/testify/assert"
)

const (
	defaultTarget = "/dest"
)

func TestConvertMountToCliString(t *testing.T) {
	tests := []struct {
		name     string
		mount    mount.Mount
		expected string
	}{
		{
			name: "bind mount, ro",
			mount: mount.Mount{
				Type:     mount.TypeBind,
				Source:   "/src",
				Target:   defaultTarget,
				ReadOnly: true,
			},
			expected: "type=bind,source=/src,target=/dest,ro=true",
		},
		{
			name: "image volume mount",
			mount: mount.Mount{
				Type:   mount.TypeImage,
				Source: "localhost/myimage:latest",
				Target: defaultTarget,
				ImageOptions: &mount.ImageOptions{
					Subpath: "/img",
				},
			},
			expected: "type=image,source=localhost/myimage:latest,target=/dest,subpath=/img",
		},
		{
			name: "volume mount",
			mount: mount.Mount{
				Type:   mount.TypeVolume,
				Source: "vol",
				Target: defaultTarget,
				VolumeOptions: &mount.VolumeOptions{
					Subpath: "/vol",
				},
			},
			expected: "type=volume,source=vol,target=/dest,subpath=/vol",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mountString, err := convertMountToCliString(test.mount)
			assert.NoError(t, err)
			assert.Equal(t, test.expected, mountString)
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
