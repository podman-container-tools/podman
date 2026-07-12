//go:build (windows || plan9) && !unix

package xattr

import "github.com/hugelgupf/p9/linux"

func List(p string) ([]string, error) {
	return nil, linux.ENOSYS
}

func Get(p string, attr string) ([]byte, error) {
	return nil, linux.ENOSYS
}
