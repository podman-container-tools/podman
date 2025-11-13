package crutils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCRRuntimeIsGVisor(t *testing.T) {
	assert.True(t, CRRuntimeIsGVisor("/usr/local/bin/runsc"))
	assert.True(t, CRRuntimeIsGVisor("runsc"))
	assert.False(t, CRRuntimeIsGVisor("/usr/bin/crun"))
	assert.False(t, CRRuntimeIsGVisor("/usr/bin/runc"))
	assert.False(t, CRRuntimeIsGVisor("/opt/runsc/bin/crun"))
}

func TestCRRuntimeSupportsCheckpointRestoreGVisor(t *testing.T) {
	// runsc is not executed, so support does not depend on its exit status.
	assert.True(t, CRRuntimeSupportsCheckpointRestore("/nonexistent/runsc"))
	assert.False(t, CRRuntimeSupportsCheckpointRestore("/nonexistent/crun"))
}
