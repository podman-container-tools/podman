//go:build !remote

package generate

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShouldMask(t *testing.T) {
	tests := []struct {
		mask       string
		unmask     []string
		shouldMask bool
	}{
		{"/proc/foo", []string{"all"}, false},
		{"/proc/foo", []string{"ALL"}, false},
		{"/proc/foo", []string{"/proc/foo"}, false},
		{"/proc/foo", []string{"/proc/*"}, false},
		{"/proc/foo", []string{"/proc/bar", "all"}, false},
		{"/proc/foo", []string{"/proc/f*"}, false},
		{"/proc/foo", []string{"/proc/b*"}, true},
		{"/proc/foo", []string{}, true},
	}
	for _, test := range tests {
		val := shouldMask(test.mask, test.unmask)
		assert.Equal(t, val, test.shouldMask)
	}
}
