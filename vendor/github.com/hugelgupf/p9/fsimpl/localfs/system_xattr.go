//go:build unix && !solaris && !openbsd

package localfs

import (
	"github.com/hugelgupf/p9/fsimpl/xattr"
	"github.com/hugelgupf/p9/p9"
	"golang.org/x/sys/unix"
)

func (l *Local) SetXattr(attr string, data []byte, flags p9.XattrFlags) error {
	return unix.Setxattr(l.path, attr, data, int(flags))
}

func (l *Local) ListXattrs() ([]string, error) {
	return xattr.List(l.path)
}

func (l *Local) GetXattr(attr string) ([]byte, error) {
	return xattr.Get(l.path, attr)
}

func (l *Local) RemoveXattr(attr string) error {
	return unix.Removexattr(l.path, attr)
}
