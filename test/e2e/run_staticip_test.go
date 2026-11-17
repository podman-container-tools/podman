//go:build linux || freebsd

package integration

import (
	"fmt"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
	"go.podman.io/storage/pkg/stringid"
)

var _ = Describe("Podman run with --ip flag", func() {
	BeforeEach(func() {
		SkipIfRootless("rootless does not support --ip without network")
	})

	It("Podman run --ip with garbage address", func() {
		result := podmanTest.Podman([]string{"run", "--ip", "114232346", ALPINE, "ls"})
		result.WaitWithDefaultTimeout()
		Expect(result).To(ExitWithError(125, `"114232346" is not an ip address`))
	})

	It("Podman run --ip with v6 address", func() {
		result := podmanTest.Podman([]string{"run", "--ip", "2001:db8:bad:beef::1", ALPINE, "ls"})
		result.WaitWithDefaultTimeout()
		Expect(result).To(ExitWithError(126, "requested static ip 2001:db8:bad:beef::1 not in any subnet on network podman"))
	})

	It("Podman run --ip with non-allocatable IP", func() {
		result := podmanTest.Podman([]string{"run", "--ip", "203.0.113.124", ALPINE, "ls"})
		result.WaitWithDefaultTimeout()
		Expect(result).To(ExitWithError(126, "requested static ip 203.0.113.124 not in any subnet on network podman"))
	})

	It("Podman run with specified static IP has correct IP", func() {
		ip := GetSafeIPAddress()
		result := podmanTest.Podman([]string{"run", "--ip", ip, ALPINE, "ip", "addr"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToString()).To(ContainSubstring(ip + "/16"))
	})

	It("Podman run with specified static IPv6 has correct IP", func() {
		netName := "ipv6-" + stringid.GenerateRandomID()
		ipv6 := "fd46:db93:aa76:ac37::10"
		net := podmanTest.Podman([]string{"network", "create", "--subnet", "fd46:db93:aa76:ac37::/64", netName})
		net.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(netName)
		Expect(net).To(ExitCleanly())

		result := podmanTest.Podman([]string{"run", "--network", netName, "--ip6", ipv6, ALPINE, "ip", "addr"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToString()).To(ContainSubstring(ipv6 + "/64"))
	})

	It("Podman run with --network bridge:ip=", func() {
		ip := GetSafeIPAddress()
		result := podmanTest.Podman([]string{"run", "--network", "bridge:ip=" + ip, ALPINE, "ip", "addr"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToString()).To(ContainSubstring(ip + "/16"))
	})

	It("Podman run with --network net:ip=,mac=,interface_name=", func() {
		ip := GetSafeIPAddress()
		mac := "44:33:22:11:00:99"
		intName := "myeth"
		result := podmanTest.Podman([]string{"run", "--network", "bridge:ip=" + ip + ",mac=" + mac + ",interface_name=" + intName, ALPINE, "ip", "addr"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToString()).To(ContainSubstring(ip + "/16"))
		Expect(result.OutputToString()).To(ContainSubstring(mac))
		Expect(result.OutputToString()).To(ContainSubstring(intName))
	})

	It("Podman run two containers with the same IP", func() {
		ip := GetSafeIPAddress()
		result := podmanTest.Podman([]string{"run", "-d", "--name", "nginx", "--ip", ip, NGINX_IMAGE})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		cid := result.OutputToString()

		// This test should not use a proxy
		client := &http.Client{
			Transport: &http.Transport{
				Proxy: nil,
			},
		}

		for retries := 20; retries > 0; retries-- {
			response, err := client.Get(fmt.Sprintf("http://%s", ip))
			if err == nil && response.StatusCode == http.StatusOK {
				break
			}
			if retries == 1 {
				logps := podmanTest.Podman([]string{"ps", "-a"})
				logps.WaitWithDefaultTimeout()
				logps = podmanTest.Podman([]string{"logs", "nginx"})
				logps.WaitWithDefaultTimeout()
				Fail("Timed out waiting for nginx container, see ps & log above.")
			}

			if err != nil {
				GinkgoWriter.Printf("nginx not ready yet; error=%v; %d retries left...\n", err, retries)
			} else {
				GinkgoWriter.Printf("nginx not ready yet; response=%v; %d retries left...\n", response.StatusCode, retries)
			}
			time.Sleep(1 * time.Second)
		}
		result = podmanTest.Podman([]string{"run", "--ip", ip, ALPINE, "ip", "addr"})
		result.WaitWithDefaultTimeout()
		Expect(result).To(ExitWithError(126, fmt.Sprintf("IPAM error: requested ip address %s is already allocated to container ID %s", ip, cid)))
	})
})
