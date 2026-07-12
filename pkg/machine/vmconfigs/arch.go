package vmconfigs

import (
	"fmt"
)

func NormalizeMachineArch(arch string) (string, error) {
	switch arch {
	case "arm64", "aarch64":
		return "aarch64", nil
	case "x86_64", "amd64":
		return "x86_64", nil
	}
	return "", fmt.Errorf("unsupported platform %s", arch)
}
