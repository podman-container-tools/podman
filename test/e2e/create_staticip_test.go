//go:build linux || freebsd

package integration

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("Podman create with --ip flag", func() {
	It("Podman create --ip with garbage address", func() {
		result := podmanTest.Podman([]string{"create", "--name", "test", "--ip", "114232346", ALPINE, "ls"})
		result.WaitWithDefaultTimeout()
		Expect(result).To(ExitWithError(125, `"114232346" is not an ip address`))
	})

	It("Podman create --ip with non-allocatable IP", func() {
		SkipIfRootless("--ip not supported without network in rootless mode")
		result := podmanTest.Podman([]string{"create", "--name", "test", "--ip", "203.0.113.124", ALPINE, "ls"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())

		result = podmanTest.Podman([]string{"start", "test"})
		result.WaitWithDefaultTimeout()
		Expect(result).To(ExitWithError(125, "requested static ip 203.0.113.124 not in any subnet on network podman"))
	})

	It("Podman create with specified static IP has correct IP", func() {
		ip := GetSafeIPAddress()
		result := podmanTest.Podman([]string{"create", "--name", "test", "--ip", ip, ALPINE, "ip", "addr"})
		result.WaitWithDefaultTimeout()
		// Rootless static ip assignment without network should error
		if isRootless() {
			Expect(result).Should(ExitWithError(125, "invalid config provided: networks and static ip/mac address can only be used with Bridge mode networking"))
		} else {
			Expect(result).Should(ExitCleanly())

			result = podmanTest.Podman([]string{"start", "-a", "test"})
			result.WaitWithDefaultTimeout()
			Expect(result).Should(ExitCleanly())
			Expect(result.OutputToString()).To(ContainSubstring(ip + "/16"))
		}
	})

	It("Podman create two containers with the same IP", func() {
		SkipIfRootless("--ip not supported without network in rootless mode")
		ip := GetSafeIPAddress()
		result := podmanTest.Podman([]string{"create", "--log-driver", "k8s-file", "--name", "test1", "--ip", ip, ALPINE, "sleep", "999"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		result = podmanTest.Podman([]string{"create", "--log-driver", "k8s-file", "--name", "test2", "--ip", ip, ALPINE, "ip", "addr"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		result = podmanTest.Podman([]string{"start", "test1"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())

		// race prevention: wait until IP address is assigned and
		// container is running.
		for range 5 {
			result = podmanTest.Podman([]string{"inspect", "--format", "{{.State.Status}} {{.NetworkSettings.IPAddress}}", "test1"})
			result.WaitWithDefaultTimeout()
			Expect(result).Should(ExitCleanly())
			if result.OutputToString() == "running "+ip {
				break
			}
			time.Sleep(1 * time.Second)
		}
		Expect(result.OutputToString()).To(Equal("running " + ip))

		// test1 container is running with the given IP.
		result = podmanTest.Podman([]string{"start", "-a", "test2"})
		result.WaitWithDefaultTimeout()
		Expect(result).To(ExitWithError(125, fmt.Sprintf("requested ip address %s is already allocated", ip)))
	})
})
