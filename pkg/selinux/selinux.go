package selinux

import (
	"github.com/opencontainers/selinux/go-selinux"
)

// KVMLabel returns labels for running kvm isolated containers.
//
// Deprecated: use [selinux.SetProcessKind].
//
//go:fix inline
func KVMLabel(cLabel string) (string, error) {
	return selinux.SetProcessKind(cLabel, selinux.ProcessKindKVM)
}

// InitLabel returns labels for running systemd based containers.
//
// Deprecated: use [selinux.SetProcessKind].
//
//go:fix inline
func InitLabel(cLabel string) (string, error) {
	return selinux.SetProcessKind(cLabel, selinux.ProcessKindInit)
}
