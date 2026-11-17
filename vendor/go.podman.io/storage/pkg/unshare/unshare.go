package unshare

import (
	"fmt"
	"os"
	"os/user"
	"sync"
)

var lookupHomeDir = sync.OnceValues(func() (string, error) {
	usr, err := user.LookupId(fmt.Sprintf("%d", GetRootlessUID()))
	if err != nil {
		return "", fmt.Errorf("unable to resolve HOME directory: %w", err)
	}
	return usr.HomeDir, nil
})

// HomeDir returns the home directory for the current user.
func HomeDir() (string, error) {
	if home := os.Getenv("HOME"); home != "" {
		return home, nil
	}
	return lookupHomeDir()
}
