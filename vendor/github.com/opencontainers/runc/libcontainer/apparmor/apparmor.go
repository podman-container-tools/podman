// Package apparmor provides a minimal set of helpers to configure the AppArmor
// profile of the current process, effectively acting as a very stripped-down
// version of libapparmor.
package apparmor

import "errors"

// IsEnabled returns true if apparmor is enabled for the host.
func IsEnabled() bool {
	return isEnabled()
}

// ApplyProfile will apply the profile with the specified name to the process
// after the next exec. It is only supported on Linux and produces an
// [ErrApparmorNotEnabled] on other platforms.
func ApplyProfile(name string) error {
	return applyProfile(name)
}

// ErrApparmorNotEnabled indicates that AppArmor is not enabled or not supported.
var ErrApparmorNotEnabled = errors.New("apparmor: config provided but apparmor not supported")
