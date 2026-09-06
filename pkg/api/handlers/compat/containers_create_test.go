//go:build !remote && (linux || freebsd)

package compat

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
