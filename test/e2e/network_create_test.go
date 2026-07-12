//go:build linux || freebsd

package integration

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.podman.io/common/libnetwork/types"
	"go.podman.io/podman/v6/pkg/domain/entities"
	. "go.podman.io/podman/v6/test/utils"
	"go.podman.io/storage/pkg/stringid"
)

func removeNetworkDevice(name string) {
	session := SystemExec("ip", []string{"link", "delete", name})
	session.WaitWithDefaultTimeout()
}

func uintPtr(u uint32) *uint32 {
	return &u
}

var _ = Describe("Podman network create", func() {
	It("podman network create with name and subnet", func() {
		netName := "subnet-" + stringid.GenerateRandomID()
		nc := podmanTest.Podman([]string{"network", "create", "--subnet", "10.11.17.0/24", "--ip-range", "10.11.17.0/26", netName})
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(netName)
		Expect(nc).Should(ExitCleanly())

		// Inspect the network configuration
		results := podmanTest.InspectNetwork(netName)
		Expect(results).To(HaveLen(1))
		result := results[0]
		Expect(result).To(HaveField("Name", netName))
		Expect(result.Subnets).To(HaveLen(1))
		Expect(result.Subnets[0].Subnet.String()).To(Equal("10.11.17.0/24"))
		Expect(result.Subnets[0].Gateway.String()).To(Equal("10.11.17.1"))
		Expect(result.Subnets[0].LeaseRange).ToNot(BeNil())
		Expect(result.Subnets[0].LeaseRange.StartIP.String()).To(Equal("10.11.17.1"))
		Expect(result.Subnets[0].LeaseRange.EndIP.String()).To(Equal("10.11.17.63"))

		// Once a container executes a new network, the nic will be created. We should clean those up
		// best we can
		defer removeNetworkDevice(result.NetworkInterface)

		try := podmanTest.Podman([]string{"run", "--rm", "--network", netName, ALPINE, "sh", "-c", "ip addr show eth0 |  awk ' /inet / {print $2}'"})
		try.WaitWithDefaultTimeout()
		Expect(try).To(ExitCleanly())

		_, subnet, err := net.ParseCIDR("10.11.17.0/24")
		Expect(err).ToNot(HaveOccurred())
		// Note this is an IPv4 test only!
		containerIP, _, err := net.ParseCIDR(try.OutputToString())
		Expect(err).ToNot(HaveOccurred())
		// Ensure that the IP the container got is within the subnet the user asked for
		Expect(subnet.Contains(containerIP)).To(BeTrue(), "subnet contains containerIP")
	})

	It("podman network create with name and subnet and static route", func() {
		netName := "subnet-" + stringid.GenerateRandomID()
		nc := podmanTest.Podman([]string{
			"network",
			"create",
			"--subnet",
			"10.19.12.0/24",
			"--route",
			"10.21.0.0/24,10.19.12.250",
			netName,
		})
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(netName)
		Expect(nc).Should(ExitCleanly())

		// Inspect the network configuration
		results := podmanTest.InspectNetwork(netName)
		Expect(results).To(HaveLen(1))
		result := results[0]
		Expect(result).To(HaveField("Name", netName))
		Expect(result.Subnets).To(HaveLen(1))
		Expect(result.Subnets[0].Subnet.String()).To(Equal("10.19.12.0/24"))
		Expect(result.Subnets[0].Gateway.String()).To(Equal("10.19.12.1"))
		Expect(result.Routes[0].Destination.String()).To(Equal("10.21.0.0/24"))
		Expect(result.Routes[0].Gateway.String()).To(Equal("10.19.12.250"))
		Expect(result.Routes[0].Metric).To(BeNil())

		// Once a container executes a new network, the nic will be created. We should clean those up
		// best we can
		defer removeNetworkDevice(result.NetworkInterface)
	})

	It("podman network create with name and subnet and static route and metric", func() {
		netName := "subnet-" + stringid.GenerateRandomID()
		nc := podmanTest.Podman([]string{
			"network",
			"create",
			"--subnet",
			"10.19.13.0/24",
			"--route",
			"10.21.1.0/24,10.19.13.250,120",
			netName,
		})
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(netName)
		Expect(nc).Should(ExitCleanly())

		// Inspect the network configuration
		results := podmanTest.InspectNetwork(netName)
		Expect(results).To(HaveLen(1))
		result := results[0]
		Expect(result).To(HaveField("Name", netName))
		Expect(result.Subnets).To(HaveLen(1))
		Expect(result.Subnets[0].Subnet.String()).To(Equal("10.19.13.0/24"))
		Expect(result.Subnets[0].Gateway.String()).To(Equal("10.19.13.1"))
		Expect(result.Routes[0].Destination.String()).To(Equal("10.21.1.0/24"))
		Expect(result.Routes[0].Gateway.String()).To(Equal("10.19.13.250"))
		Expect(*result.Routes[0].Metric).To(Equal(uint32(120)))

		// Once a container executes a new network, the nic will be created. We should clean those up
		// best we can
		defer removeNetworkDevice(result.NetworkInterface)
	})

	It("podman network create with name and subnet and two static routes", func() {
		netName := "subnet-" + stringid.GenerateRandomID()
		nc := podmanTest.Podman([]string{
			"network",
			"create",
			"--subnet",
			"10.19.14.0/24",
			"--route",
			"10.21.2.0/24,10.19.14.250",
			"--route",
			"10.21.3.0/24,10.19.14.251,120",
			netName,
		})
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(netName)
		Expect(nc).Should(ExitCleanly())

		// Inspect the network configuration
		results := podmanTest.InspectNetwork(netName)
		Expect(results).To(HaveLen(1))
		result := results[0]
		Expect(result).To(HaveField("Name", netName))
		Expect(result.Subnets).To(HaveLen(1))
		Expect(result.Subnets[0].Subnet.String()).To(Equal("10.19.14.0/24"))
		Expect(result.Subnets[0].Gateway.String()).To(Equal("10.19.14.1"))
		Expect(result.Routes).To(HaveLen(2))
		Expect(result.Routes[0].Destination.String()).To(Equal("10.21.2.0/24"))
		Expect(result.Routes[0].Gateway.String()).To(Equal("10.19.14.250"))
		Expect(result.Routes[0].Metric).To(BeNil())
		Expect(result.Routes[1].Destination.String()).To(Equal("10.21.3.0/24"))
		Expect(result.Routes[1].Gateway.String()).To(Equal("10.19.14.251"))
		Expect(*result.Routes[1].Metric).To(Equal(uint32(120)))

		// Once a container executes a new network, the nic will be created. We should clean those up
		// best we can
		defer removeNetworkDevice(result.NetworkInterface)
	})

	It("podman network create with name and subnet and static route (ipv6)", func() {
		netName := "subnet-" + stringid.GenerateRandomID()
		nc := podmanTest.Podman([]string{
			"network",
			"create",
			"--subnet",
			"fd:ab04::/64",
			"--route",
			"fd:1::/64,fd::1,120",
			netName,
		})
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(netName)
		Expect(nc).Should(ExitCleanly())

		// Inspect the network configuration
		results := podmanTest.InspectNetwork(netName)
		Expect(results).To(HaveLen(1))
		result := results[0]
		Expect(result).To(HaveField("Name", netName))
		Expect(result.Subnets).To(HaveLen(1))
		Expect(result.Subnets[0].Subnet.String()).To(Equal("fd:ab04::/64"))
		Expect(result.Subnets[0].Gateway.String()).To(Equal("fd:ab04::1"))
		Expect(result.Routes[0].Destination.String()).To(Equal("fd:1::/64"))
		Expect(result.Routes[0].Gateway.String()).To(Equal("fd::1"))
		Expect(*result.Routes[0].Metric).To(Equal(uint32(120)))

		// Once a container executes a new network, the nic will be created. We should clean those up
		// best we can
		defer removeNetworkDevice(result.NetworkInterface)
	})

	It("podman network create with name and subnet with --opt no_default_route=true", func() {
		netName := "subnet-" + stringid.GenerateRandomID()
		nc := podmanTest.Podman([]string{
			"network",
			"create",
			"--subnet",
			"10.19.15.0/24",
			"--opt",
			"no_default_route=true",
			netName,
		})
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(netName)
		Expect(nc).Should(ExitCleanly())

		// Inspect the network configuration
		results := podmanTest.InspectNetwork(netName)
		Expect(results).To(HaveLen(1))
		result := results[0]
		Expect(result).To(HaveField("Name", netName))
		Expect(result.Subnets).To(HaveLen(1))
		Expect(result.Subnets[0].Subnet.String()).To(Equal("10.19.15.0/24"))
		Expect(result.Subnets[0].Gateway.String()).To(Equal("10.19.15.1"))
		Expect(result.Options[types.NoDefaultRoute]).To(Equal("true"))

		// Once a container executes a new network, the nic will be created. We should clean those up
		// best we can
		defer removeNetworkDevice(result.NetworkInterface)
	})

	It("podman network create with name and IPv6 subnet", func() {
		netName := "ipv6-" + stringid.GenerateRandomID()
		nc := podmanTest.Podman([]string{"network", "create", "--subnet", "fd00:1:2:3:4::/64", netName})
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(netName)
		Expect(nc).Should(ExitCleanly())

		// Inspect the network configuration
		results := podmanTest.InspectNetwork(netName)
		Expect(results).To(HaveLen(1))
		result := results[0]
		Expect(result).To(HaveField("Name", netName))
		Expect(result.Subnets).To(HaveLen(1))
		Expect(result.Subnets[0].Gateway.String()).To(Equal("fd00:1:2:3::1"))
		Expect(result.Subnets[0].Subnet.String()).To(Equal("fd00:1:2:3::/64"))

		// Once a container executes a new network, the nic will be created. We should clean those up
		// best we can
		defer removeNetworkDevice(result.NetworkInterface)

		try := podmanTest.Podman([]string{"run", "--rm", "--network", netName, ALPINE, "sh", "-c", "ip addr show eth0 |  grep global | awk ' /inet6 / {print $2}'"})
		try.WaitWithDefaultTimeout()
		Expect(try).To(ExitCleanly())

		_, subnet, err := net.ParseCIDR("fd00:1:2:3:4::/64")
		Expect(err).ToNot(HaveOccurred())
		containerIP, _, err := net.ParseCIDR(try.OutputToString())
		Expect(err).ToNot(HaveOccurred())
		// Ensure that the IP the container got is within the subnet the user asked for
		Expect(subnet.Contains(containerIP)).To(BeTrue(), "subnet contains containerIP")
	})

	It("podman network create with name and IPv6 flag (dual-stack)", func() {
		netName := "dual-" + stringid.GenerateRandomID()
		nc := podmanTest.Podman([]string{"network", "create", "--subnet", "fd00:4:3:2::/64", "--ipv6", netName})
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(netName)
		Expect(nc).Should(ExitCleanly())

		// Inspect the network configuration
		results := podmanTest.InspectNetwork(netName)
		Expect(results).To(HaveLen(1))
		result := results[0]
		Expect(result).To(HaveField("Name", netName))
		Expect(result.Subnets).To(HaveLen(2))
		Expect(result.Subnets[0].Subnet.IP).ToNot(BeNil())
		Expect(result.Subnets[1].Subnet.IP).ToNot(BeNil())

		_, subnet11, err := net.ParseCIDR(result.Subnets[0].Subnet.String())
		Expect(err).ToNot(HaveOccurred())
		_, subnet12, err := net.ParseCIDR(result.Subnets[1].Subnet.String())
		Expect(err).ToNot(HaveOccurred())

		// Once a container executes a new network, the nic will be created. We should clean those up
		// best we can
		defer removeNetworkDevice(result.NetworkInterface)

		// create a second network to check the auto assigned ipv4 subnet does not overlap
		// https://github.com/containers/podman/issues/11032
		netName2 := "dual-" + stringid.GenerateRandomID()
		nc = podmanTest.Podman([]string{"network", "create", "--subnet", "fd00:10:3:2::/64", "--ipv6", netName2})
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(netName2)
		Expect(nc).Should(ExitCleanly())

		// Inspect the network configuration
		results = podmanTest.InspectNetwork(netName2)
		Expect(results).To(HaveLen(1))
		result = results[0]
		Expect(result).To(HaveField("Name", netName2))
		Expect(result.Subnets).To(HaveLen(2))
		Expect(result.Subnets[0].Subnet.IP).ToNot(BeNil())
		Expect(result.Subnets[1].Subnet.IP).ToNot(BeNil())

		_, subnet21, err := net.ParseCIDR(result.Subnets[0].Subnet.String())
		Expect(err).ToNot(HaveOccurred())
		_, subnet22, err := net.ParseCIDR(result.Subnets[1].Subnet.String())
		Expect(err).ToNot(HaveOccurred())

		// check that the subnets do not overlap
		Expect(subnet11.Contains(subnet21.IP)).To(BeFalse())
		Expect(subnet12.Contains(subnet22.IP)).To(BeFalse())

		try := podmanTest.Podman([]string{"run", "--rm", "--network", netName, ALPINE, "sh", "-c", "ip addr show eth0 |  grep global | awk ' /inet6 / {print $2}'"})
		try.WaitWithDefaultTimeout()

		_, subnet, err := net.ParseCIDR("fd00:4:3:2:1::/64")
		Expect(err).ToNot(HaveOccurred())
		containerIP, _, err := net.ParseCIDR(try.OutputToString())
		Expect(err).ToNot(HaveOccurred())
		// Ensure that the IP the container got is within the subnet the user asked for
		Expect(subnet.Contains(containerIP)).To(BeTrue(), "subnet contains containerIP")
		// verify the container has an IPv4 address too (the IPv4 subnet is autogenerated)
		try = podmanTest.Podman([]string{"run", "--rm", "--network", netName, ALPINE, "sh", "-c", "ip addr show eth0 |  awk ' /inet / {print $2}'"})
		try.WaitWithDefaultTimeout()
		containerIP, _, err = net.ParseCIDR(try.OutputToString())
		Expect(err).ToNot(HaveOccurred())
		Expect(containerIP.To4()).To(Not(BeNil()))
	})

	It("podman network create with invalid subnet", func() {
		nc := podmanTest.Podman([]string{"network", "create", "--subnet", "10.11.12.0/17000", stringid.GenerateRandomID()})
		nc.WaitWithDefaultTimeout()
		Expect(nc).To(ExitWithError(125, "invalid CIDR address: 10.11.12.0/17000"))
	})

	It("podman network create with ipv4 subnet and ipv6 flag", func() {
		name := stringid.GenerateRandomID()
		nc := podmanTest.Podman([]string{"network", "create", "--subnet", "10.11.14.0/24", "--ipv6", name})
		nc.WaitWithDefaultTimeout()
		Expect(nc).To(ExitCleanly())
		defer podmanTest.removeNetwork(name)

		results := podmanTest.InspectNetwork(name)
		Expect(results).To(HaveLen(1))
		Expect(results[0].Subnets).To(HaveLen(2))
		// Check for both IPv4 and IPv6 subnets
		hasIPv4 := false
		hasIPv6 := false
		for _, subnet := range results[0].Subnets {
			if subnet.Subnet.String() == "10.11.14.0/24" {
				hasIPv4 = true
			}
			if strings.HasSuffix(subnet.Subnet.String(), "::/64") {
				hasIPv6 = true
			}
		}
		Expect(hasIPv4).To(BeTrue())
		Expect(hasIPv6).To(BeTrue())
	})

	It("podman network create with empty subnet and ipv6 flag", func() {
		name := stringid.GenerateRandomID()
		nc := podmanTest.Podman([]string{"network", "create", "--ipv6", name})
		nc.WaitWithDefaultTimeout()
		Expect(nc).To(ExitCleanly())
		defer podmanTest.removeNetwork(name)

		results := podmanTest.InspectNetwork(name)
		Expect(results).To(HaveLen(1))
		Expect(results[0].Subnets).To(HaveLen(2))
		// Check for both IPv4 and IPv6 subnets
		hasIPv4 := false
		hasIPv6 := false
		for _, subnet := range results[0].Subnets {
			if strings.HasSuffix(subnet.Subnet.String(), ".0/24") {
				hasIPv4 = true
			}
			if strings.HasSuffix(subnet.Subnet.String(), "::/64") {
				hasIPv6 = true
			}
		}
		Expect(hasIPv4).To(BeTrue())
		Expect(hasIPv6).To(BeTrue())
	})

	It("podman network create with invalid IP arguments", func() {
		nc := podmanTest.Podman([]string{"network", "create", "--subnet", "10.11.12.0/24", "--ip-range", "10.11.12.345-10.11.12.999"})
		nc.WaitWithDefaultTimeout()
		Expect(nc).To(ExitWithError(125, `range start ip "10.11.12.345" is not a ip address`))

		nc = podmanTest.Podman([]string{"network", "create", "--subnet", "10.11.12.0/24", "--ip-range", "10.11.12.3-10.11.12.999"})
		nc.WaitWithDefaultTimeout()
		Expect(nc).To(ExitWithError(125, `range end ip "10.11.12.999" is not a ip address`))

		nc = podmanTest.Podman([]string{"network", "create", "--gateway", "10.11.12.256"})
		nc.WaitWithDefaultTimeout()
		Expect(nc).To(ExitWithError(125, `invalid argument "10.11.12.256" for "--gateway" flag: invalid string being converted to IP address: 10.11.12.256`))
	})

	It("podman network create with invalid gateway for subnet", func() {
		nc := podmanTest.Podman([]string{"network", "create", "--subnet", "10.11.15.0/24", "--gateway", "192.168.1.1", stringid.GenerateRandomID()})
		nc.WaitWithDefaultTimeout()
		Expect(nc).To(ExitWithError(125, "gateway 192.168.1.1 not in subnet 10.11.15.0/24"))
	})

	It("podman network create two networks with same name should fail", func() {
		netName := "same-" + stringid.GenerateRandomID()
		nc := podmanTest.Podman([]string{"network", "create", netName})
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(netName)
		Expect(nc).Should(ExitCleanly())

		ncFail := podmanTest.Podman([]string{"network", "create", netName})
		ncFail.WaitWithDefaultTimeout()
		Expect(ncFail).To(ExitWithError(125, "network already exists"))
	})

	It("podman network create two networks with same subnet should fail", func() {
		netName1 := "sub1-" + stringid.GenerateRandomID()
		nc := podmanTest.Podman([]string{"network", "create", "--subnet", "10.11.13.0/24", netName1})
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(netName1)
		Expect(nc).Should(ExitCleanly())

		netName2 := "sub2-" + stringid.GenerateRandomID()
		ncFail := podmanTest.Podman([]string{"network", "create", "--subnet", "10.11.13.0/24", netName2})
		ncFail.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(netName2)
		Expect(ncFail).To(ExitWithError(125, "subnet 10.11.13.0/24 is already used on the host or by another config"))
	})

	It("podman network create two IPv6 networks with same subnet should fail", func() {
		netName1 := "subipv61-" + stringid.GenerateRandomID()
		nc := podmanTest.Podman([]string{"network", "create", "--subnet", "fd00:4:4:4:4::/64", "--ipv6", netName1})
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(netName1)
		Expect(nc).Should(ExitCleanly())

		netName2 := "subipv62-" + stringid.GenerateRandomID()
		ncFail := podmanTest.Podman([]string{"network", "create", "--subnet", "fd00:4:4:4:4::/64", "--ipv6", netName2})
		ncFail.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(netName2)
		Expect(ncFail).To(ExitWithError(125, "subnet fd00:4:4:4:4::/64 is already used on the host or by another config"))
	})

	It("podman network create with invalid network name", func() {
		nc := podmanTest.Podman([]string{"network", "create", "2bad!"})
		nc.WaitWithDefaultTimeout()
		Expect(nc).To(ExitWithError(125, "Invalid characters in network name \"2bad!\": must match [a-zA-Z0-9][a-zA-Z0-9_.-]*"))
	})

	It("podman network create with mtu option", func() {
		net := "mtu-test" + stringid.GenerateRandomID()
		nc := podmanTest.Podman([]string{"network", "create", "--opt", "mtu=9000", net})
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(net)
		Expect(nc).Should(ExitCleanly())

		results := podmanTest.InspectNetwork(net)
		Expect(results).To(HaveLen(1))
		Expect(results[0].Options).To(HaveKeyWithValue("mtu", "9000"))
	})

	It("podman network create with vlan option", func() {
		net := "vlan-test" + stringid.GenerateRandomID()
		nc := podmanTest.Podman([]string{"network", "create", "--opt", "vlan=9", net})
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(net)
		Expect(nc).Should(ExitCleanly())

		results := podmanTest.InspectNetwork(net)
		Expect(results).To(HaveLen(1))
		Expect(results[0].Options).To(HaveKeyWithValue("vlan", "9"))
	})

	It("podman network create with invalid option", func() {
		net := "invalid-test" + stringid.GenerateRandomID()
		nc := podmanTest.Podman([]string{"network", "create", "--opt", "foo=bar", net})
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(net)
		Expect(nc).To(ExitWithError(125, "unsupported bridge network option: foo"))
	})

	It("podman Netavark network create with internal should have dnsname", func() {
		net := "internal-test" + stringid.GenerateRandomID()
		nc := podmanTest.Podman([]string{"network", "create", "--internal", net})
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(net)
		Expect(nc).Should(ExitCleanly())
		// Not performing this check on remote tests because it is a logrus error which does
		// not come back via stderr on the remote client.
		if !IsRemote() {
			Expect(nc.ErrorToString()).To(BeEmpty())
		}
		results := podmanTest.InspectNetwork(net)
		Expect(results).To(HaveLen(1))
		Expect(results[0].DNSEnabled).To(BeTrue())
	})

	It("podman network create with invalid name", func() {
		for _, name := range []string{"none", "host", "bridge", "private", "pasta", "container", "ns", "default"} {
			nc := podmanTest.Podman([]string{"network", "create", name})
			nc.WaitWithDefaultTimeout()
			Expect(nc).To(ExitWithError(125, fmt.Sprintf("cannot create network with name %q because it conflicts with a valid network mode", name)))
		}
	})

	It("podman network create with multiple subnets", func() {
		name := "subnets-" + stringid.GenerateRandomID()
		subnet1 := "10.10.0.0/24"
		subnet2 := "10.10.1.0/24"
		nc := podmanTest.Podman([]string{"network", "create", "--subnet", subnet1, "--subnet", subnet2, name})
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(name)
		Expect(nc).To(ExitCleanly())
		Expect(nc.OutputToString()).To(Equal(name))

		results := podmanTest.InspectNetwork(name)
		Expect(results).To(HaveLen(1))
		Expect(results[0].Subnets).To(HaveLen(2))
		Expect(results[0].Subnets[0].Subnet.String()).To(Equal(subnet1))
		Expect(results[0].Subnets[1].Subnet.String()).To(Equal(subnet2))
		Expect(results[0].IPv6Enabled).To(BeFalse())
	})

	It("podman network create with multiple subnets dual stack", func() {
		name := "subnets-" + stringid.GenerateRandomID()
		subnet1 := "10.10.2.0/24"
		subnet2 := "fd52:2a5a:747e:3acd::/64"
		nc := podmanTest.Podman([]string{"network", "create", "--subnet", subnet1, "--subnet", subnet2, name})
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(name)
		Expect(nc).To(ExitCleanly())
		Expect(nc.OutputToString()).To(Equal(name))

		results := podmanTest.InspectNetwork(name)
		Expect(results).To(HaveLen(1))
		Expect(results[0].Subnets).To(HaveLen(2))
		Expect(results[0].Subnets[0].Subnet.String()).To(Equal(subnet1))
		Expect(results[0].Subnets[1].Subnet.String()).To(Equal(subnet2))
		Expect(results[0].IPv6Enabled).To(BeTrue())
	})

	It("podman network create with multiple subnets dual stack with gateway and range", func() {
		name := "subnets-" + stringid.GenerateRandomID()
		subnet1 := "10.10.3.0/24"
		gw1 := "10.10.3.10"
		range1 := "10.10.3.0/26"
		subnet2 := "fd52:2a5a:747e:3ace::/64"
		gw2 := "fd52:2a5a:747e:3ace::10"
		nc := podmanTest.Podman([]string{"network", "create", "--subnet", subnet1, "--gateway", gw1, "--ip-range", range1, "--subnet", subnet2, "--gateway", gw2, name})
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(name)
		Expect(nc).To(ExitCleanly())
		Expect(nc.OutputToString()).To(Equal(name))

		results := podmanTest.InspectNetwork(name)
		Expect(results).To(HaveLen(1))
		Expect(results[0].Subnets).To(HaveLen(2))
		Expect(results[0].Subnets[0].Subnet.String()).To(Equal(subnet1))
		Expect(results[0].Subnets[0].Gateway.String()).To(Equal(gw1))
		Expect(results[0].Subnets[0].LeaseRange.StartIP.String()).To(Equal("10.10.3.1"))
		Expect(results[0].Subnets[0].LeaseRange.EndIP.String()).To(Equal("10.10.3.63"))
		Expect(results[0].Subnets[1].Subnet.String()).To(Equal(subnet2))
		Expect(results[0].Subnets[1].Gateway.String()).To(Equal(gw2))
		Expect(results[0].IPv6Enabled).To(BeTrue())
	})

	It("podman network create invalid options with multiple subnets", func() {
		name := "subnets-" + stringid.GenerateRandomID()
		subnet1 := "10.10.3.0/24"
		gw1 := "10.10.3.10"
		gw2 := "fd52:2a5a:747e:3acf::10"
		nc := podmanTest.Podman([]string{"network", "create", "--subnet", subnet1, "--gateway", gw1, "--gateway", gw2, name})
		nc.WaitWithDefaultTimeout()
		Expect(nc).To(ExitWithError(125, "cannot set more gateways than subnets"))

		range1 := "10.10.3.0/26"
		range2 := "10.10.3.0/28"
		nc = podmanTest.Podman([]string{"network", "create", "--subnet", subnet1, "--ip-range", range1, "--ip-range", range2, name})
		nc.WaitWithDefaultTimeout()
		Expect(nc).To(ExitWithError(125, "cannot set more ranges than subnets"))
	})

	It("podman network create same name - fail", func() {
		name := "same-name-" + stringid.GenerateRandomID()
		networkCreateCommand := []string{"network", "create", name}
		nc := podmanTest.Podman(networkCreateCommand)
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(name)
		Expect(nc).To(ExitCleanly())
		Expect(nc.OutputToString()).To(Equal(name))

		nc = podmanTest.Podman(networkCreateCommand)
		nc.WaitWithDefaultTimeout()
		Expect(nc).To(ExitWithError(125, fmt.Sprintf("network already exists %s", name)))
	})

	It("podman network create same name - succeed with ignore", func() {
		name := "same-name-" + stringid.GenerateRandomID()
		networkCreateCommand := []string{"network", "create", "--ignore", name}
		nc := podmanTest.Podman(networkCreateCommand)
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(name)
		Expect(nc).To(ExitCleanly())
		Expect(nc.OutputToString()).To(Equal(name))

		nc = podmanTest.Podman(networkCreateCommand)
		nc.WaitWithDefaultTimeout()
		Expect(nc).To(ExitCleanly())
		Expect(nc.OutputToString()).To(Equal(name))
	})

	It("podman network create --interface-name", func() {
		netName := "bridge-" + stringid.GenerateRandomID()
		bridgeName := "mybridge" + stringid.GenerateRandomID()[:4]
		nc := podmanTest.Podman([]string{"network", "create", "--interface-name", bridgeName, netName})
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(netName)
		Expect(nc).To(ExitCleanly())
		Expect(nc.OutputToString()).To(Equal(netName))

		session := podmanTest.Podman([]string{"network", "inspect", "--format", "{{.NetworkInterface}}", netName})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitCleanly())
		Expect(session.OutputToString()).To(Equal(bridgeName))

		session = podmanTest.Podman([]string{"run", "-d", "--network", netName, ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitCleanly())

		// can only check this as root
		if !isRootless() {
			// make sure netavark created bridge with expected name
			bridge, err := net.InterfaceByName(bridgeName)
			Expect(err).ToNot(HaveOccurred())
			Expect(bridge.Name).To(Equal(bridgeName))
		}
	})

	It("podman network create --ip-range sip-eip", func() {
		netName := "subnet-" + stringid.GenerateRandomID()
		nc := podmanTest.Podman([]string{"network", "create", "--subnet", "10.11.16.0/24", "--ip-range", "10.11.16.11-10.11.16.12", netName})
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(netName)
		Expect(nc).Should(ExitCleanly())

		// Inspect the network configuration
		results := podmanTest.InspectNetwork(netName)
		Expect(results).To(HaveLen(1))
		result := results[0]
		Expect(result).To(HaveField("Name", netName))
		Expect(result.Subnets).To(HaveLen(1))
		Expect(result.Subnets[0].Subnet.String()).To(Equal("10.11.16.0/24"))
		Expect(result.Subnets[0].Gateway.String()).To(Equal("10.11.16.1"))
		Expect(result.Subnets[0].LeaseRange).ToNot(BeNil())
		Expect(result.Subnets[0].LeaseRange.StartIP.String()).To(Equal("10.11.16.11"))
		Expect(result.Subnets[0].LeaseRange.EndIP.String()).To(Equal("10.11.16.12"))

		try := podmanTest.Podman([]string{"run", "--rm", "--network", netName, ALPINE, "sh", "-c", "ip addr show eth0 |  awk ' /inet / {print $2}'"})
		try.WaitWithDefaultTimeout()
		Expect(try).To(ExitCleanly())

		containerIP, _, err := net.ParseCIDR(try.OutputToString())
		Expect(err).ToNot(HaveOccurred())
		// Note as of today (June 2023) we always get the first ip from netavark but let's not depend on that.
		// All we care about is the ip is from the range which allows for both.
		Expect(containerIP.String()).To(Or(Equal("10.11.16.11"), Equal("10.11.16.12")), "ip address must be in --ip-range")
	})

	DescribeTable("podman network create with special route types",
		func(subnet string, route string, expectedDest string, expectedRouteType types.RouteType, expectedMetric *uint32) {
			netName := "subnet-" + stringid.GenerateRandomID()
			nc := podmanTest.Podman([]string{
				"network",
				"create",
				"--subnet",
				subnet,
				"--route",
				route,
				netName,
			})
			nc.WaitWithDefaultTimeout()
			defer podmanTest.removeNetwork(netName)
			Expect(nc).Should(ExitCleanly())

			inspect := podmanTest.Podman([]string{"network", "inspect", netName})
			inspect.WaitWithDefaultTimeout()
			Expect(inspect).Should(ExitCleanly())

			var results []entities.NetworkInspectReport
			err := json.Unmarshal([]byte(inspect.OutputToString()), &results)
			Expect(err).ToNot(HaveOccurred())
			Expect(results).To(HaveLen(1))
			result := results[0]
			Expect(result).To(HaveField("Name", netName))
			Expect(result.Subnets).To(HaveLen(1))
			Expect(result.Subnets[0].Subnet.String()).To(Equal(subnet))
			Expect(result.Routes).To(HaveLen(1))
			Expect(result.Routes[0].Destination.String()).To(Equal(expectedDest))
			Expect(result.Routes[0].Gateway).To(BeNil())
			Expect(result.Routes[0].RouteType).To(Equal(expectedRouteType))
			if expectedMetric != nil {
				Expect(*result.Routes[0].Metric).To(Equal(*expectedMetric))
			} else {
				Expect(result.Routes[0].Metric).To(BeNil())
			}

			defer removeNetworkDevice(result.NetworkInterface)
		},
		Entry("blackhole route", "10.19.20.0/24", "10.21.10.0/24,blackhole", "10.21.10.0/24", types.RouteTypeBlackhole, nil),
		Entry("unreachable route", "10.19.21.0/24", "10.21.11.0/24,unreachable", "10.21.11.0/24", types.RouteTypeUnreachable, nil),
		Entry("prohibit route", "10.19.22.0/24", "10.21.12.0/24,prohibit", "10.21.12.0/24", types.RouteTypeProhibit, nil),
		Entry("blackhole route with metric", "10.19.23.0/24", "10.21.13.0/24,blackhole,250", "10.21.13.0/24", types.RouteTypeBlackhole, uintPtr(250)),
	)
})
