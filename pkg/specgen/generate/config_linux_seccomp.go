//go:build linux && !remote

package generate

import (
	"context"
	"errors"
	"fmt"
	"os"

	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
	"go.podman.io/common/libimage"
	goSeccomp "go.podman.io/common/pkg/seccomp"
	"go.podman.io/podman/v6/pkg/seccomp"
	"go.podman.io/podman/v6/pkg/specgen"
)

func getSeccompConfig(s *specgen.SpecGenerator, configSpec *spec.Spec, img *libimage.Image) (*spec.LinuxSeccomp, error) {
	var seccompConfig *spec.LinuxSeccomp
	var err error
	scp, err := seccomp.LookupPolicy(s.SeccompPolicy)
	if err != nil {
		return nil, err
	}

	if scp == seccomp.PolicyImage {
		if img == nil {
			return nil, errors.New("cannot read seccomp profile without a valid image")
		}
		labels, err := img.Labels(context.Background())
		if err != nil {
			return nil, err
		}
		imagePolicy := labels[seccomp.ContainerImageLabel]
		if len(imagePolicy) < 1 {
			return nil, errors.New("no seccomp policy defined by image")
		}
		logrus.Debug("Loading seccomp profile from the security config")
		seccompConfig, err = goSeccomp.LoadProfile(imagePolicy, configSpec)
		if err != nil {
			return nil, fmt.Errorf("loading seccomp profile failed: %w", err)
		}
		return seccompConfig, nil
	}

	if s.SeccompProfilePath != "" {
		logrus.Debugf("Loading seccomp profile from %q", s.SeccompProfilePath)
		seccompProfile, err := os.ReadFile(s.SeccompProfilePath)
		if err != nil {
			return nil, fmt.Errorf("opening seccomp profile failed: %w", err)
		}
		seccompConfig, err = goSeccomp.LoadProfile(string(seccompProfile), configSpec)
		if err != nil {
			return nil, fmt.Errorf("loading seccomp profile (%s) failed: %w", s.SeccompProfilePath, err)
		}
	} else {
		logrus.Debug("Loading default seccomp profile")
		seccompConfig, err = goSeccomp.GetDefaultProfile(configSpec)
		if err != nil {
			return nil, fmt.Errorf("loading seccomp profile (%s) failed: %w", s.SeccompProfilePath, err)
		}
	}

	return seccompConfig, nil
}
