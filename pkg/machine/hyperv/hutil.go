//go:build windows

package hyperv

import (
	"errors"
	"fmt"
	"os/user"
	"unsafe"

	"github.com/sirupsen/logrus"
	"go.podman.io/podman/v6/pkg/machine/windows"
	syswindows "golang.org/x/sys/windows"
)

var (
	ErrHypervUserNotInAdminGroup             = errors.New("Hyper-V machines require Hyper-V admin rights to be managed. Please add the current user to the Hyper-V Administrators group or run Podman as an administrator")
	ErrHypervRegistryInitRequiresElevation   = errors.New("the first time Podman initializes a Hyper-V machine, it requires admin rights. Please run Podman as an administrator")
	ErrHypervRegistryRemoveRequiresElevation = errors.New("removing this Hyper-V machine requires admin rights to clean up the Windows Registry. Please run Podman as an administrator")
	ErrHypervRegistryUpdateRequiresElevation = errors.New("this machine's configuration requires additional Hyper-V networking (hvsock) entries in the Windows Registry. Please run Podman as an administrator")
	ErrHypervLegacyMachineRequiresElevation  = errors.New("starting or stopping Hyper-V machines created with Podman 5.x or earlier requires admin rights. Please run Podman as an administrator")
	ErrHypervPrepareHostForHyperV            = errors.New("podman needs to prepare the host to run Hyper-V machines without requiring Administrator rights in the future. This involves a one-time setup of the Windows Registry and adding your account to the 'Hyper-V Administrators' group")
	ErrHypervUserSessionNotUpdated           = errors.New("we have added you to the Hyper-V Administrators group, but your active session needs to be updated. Log out and log back in to apply the new permissions. Or, if you can't log out, run the command as an administrator")

	// Lazily load the NetAPI32 DLL and the function we need
	modnetapi32                 = syswindows.NewLazySystemDLL("netapi32.dll")
	procNetLocalGroupAddMembers = modnetapi32.NewProc("NetLocalGroupAddMembers")
	procNetLocalGroupDelMembers = modnetapi32.NewProc("NetLocalGroupDelMembers")
	procNetLocalGroupGetMembers = modnetapi32.NewProc("NetLocalGroupGetMembers")
	procNetApiBufferFree        = modnetapi32.NewProc("NetApiBufferFree")
)

type localGroupMembersInfo0 struct {
	Sid *syswindows.SID
}

func IsHyperVAdminsGroupMember() bool {
	sid, err := syswindows.CreateWellKnownSid(syswindows.WinBuiltinHyperVAdminsSid)
	if err != nil {
		return false
	}

	//  From MS docs:
	// "If TokenHandle is NULL, CheckTokenMembership uses the impersonation
	//  token of the calling thread. If the thread is not impersonating,
	//  the function duplicates the thread's primary token to create an
	//  impersonation token."
	token := syswindows.Token(0)
	member, err := token.IsMember(sid)
	if err != nil {
		logrus.Warnf("Token Membership Error: %s", err)
		return false
	}

	return member
}

// AddUserToHyperVAdminGroup adds the specified user to the local Hyper-V Administrators group.
// The Win32 syscall logic used here is based on Microsoft's hcsshim implementation:
// https://github.com/microsoft/hcsshim/blob/20ecf50b2ef0c8884448be87d4b6b58b8d62ce94/internal/winapi/user.go#L170
func AddUserToHyperVAdminGroup(username string) error {
	groupPtr, err := getHyperVAdminGroupNamePtr()
	if err != nil {
		return err
	}

	userSid, _, _, err := syswindows.LookupSID("", username)
	if err != nil {
		return fmt.Errorf("failed to look up SID for user %q: %w", username, err)
	}

	info := localGroupMembersInfo0{
		Sid: userSid,
	}

	// C Signature: NET_API_STATUS NetLocalGroupAddMembers(LPCWSTR servername, LPCWSTR groupname, DWORD level, LPBYTE buf, DWORD totalentries)
	ret, _, _ := procNetLocalGroupAddMembers.Call(
		0,                                 // ServerName (0 = local machine)
		uintptr(unsafe.Pointer(groupPtr)), // GroupName
		0,                                 // Level 0 (we are passing a SID)
		uintptr(unsafe.Pointer(&info)),    // Buffer containing our struct
		1,                                 // Total Entries being added
	)

	errno := syswindows.Errno(ret)
	// ERROR_MEMBER_IN_ALIAS = "The specified account name is already a member of the group."
	// https://learn.microsoft.com/en-us/windows/win32/debug/system-error-codes--1300-1699-
	if errno != syswindows.NO_ERROR && errno != syswindows.ERROR_MEMBER_IN_ALIAS {
		return fmt.Errorf("NetLocalGroupAddMembers failed with error: %w", errno)
	}

	return nil
}

// RemoveUserFromHyperVAdminGroup removes the specified user from the local Hyper-V Administrators group.
func RemoveUserFromHyperVAdminGroup(username string) error {
	groupPtr, err := getHyperVAdminGroupNamePtr()
	if err != nil {
		return err
	}

	userSid, _, _, err := syswindows.LookupSID("", username)
	if err != nil {
		return fmt.Errorf("failed to look up SID for user %q: %w", username, err)
	}

	info := localGroupMembersInfo0{
		Sid: userSid,
	}

	// C Signature: NET_API_STATUS NetLocalGroupDelMembers(LPCWSTR servername, LPCWSTR groupname, DWORD level, LPBYTE buf, DWORD totalentries)
	ret, _, _ := procNetLocalGroupDelMembers.Call(
		0,                                 // ServerName (0 = local machine)
		uintptr(unsafe.Pointer(groupPtr)), // GroupName
		0,                                 // Level 0 (we are passing a SID)
		uintptr(unsafe.Pointer(&info)),    // Buffer containing our struct
		1,                                 // Total Entries being removed
	)

	errno := syswindows.Errno(ret)
	// ERROR_MEMBER_NOT_IN_ALIAS = "The specified account name is not a member of the group."
	if errno != syswindows.NO_ERROR && errno != syswindows.ERROR_MEMBER_NOT_IN_ALIAS {
		return fmt.Errorf("NetLocalGroupDelMembers failed with error: %w", errno)
	}

	return nil
}

