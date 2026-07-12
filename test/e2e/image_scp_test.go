//go:build !remote_testing && (linux || freebsd)

package integration

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
	"go.podman.io/storage/pkg/homedir"
)

var _ = Describe("podman image scp", func() {
	BeforeEach(setupConnectionsConf)

	It("podman image scp bogus image", func() {
		scp := podmanTest.Podman([]string{"image", "scp", "FOOBAR"})
		scp.WaitWithDefaultTimeout()
		Expect(scp).Should(ExitWithError(125, "must specify a destination: invalid argument"))
	})

	It("podman image scp with proper connection", func() {
		if _, err := os.Stat(filepath.Join(homedir.Get(), ".ssh", "known_hosts")); err != nil {
			Skip("known_hosts does not exist or is not accessible")
		}

		ensureImage := podmanTest.Podman([]string{"pull", "-q", ALPINE})
		ensureImage.WaitWithDefaultTimeout()
		Expect(ensureImage).Should(ExitCleanly())

		cmd := []string{
			"system", "connection", "add",
			"--default",
			"QA",
			"ssh://root@podman.test:2222/run/podman/podman.sock",
		}
		session := podmanTest.Podman(cmd)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		scp := podmanTest.Podman([]string{"image", "scp", ALPINE, "QA::"})
		scp.WaitWithDefaultTimeout()
		// exit with error because we cannot make an actual ssh connection
		// This tests that the input we are given is validated and prepared correctly
		// The error given should either be a missing image (due to testing suite complications) or a no such host timeout on ssh
		Expect(scp).Should(ExitWithError(125, "failed to connect: dial tcp: lookup "))
	})

	It("podman image scp preserves a username containing an @", func() {
		// The user-to-user transfer path that looks up the local username
		// only runs rootful.
		SkipIfRootless("the local user lookup only happens during a rootful transfer")

		// Regression test for https://github.com/containers/podman/issues/27655:
		// a username that itself contains an "@" (e.g. an Active Directory
		// "user@domain") must be parsed as a whole. Before the fix it was
		// truncated at the first "@", so the lookup failed for "user" instead
		// of "user@domain". The lookup happens before any image is touched, so
		// the bogus user is enough to exercise the parsing.
		scp := podmanTest.Podman([]string{"image", "scp", "user@domain@localhost::" + ALPINE})
		scp.WaitWithDefaultTimeout()
		Expect(scp).Should(ExitWithError(125, "unknown user user@domain"))
	})
})
