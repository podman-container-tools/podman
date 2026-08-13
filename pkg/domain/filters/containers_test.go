//go:build !remote && (linux || freebsd)

package filters

import "testing"

// Regression test for
// https://github.com/podman-container-tools/podman/issues/28904
// ("Status filter does not accept 'dead' and 'restarting' in list
// containers podman API").
//
// Podman has no internal ContainerStatus that corresponds to Docker's
// "dead" or "restarting" states, but both the libpod and Docker-compat
// `containers/json` API docs list them as valid `status` filter values.
// Before this fix, GenerateContainerFilterFuncs("status", ...) rejected
// them with "unknown container state: ...: invalid argument" (surfaced by
// the API as a 500), instead of accepting the filter and simply matching
// zero containers the way an unmatched, but syntactically valid, Docker
// status filter would.
func TestGenerateContainerFilterFuncsStatusAcceptsDockerOnlyStates(t *testing.T) {
	for _, status := range []string{"dead", "restarting"} {
		fn, err := GenerateContainerFilterFuncs("status", []string{status}, nil)
		if err != nil {
			t.Errorf("status=%q: expected no error, got: %v", status, err)
		}
		if fn == nil {
			t.Errorf("status=%q: expected a non-nil filter func", status)
		}
	}
}

// Sanity check that genuinely invalid status values are still rejected, so
// the fix above does not silently accept arbitrary garbage.
func TestGenerateContainerFilterFuncsStatusStillRejectsUnknownValues(t *testing.T) {
	_, err := GenerateContainerFilterFuncs("status", []string{"not-a-real-status"}, nil)
	if err == nil {
		t.Fatal("expected an error for an unknown, non-Docker status value, got nil")
	}
}

// Known-good status values (including the "stopped" -> "exited" Docker
// compat alias) must keep working unmodified.
func TestGenerateContainerFilterFuncsStatusAcceptsKnownValues(t *testing.T) {
	for _, status := range []string{"created", "running", "paused", "exited", "stopped"} {
		if _, err := GenerateContainerFilterFuncs("status", []string{status}, nil); err != nil {
			t.Errorf("status=%q: expected no error, got: %v", status, err)
		}
	}
}
