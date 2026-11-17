//go:build !remote

package registry

func init() {
	abiSupport = true
}
