//go:build !remote

package emulation

import "github.com/sirupsen/logrus"

// Registered returns a list of platforms for which we think we have user
// space emulation available.
func Registered() []string {
	binfmt, err := registeredBinfmtMisc()
	if err != nil {
		logrus.Warnf("registeredBinfmtMisc(): %v", err)
		return nil
	}
	registered := make([]string, 0, len(binfmt))
	registered = append(registered, binfmt...)
	return registered
}
