package systemd

import (
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSocketActivated(t *testing.T) {
	assert := assert.New(t)

	assert.False(SocketActivated())

	// different pid
	t.Setenv("LISTEN_PID", "1")
	assert.False(SocketActivated())

	// same pid no fds
	t.Setenv("LISTEN_PID", strconv.Itoa(os.Getpid()))
	t.Setenv("LISTEN_FDS", "0")
	assert.False(SocketActivated())

	// same pid some fds
	t.Setenv("LISTEN_FDS", "1")
	assert.True(SocketActivated())

	// FDNAME is ok too (but not required)
	t.Setenv("LISTEN_FDNAMES", "/meshuggah/rocks")
	assert.True(SocketActivated())
}
