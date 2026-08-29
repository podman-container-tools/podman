//go:build !seccomp || !linux

package buildah

import (
	"fmt"

	"github.com/opencontainers/runtime-spec/specs-go"
)

func setupSeccomp(spec *specs.Spec, seccompProfilePath string) error {
	if seccompProfilePath != "" && seccompProfilePath != "unconfined" {
		return fmt.Errorf("seccomp profile %q requested, but seccomp support is not enabled in this build", seccompProfilePath)
	}
	if spec.Linux != nil {
		// runtime-tools may have supplied us with a default filter
		spec.Linux.Seccomp = nil
	}
	return nil
}
