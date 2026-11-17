//go:build !amd64 && !arm64

package os

// init do not register _podman machine os_ command on unsupported platforms
func init() {}
