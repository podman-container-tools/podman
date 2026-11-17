//go:build linux || freebsd

package integration

import (
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("Podman port", func() {
	It("podman port all and latest", func() {
		result := podmanTest.Podman([]string{"port", "-a", "-l"})
		result.WaitWithDefaultTimeout()
		if IsRemote() {
			Expect(result).To(ExitWithError(125, "unknown shorthand flag: 'l' in -l"))
		} else {
			Expect(result).To(ExitWithError(125, "--all and --latest cannot be used together"))
		}
	})

	It("podman port all and extra", func() {
		result := podmanTest.Podman([]string{"port", "-a", "foobar"})
		result.WaitWithDefaultTimeout()
		Expect(result).To(ExitWithError(125, "no arguments are needed with --all"))
	})

	It("podman port -l nginx", func() {
		session, cid := podmanTest.RunNginxWithHealthCheck("test1")
		Expect(session).Should(ExitCleanly())

		if err := podmanTest.RunHealthCheck(cid); err != nil {
			Fail(err.Error())
		}

		if !IsRemote() {
			cid = "-l"
		}
		result := podmanTest.Podman([]string{"port", cid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		port := strings.Split(result.OutputToStringArray()[0], ":")[1]
		Expect(result.OutputToStringArray()).To(ContainElement(HavePrefix(fmt.Sprintf("80/tcp -> 0.0.0.0:%s", port))))
	})

	It("podman container port  -l nginx", func() {
		session, cid := podmanTest.RunNginxWithHealthCheck("")
		Expect(session).Should(ExitCleanly())

		if err := podmanTest.RunHealthCheck(cid); err != nil {
			Fail(err.Error())
		}

		if !IsRemote() {
			cid = "-l"
		}
		result := podmanTest.Podman([]string{"container", "port", cid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		port := strings.Split(result.OutputToStringArray()[0], ":")[1]
		Expect(result.OutputToStringArray()).To(ContainElement(HavePrefix(fmt.Sprintf("80/tcp -> 0.0.0.0:%s", port))))
	})

	It("podman port -l port nginx", func() {
		session, cid := podmanTest.RunNginxWithHealthCheck("")
		Expect(session).Should(ExitCleanly())

		if err := podmanTest.RunHealthCheck(cid); err != nil {
			Fail(err.Error())
		}

		if !IsRemote() {
			cid = "-l"
		}
		result := podmanTest.Podman([]string{"port", cid, "80"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		port := strings.Split(result.OutputToStringArray()[0], ":")[1]
		Expect(result.OutputToStringArray()).To(ContainElement(HavePrefix(fmt.Sprintf("0.0.0.0:%s", port))))
	})

	It("podman port -a nginx", func() {
		session, cid := podmanTest.RunNginxWithHealthCheck("")
		Expect(session).Should(ExitCleanly())

		if err := podmanTest.RunHealthCheck(cid); err != nil {
			Fail(err.Error())
		}

		result := podmanTest.Podman([]string{"port", "-a"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
	})

	It("podman port nginx by name", func() {
		session, cid := podmanTest.RunNginxWithHealthCheck("portcheck")
		Expect(session).Should(ExitCleanly())

		if err := podmanTest.RunHealthCheck(cid); err != nil {
			Fail(err.Error())
		}

		result := podmanTest.Podman([]string{"port", "portcheck"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToStringArray()).To(ContainElement(HavePrefix("80/tcp -> 0.0.0.0:")))
	})

	It("podman port multiple ports", func() {
		// Acquire and release locks
		lock1 := GetPortLock("5010")
		defer lock1.Unlock()
		lock2 := GetPortLock("5011")
		defer lock2.Unlock()

		setup := podmanTest.Podman([]string{"run", "--name", "test", "-dt", "-p", "5010:5000", "-p", "5011:5001", ALPINE, "top"})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(ExitCleanly())

		// Check that the first port was honored
		result1 := podmanTest.Podman([]string{"port", "test", "5000"})
		result1.WaitWithDefaultTimeout()
		Expect(result1).Should(ExitCleanly())
		Expect(result1.OutputToStringArray()).To(ContainElement(HavePrefix("0.0.0.0:5010")))

		// Check that the second port was honored
		result2 := podmanTest.Podman([]string{"port", "test", "5001"})
		result2.WaitWithDefaultTimeout()
		Expect(result2).Should(ExitCleanly())
		Expect(result2.OutputToStringArray()).To(ContainElement(HavePrefix("0.0.0.0:5011")))
	})
})
