//go:build linux

package integration

import (
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
	"github.com/vishvananda/netlink"
	"go.podman.io/common/pkg/netns"
	. "go.podman.io/podman/v6/test/utils"
	"go.podman.io/storage/pkg/stringid"
)

var _ = Describe("Podman run networking", func() {
	hostname, _ := os.Hostname()

	It("podman verify network scoped DNS server and also verify updating network dns server", func() {
		// Following test is only functional with netavark and aardvark
		net := createNetworkName("IntTest")
		session := podmanTest.Podman([]string{"network", "create", net, "--dns", "1.1.1.1"})
		session.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(net)
		Expect(session).Should(ExitCleanly())

		results := podmanTest.InspectNetwork(net)
		Expect(results).To(HaveLen(1))
		result := results[0]
		Expect(result.Subnets).To(HaveLen(1))
		aardvarkDNSGateway := result.Subnets[0].Gateway.String()
		Expect(result.NetworkDNSServers).To(Equal([]string{"1.1.1.1"}))

		session = podmanTest.Podman([]string{"run", "-d", "--name", "con1", "--network", net, "busybox", "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"exec", "con1", "nslookup", "google.com", aardvarkDNSGateway})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("Non-authoritative answer: Name: google.com Address:"))

		// Update to a bad DNS Server
		session = podmanTest.Podman([]string{"network", "update", net, "--dns-add", "127.0.0.255"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// Remove good DNS server
		session = podmanTest.Podman([]string{"network", "update", net, "--dns-drop=1.1.1.1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"exec", "con1", "nslookup", "google.com", aardvarkDNSGateway})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(1, ""))
		Expect(session.OutputToString()).To(ContainSubstring(";; connection timed out; no servers could be reached"))
	})

	It("podman network dns multiple servers", func() {
		// Following test is only functional with netavark and aardvark
		net := createNetworkName("IntTest")
		session := podmanTest.Podman([]string{"network", "create", net, "--dns", "1.1.1.1,8.8.8.8", "--dns", "8.4.4.8"})
		session.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(net)
		Expect(session).Should(ExitCleanly())

		results := podmanTest.InspectNetwork(net)
		Expect(results).To(HaveLen(1))
		result := results[0]
		Expect(result.Subnets).To(HaveLen(1))
		aardvarkDNSGateway := result.Subnets[0].Gateway.String()
		Expect(result.NetworkDNSServers).To(Equal([]string{"1.1.1.1", "8.8.8.8", "8.4.4.8"}))

		session = podmanTest.Podman([]string{"run", "-d", "--name", "con1", "--network", net, "busybox", "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"exec", "con1", "nslookup", "google.com", aardvarkDNSGateway})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("Non-authoritative answer: Name: google.com Address:"))

		// Update DNS server
		session = podmanTest.Podman([]string{
			"network", "update", net, "--dns-drop=1.1.1.1,8.8.8.8",
			"--dns-drop", "8.4.4.8", "--dns-add", "127.0.0.253,127.0.0.254", "--dns-add", "127.0.0.255",
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		results = podmanTest.InspectNetwork(net)
		Expect(results).To(HaveLen(1))
		Expect(results[0].NetworkDNSServers).To(Equal([]string{"127.0.0.253", "127.0.0.254", "127.0.0.255"}))

		session = podmanTest.Podman([]string{"exec", "con1", "nslookup", "google.com", aardvarkDNSGateway})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(1, ""))
		Expect(session.OutputToString()).To(ContainSubstring(";; connection timed out; no servers could be reached"))
	})

	It("podman run network connection with default bridge", func() {
		session := podmanTest.RunContainerWithNetworkTest("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman run network connection with host", func() {
		session := podmanTest.RunContainerWithNetworkTest("host")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman run network connection with default", func() {
		session := podmanTest.RunContainerWithNetworkTest("default")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman run network connection with none", func() {
		session := podmanTest.RunContainerWithNetworkTest("none")
		session.WaitWithDefaultTimeout()
		if _, found := os.LookupEnv("http_proxy"); found {
			Expect(session).Should(ExitWithError(5, "Could not resolve proxy:"))
		} else {
			Expect(session).Should(ExitWithError(6, "Could not resolve host: www.redhat.com"))
		}
	})

	It("podman run network connection with private", func() {
		session := podmanTest.RunContainerWithNetworkTest("private")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman verify resolv.conf with --dns + --network", func() {
		// Following test is only functional with netavark and aardvark
		// since new behaviour depends upon output from of statusBlock
		net := createNetworkName("IntTest")
		session := podmanTest.Podman([]string{"network", "create", net})
		session.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(net)
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--name", "con1", "--dns", "1.1.1.1", "--network", net, ALPINE, "cat", "/etc/resolv.conf"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		// Must not contain custom dns server in containers
		// `/etc/resolv.conf` since custom dns-server is
		// already expected to be present and processed by
		// Podman's DNS resolver i.e ( aarvark-dns or dnsname ).
		Expect(session.OutputToString()).ToNot(ContainSubstring("nameserver 1.1.1.1"))
		// But /etc/resolve.conf must contain other nameserver
		// i.e dns server configured for network.
		Expect(session.OutputToString()).To(ContainSubstring("nameserver"))

		session = podmanTest.Podman([]string{"run", "--name", "con2", "--dns", "1.1.1.1", ALPINE, "cat", "/etc/resolv.conf"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		// All the networks being used by following container
		// don't have dns_enabled in such scenario `/etc/resolv.conf`
		// must contain nameserver which were specified via `--dns`.
		Expect(session.OutputToString()).To(ContainSubstring("nameserver 1.1.1.1"))
	})

	It("podman run -p 80", func() {
		name := "testctr"
		session := podmanTest.Podman([]string{"create", "-t", "-p", "80", "--name", name, ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		inspectOut := podmanTest.InspectContainer(name)
		Expect(inspectOut).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"]).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0].HostPort).To(Not(Equal("80")))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0]).To(HaveField("HostIP", "0.0.0.0"))
	})

	It("podman run -p 80-82 -p 8090:8090", func() {
		name := "testctr"
		session := podmanTest.Podman([]string{"create", "-t", "-p", "80-82", "-p", "8090:8090", "--name", name, ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		inspectOut := podmanTest.InspectContainer(name)
		Expect(inspectOut).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports).To(HaveLen(4))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"]).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0].HostPort).To(Not(Equal("80")))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0]).To(HaveField("HostIP", "0.0.0.0"))
		Expect(inspectOut[0].NetworkSettings.Ports["81/tcp"]).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["81/tcp"][0].HostPort).To(Not(Equal("81")))
		Expect(inspectOut[0].NetworkSettings.Ports["81/tcp"][0]).To(HaveField("HostIP", "0.0.0.0"))
		Expect(inspectOut[0].NetworkSettings.Ports["82/tcp"]).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["82/tcp"][0].HostPort).To(Not(Equal("82")))
		Expect(inspectOut[0].NetworkSettings.Ports["82/tcp"][0]).To(HaveField("HostIP", "0.0.0.0"))
		Expect(inspectOut[0].NetworkSettings.Ports["8090/tcp"]).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["8090/tcp"][0]).To(HaveField("HostPort", "8090"))
		Expect(inspectOut[0].NetworkSettings.Ports["8090/tcp"][0]).To(HaveField("HostIP", "0.0.0.0"))
	})

	It("podman run -p 80-81 -p 8180-8181", func() {
		name := "testctr"
		session := podmanTest.Podman([]string{"create", "-t", "-p", "80-81", "-p", "8180-8181", "--name", name, ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		inspectOut := podmanTest.InspectContainer(name)
		Expect(inspectOut).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports).To(HaveLen(4))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"]).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0].HostPort).To(Not(Equal("80")))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0]).To(HaveField("HostIP", "0.0.0.0"))
		Expect(inspectOut[0].NetworkSettings.Ports["81/tcp"]).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["81/tcp"][0].HostPort).To(Not(Equal("81")))
		Expect(inspectOut[0].NetworkSettings.Ports["81/tcp"][0]).To(HaveField("HostIP", "0.0.0.0"))
		Expect(inspectOut[0].NetworkSettings.Ports["8180/tcp"]).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["8180/tcp"][0].HostPort).To(Not(Equal("8180")))
		Expect(inspectOut[0].NetworkSettings.Ports["8180/tcp"][0]).To(HaveField("HostIP", "0.0.0.0"))
		Expect(inspectOut[0].NetworkSettings.Ports["8181/tcp"]).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["8181/tcp"][0].HostPort).To(Not(Equal("8181")))
		Expect(inspectOut[0].NetworkSettings.Ports["8181/tcp"][0]).To(HaveField("HostIP", "0.0.0.0"))
	})

	It("podman run -p 80 -p 8280-8282:8280-8282", func() {
		name := "testctr"
		session := podmanTest.Podman([]string{"create", "-t", "-p", "80", "-p", "8280-8282:8280-8282", "--name", name, ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		inspectOut := podmanTest.InspectContainer(name)
		Expect(inspectOut).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports).To(HaveLen(4))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"]).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0].HostPort).To(Not(Equal("80")))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0]).To(HaveField("HostIP", "0.0.0.0"))
		Expect(inspectOut[0].NetworkSettings.Ports["8280/tcp"]).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["8280/tcp"][0]).To(HaveField("HostPort", "8280"))
		Expect(inspectOut[0].NetworkSettings.Ports["8280/tcp"][0]).To(HaveField("HostIP", "0.0.0.0"))
		Expect(inspectOut[0].NetworkSettings.Ports["8281/tcp"]).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["8281/tcp"][0]).To(HaveField("HostPort", "8281"))
		Expect(inspectOut[0].NetworkSettings.Ports["8281/tcp"][0]).To(HaveField("HostIP", "0.0.0.0"))
		Expect(inspectOut[0].NetworkSettings.Ports["8282/tcp"]).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["8282/tcp"][0]).To(HaveField("HostPort", "8282"))
		Expect(inspectOut[0].NetworkSettings.Ports["8282/tcp"][0]).To(HaveField("HostIP", "0.0.0.0"))
	})

	It("podman run -p 8380:80", func() {
		name := "testctr"
		session := podmanTest.Podman([]string{"create", "-t", "-p", "8380:80", "--name", name, ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		inspectOut := podmanTest.InspectContainer(name)
		Expect(inspectOut).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"]).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0]).To(HaveField("HostPort", "8380"))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0]).To(HaveField("HostIP", "0.0.0.0"))
	})

	It("podman run -p 8480:80/TCP", func() {
		name := "testctr"
		// "TCP" in upper characters
		session := podmanTest.Podman([]string{"create", "-t", "-p", "8480:80/TCP", "--name", name, ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		inspectOut := podmanTest.InspectContainer(name)
		Expect(inspectOut).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports).To(HaveLen(1))
		// "tcp" in lower characters
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"]).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0]).To(HaveField("HostPort", "8480"))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0]).To(HaveField("HostIP", "0.0.0.0"))
	})

	It("podman run -p 80/udp", func() {
		name := "testctr"
		session := podmanTest.Podman([]string{"create", "-t", "-p", "80/udp", "--name", name, ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		inspectOut := podmanTest.InspectContainer(name)
		Expect(inspectOut).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/udp"]).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/udp"][0].HostPort).To(Not(Equal("80")))
		Expect(inspectOut[0].NetworkSettings.Ports["80/udp"][0]).To(HaveField("HostIP", "0.0.0.0"))
	})

	It("podman run -p 127.0.0.1:8580:80", func() {
		name := "testctr"
		session := podmanTest.Podman([]string{"create", "-t", "-p", "127.0.0.1:8580:80", "--name", name, ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		inspectOut := podmanTest.InspectContainer(name)
		Expect(inspectOut).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"]).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0]).To(HaveField("HostPort", "8580"))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0]).To(HaveField("HostIP", "127.0.0.1"))
	})

	It("podman run -p 127.0.0.1:8680:80/udp", func() {
		name := "testctr"
		session := podmanTest.Podman([]string{"create", "-t", "-p", "127.0.0.1:8680:80/udp", "--name", name, ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		inspectOut := podmanTest.InspectContainer(name)
		Expect(inspectOut).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/udp"]).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/udp"][0]).To(HaveField("HostPort", "8680"))
		Expect(inspectOut[0].NetworkSettings.Ports["80/udp"][0]).To(HaveField("HostIP", "127.0.0.1"))
	})

	It("podman run -p [::1]:8780:80/udp", func() {
		name := "testctr"
		session := podmanTest.Podman([]string{"create", "-t", "-p", "[::1]:8780:80/udp", "--name", name, ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		inspectOut := podmanTest.InspectContainer(name)
		Expect(inspectOut).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/udp"]).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/udp"][0]).To(HaveField("HostPort", "8780"))
		Expect(inspectOut[0].NetworkSettings.Ports["80/udp"][0]).To(HaveField("HostIP", "::1"))
	})

	It("podman run -p [::1]:8880:80/tcp", func() {
		name := "testctr"
		session := podmanTest.Podman([]string{"create", "-t", "-p", "[::1]:8880:80/tcp", "--name", name, ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		inspectOut := podmanTest.InspectContainer(name)
		Expect(inspectOut).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"]).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0]).To(HaveField("HostPort", "8880"))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0]).To(HaveField("HostIP", "::1"))
	})

	It("podman run --expose 80 -P", func() {
		name := "testctr"
		session := podmanTest.Podman([]string{"run", "-d", "--expose", "80", "-P", "--name", name, ALPINE, "sleep", "100"})
		session.WaitWithDefaultTimeout()
		inspectOut := podmanTest.InspectContainer(name)
		Expect(inspectOut).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"]).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0].HostPort).To(Not(Equal("0")))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0]).To(HaveField("HostIP", "0.0.0.0"))
	})

	It("podman run --expose 80/udp -P", func() {
		name := "testctr"
		session := podmanTest.Podman([]string{"run", "-d", "--expose", "80/udp", "-P", "--name", name, ALPINE, "sleep", "100"})
		session.WaitWithDefaultTimeout()
		inspectOut := podmanTest.InspectContainer(name)
		Expect(inspectOut).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/udp"]).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/udp"][0].HostPort).To(Not(Equal("0")))
		Expect(inspectOut[0].NetworkSettings.Ports["80/udp"][0]).To(HaveField("HostIP", "0.0.0.0"))
	})

	It("podman run --expose port range", func() {
		session := podmanTest.Podman([]string{"run", "-d", "--expose", "1000-9999", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"ps", "-a", "--format", "{{.Ports}}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		// This must use Equal() to ensure we do not see anything extra
		Expect(session.OutputToString()).To(Equal("1000-9999/tcp"))

		name := "testctr"
		session = podmanTest.Podman([]string{"run", "-d", "--expose", "222-223", "-P", "--name", name, ALPINE, "sleep", "100"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		inspectOut := podmanTest.InspectContainer(name)
		Expect(inspectOut).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports).To(HaveLen(2))
		Expect(inspectOut[0].NetworkSettings.Ports["222/tcp"]).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["222/tcp"][0].HostPort).To(Not(Equal("0")))
		Expect(inspectOut[0].NetworkSettings.Ports["222/tcp"][0]).To(HaveField("HostIP", "0.0.0.0"))
		Expect(inspectOut[0].NetworkSettings.Ports["223/tcp"]).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["223/tcp"][0].HostPort).To(Not(Equal("0")))
		Expect(inspectOut[0].NetworkSettings.Ports["223/tcp"][0]).To(HaveField("HostIP", "0.0.0.0"))
	})

	It("podman run --expose 80 -p 80", func() {
		name := "testctr"
		session := podmanTest.Podman([]string{"create", "-t", "--expose", "80", "-p", "80", "--name", name, ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		inspectOut := podmanTest.InspectContainer(name)
		Expect(inspectOut).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"]).To(HaveLen(1))
		hostPort := inspectOut[0].NetworkSettings.Ports["80/tcp"][0].HostPort
		Expect(hostPort).To(Not(Equal("80")))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0]).To(HaveField("HostIP", "0.0.0.0"))

		session = podmanTest.Podman([]string{"ps", "-a", "--format", "{{.Ports}}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		// This must use Equal() to ensure we do not see the extra ", 80/tcp" from the exposed port
		Expect(session.OutputToString()).To(Equal("0.0.0.0:" + hostPort + "->80/tcp"))
	})

	It("podman run --publish-all with EXPOSE port ranges in Dockerfile", func() {
		// Test port ranges, range with protocol and with an overlapping port
		podmanTest.AddImageToRWStore(ALPINE)
		dockerfile := fmt.Sprintf(`FROM %s
EXPOSE 2002
EXPOSE 2001-2003
EXPOSE 2004-2005/tcp`, ALPINE)
		imageName := "testimg"
		podmanTest.BuildImage(dockerfile, imageName, "false")

		// Verify that the buildah is just passing through the EXPOSE keys
		inspect := podmanTest.Podman([]string{"inspect", imageName})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		image := inspect.InspectImageJSON()
		Expect(image).To(HaveLen(1))
		Expect(image[0].Config.ExposedPorts).To(HaveLen(3))
		Expect(image[0].Config.ExposedPorts).To(HaveKey("2002/tcp"))
		Expect(image[0].Config.ExposedPorts).To(HaveKey("2001-2003/tcp"))
		Expect(image[0].Config.ExposedPorts).To(HaveKey("2004-2005/tcp"))

		session := podmanTest.Podman([]string{"create", imageName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"ps", "-a", "--format", "{{.Ports}}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		// This must use Equal() to ensure we do not see anything extra
		Expect(session.OutputToString()).To(Equal("2001-2005/tcp"))

		containerName := "testcontainer"
		session = podmanTest.Podman([]string{"create", "--publish-all", "--name", containerName, imageName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		inspectOut := podmanTest.InspectContainer(containerName)
		Expect(inspectOut).To(HaveLen(1))

		// Inspect the network settings with available ports to be mapped to the host
		// Don't need to verity HostConfig.PortBindings since we used --publish-all
		Expect(inspectOut[0].NetworkSettings.Ports).To(HaveLen(5))
		Expect(inspectOut[0].NetworkSettings.Ports).To(HaveKey("2001/tcp"))
		Expect(inspectOut[0].NetworkSettings.Ports).To(HaveKey("2002/tcp"))
		Expect(inspectOut[0].NetworkSettings.Ports).To(HaveKey("2003/tcp"))
		Expect(inspectOut[0].NetworkSettings.Ports).To(HaveKey("2004/tcp"))
		Expect(inspectOut[0].NetworkSettings.Ports).To(HaveKey("2005/tcp"))
		Expect(inspectOut[0].HostConfig.PublishAllPorts).To(BeTrue())
	})

	It("podman run --net=host --expose includes ports in inspect output", func() {
		containerName := "testctr"
		session := podmanTest.Podman([]string{"run", "--net=host", "--name", containerName, "-d", "--expose", "8080/tcp", NGINX_IMAGE, "sleep", "+inf"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		inspectOut := podmanTest.InspectContainer(containerName)
		Expect(inspectOut).To(HaveLen(1))

		// Ports is empty. ExposedPorts is not.
		Expect(inspectOut[0].NetworkSettings.Ports).To(BeEmpty())

		// 80 from the image, 8080 from the expose
		Expect(inspectOut[0].Config.ExposedPorts).To(HaveLen(2))
		Expect(inspectOut[0].Config.ExposedPorts).To(HaveKey("80/tcp"))
		Expect(inspectOut[0].Config.ExposedPorts).To(HaveKey("8080/tcp"))
	})

	It("podman run --net=container --expose exposed port from own container", func() {
		ctr1 := "test1"
		session1 := podmanTest.Podman([]string{"run", "-d", "--name", ctr1, "--expose", "8080/tcp", ALPINE, "top"})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(ExitCleanly())

		ctr2 := "test2"
		session2 := podmanTest.Podman([]string{"run", "-d", "--name", ctr2, "--net", fmt.Sprintf("container:%s", ctr1), "--expose", "8090/tcp", ALPINE, "top"})
		session2.WaitWithDefaultTimeout()
		Expect(session2).Should(ExitCleanly())

		inspectOut := podmanTest.InspectContainer(ctr2)
		Expect(inspectOut).To(HaveLen(1))
		// Ports will not be populated. ExposedPorts will be.
		Expect(inspectOut[0].NetworkSettings.Ports).To(BeEmpty())
		Expect(inspectOut[0].Config.ExposedPorts).To(HaveLen(1))
		Expect(inspectOut[0].Config.ExposedPorts).To(HaveKey("8090/tcp"))
	})

	It("podman run -p 127.0.0.1::8980/udp", func() {
		name := "testctr"
		session := podmanTest.Podman([]string{"create", "-t", "-p", "127.0.0.1::8980/udp", "--name", name, ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		inspectOut := podmanTest.InspectContainer(name)
		Expect(inspectOut).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["8980/udp"]).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["8980/udp"][0].HostPort).To(Not(Equal("8980")))
		Expect(inspectOut[0].NetworkSettings.Ports["8980/udp"][0]).To(HaveField("HostIP", "127.0.0.1"))
	})

	It("podman run -p :8181", func() {
		name := "testctr"
		session := podmanTest.Podman([]string{"create", "-t", "-p", ":8181", "--name", name, ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		inspectOut := podmanTest.InspectContainer(name)
		Expect(inspectOut).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["8181/tcp"]).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["8181/tcp"][0].HostPort).To(Not(Equal("8181")))
		Expect(inspectOut[0].NetworkSettings.Ports["8181/tcp"][0]).To(HaveField("HostIP", "0.0.0.0"))
	})

	It("podman run -p xxx:8080 -p yyy:8080", func() {
		name := "testctr"
		session := podmanTest.Podman([]string{"create", "-t", "-p", "4444:8080", "-p", "5555:8080", "--name", name, ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		inspectOut := podmanTest.InspectContainer(name)
		Expect(inspectOut).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["8080/tcp"]).To(HaveLen(2))

		hp1 := inspectOut[0].NetworkSettings.Ports["8080/tcp"][0].HostPort
		hp2 := inspectOut[0].NetworkSettings.Ports["8080/tcp"][1].HostPort

		// We can't guarantee order
		Expect((hp1 == "4444" && hp2 == "5555") || (hp1 == "5555" && hp2 == "4444")).To(BeTrue())
	})

	It("podman run -p 0.0.0.0:9280:80", func() {
		name := "testctr"
		session := podmanTest.Podman([]string{"create", "-t", "-p", "0.0.0.0:9280:80", "--name", name, ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		inspectOut := podmanTest.InspectContainer(name)
		Expect(inspectOut).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"]).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0]).To(HaveField("HostPort", "9280"))
		Expect(inspectOut[0].NetworkSettings.Ports["80/tcp"][0]).To(HaveField("HostIP", "0.0.0.0"))
	})

	It("podman run network expose ports in image metadata", func() {
		session := podmanTest.Podman([]string{"create", "--name", "test", "-t", "-P", NGINX_IMAGE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		results := podmanTest.Podman([]string{"inspect", "test"})
		results.WaitWithDefaultTimeout()
		Expect(results).Should(ExitCleanly())
		Expect(results.OutputToString()).To(ContainSubstring(`"80/tcp":`))
	})

	It("podman run network expose duplicate host port results in error", func() {
		port := "8190" // Make sure this isn't used anywhere else

		session := podmanTest.Podman([]string{"run", "--name", "test", "-dt", "-p", port, ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"inspect", "test"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())

		containerConfig := inspect.InspectContainerToJSON()
		Expect(containerConfig[0].NetworkSettings.Ports).To(Not(BeNil()))
		Expect(containerConfig[0].NetworkSettings.Ports).To(HaveKeyWithValue(port+"/tcp", Not(BeNil())))
		Expect(containerConfig[0].NetworkSettings.Ports[port+"/tcp"][0].HostPort).ToNot(Equal(port))
	})

	It("podman run forward sctp protocol", func() {
		SkipIfRootless("sctp protocol only works as root")
		session := podmanTest.Podman([]string{"--log-level=info", "run", "--name=test", "-p", "80/sctp", "-p", "81/sctp", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		// we can only check logrus on local podman
		if !IsRemote() {
			// check that the info message for sctp protocol is only displayed once
			Expect(strings.Count(session.ErrorToString(), "Port reservation for SCTP is not supported")).To(Equal(1), "`Port reservation for SCTP is not supported` is not displayed exactly one time in the logrus logs")
		}
		results := podmanTest.Podman([]string{"inspect", "test"})
		results.WaitWithDefaultTimeout()
		Expect(results).Should(ExitCleanly())
		Expect(results.OutputToString()).To(ContainSubstring(`"80/sctp":`))
		Expect(results.OutputToString()).To(ContainSubstring(`"81/sctp":`))
	})

	It("podman run hostname test", func() {
		session := podmanTest.Podman([]string{"run", "--rm", ALPINE, "printenv", "HOSTNAME"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Not(ContainSubstring(hostname)))
	})

	It("podman run --net host hostname test", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "--net", "host", ALPINE, "printenv", "HOSTNAME"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(hostname))
	})
	It("podman run --net host --uts host hostname test", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "--net", "host", "--uts", "host", ALPINE, "printenv", "HOSTNAME"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(hostname))
	})
	It("podman run --uts host hostname test", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "--uts", "host", ALPINE, "printenv", "HOSTNAME"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(hostname))
	})

	It("podman run --net host --hostname ... hostname test", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "--net", "host", "--hostname", "foobar", ALPINE, "printenv", "HOSTNAME"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("foobar"))
	})

	It("podman run --hostname ... hostname test", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "--hostname", "foobar", ALPINE, "printenv", "HOSTNAME"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("foobar"))
	})

	It("podman run --net container: and --uts container:", func() {
		ctrName := "ctrToJoin"
		ctr1 := podmanTest.RunTopContainer(ctrName)
		ctr1.WaitWithDefaultTimeout()
		Expect(ctr1).Should(ExitCleanly())

		ctr2 := podmanTest.Podman([]string{"run", "-d", "--net=container:" + ctrName, "--uts=container:" + ctrName, ALPINE, "true"})
		ctr2.WaitWithDefaultTimeout()
		Expect(ctr2).Should(ExitCleanly())
	})

	It("podman run --net container: and --add-host should fail", func() {
		ctrName := "ctrToJoin"
		ctr1 := podmanTest.RunTopContainer(ctrName)
		ctr1.WaitWithDefaultTimeout()
		Expect(ctr1).Should(ExitCleanly())

		ctr2 := podmanTest.Podman([]string{"run", "-d", "--net=container:" + ctrName, "--add-host", "host1:127.0.0.1", ALPINE, "true"})
		ctr2.WaitWithDefaultTimeout()
		Expect(ctr2).Should(ExitWithError(125, "cannot set extra host entries when the container is joined to another containers network namespace: invalid configuration"))
	})

	It("podman run --net container: copies hosts and resolv", func() {
		ctrName := "ctr1"
		ctr1 := podmanTest.RunTopContainer(ctrName)
		ctr1.WaitWithDefaultTimeout()
		Expect(ctr1).Should(ExitCleanly())

		// Exec in and modify /etc/resolv.conf and /etc/hosts
		exec1 := podmanTest.Podman([]string{"exec", ctrName, "sh", "-c", "echo nameserver 192.0.2.1 > /etc/resolv.conf"})
		exec1.WaitWithDefaultTimeout()
		Expect(exec1).Should(ExitCleanly())

		exec2 := podmanTest.Podman([]string{"exec", ctrName, "sh", "-c", "echo 192.0.2.2 test1 > /etc/hosts"})
		exec2.WaitWithDefaultTimeout()
		Expect(exec2).Should(ExitCleanly())

		ctrName2 := "ctr2"
		ctr2 := podmanTest.Podman([]string{"run", "-d", "--net=container:" + ctrName, "--name", ctrName2, ALPINE, "top"})
		ctr2.WaitWithDefaultTimeout()
		Expect(ctr2).Should(ExitCleanly())

		exec3 := podmanTest.Podman([]string{"exec", ctrName2, "cat", "/etc/resolv.conf"})
		exec3.WaitWithDefaultTimeout()
		Expect(exec3).Should(ExitCleanly())
		Expect(exec3.OutputToString()).To(ContainSubstring("nameserver 192.0.2.1"))

		exec4 := podmanTest.Podman([]string{"exec", ctrName2, "cat", "/etc/hosts"})
		exec4.WaitWithDefaultTimeout()
		Expect(exec4).Should(ExitCleanly())
		Expect(exec4.OutputToString()).To(ContainSubstring("192.0.2.2 test1"))
	})

	It("podman run /etc/hosts contains --hostname", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "--hostname", "foohostname", ALPINE, "grep", "foohostname", "/etc/hosts"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman run --uidmap /etc/hosts contains --hostname", func() {
		SkipIfRootless("uidmap population of networks not supported for rootless users")
		session := podmanTest.Podman([]string{"run", "--uidmap", "0:100000:1000", "--rm", "--hostname", "foohostname", ALPINE, "grep", "foohostname", "/etc/hosts"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--uidmap", "0:100000:1000", "--rm", "--hostname", "foohostname", "-v", "/etc/hosts:/etc/hosts", ALPINE, "grep", "foohostname", "/etc/hosts"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(1, ""))
	})

	It("podman run network in user created network namespace", func() {
		SkipIfRootless("ip netns is not supported for rootless users")
		if Containerized() {
			Skip("Cannot be run within a container.")
		}
		addXXX := SystemExec("ip", []string{"netns", "add", "xxx"})
		Expect(addXXX).Should(ExitCleanly())
		defer func() {
			delXXX := SystemExec("ip", []string{"netns", "delete", "xxx"})
			Expect(delXXX).Should(ExitCleanly())
		}()

		session := podmanTest.Podman([]string{"run", "-dt", "--net", "ns:/run/netns/xxx", ALPINE, "wget", "www.redhat.com"})
		session.Wait(90)
		Expect(session).Should(ExitCleanly())
	})

	It("podman run n user created network namespace with resolv.conf", func() {
		SkipIfRootless("ip netns is not supported for rootless users")
		if Containerized() {
			Skip("Cannot be run within a container.")
		}
		addXXX2 := SystemExec("ip", []string{"netns", "add", "xxx2"})
		Expect(addXXX2).Should(ExitCleanly())
		defer func() {
			delXXX2 := SystemExec("ip", []string{"netns", "delete", "xxx2"})
			Expect(delXXX2).Should(ExitCleanly())
		}()

		mdXXX2 := SystemExec("mkdir", []string{"-p", "/etc/netns/xxx2"})
		Expect(mdXXX2).Should(ExitCleanly())
		defer os.RemoveAll("/etc/netns/xxx2")

		nsXXX2 := SystemExec("bash", []string{"-c", "echo nameserver 11.11.11.11 > /etc/netns/xxx2/resolv.conf"})
		Expect(nsXXX2).Should(ExitCleanly())

		session := podmanTest.Podman([]string{"run", "--net", "ns:/run/netns/xxx2", ALPINE, "cat", "/etc/resolv.conf"})
		session.Wait(90)
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("11.11.11.11"))
	})

	addAddr := func(cidr string, containerInterface netlink.Link) error {
		_, ipnet, err := net.ParseCIDR(cidr)
		Expect(err).ToNot(HaveOccurred())
		addr := &netlink.Addr{IPNet: ipnet, Label: ""}
		if err := netlink.AddrAdd(containerInterface, addr); err != nil && err != syscall.EEXIST {
			return err
		}
		return nil
	}

	loopbackup := func() {
		lo, err := netlink.LinkByName("lo")
		Expect(err).ToNot(HaveOccurred())
		err = netlink.LinkSetUp(lo)
		Expect(err).ToNot(HaveOccurred())
	}

	linkup := func(name string, mac string, addresses []string) {
		linkAttr := netlink.NewLinkAttrs()
		linkAttr.Name = name
		m, err := net.ParseMAC(mac)
		Expect(err).ToNot(HaveOccurred())
		linkAttr.HardwareAddr = m
		eth := &netlink.Dummy{LinkAttrs: linkAttr}
		err = netlink.LinkAdd(eth)
		Expect(err).ToNot(HaveOccurred())
		err = netlink.LinkSetUp(eth)
		Expect(err).ToNot(HaveOccurred())
		for _, address := range addresses {
			err := addAddr(address, eth)
			Expect(err).ToNot(HaveOccurred())
		}
	}

	routeAdd := func(gateway string) {
		gw := net.ParseIP(gateway)
		route := &netlink.Route{Dst: nil, Gw: gw}
		err = netlink.RouteAdd(route)
		Expect(err).ToNot(HaveOccurred())
	}

	setupNetworkNs := func(networkNSName string) {
		_ = netns.WithNetNSPath("/run/netns/"+networkNSName, func(_ netns.NetNS) error {
			loopbackup()
			linkup("eth0", "46:7f:45:6e:4f:c8", []string{"10.25.40.0/24", "fd04:3e42:4a4e:3381::/64"})
			linkup("eth1", "56:6e:35:5d:3e:a8", []string{"10.88.0.0/16"})

			routeAdd("10.25.40.0")
			return nil
		})
	}

	checkNetworkNsInspect := func(name string) {
		inspectOut := podmanTest.InspectContainer(name)
		Expect(inspectOut[0].NetworkSettings).To(HaveField("IPAddress", "10.25.40.0"))
		Expect(inspectOut[0].NetworkSettings).To(HaveField("IPPrefixLen", 24))
		Expect(inspectOut[0].NetworkSettings.SecondaryIPAddresses).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.SecondaryIPAddresses[0]).To(HaveField("Addr", "10.88.0.0"))
		Expect(inspectOut[0].NetworkSettings.SecondaryIPAddresses[0]).To(HaveField("PrefixLength", 16))
		Expect(inspectOut[0].NetworkSettings).To(HaveField("GlobalIPv6Address", "fd04:3e42:4a4e:3381::"))
		Expect(inspectOut[0].NetworkSettings).To(HaveField("GlobalIPv6PrefixLen", 64))
		Expect(inspectOut[0].NetworkSettings.SecondaryIPv6Addresses).To(BeEmpty())
		Expect(inspectOut[0].NetworkSettings).To(HaveField("MacAddress", "46:7f:45:6e:4f:c8"))
		Expect(inspectOut[0].NetworkSettings.AdditionalMacAddresses).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.AdditionalMacAddresses[0]).To(Equal("56:6e:35:5d:3e:a8"))
		Expect(inspectOut[0].NetworkSettings).To(HaveField("Gateway", "10.25.40.0"))
	}

	It("podman run network inspect fails gracefully on non-reachable network ns", func() {
		SkipIfRootless("ip netns is not supported for rootless users")

		networkNSName := RandomString(12)
		addNamedNetwork := SystemExec("ip", []string{"netns", "add", networkNSName})
		Expect(addNamedNetwork).Should(ExitCleanly())

		setupNetworkNs(networkNSName)

		name := RandomString(12)
		session := podmanTest.Podman([]string{"run", "-d", "--name", name, "--net", "ns:/run/netns/" + networkNSName, ALPINE, "top"})
		session.WaitWithDefaultTimeout()

		// delete the named network ns before inspect
		delNetworkNamespace := SystemExec("ip", []string{"netns", "delete", networkNSName})
		Expect(delNetworkNamespace).Should(ExitCleanly())

		inspectOut := podmanTest.InspectContainer(name)
		Expect(inspectOut[0].NetworkSettings).To(HaveField("IPAddress", ""))
		Expect(inspectOut[0].NetworkSettings.Networks).To(BeEmpty())
	})

	It("podman inspect can handle joined network ns with multiple interfaces", func() {
		SkipIfRootless("ip netns is not supported for rootless users")

		networkNSName := RandomString(12)
		addNamedNetwork := SystemExec("ip", []string{"netns", "add", networkNSName})
		Expect(addNamedNetwork).Should(ExitCleanly())
		defer func() {
			delNetworkNamespace := SystemExec("ip", []string{"netns", "delete", networkNSName})
			Expect(delNetworkNamespace).Should(ExitCleanly())
		}()
		setupNetworkNs(networkNSName)

		name := RandomString(12)
		session := podmanTest.Podman([]string{"run", "--name", name, "--net", "ns:/run/netns/" + networkNSName, ALPINE})
		session.WaitWithDefaultTimeout()

		session = podmanTest.Podman([]string{"container", "rm", name})
		session.WaitWithDefaultTimeout()

		// no network teardown should touch joined network ns interfaces
		session = podmanTest.Podman([]string{"run", "-d", "--replace", "--name", name, "--net", "ns:/run/netns/" + networkNSName, ALPINE, "top"})
		session.WaitWithDefaultTimeout()

		checkNetworkNsInspect(name)
	})

	It("podman do not tamper with joined network ns interfaces", func() {
		SkipIfRootless("ip netns is not supported for rootless users")

		networkNSName := RandomString(12)
		addNamedNetwork := SystemExec("ip", []string{"netns", "add", networkNSName})
		Expect(addNamedNetwork).Should(ExitCleanly())
		defer func() {
			delNetworkNamespace := SystemExec("ip", []string{"netns", "delete", networkNSName})
			Expect(delNetworkNamespace).Should(ExitCleanly())
		}()

		setupNetworkNs(networkNSName)

		name := RandomString(12)
		session := podmanTest.Podman([]string{"run", "--name", name, "--net", "ns:/run/netns/" + networkNSName, ALPINE})
		session.WaitWithDefaultTimeout()

		checkNetworkNsInspect(name)

		name = RandomString(12)
		session = podmanTest.Podman([]string{"run", "--name", name, "--net", "ns:/run/netns/" + networkNSName, ALPINE})
		session.WaitWithDefaultTimeout()

		checkNetworkNsInspect(name)

		// delete container, the network inspect should not change
		session = podmanTest.Podman([]string{"container", "rm", name})
		session.WaitWithDefaultTimeout()

		session = podmanTest.Podman([]string{"run", "-d", "--replace", "--name", name, "--net", "ns:/run/netns/" + networkNSName, ALPINE, "top"})
		session.WaitWithDefaultTimeout()

		checkNetworkNsInspect(name)
	})

	It("podman run network in bogus user created network namespace", func() {
		session := podmanTest.Podman([]string{"run", "-dt", "--net", "ns:/run/netns/xxy", ALPINE, "wget", "www.redhat.com"})
		session.Wait(90)
		Expect(session).To(ExitWithError(125, "faccessat /run/netns/xxy: no such file or directory"))
	})

	It("podman run in custom network with --static-ip", func() {
		netName := stringid.GenerateRandomID()
		ipAddr := "10.25.30.128"
		create := podmanTest.Podman([]string{"network", "create", "--subnet", "10.25.30.0/24", netName})
		create.WaitWithDefaultTimeout()
		Expect(create).Should(ExitCleanly())
		defer podmanTest.removeNetwork(netName)

		run := podmanTest.Podman([]string{"run", "--rm", "--net", netName, "--ip", ipAddr, ALPINE, "ip", "addr"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(ExitCleanly())
		Expect(run.OutputToString()).To(ContainSubstring(ipAddr))
	})

	It("podman network works across user ns", func() {
		netName := createNetworkName("")
		create := podmanTest.Podman([]string{"network", "create", netName})
		create.WaitWithDefaultTimeout()
		Expect(create).Should(ExitCleanly())
		defer podmanTest.removeNetwork(netName)

		name := "nc-server"
		run := podmanTest.Podman([]string{"run", "--log-driver", "k8s-file", "-d", "--name", name, "--net", netName, ALPINE, "nc", "-l", "-p", "9480"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(ExitCleanly())

		// NOTE: we force the k8s-file log driver to make sure the
		// tests are passing inside a container.
		// "sleep" needed to give aardvark-dns time to come up; #16272
		run = podmanTest.Podman([]string{"run", "--log-driver", "k8s-file", "--rm", "--net", netName, "--uidmap", "0:1:4096", ALPINE, "sh", "-c", fmt.Sprintf("sleep 2;echo podman | nc -w 1 %s.dns.podman 9480", name)})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(ExitCleanly())

		log := podmanTest.Podman([]string{"logs", name})
		log.WaitWithDefaultTimeout()
		Expect(log).Should(ExitCleanly())
		Expect(log.OutputToString()).To(Equal("podman"))
	})

	It("podman run with new:pod and static-ip", func() {
		netName := stringid.GenerateRandomID()
		ipAddr := "10.25.44.128"
		podname := "testpod"
		create := podmanTest.Podman([]string{"network", "create", "--subnet", "10.25.44.0/24", netName})
		create.WaitWithDefaultTimeout()
		Expect(create).Should(ExitCleanly())
		defer podmanTest.removeNetwork(netName)

		run := podmanTest.Podman([]string{"run", "--rm", "--pod", "new:" + podname, "--net", netName, "--ip", ipAddr, ALPINE, "ip", "addr"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(ExitCleanly())
		Expect(run.OutputToString()).To(ContainSubstring(ipAddr))

		podrm := podmanTest.Podman([]string{"pod", "rm", "-t", "0", "-f", podname})
		podrm.WaitWithDefaultTimeout()
		Expect(podrm).Should(ExitCleanly())
	})

	It("podman run with --net=host and --hostname sets correct hostname", func() {
		hostname := "testctr"
		run := podmanTest.Podman([]string{"run", "--net=host", "--hostname", hostname, ALPINE, "hostname"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(ExitCleanly())
		Expect(run.OutputToString()).To(ContainSubstring(hostname))
	})

	It("podman run with --net=none sets hostname", func() {
		hostname := "testctr"
		run := podmanTest.Podman([]string{"run", "--net=none", "--hostname", hostname, ALPINE, "hostname"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(ExitCleanly())
		Expect(run.OutputToString()).To(ContainSubstring(hostname))
	})

	It("podman run with --net=none adds hostname to /etc/hosts", func() {
		hostname := "testctr"
		run := podmanTest.Podman([]string{"run", "--net=none", "--hostname", hostname, ALPINE, "cat", "/etc/hosts"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(ExitCleanly())
		Expect(run.OutputToString()).To(ContainSubstring(hostname))
	})

	It("podman run with pod does not add extra 127 entry to /etc/hosts", func() {
		pod := "testpod"
		hostname := "test-hostname"
		run := podmanTest.Podman([]string{"pod", "create", "--hostname", hostname, "--name", pod})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(ExitCleanly())
		run = podmanTest.Podman([]string{"run", "--pod", pod, ALPINE, "cat", "/etc/hosts"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(ExitCleanly())
		Expect(run.OutputToString()).ToNot(ContainSubstring("127.0.0.1 %s", hostname))
	})

	pingTest := func(netns string) {
		hostname := "testctr"
		run := podmanTest.Podman([]string{"run", netns, "--cap-add", "net_raw", "--hostname", hostname, ALPINE, "ping", "-c", "1", hostname})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(ExitCleanly())

		run = podmanTest.Podman([]string{"run", netns, "--cap-add", "net_raw", "--hostname", hostname, "--name", "test", ALPINE, "ping", "-c", "1", "test"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(ExitCleanly())
	}

	It("podman attempt to ping container name and hostname --net=none", func() {
		pingTest("--net=none")
	})

	It("podman attempt to ping container name and hostname --net=private", func() {
		pingTest("--net=private")
	})

	It("podman run check dns", func() {
		pod := "testpod"
		session := podmanTest.Podman([]string{"pod", "create", "--name", pod})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		net := createNetworkName("IntTest")
		session = podmanTest.Podman([]string{"network", "create", net})
		session.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(net)
		Expect(session).Should(ExitCleanly())

		pod2 := "testpod2"
		hostname := "hostn1"
		session = podmanTest.Podman([]string{"pod", "create", "--network", net, "--name", pod2, "--hostname", hostname})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// Note apline nslookup tries to resolve all search domains always and returns an error if one does not resolve.
		// Because we leak all host search domain into the container we have no control over if it resolves or not.
		// Thus use "NAME." to indicate the name is full and no search domain should be tried.
		session = podmanTest.Podman([]string{"run", "--name", "con1", "--network", net, CITEST_IMAGE, "nslookup", "con1."})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--name", "con2", "--pod", pod, "--network", net, CITEST_IMAGE, "nslookup", "con2."})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--name", "con3", "--pod", pod2, CITEST_IMAGE, "nslookup", "con1."})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(1, ""))
		// This flakes on debian with systemd-resolved, also resolved behavior between A and AAAA lookups differ:
		// https://github.com/systemd/systemd/issues/37969
		// In short we can get NXDOMAIN or REFUSED as reply. Both seems fine for the purpose of a negative lookup.
		Expect(session.OutputToString()).To(Or(ContainSubstring("NXDOMAIN"), ContainSubstring("REFUSED")))

		session = podmanTest.Podman([]string{"run", "--name", "con4", "--network", net, CITEST_IMAGE, "nslookup", pod2 + ".dns.podman"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--network", net, CITEST_IMAGE, "nslookup", hostname + "."})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	// configurePortForwarder sets rootless_port_forwarder via
	// CONTAINERS_CONF_OVERRIDE for the duration of this test.
	// For "pasta": requires rootless and pesto binary, skips otherwise.
	// For "rootlessport": no-op (it's the default).
	configurePortForwarder := func(forwarder string) {
		GinkgoHelper()
		if forwarder == "pasta" {
			// TODO: enable after new passt release with SELinux fix for pasta.sock (pasta_t + ifconfig_var_run_t)
			Skip("disabled: pasta SELinux policy denies pasta.sock creation (needs passt release with fix)")
			if !isRootless() {
				Skip("pasta port forwarding requires rootless")
			}
			if _, err := exec.LookPath("pesto"); err != nil {
				Skip("pesto binary not found (requires passt >= passt-0^20260507.g1afd4ed)")
			}
		}
		conffile := filepath.Join(podmanTest.TempDir, forwarder+"-forwarder.conf")
		err := os.WriteFile(conffile, fmt.Appendf(nil, "[network]\nrootless_port_forwarder=\"%s\"\n", forwarder), 0o755)
		Expect(err).ToNot(HaveOccurred())
		GinkgoT().Setenv("CONTAINERS_CONF_OVERRIDE", conffile)
		if IsRemote() {
			podmanTest.RestartRemoteService()
		}
	}

	for _, forwarder := range []string{"rootlessport", "pasta"} {
		for i, tc := range []struct {
			name      string
			ipv6      bool
			hostIP    string
			diffPorts bool
		}{
			{name: "IPv4"},
			{name: "IPv4 explicit HostIP", hostIP: "127.0.0.1"},
			{name: "IPv4 different ports", diffPorts: true},
			{name: "IPv6", ipv6: true},
			{name: "IPv6 explicit HostIP", ipv6: true, hostIP: "[::1]"},
			{name: "IPv6 different ports", ipv6: true, diffPorts: true},
		} {
			It(fmt.Sprintf("podman run bridge source IP %s %s", forwarder, tc.name), func() {
				if tc.ipv6 {
					SkipIfNotRootless("netavark does not support IPv6 port forwarding")
					if forwarder == "rootlessport" {
						Skip("rootlessport does not support native IPv6 port forwarding")
					}
				}
				configurePortForwarder(forwarder)

				// Each forwarder+test combination needs a unique subnet
				// to avoid collisions when running in parallel.
				pastaOffset := 0
				if forwarder == "pasta" {
					pastaOffset = 10
				}
				var subnet, subnetMatch, connectAddr string
				if tc.ipv6 {
					subnet = fmt.Sprintf("fd00:%x::/64", 0x42+i+pastaOffset)
					subnetMatch = fmt.Sprintf("fd00:%x:", 0x42+i+pastaOffset)
					connectAddr = "[::1]"
				} else {
					subnet = fmt.Sprintf("172.%d.0.0/24", 30+i+pastaOffset)
					subnetMatch = fmt.Sprintf(`172\.%d\.`, 30+i+pastaOffset)
					connectAddr = "127.0.0.1"
				}

				createArgs := []string{"network", "create"}
				if tc.ipv6 {
					createArgs = append(createArgs, "--ipv6")
				}
				createArgs = append(createArgs, "--subnet", subnet)
				netName := createNetworkName("srcip")
				createArgs = append(createArgs, netName)
				podmanTest.PodmanExitCleanly(createArgs...)
				defer podmanTest.removeNetwork(netName)

				hostPort := GetPort()
				ctrPort := hostPort
				if tc.diffPorts {
					ctrPort = GetPort()
				}
				portFlag := fmt.Sprintf("%d:%d", hostPort, ctrPort)
				if tc.hostIP != "" {
					portFlag = fmt.Sprintf("%s:%d:%d", tc.hostIP, hostPort, ctrPort)
				}
				ctr := podmanTest.startNCContainer(
					"srcip-ctr", ctrPort,
					"--network", netName,
					"-p", portFlag,
				)

				msg := RandomString(20)
				sendMessageToAddr(fmt.Sprintf("%s:%d", connectAddr, hostPort), msg)
				podmanTest.WaitForContainerLog(ctr, msg)

				logs := podmanTest.PodmanExitCleanly("logs", ctr)
				output := logs.OutputToString()
				Expect(output).To(MatchRegexp(`connect to .* from`))

				if forwarder == "rootlessport" {
					Expect(output).To(MatchRegexp(`connect to .* from .*` + subnetMatch))
				} else {
					Expect(output).ToNot(MatchRegexp(`connect to .* from .*` + subnetMatch))
				}

				podmanTest.PodmanExitCleanly("rm", "-f", ctr)
				podmanTest.PodmanExitCleanly("rm", "-f", netName)
			})
		}

		It(fmt.Sprintf("podman run bridge network port cleanup on container stop with %s", forwarder), func() {
			configurePortForwarder(forwarder)
			netName := createNetworkName("cleanup")
			podmanTest.PodmanExitCleanly("network", "create", netName)
			defer podmanTest.removeNetwork(netName)

			port := GetPort()
			podmanTest.PodmanExitCleanly(
				"run", "-d",
				"--name", "cleanup-ctr",
				"--network", netName,
				"-p", fmt.Sprintf("127.0.0.1:%d:80", port),
				NGINX_IMAGE,
			)

			testPortConnection(port)
			podmanTest.PodmanExitCleanly("rm", "-f", "cleanup-ctr")

			podmanTest.PodmanExitCleanly(
				"run", "-d",
				"--name", "cleanup-ctr2",
				"--network", netName,
				"-p", fmt.Sprintf("127.0.0.1:%d:80", port),
				NGINX_IMAGE,
			)
			testPortConnection(port)

			podmanTest.PodmanExitCleanly("rm", "-f", "cleanup-ctr2")
		})

		It(fmt.Sprintf("podman run bridge dual-stack network IPv4 and IPv6 port forwarding with %s", forwarder), func() {
			SkipIfNotRootless("netavark does not support IPv6 port forwarding")
			configurePortForwarder(forwarder)

			netName := createNetworkName("dual-stack")
			podmanTest.PodmanExitCleanly("network", "create", "--ipv6",
				"--subnet", "fd00:42::/64", "--subnet", "172.42.0.0/24", netName)
			defer podmanTest.removeNetwork(netName)

			port6 := GetPort()
			ctr6 := podmanTest.startNCContainer(
				"c-ipv6", port6,
				"--network", netName,
				"-p", fmt.Sprintf("%d:%d", port6, port6),
			)
			msg6 := RandomString(20)
			sendMessageToAddr(fmt.Sprintf("[::1]:%d", port6), msg6)
			podmanTest.WaitForContainerLog(ctr6, msg6)

			port4 := GetPort()
			ctr4 := podmanTest.startNCContainer(
				"c-ipv4", port4,
				"--network", netName,
				"-p", fmt.Sprintf("%d:%d", port4, port4),
			)
			msg4 := RandomString(20)
			sendMessageToAddr(fmt.Sprintf("127.0.0.1:%d", port4), msg4)
			podmanTest.WaitForContainerLog(ctr4, msg4)
		})
	}

	It("podman run pasta network preserves source IP", func() {
		SkipIfNotRootless("pasta network mode is only supported rootless")
		port := GetPort()
		ctrName := podmanTest.startNCContainer(
			"srcip-pasta-ctr", port,
			"--net=pasta",
			"-p", fmt.Sprintf("%d:%d", port, port),
		)

		msg := RandomString(20)
		sendMessageToAddr(fmt.Sprintf("127.0.0.1:%d", port), msg)
		podmanTest.WaitForContainerLog(ctrName, msg)

		logs := podmanTest.PodmanExitCleanly("logs", ctrName)
		output := logs.OutputToString()
		// With --net=pasta, pasta handles port forwarding directly without
		// a bridge or netavark. The source IP is the host address (not a
		// bridge gateway), confirming pasta's native source preservation.
		Expect(output).To(MatchRegexp(`connect to .* from`))
		Expect(output).ToNot(MatchRegexp(`connect to .* from .*127\.0\.0\.`))

		podmanTest.PodmanExitCleanly("rm", "-f", "srcip-pasta-ctr")
	})

	It("Rootless podman run with --net=bridge works and connects to default network", func() {
		// This is harmless when run as root, so we'll just let it run.
		ctrName := "testctr"
		ctr := podmanTest.Podman([]string{"run", "-d", "--net=bridge", "--name", ctrName, ALPINE, "top"})
		ctr.WaitWithDefaultTimeout()
		Expect(ctr).Should(ExitCleanly())

		inspectOut := podmanTest.InspectContainer(ctrName)
		Expect(inspectOut).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Networks).To(HaveLen(1))
		Expect(inspectOut[0].NetworkSettings.Networks).To(HaveKey("podman"))
	})

	// see https://github.com/containers/podman/issues/12972
	It("podman run check network-alias works on networks without dns", func() {
		net := "dns" + stringid.GenerateRandomID()
		session := podmanTest.Podman([]string{"network", "create", "--disable-dns", net})
		session.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(net)
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--network", net, "--network-alias", "abcdef", ALPINE, "true"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman run with ipam none driver", func() {
		net := "ipam" + stringid.GenerateRandomID()
		session := podmanTest.Podman([]string{"network", "create", "--ipam-driver=none", net})
		session.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(net)
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--network", net, ALPINE, "ip", "addr", "show", "eth0"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(4), "output should only show link local address")
	})

	It("podman run with macvlan network", func() {
		net := "mv-" + stringid.GenerateRandomID()
		session := podmanTest.Podman([]string{"network", "create", "-d", "macvlan", "--subnet", "10.10.0.0/24", net})
		session.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(net)
		Expect(session).Should(ExitCleanly())

		// use options and search to make sure we get the same resolv.conf everywhere
		run := podmanTest.Podman([]string{
			"run", "--network", net, "--dns", "127.0.0.128",
			"--dns-option", "ndots:1", "--dns-search", ".", ALPINE, "cat", "/etc/resolv.conf",
		})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(ExitCleanly())
		Expect(string(run.Out.Contents())).To(Equal(`nameserver 127.0.0.128
options ndots:1
`))
	})

	It("podman network create with same subnets", func() {
		netName := "multi-subnet-" + stringid.GenerateRandomID()
		subnet := "10.64.0.0/16"

		nc := podmanTest.Podman([]string{"network", "create", "--subnet", subnet, "--subnet", subnet, netName})
		nc.WaitWithDefaultTimeout()
		Expect(nc).ShouldNot(ExitCleanly())
		Expect(nc.ErrorToString()).To(ContainSubstring("duplicate subnet"))
	})

	It("podman network create with same IPv6 subnets", func() {
		netName := "multi-subnet-" + stringid.GenerateRandomID()
		subnet := "fd10:64::/48"

		nc := podmanTest.Podman([]string{"network", "create", "--subnet", subnet, "--subnet", subnet, netName})
		nc.WaitWithDefaultTimeout()
		Expect(nc).ShouldNot(ExitCleanly())
		Expect(nc.ErrorToString()).To(ContainSubstring("duplicate subnet"))
	})

	It("podman network create with overlapping subnets", func() {
		// Config file defines overlapping subnets:
		// - 10.1.2.0/23 covers 10.1.2.0-10.1.3.255
		// - 10.1.3.248/30 covers 10.1.3.248-10.1.3.251 (subset of the /23)

		netName := "multi-subnet-" + stringid.GenerateRandomID()
		subnetA := "10.1.2.0/23"
		subnetB := "10.1.3.248/30"

		nc := podmanTest.Podman([]string{"network", "create", "--subnet", subnetA, "--subnet", subnetB, netName})
		nc.WaitWithDefaultTimeout()
		Expect(nc).ShouldNot(ExitCleanly())
		Expect(nc.ErrorToString()).To(ContainSubstring("overlapping subnets"))
	})

	It("podman network create with overlapping IPv6 subnets", func() {
		// Config file defines overlapping IPv6 subnets:
		// - fd20:1::/48 covers fd20:1:: through fd20:1:0:ffff:...
		// - fd20:1:0:f8::/62 is a subset of fd20:1::/48

		netName := "multi-subnet-" + stringid.GenerateRandomID()
		subnetA := "fd20:1::/48"
		subnetB := "fd20:1:0:f8::/62"

		nc := podmanTest.Podman([]string{"network", "create", "--subnet", subnetA, "--subnet", subnetB, netName})
		nc.WaitWithDefaultTimeout()
		Expect(nc).ShouldNot(ExitCleanly())
		Expect(nc.ErrorToString()).To(ContainSubstring("overlapping subnets"))
	})

	// sortBySubnet reorders IPs to match the subnet order used in network creation.
	// For multi-subnet networks, IPs are grouped by subnet in the order the subnets
	// were defined with --subnet flags, regardless of the order IPs were specified.
	// Returns a new slice with IPs sorted by their corresponding subnet.
	sortBySubnet := func(ips []string, subnets []string) []string {
		sortedExpectedIPs := make([]string, 0, len(ips))
		for _, subnet := range subnets {
			_, subnetNet, err := net.ParseCIDR(subnet)
			Expect(err).ToNot(HaveOccurred())
			for _, ip := range ips {
				parsedIP := net.ParseIP(ip)
				if subnetNet.Contains(parsedIP) {
					sortedExpectedIPs = append(sortedExpectedIPs, ip)
				}
			}
		}
		return sortedExpectedIPs
	}

	type multiIPTestCase struct {
		name               string
		subnets            []string
		staticIPs          []string
		subnetGateways     map[string]string
		hasDynamicIP       bool
		numberOfDynamicIPs int
		isIPv6             bool
	}

	multiIPTests := []multiIPTestCase{
		{
			name:      "multiple static IPs in single subnet",
			subnets:   []string{"10.86.0.0/16"},
			staticIPs: []string{"10.86.0.10", "10.86.0.11", "10.86.0.12"},
			subnetGateways: map[string]string{
				"10.86.": "10.86.0.1",
			},
		},
		{
			name:      "multiple static IPs across multiple subnets",
			subnets:   []string{"10.92.0.0/24", "10.91.0.0/24"},
			staticIPs: []string{"10.92.0.21", "10.92.0.20", "10.91.0.10", "10.91.0.11"},
			subnetGateways: map[string]string{
				"10.91.": "10.91.0.1",
				"10.92.": "10.92.0.1",
			},
		},
		{
			name:      "static IP on one subnet and dynamic IP on another",
			subnets:   []string{"10.93.0.0/24", "10.94.0.0/24"},
			staticIPs: []string{"10.93.0.50"},
			subnetGateways: map[string]string{
				"10.93.": "10.93.0.1",
				"10.94.": "10.94.0.1",
			},
			hasDynamicIP:       true,
			numberOfDynamicIPs: 1,
		},
		{
			name:      "multiple static IPs across three subnets",
			subnets:   []string{"10.95.0.0/24", "10.96.0.0/24", "10.97.0.0/24"},
			staticIPs: []string{"10.95.0.10", "10.96.0.20", "10.97.0.30", "10.95.0.11", "10.96.0.21"},
			subnetGateways: map[string]string{
				"10.95.": "10.95.0.1",
				"10.96.": "10.96.0.1",
				"10.97.": "10.97.0.1",
			},
		},
		{
			name:      "single static IP in single subnet",
			subnets:   []string{"10.98.0.0/24"},
			staticIPs: []string{"10.98.0.100"},
			subnetGateways: map[string]string{
				"10.98.": "10.98.0.1",
			},
		},
		{
			name:      "two static IPs one per subnet",
			subnets:   []string{"10.99.0.0/24", "10.100.0.0/24"},
			staticIPs: []string{"10.99.0.50", "10.100.0.50"},
			subnetGateways: map[string]string{
				"10.99.":  "10.99.0.1",
				"10.100.": "10.100.0.1",
			},
		},
		{
			name:      "many static IPs in single subnet",
			subnets:   []string{"10.101.0.0/24"},
			staticIPs: []string{"10.101.0.10", "10.101.0.11", "10.101.0.12", "10.101.0.13", "10.101.0.14", "10.101.0.15"},
			subnetGateways: map[string]string{
				"10.101.": "10.101.0.1",
			},
		},
		{
			name:      "single static IP with multiple dynamic IPs across three subnets",
			subnets:   []string{"10.102.0.0/24", "10.103.0.0/24", "10.104.0.0/24"},
			staticIPs: []string{"10.102.0.50"},
			subnetGateways: map[string]string{
				"10.102.": "10.102.0.1",
				"10.103.": "10.103.0.1",
				"10.104.": "10.104.0.1",
			},
			hasDynamicIP:       true,
			numberOfDynamicIPs: 2,
		},
		// IPv6 variants
		{
			name:      "multiple static IPv6 IPs in single subnet",
			subnets:   []string{"fd86:1::/64"},
			staticIPs: []string{"fd86:1::10", "fd86:1::11", "fd86:1::12"},
			subnetGateways: map[string]string{
				"fd86:1::": "fd86:1::1",
			},
			isIPv6: true,
		},
		{
			name:      "multiple static IPv6 IPs across multiple subnets",
			subnets:   []string{"fd92:1::/64", "fd91:1::/64"},
			staticIPs: []string{"fd92:1::21", "fd92:1::20", "fd91:1::10", "fd91:1::11"},
			subnetGateways: map[string]string{
				"fd91:1::": "fd91:1::1",
				"fd92:1::": "fd92:1::1",
			},
			isIPv6: true,
		},
		{
			name:      "static IPv6 IP on one subnet and dynamic IPv6 IP on another",
			subnets:   []string{"fd93:1::/64", "fd94:1::/64"},
			staticIPs: []string{"fd93:1::50"},
			subnetGateways: map[string]string{
				"fd93:1::": "fd93:1::1",
				"fd94:1::": "fd94:1::1",
			},
			hasDynamicIP:       true,
			numberOfDynamicIPs: 1,
			isIPv6:             true,
		},
		{
			name:      "multiple static IPv6 IPs across three subnets",
			subnets:   []string{"fd95:1::/64", "fd96:1::/64", "fd97:1::/64"},
			staticIPs: []string{"fd95:1::10", "fd96:1::20", "fd97:1::30", "fd95:1::11", "fd96:1::21"},
			subnetGateways: map[string]string{
				"fd95:1::": "fd95:1::1",
				"fd96:1::": "fd96:1::1",
				"fd97:1::": "fd97:1::1",
			},
			isIPv6: true,
		},
		{
			name:      "single static IPv6 IP in single subnet",
			subnets:   []string{"fd98:1::/64"},
			staticIPs: []string{"fd98:1::64"},
			subnetGateways: map[string]string{
				"fd98:1::": "fd98:1::1",
			},
			isIPv6: true,
		},
		{
			name:      "two static IPv6 IPs one per subnet",
			subnets:   []string{"fd99:1::/64", "fd9a:1::/64"},
			staticIPs: []string{"fd99:1::50", "fd9a:1::50"},
			subnetGateways: map[string]string{
				"fd99:1::": "fd99:1::1",
				"fd9a:1::": "fd9a:1::1",
			},
			isIPv6: true,
		},
		{
			name:      "many static IPv6 IPs in single subnet",
			subnets:   []string{"fda1:1::/64"},
			staticIPs: []string{"fda1:1::10", "fda1:1::11", "fda1:1::12", "fda1:1::13", "fda1:1::14", "fda1:1::15"},
			subnetGateways: map[string]string{
				"fda1:1::": "fda1:1::1",
			},
			isIPv6: true,
		},
		{
			name:      "single static IPv6 IP with multiple dynamic IPv6 IPs across three subnets",
			subnets:   []string{"fda2:1::/64", "fda3:1::/64", "fda4:1::/64"},
			staticIPs: []string{"fda2:1::50"},
			subnetGateways: map[string]string{
				"fda2:1::": "fda2:1::1",
				"fda3:1::": "fda3:1::1",
				"fda4:1::": "fda4:1::1",
			},
			hasDynamicIP:       true,
			numberOfDynamicIPs: 2,
			isIPv6:             true,
		},
	}

	for _, tc := range multiIPTests {
		It(fmt.Sprintf("podman run container with %s", tc.name), func() {
			netName := "test-net-" + stringid.GenerateRandomID()
			netCreateArgs := []string{"network", "create"}
			for _, subnet := range tc.subnets {
				netCreateArgs = append(netCreateArgs, "--subnet", subnet)
			}
			netCreateArgs = append(netCreateArgs, netName)
			podmanTest.PodmanExitCleanly(netCreateArgs...)
			defer podmanTest.removeNetwork(netName)

			ipsToUse := make([]string, len(tc.staticIPs))
			copy(ipsToUse, tc.staticIPs)
			rand.Shuffle(len(ipsToUse), func(i, j int) {
				ipsToUse[i], ipsToUse[j] = ipsToUse[j], ipsToUse[i]
			})
			networkArg := fmt.Sprintf("%s:ip=%s", netName, strings.Join(ipsToUse, ",ip="))
			cName := "test-ctr-" + stringid.GenerateRandomID()
			podmanTest.PodmanExitCleanly("run", "-d", "--name", cName, "--network", networkArg, ALPINE, "top")
			defer podmanTest.PodmanExitCleanly("rm", "-f", cName)

			data := podmanTest.InspectContainer(cName)
			Expect(data).To(HaveLen(1))
			containerInspect := data[0]
			containerID := data[0].ID

			interfaceName := "eth0"
			var ipAddrCmd string
			if tc.isIPv6 {
				ipAddrCmd = fmt.Sprintf("ip addr show %s | awk '/inet6/ && !/fe80:/{print $2}'", interfaceName)
			} else {
				ipAddrCmd = fmt.Sprintf("ip addr show %s | awk ' /inet / {print $2}'", interfaceName)
			}
			showIPs := podmanTest.PodmanExitCleanly("exec", cName, "sh", "-c", ipAddrCmd)
			outputIPs := showIPs.OutputToStringArray()

			actualIPs := make([]string, len(outputIPs))
			for i, ipCIDR := range outputIPs {
				containerIP, _, err := net.ParseCIDR(ipCIDR)
				Expect(err).ToNot(HaveOccurred())
				actualIPs[i] = containerIP.String()
			}

			expectedIPs := sortBySubnet(ipsToUse, tc.subnets)
			if tc.hasDynamicIP {
				Expect(actualIPs).To(HaveLen(len(tc.staticIPs) + tc.numberOfDynamicIPs))
				Expect(actualIPs).To(ContainElement(tc.staticIPs[0]))

				for _, dynamicIP := range actualIPs {
					if dynamicIP == tc.staticIPs[0] {
						continue
					}
					parsedDynamicIP := net.ParseIP(dynamicIP)
					foundInDynamicSubnet := false
					for j := len(tc.staticIPs); j < len(tc.subnets); j++ {
						_, subnetNet, err := net.ParseCIDR(tc.subnets[j])
						Expect(err).ToNot(HaveOccurred())
						if subnetNet.Contains(parsedDynamicIP) {
							foundInDynamicSubnet = true
							break
						}
					}
					Expect(foundInDynamicSubnet).To(BeTrue(), fmt.Sprintf("dynamic IP %s not in any expected dynamic subnet", dynamicIP))

					expectedIPs = append(expectedIPs, dynamicIP)
				}
				expectedIPs = sortBySubnet(expectedIPs, tc.subnets)
			}
			Expect(actualIPs).To(ConsistOf(expectedIPs))

			Expect(containerInspect.NetworkSettings.Networks).To(HaveKey(netName))
			network := containerInspect.NetworkSettings.Networks[netName]

			primaryIP := expectedIPs[0]
			expectedSecondaryIPs := expectedIPs[1:]
			if tc.isIPv6 {
				Expect(network.GlobalIPv6Address).To(Equal(primaryIP))
				Expect(network.SecondaryIPv6Addresses).To(HaveLen(len(expectedSecondaryIPs)))
				for i, ip := range network.SecondaryIPv6Addresses {
					Expect(ip.Addr).To(Equal(expectedSecondaryIPs[i]))
				}
			} else {
				Expect(network.IPAddress).To(Equal(primaryIP))
				Expect(network.SecondaryIPAddresses).To(HaveLen(len(expectedSecondaryIPs)))
				for i, ip := range network.SecondaryIPAddresses {
					Expect(ip.Addr).To(Equal(expectedSecondaryIPs[i]))
				}
			}

			networkReports := podmanTest.InspectNetwork(netName)
			Expect(networkReports).To(HaveLen(1))
			netReport := networkReports[0]
			Expect(netReport.Containers).To(HaveKey(containerID))

			containerInfo := netReport.Containers[containerID]
			Expect(containerInfo.Interfaces).To(HaveKey(interfaceName))

			netInterface := containerInfo.Interfaces[interfaceName]
			Expect(netInterface.Subnets).To(HaveLen(len(expectedIPs)))
			for i, subnetInfo := range netInterface.Subnets {
				ip := subnetInfo.IPNet.IP.String()
				Expect(ip).To(Equal(expectedIPs[i]))
				var expectedGateway string
				for subnetPrefix, gateway := range tc.subnetGateways {
					if strings.HasPrefix(ip, subnetPrefix) {
						expectedGateway = gateway
						break
					}
				}
				Expect(expectedGateway).ToNot(BeEmpty())
				Expect(subnetInfo.Gateway.String()).To(Equal(expectedGateway))
			}
		})
	}
})

// sendMessageToAddr sends a message to the given tcp address (host:port).
func sendMessageToAddr(addr string, message string) {
	GinkgoHelper()
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	Expect(err).ToNot(HaveOccurred(), "should connect to %s", addr)

	tcpConn := conn.(*net.TCPConn)
	_, err = tcpConn.Write([]byte(message))
	Expect(err).ToNot(HaveOccurred())

	err = tcpConn.CloseWrite()
	Expect(err).ToNot(HaveOccurred())

	err = tcpConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	Expect(err).ToNot(HaveOccurred())
	_, _ = io.Copy(io.Discard, tcpConn)
	tcpConn.Close()
}
