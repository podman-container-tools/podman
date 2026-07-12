//go:build !amd64 && !arm64

package machine

// init do not register _podman machine_ command on unsupported platforms
func init() {}