func getHyperVAdminGroupNamePtr() (*uint16, error) {
	hypervAdminsSid, err := syswindows.CreateWellKnownSid(syswindows.WinBuiltinHyperVAdminsSid)
	if err != nil {
		return nil, fmt.Errorf("failed to create well-known SID for Hyper-V Admins: %w", err)
	}

	// Look up the localized name of the group from the SID
	// (e.g., "Hyper-V Administrators" in EN, "Amministratori Hyper-V" in IT)
	groupName, _, _, err := hypervAdminsSid.LookupAccount("")
	if err != nil {
		return nil, fmt.Errorf("failed to look up group name for Hyper-V Admins: %w", err)
	}

	return syswindows.UTF16PtrFromString(groupName)
}

// getCurrentUserSID retrieves the SID of the user running the current process
func getCurrentUserSID() (*syswindows.SID, error) {
	u, err := user.Current()
	if err != nil {
		return nil, err
	}

	userSid, _, _, err := syswindows.LookupSID("", u.Username)
	if err != nil {
		return nil, fmt.Errorf("failed to look up SID for user %q: %w", u.Username, err)
	}

	return userSid, nil
}

func isUserInGroup(groupPtr *uint16, userSid *syswindows.SID) (bool, error) {
	if userSid == nil {
		return false, errors.New("provided userSid is nil")
	}

	var (
		bufPtr    unsafe.Pointer
		entries   uint32
		total     uint32
		resumeHnd uintptr
	)

	// Call NetLocalGroupGetMembers - it retrieves a list of the members of a particular local group
	// See: https://learn.microsoft.com/en-us/windows/win32/api/lmaccess/nf-lmaccess-netlocalgroupgetmembers
	ret, _, _ := procNetLocalGroupGetMembers.Call(
		0,
		uintptr(unsafe.Pointer(groupPtr)),
		0,
		uintptr(unsafe.Pointer(&bufPtr)),
		uintptr(0xFFFFFFFF),
		uintptr(unsafe.Pointer(&entries)),
		uintptr(unsafe.Pointer(&total)),
		uintptr(unsafe.Pointer(&resumeHnd)),
	)

	defer func() {
		// This buffer is allocated by the system and must be freed using the NetApiBufferFree function.
		// Note that you must free the buffer even if the function fails with ERROR_MORE_DATA.
		// See: https://learn.microsoft.com/en-us/windows/win32/api/lmaccess/nf-lmaccess-netlocalgroupgetmembers
		if bufPtr != nil {
			_, _, _ = procNetApiBufferFree.Call(uintptr(bufPtr))
		}
	}()

	if ret != 0 {
		return false, fmt.Errorf("NetLocalGroupGetMembers failed with code: %d", ret)
	}

	if entries == 0 || bufPtr == nil {
		return false, nil
	}

	// Converts the raw pointer into a slice of the given length
	members := unsafe.Slice((*localGroupMembersInfo0)(bufPtr), entries)

	for _, member := range members {
		if member.Sid != nil && syswindows.EqualSid(member.Sid, userSid) {
			return true, nil
		}
	}

	return false, nil
}

// IsCurrentUserInHyperVAdminGroup checks the group database directly
// (via NetLocalGroupGetMembers) rather than the process token, so it
// reflects membership changes that haven't taken effect in the current
// logon session yet.
func IsCurrentUserInHyperVAdminGroup() (bool, error) {
	userSid, err := getCurrentUserSID()
	if err != nil {
		return false, fmt.Errorf("failed to look up SID for current user: %w", err)
	}

	groupPtr, err := getHyperVAdminGroupNamePtr()
	if err != nil {
		return false, fmt.Errorf("failed to get localized Hyper-V Admin group name: %w", err)
	}

	return isUserInGroup(groupPtr, userSid)
}

// VerifyHyperVPermissions returns `nil` if the user has the privileges to manage
// Hyper-V VMs. This can be either because the command is run
// as an administrator or because the user is a member of the Hyper-V Admins group.
// It returns a specific error otherwise.
// It gracefully detects if a user was added to the admin group but hasn't restarted their session.
func VerifyHyperVPermissions() error {
	if windows.HasAdminRights() || IsHyperVAdminsGroupMember() {
		return nil
	}

	userSid, err := getCurrentUserSID()
	if err != nil {
		return fmt.Errorf("failed to look up SID for current user: %w", err)
	}

	groupPtr, err := getHyperVAdminGroupNamePtr()
	if err != nil {
		return fmt.Errorf("failed to get localized Hyper-V Admin group name: %w", err)
	}

	inGroup, _ := isUserInGroup(groupPtr, userSid)
	if inGroup {
		return ErrHypervUserSessionNotUpdated
	}

	// They are not an Admin, not in the token, and not in the group database.
	return ErrHypervUserNotInAdminGroup
}
