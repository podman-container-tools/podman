//go:build windows

package ignition

func getLocalTimeZone() (string, error) {
	return "", nil
}
