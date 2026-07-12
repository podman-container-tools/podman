//go:build !remote

package abi

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.podman.io/image/v5/manifest"
)

func TestDecodeOverrideConfig(t *testing.T) {
	testCases := []struct {
		description   string
		body          string
		expectedValue *manifest.Schema2Config
		expectedError bool
	}{
		{
			description:   "nothing",
			body:          ``,
			expectedValue: &manifest.Schema2Config{},
		},
		{
			description:   "empty",
			body:          `{}`,
			expectedValue: &manifest.Schema2Config{},
		},
		{
			description:   "user",
			body:          `{"User":"0:0"}`,
			expectedValue: &manifest.Schema2Config{User: "0:0"},
		},
		{
			description:   "malformed",
			body:          `{"User":`,
			expectedError: true,
		},
	}
	t.Run("no reader", func(t *testing.T) {
		value, err := DecodeOverrideConfig(nil)
		assert.NoErrorf(t, err, "decoding nothing")
		assert.NotNilf(t, value, "decoded value was unexpectedly nil")
	})
	for _, testCase := range testCases {
		t.Run(testCase.description, func(t *testing.T) {
			value, err := DecodeOverrideConfig(strings.NewReader(testCase.body))
			if testCase.expectedError {
				assert.Errorf(t, err, "decoding sample data")
			} else {
				assert.NoErrorf(t, err, "decoding sample data")
				assert.NotNilf(t, value, "decoded value was unexpectedly nil")
				assert.Equalf(t, *testCase.expectedValue, *value, "decoded value was not what we expected")
			}
		})
	}
}
