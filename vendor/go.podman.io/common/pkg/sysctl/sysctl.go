package sysctl

import (
	"fmt"
	"strings"
)

// Validate validates a list of sysctl and returns it.
func Validate(strSlice []string) (map[string]string, error) {
	sysctl := make(map[string]string)
	validSysctlMap := map[string]bool{
		"kernel.msgmax":          true,
		"kernel.msgmnb":          true,
		"kernel.msgmni":          true,
		"kernel.sem":             true,
		"kernel.shmall":          true,
		"kernel.shmmax":          true,
		"kernel.shmmni":          true,
		"kernel.shm_rmid_forced": true,
	}
	validSysctlPrefixes := []string{
		"net.",
		"fs.mqueue.",
	}

	for _, val := range strSlice {
		foundMatch := false
		arr := strings.Split(val, "=")
		if len(arr) < 2 {
			return nil, fmt.Errorf("%s is invalid, sysctl values must be in the form of KEY=VALUE", val)
		}

		trimmed := fmt.Sprintf("%s=%s", strings.TrimSpace(arr[0]), strings.TrimSpace(arr[1]))
		if trimmed != val {
			return nil, fmt.Errorf("%q is invalid, extra spaces found", val)
		}

		if validSysctlMap[arr[0]] {
			sysctl[arr[0]] = arr[1]
			continue
		}

		for _, prefix := range validSysctlPrefixes {
			if strings.HasPrefix(arr[0], prefix) {
				sysctl[arr[0]] = arr[1]
				foundMatch = true
				break
			}
		}
		if !foundMatch {
			return nil, fmt.Errorf("sysctl %q is not allowed", arr[0])
		}
	}
	return sysctl, nil
}
