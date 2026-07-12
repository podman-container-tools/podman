//go:build freebsd && !remote

package system

// Currently, we only need servicereaper on Linux for rootless networking.
func maybeStartServiceReaper() {
}

// There is no cgroup on non linux.
func maybeMoveToSubCgroup() {}
