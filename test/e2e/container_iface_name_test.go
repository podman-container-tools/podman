//go:build linux || freebsd

package integration

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
)

func createNetworkDevice(name string) {
	session := SystemExec("ip", []string{"link", "add", name, "type", "bridge"})
	session.WaitWithDefaultTimeout()
	Expect(session).Should(ExitCleanly())
}

func deleteNetworkDevice(name string) {
	session := SystemExec("ip", []string{"link", "delete", name})
	session.WaitWithDefaultTimeout()
	Expect(session).Should(ExitCleanly())
}

func createContainersConfFileWithDeviceIfaceName(pTest *PodmanTestIntegration) {
	configPath := filepath.Join(pTest.TempDir, "containers.conf")
	containersConf := []byte("[containers]\ninterface_name = \"device\"\n")
	err := os.WriteFile(configPath, containersConf, os.ModePerm)
	Expect(err).ToNot(HaveOccurred())

	// Set custom containers.conf file
	os.Setenv("CONTAINERS_CONF_OVERRIDE", configPath)
	if IsRemote() {
		pTest.RestartRemoteService()
	}
}

var _ = Describe("Podman container interface name", func() {
	It("podman container interface name for bridge network", func() {
		// Assert that the network interface name inside container for
		// bridge network is ethX regardless of interface_name setting
		// in the containers.conf file.

		netName1 := createNetworkName("bridge")
		netName2 := createNetworkName("bridge")

		defer podmanTest.removeNetwork(netName1)
		nc1 := podmanTest.Podman([]string{"network", "create", netName1})
		nc1.WaitWithDefaultTimeout()
		Expect(nc1).Should(ExitCleanly())

		defer podmanTest.removeNetwork(netName2)
		nc2 := podmanTest.Podman([]string{"network", "create", netName2})
		nc2.WaitWithDefaultTimeout()
		Expect(nc2).Should(ExitCleanly())

		for _, override := range []bool{false, true} {
			if override {
				createContainersConfFileWithDeviceIfaceName(podmanTest)
			}

			ctr := podmanTest.Podman([]string{"run", "-d", "--network", netName1, "--name", "test", ALPINE, "top"})
			ctr.WaitWithDefaultTimeout()
			Expect(ctr).Should(ExitCleanly())

			exec1 := podmanTest.Podman([]string{"exec", "test", "ip", "addr", "show", "eth0"})
			exec1.WaitWithDefaultTimeout()
			Expect(exec1).Should(ExitCleanly())
			Expect(exec1.OutputToString()).Should(ContainSubstring("eth0"))

			conn := podmanTest.Podman([]string{"network", "connect", netName2, "test"})
			conn.WaitWithDefaultTimeout()
			Expect(conn).Should(ExitCleanly())

			exec2 := podmanTest.Podman([]string{"exec", "test", "ip", "addr", "show", "eth1"})
			exec2.WaitWithDefaultTimeout()
			Expect(exec2).Should(ExitCleanly())
			Expect(exec2.OutputToString()).Should(ContainSubstring("eth1"))

			rm := podmanTest.Podman([]string{"rm", "--time=0", "-f", "test"})
			rm.WaitWithDefaultTimeout()
			Expect(rm).Should(ExitCleanly())
		}
	})

	It("podman container interface name for macvlan/ipvlan network with no parent", func() {
		// Assert that the network interface name inside container for
		// macvlan/ipvlan network with no parent interface is ethX
		// regardless of interface_name setting in the containers.conf
		// file.

		for _, override := range []bool{false, true} {
			if override {
				createContainersConfFileWithDeviceIfaceName(podmanTest)
			}

			for _, driverType := range []string{"macvlan", "ipvlan"} {
				netName1 := createNetworkName(driverType)
				netName2 := createNetworkName(driverType)

				// There is no nic created by the macvlan/ipvlan driver.
				defer podmanTest.removeNetwork(netName1)
				nc1 := podmanTest.Podman([]string{"network", "create", "-d", driverType, "--subnet", "10.9.0.0/24", netName1})
				nc1.WaitWithDefaultTimeout()
				Expect(nc1).Should(ExitCleanly())

				ctr := podmanTest.Podman([]string{"run", "-d", "--network", netName1, "--name", "test", ALPINE, "top"})
				ctr.WaitWithDefaultTimeout()
				Expect(ctr).Should(ExitCleanly())

				exec1 := podmanTest.Podman([]string{"exec", "test", "ip", "addr", "show", "eth0"})
				exec1.WaitWithDefaultTimeout()
				Expect(exec1).Should(ExitCleanly())
				Expect(exec1.OutputToString()).Should(ContainSubstring("eth0"))

				defer podmanTest.removeNetwork(netName2)
				nc2 := podmanTest.Podman([]string{"network", "create", "-d", driverType, "--subnet", "10.25.41.0/24", netName2})
				nc2.WaitWithDefaultTimeout()
				Expect(nc2).Should(ExitCleanly())

				conn := podmanTest.Podman([]string{"network", "connect", netName2, "test"})
				conn.WaitWithDefaultTimeout()
				Expect(conn).Should(ExitCleanly())

				exec2 := podmanTest.Podman([]string{"exec", "test", "ip", "addr", "show", "eth1"})
				exec2.WaitWithDefaultTimeout()
				Expect(exec2).Should(ExitCleanly())
				Expect(exec2.OutputToString()).Should(ContainSubstring("eth1"))

				rm := podmanTest.Podman([]string{"rm", "--time=0", "-f", "test"})
				rm.WaitWithDefaultTimeout()
				Expect(rm).Should(ExitCleanly())
			}
		}
	})

	It("podman container interface name with default scheme for macvlan/ipvlan network with parent", func() {
		// Assert that the network interface name inside container for
		// macvlan/ipvlan network, created with a specific parent
		// interface, continues to be ethX when interface_name in the
		// containers.conf file is set to default value, i.e., "".

		SkipIfRootless("cannot create network device in rootless mode.")

		for _, driverType := range []string{"macvlan", "ipvlan"} {
			// Create a nic to be used as a parent for macvlan/ipvlan network.
			nicName1 := createNetworkName("nic")[:8]
			nicName2 := createNetworkName("nic")[:8]

			netName1 := createNetworkName(driverType)
			netName2 := createNetworkName(driverType)

			parent1 := "parent=" + nicName1
			parent2 := "parent=" + nicName2

			defer deleteNetworkDevice(nicName1)
			createNetworkDevice(nicName1)

			defer podmanTest.removeNetwork(netName1)
			nc1 := podmanTest.Podman([]string{"network", "create", "-d", driverType, "-o", parent1, "--subnet", "10.9.1.0/24", netName1})
			nc1.WaitWithDefaultTimeout()
			Expect(nc1).Should(ExitCleanly())

			ctr := podmanTest.Podman([]string{"run", "-d", "--network", netName1, "--name", "test", ALPINE, "top"})
			ctr.WaitWithDefaultTimeout()
			Expect(ctr).Should(ExitCleanly())

			exec1 := podmanTest.Podman([]string{"exec", "test", "ip", "addr", "show", "eth0"})
			exec1.WaitWithDefaultTimeout()
			Expect(exec1).Should(ExitCleanly())
			Expect(exec1.OutputToString()).Should(ContainSubstring("eth0"))

			defer deleteNetworkDevice(nicName2)
			createNetworkDevice(nicName2)

			defer podmanTest.removeNetwork(netName2)
			nc2 := podmanTest.Podman([]string{"network", "create", "-d", driverType, "-o", parent2, "--subnet", "10.25.42.0/24", netName2})
			nc2.WaitWithDefaultTimeout()
			Expect(nc2).Should(ExitCleanly())

			conn := podmanTest.Podman([]string{"network", "connect", netName2, "test"})
			conn.WaitWithDefaultTimeout()
			Expect(conn).Should(ExitCleanly())

			exec2 := podmanTest.Podman([]string{"exec", "test", "ip", "addr", "show", "eth1"})
			exec2.WaitWithDefaultTimeout()
			Expect(exec2).Should(ExitCleanly())
			Expect(exec2.OutputToString()).Should(ContainSubstring("eth1"))

			rm := podmanTest.Podman([]string{"rm", "--time=0", "-f", "test"})
			rm.WaitWithDefaultTimeout()
			Expect(rm).Should(ExitCleanly())
		}
	})

	It("podman container interface name with device scheme for macvlan/ipvlan network with parent", func() {
		// Assert that the network interface name inside container for
		// macvlan/ipvlan network, created with a specific parent
		// interface, is the parent interface name ethX when
		// interface_name in the containers.conf file is set to "device"

		SkipIfRootless("cannot create network device in rootless mode.")

		createContainersConfFileWithDeviceIfaceName(podmanTest)

		for _, driverType := range []string{"macvlan", "ipvlan"} {
			// Create a nic to be used as a parent for the network.
			nicName1 := createNetworkName("nic")[:8]
			nicName2 := createNetworkName("nic")[:8]

			netName1 := createNetworkName(driverType)
			netName2 := createNetworkName(driverType)

			parent1 := "parent=" + nicName1
			parent2 := "parent=" + nicName2

			defer deleteNetworkDevice(nicName1)
			createNetworkDevice(nicName1)

			defer podmanTest.removeNetwork(netName1)
			nc1 := podmanTest.Podman([]string{"network", "create", "-d", driverType, "-o", parent1, "--subnet", "10.9.2.0/24", netName1})
			nc1.WaitWithDefaultTimeout()
			Expect(nc1).Should(ExitCleanly())

			ctr := podmanTest.Podman([]string{"run", "-d", "--network", netName1, "--name", "test", ALPINE, "top"})
			ctr.WaitWithDefaultTimeout()
			Expect(ctr).Should(ExitCleanly())

			exec1 := podmanTest.Podman([]string{"exec", "test", "ip", "addr", "show", nicName1})
			exec1.WaitWithDefaultTimeout()
			Expect(exec1).Should(ExitCleanly())
			Expect(exec1.OutputToString()).Should(ContainSubstring(nicName1))

			defer deleteNetworkDevice(nicName2)
			createNetworkDevice(nicName2)

			defer podmanTest.removeNetwork(netName2)
			nc2 := podmanTest.Podman([]string{"network", "create", "-d", driverType, "-o", parent2, "--subnet", "10.25.43.0/24", netName2})
			nc2.WaitWithDefaultTimeout()
			Expect(nc2).Should(ExitCleanly())

			conn := podmanTest.Podman([]string{"network", "connect", netName2, "test"})
			conn.WaitWithDefaultTimeout()
			Expect(conn).Should(ExitCleanly())

			exec2 := podmanTest.Podman([]string{"exec", "test", "ip", "addr", "show", nicName2})
			exec2.WaitWithDefaultTimeout()
			Expect(exec2).Should(ExitCleanly())
			Expect(exec2.OutputToString()).Should(ContainSubstring(nicName2))

			rm := podmanTest.Podman([]string{"rm", "--time=0", "-f", "test"})
			rm.WaitWithDefaultTimeout()
			Expect(rm).Should(ExitCleanly())
		}
	})
})
