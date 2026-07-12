//go:build !remote && (linux || freebsd)

package libpod

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_generateName(t *testing.T) {
	state, _ := getEmptySqliteState(t)

	r := &Runtime{
		state: state,
	}

	// Test that (*Runtime).generateName returns different names
	// if called twice.
	n1, _ := r.generateName()
	n2, _ := r.generateName()
	assert.NotEqual(t, n1, n2)
}
