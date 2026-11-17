//go:build !remote && (linux || freebsd)

package kube

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"go.podman.io/podman/v6/libpod"
	v1 "go.podman.io/podman/v6/pkg/k8s.io/api/core/v1"
)

// SeccompAnnotationPaths holds information about a pod YAML's seccomp configuration
// it holds both container and pod seccomp paths
type SeccompAnnotationPaths struct {
	containerPaths map[string]string
	podPath        string
}

// FindForContainer checks whether a container has a seccomp path configured for it.
func (k *SeccompAnnotationPaths) FindForContainer(ctrName string) (string, bool) {
	path, ok := k.containerPaths[ctrName]
	return path, ok
}

// InitializeSeccompAnnotationPaths takes annotations from the pod object metadata and finds annotations pertaining to seccomp
// it parses both pod and container level
// if the annotation is of the form "localhost/%s", the seccomp profile will be set to profileRoot/%s
func InitializeSeccompAnnotationPaths(annotations map[string]string, profileRoot string) (*SeccompAnnotationPaths, error) {
	seccompAnnotationPaths := &SeccompAnnotationPaths{containerPaths: make(map[string]string)}
	var err error
	if annotations != nil {
		for annKeyValue, seccomp := range annotations {
			// check if it is prefaced with container.seccomp.security.alpha.kubernetes.io/
			prefixAndCtr := strings.Split(annKeyValue, "/")
			// TODO: Remove support for deprecated Kubernetes seccomp annotations in Podman 7.
			// See https://github.com/containers/podman/issues/27501.
			//nolint:staticcheck // deprecated k8s annotation constant
			if prefixAndCtr[0]+"/" != v1.SeccompContainerAnnotationKeyPrefix {
				continue
			} else if len(prefixAndCtr) != 2 {
				// this could be caused by a user inputting either of
				// container.seccomp.security.alpha.kubernetes.io{,/}
				// both of which are invalid
				return nil, fmt.Errorf("invalid seccomp path: %s", prefixAndCtr[0])
			}

			path, err := verifySeccompPath(seccomp, profileRoot)
			if err != nil {
				return nil, err
			}
			seccompAnnotationPaths.containerPaths[prefixAndCtr[1]] = path
		}
		// TODO: Remove support for deprecated Kubernetes seccomp annotations in Podman 7.
		// See https://github.com/containers/podman/issues/27501.
		//nolint:staticcheck // deprecated k8s annotation constant
		podSeccomp, ok := annotations[v1.SeccompPodAnnotationKey]
		if ok {
			seccompAnnotationPaths.podPath, err = verifySeccompPath(podSeccomp, profileRoot)
		}
		if err != nil {
			return nil, err
		}
	}
	return seccompAnnotationPaths, nil
}

// verifySeccompPath takes a path and checks whether it is a default, unconfined, or a path
// the available options are parsed as defined in https://kubernetes.io/docs/concepts/policy/pod-security-policy/#seccomp
func verifySeccompPath(path string, profileRoot string) (string, error) {
	switch path {
	// TODO: Remove support for deprecated Kubernetes seccomp annotations in Podman 7.
	// See https://github.com/containers/podman/issues/27501.
	//nolint:staticcheck // deprecated k8s seccomp constant
	case v1.DeprecatedSeccompProfileDockerDefault:
		fallthrough
	// TODO: Remove support for deprecated Kubernetes seccomp annotations in Podman 7.
	// See https://github.com/containers/podman/issues/27501.
	//nolint:staticcheck // deprecated k8s seccomp constant
	case v1.SeccompProfileRuntimeDefault:
		return libpod.DefaultSeccompPath()
	case "unconfined":
		return path, nil
	default:
		if profile, ok := strings.CutPrefix(path, "localhost/"); ok {
			if profile == "" {
				return "", fmt.Errorf("invalid seccomp path: %s", path)
			}
			if err := validateSeccompLocalhostProfile(profile); err != nil {
				return "", fmt.Errorf("invalid seccomp profile %q: %w", profile, err)
			}
			return filepath.Join(profileRoot, filepath.FromSlash(profile)), nil
		}
		return "", fmt.Errorf("invalid seccomp path: %s", path)
	}
}

// validateSeccompLocalhostProfile ensures that targetPath is relative
// and does not contain any ".." path elements.
func validateSeccompLocalhostProfile(targetPath string) error {
	if filepath.IsAbs(targetPath) {
		return fmt.Errorf("must be a relative path")
	}

	if slices.Contains(strings.Split(filepath.ToSlash(targetPath), "/"), "..") {
		return fmt.Errorf("must not contain '..'")
	}

	return nil
}
