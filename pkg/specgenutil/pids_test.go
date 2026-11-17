package specgenutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPidsLimitForOCI(t *testing.T) {
	assert.Equal(t, int64(-1), PidsLimitForOCI(0))
	assert.Equal(t, int64(-1), PidsLimitForOCI(-1))
	assert.Equal(t, int64(2048), PidsLimitForOCI(2048))
}
