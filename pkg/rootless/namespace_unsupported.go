//go:build !linux

package rootless

func getPlatformNamespaceInfo(_ string, _ *RootlessNamespaceInfo) {}
