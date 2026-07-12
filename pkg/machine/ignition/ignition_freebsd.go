//go:build freebsd

package ignition

func getLocalTimeZone() (string, error) {
	return "", nil
}
