package machine

import (
	"testing"
	"time"
)

// TestSSHCommandTimeoutConstant verifies the timeout constant is reasonable
func TestSSHCommandTimeoutConstant(t *testing.T) {
	// The timeout should be long enough for normal operations
	// but short enough to fail quickly when hung
	if sshCommandTimeout < 10*time.Second {
		t.Fatalf("sshCommandTimeout too short: %v (should be at least 10s)", sshCommandTimeout)
	}
	if sshCommandTimeout > 5*time.Minute {
		t.Fatalf("sshCommandTimeout too long: %v (should be at most 5m)", sshCommandTimeout)
	}
	t.Logf("sshCommandTimeout is reasonable: %v", sshCommandTimeout)
}
