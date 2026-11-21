//go:build windows

package hyperv

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetCurrentUserSIDFromTokenIsNonNil verifies that the token-based SID
// lookup works in a normal execution context.
func TestGetCurrentUserSIDFromTokenIsNonNil(t *testing.T) {
	t.Parallel()
	sid, err := getCurrentUserSIDFromToken()
	require.NoError(t, err, "getCurrentUserSIDFromToken should succeed in a normal test environment")
	assert.NotNil(t, sid, "returned SID should be non-nil")
}

// TestGetCurrentUserSIDIsNonNil verifies that getCurrentUserSID returns a
// non-nil SID.  It uses the token path first; a successful result here
// confirms the fallback path is not needed in a normal context.
func TestGetCurrentUserSIDIsNonNil(t *testing.T) {
	t.Parallel()
	sid, err := getCurrentUserSID()
	require.NoError(t, err, "getCurrentUserSID should not fail in a normal test environment")
	assert.NotNil(t, sid, "returned SID should be non-nil")
}

// TestGetCurrentUserSIDFromTokenAndCurrentConsistent verifies that the two
// SID-retrieval strategies return equal SIDs for the same process.
func TestGetCurrentUserSIDFromTokenAndCurrentConsistent(t *testing.T) {
	t.Parallel()
	tokenSID, err := getCurrentUserSIDFromToken()
	require.NoError(t, err)

	fallbackSID, err := getCurrentUserSID()
	require.NoError(t, err)

	// Both paths should identify the same user.
	assert.True(t, tokenSID.Equals(fallbackSID), "token SID and getCurrentUserSID SID should be equal")
}

// TestIsHyperVAdminsGroupMemberDoesNotPanic verifies the function returns
// without panicking in any execution context.
func TestIsHyperVAdminsGroupMemberDoesNotPanic(t *testing.T) {
	t.Parallel()
	_ = IsHyperVAdminsGroupMember()
}

// TestIsCurrentUserInHyperVAdminGroupDoesNotPanic verifies the function does
// not panic and returns a typed result.
func TestIsCurrentUserInHyperVAdminGroupDoesNotPanic(t *testing.T) {
	t.Parallel()
	inGroup, err := IsCurrentUserInHyperVAdminGroup()
	if err != nil {
		t.Logf("IsCurrentUserInHyperVAdminGroup returned error: %v", err)
		return
	}
	t.Logf("IsCurrentUserInHyperVAdminGroup returned inGroup=%v", inGroup)
}

// TestVerifyHyperVPermissionsReturnsKnownErrors verifies that
// VerifyHyperVPermissions either succeeds or returns one of the known typed errors.
func TestVerifyHyperVPermissionsReturnsKnownErrors(t *testing.T) {
	t.Parallel()
	err := VerifyHyperVPermissions()
	if err == nil {
		t.Log("VerifyHyperVPermissions: caller has Hyper-V permissions")
		return
	}

	knownErrors := []error{
		ErrHypervUserNotInAdminGroup,
		ErrHypervUserSessionNotUpdated,
	}
	for _, known := range knownErrors {
		if err == known {
			t.Logf("VerifyHyperVPermissions returned expected error: %v", err)
			return
		}
	}
	t.Errorf("VerifyHyperVPermissions returned unexpected error: %v", err)
}

// TestLookupOrResolveSIDWithEmptyUsername verifies that an empty username
// triggers the token-based fallback and still returns a non-nil SID.
func TestLookupOrResolveSIDWithEmptyUsername(t *testing.T) {
	t.Parallel()
	sid, err := lookupOrResolveSID("")
	require.NoError(t, err)
	assert.NotNil(t, sid)
}
