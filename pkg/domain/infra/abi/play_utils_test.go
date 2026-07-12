//go:build !remote && (linux || freebsd)

package abi

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.podman.io/podman/v6/libpod/define"
)

func TestGetSdNotifyMode(t *testing.T) {
	tests := []struct {
		key, value, name, result string
		mustError                bool
	}{
		{sdNotifyAnnotation, define.SdNotifyModeConmon, "", define.SdNotifyModeConmon, false},
		{sdNotifyAnnotation + "/container-a", define.SdNotifyModeContainer, "container-a", define.SdNotifyModeContainer, false},
		{sdNotifyAnnotation + "/container-b", define.SdNotifyModeIgnore, "container-b", define.SdNotifyModeIgnore, false},
		{sdNotifyAnnotation + "/container-c", "", "container-c", "", false},
		{sdNotifyAnnotation + "-/wrong-key", "xxx", "wrong-key", "", false},
		{sdNotifyAnnotation + "/container-error", "invalid", "container-error", "", true},
	}

	annotations := make(map[string]string)
	// Populate the annotations
	for _, test := range tests {
		annotations[test.key] = test.value
	}
	// Run the tests
	for _, test := range tests {
		result, err := getSdNotifyMode(annotations, test.name)
		if test.mustError {
			require.Error(t, err, "%v", test)
			continue
		}
		require.NoError(t, err, "%v", test)
		require.Equal(t, test.result, result, "%v", test)
	}
}
