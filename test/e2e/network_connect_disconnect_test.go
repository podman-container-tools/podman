//go:build linux || freebsd

package integration

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	. "go.podman.io/podman/v6/test/utils"
	"go.podman.io/storage/pkg/stringid"
)

var _ = Describe("Podman network connect and disconnect", func() {
	It("bad network name in disconnect should result in error", func() {
		dis := podmanTest.Podman([]string{"network", "disconnect", "foobar", "test"})
		dis.WaitWithDefaultTimeout()
		Expect(dis).Should(ExitWithError(125, `no container with name or ID "test" found: no such container`))
	})

	It("bad container name in network disconnect should result in error", func() {
		netName := "aliasTest" + stringid.GenerateRandomID()
		session := podmanTest.Podman([]string{"network", "create", netName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		defer podmanTest.removeNetwork(netName)

		dis := podmanTest.Podman([]string{"network", "disconnect", netName, "foobar"})
		dis.WaitWithDefaultTimeout()
		Expect(dis).Should(ExitWithError(125, `no container with name or ID "foobar" found: no such container`))
	})

	It("podman network disconnect", func() {
		netName := "aliasTest" + stringid.GenerateRandomID()
		session := podmanTest.Podman([]string{"network", "create", netName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		defer podmanTest.removeNetwork(netName)

		gw := podmanTest.Podman([]string{"network", "inspect", netName, "--format", "{{(index .Subnets 0).Gateway}}"})
		gw.WaitWithDefaultTimeout()
		Expect(gw).Should(ExitCleanly())
		ns := gw.OutputToString()

		ctr := podmanTest.Podman([]string{"run", "-dt", "--name", "test", "--network", netName, ALPINE, "top"})
		ctr.WaitWithDefaultTimeout()
		Expect(ctr).Should(ExitCleanly())

		exec := podmanTest.Podman([]string{"exec", "test", "ip", "addr", "show", "eth0"})
		exec.WaitWithDefaultTimeout()
		Expect(exec).Should(ExitCleanly())

		exec2 := podmanTest.Podman([]string{"exec", "test", "cat", "/etc/resolv.conf"})
		exec2.WaitWithDefaultTimeout()
		Expect(exec2).Should(ExitCleanly())
		Expect(exec2.OutputToString()).To(ContainSubstring(ns))

		dis := podmanTest.Podman([]string{"network", "disconnect", netName, "test"})
		dis.WaitWithDefaultTimeout()
		Expect(dis).Should(ExitCleanly())
		Expect(dis.ErrorToString()).Should(Equal(""))

		inspect := podmanTest.Podman([]string{"container", "inspect", "test", "--format", "{{len .NetworkSettings.Networks}}"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(Equal("0"))

		exec = podmanTest.Podman([]string{"exec", "test", "ip", "addr", "show", "eth0"})
		exec.WaitWithDefaultTimeout()
		Expect(exec).Should(ExitWithError(1, "ip: can't find device 'eth0'"))

		exec3 := podmanTest.Podman([]string{"exec", "test", "cat", "/etc/resolv.conf"})
		exec3.WaitWithDefaultTimeout()
		Expect(exec3).Should(ExitCleanly())
		Expect(exec3.OutputToString()).ToNot(ContainSubstring(ns))

		// make sure stats still works https://github.com/containers/podman/issues/13824
		stats := podmanTest.Podman([]string{"stats", "test", "--no-stream"})
		stats.WaitWithDefaultTimeout()
		Expect(stats).Should(ExitCleanly())
	})

	It("bad network name in connect should result in error", func() {
		session := podmanTest.Podman([]string{"create", "--name", "testContainer", "--network", "bridge", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		dis := podmanTest.Podman([]string{"network", "connect", "nonexistent-network", "testContainer"})
		dis.WaitWithDefaultTimeout()
		Expect(dis).Should(ExitWithError(125, "unable to find network with name or ID nonexistent-network: network not found"))
	})

	It("bad container name in network connect should result in error", func() {
		netName := "aliasTest" + stringid.GenerateRandomID()
		session := podmanTest.Podman([]string{"network", "create", netName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		defer podmanTest.removeNetwork(netName)

		dis := podmanTest.Podman([]string{"network", "connect", netName, "foobar"})
		dis.WaitWithDefaultTimeout()
		Expect(dis).Should(ExitWithError(125, `no container with name or ID "foobar" found: no such container`))
	})

	It("podman connect on a container that already is connected to the network should error after init", func() {
		netName := "aliasTest" + stringid.GenerateRandomID()
		session := podmanTest.Podman([]string{"network", "create", netName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		defer podmanTest.removeNetwork(netName)

		ctr := podmanTest.Podman([]string{"create", "--name", "test", "--network", netName, ALPINE, "top"})
		ctr.WaitWithDefaultTimeout()
		Expect(ctr).Should(ExitCleanly())
		cid := ctr.OutputToString()

		// network alias container short id is always added and shown in inspect
		inspect := podmanTest.Podman([]string{"container", "inspect", "test", "--format", "{{(index .NetworkSettings.Networks \"" + netName + "\").Aliases}}"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(Equal("[" + cid[0:12] + "]"))

		con := podmanTest.Podman([]string{"network", "connect", netName, "test"})
		con.WaitWithDefaultTimeout()
		Expect(con).Should(ExitCleanly())

		init := podmanTest.Podman([]string{"init", "test"})
		init.WaitWithDefaultTimeout()
		Expect(init).Should(ExitCleanly())

		con2 := podmanTest.Podman([]string{"network", "connect", netName, "test"})
		con2.WaitWithDefaultTimeout()
		Expect(con2).Should(ExitWithError(125, fmt.Sprintf("container %s is already connected to network %s: network is already connected", cid, netName)))
	})

	It("podman network connect", func() {
		netName := "aliasTest" + stringid.GenerateRandomID()
		session := podmanTest.Podman([]string{"network", "create", netName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		defer podmanTest.removeNetwork(netName)

		ctr := podmanTest.Podman([]string{"run", "-dt", "--name", "test", "--network", netName, ALPINE, "top"})
		ctr.WaitWithDefaultTimeout()
		Expect(ctr).Should(ExitCleanly())
		cid := ctr.OutputToString()

		exec := podmanTest.Podman([]string{"exec", "test", "ip", "addr", "show", "eth0"})
		exec.WaitWithDefaultTimeout()
		Expect(exec).Should(ExitCleanly())

		// Create a second network
		newNetName := "aliasTest" + stringid.GenerateRandomID()
		session = podmanTest.Podman([]string{"network", "create", newNetName, "--subnet", "10.11.100.0/24"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		defer podmanTest.removeNetwork(newNetName)

		gw := podmanTest.Podman([]string{"network", "inspect", newNetName, "--format", "{{(index .Subnets 0).Gateway}}"})
		gw.WaitWithDefaultTimeout()
		Expect(gw).Should(ExitCleanly())
		ns := gw.OutputToString()

		exec2 := podmanTest.Podman([]string{"exec", "test", "cat", "/etc/resolv.conf"})
		exec2.WaitWithDefaultTimeout()
		Expect(exec2).Should(ExitCleanly())
		Expect(exec2.OutputToString()).ToNot(ContainSubstring(ns))

		ip := "10.11.100.99"
		mac := "44:11:44:11:44:11"
		connect := podmanTest.Podman([]string{"network", "connect", "--ip", ip, "--mac-address", mac, newNetName, "test"})
		connect.WaitWithDefaultTimeout()
		Expect(connect).Should(ExitCleanly())
		Expect(connect.ErrorToString()).Should(Equal(""))

		inspect := podmanTest.Podman([]string{"container", "inspect", "test", "--format", "{{len .NetworkSettings.Networks}}"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(Equal("2"))

		// network alias container short id is always added and shown in inspect
		inspect = podmanTest.Podman([]string{"container", "inspect", "test", "--format", "{{(index .NetworkSettings.Networks \"" + newNetName + "\").Aliases}}"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(Equal("[" + cid[0:12] + "]"))

		exec = podmanTest.Podman([]string{"exec", "test", "ip", "addr", "show", "eth1"})
		exec.WaitWithDefaultTimeout()
		Expect(exec).Should(ExitCleanly())
		Expect(exec.OutputToString()).Should(ContainSubstring(ip))
		Expect(exec.OutputToString()).Should(ContainSubstring(mac))

		exec3 := podmanTest.Podman([]string{"exec", "test", "cat", "/etc/resolv.conf"})
		exec3.WaitWithDefaultTimeout()
		Expect(exec3).Should(ExitCleanly())
		Expect(exec3.OutputToString()).To(ContainSubstring(ns))

		// make sure stats works https://github.com/containers/podman/issues/13824
		stats := podmanTest.Podman([]string{"stats", "test", "--no-stream"})
		stats.WaitWithDefaultTimeout()
		Expect(stats).Should(ExitCleanly())

		// make sure no logrus errors are shown https://github.com/containers/podman/issues/9602
		rm := podmanTest.Podman([]string{"rm", "--time=0", "-f", "test"})
		rm.WaitWithDefaultTimeout()
		Expect(rm).Should(ExitCleanly())
		Expect(rm.ErrorToString()).To(Equal(""))
	})

	It("podman network connect when not running", func() {
		netName1 := "connect1" + stringid.GenerateRandomID()
		session := podmanTest.Podman([]string{"network", "create", netName1})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		defer podmanTest.removeNetwork(netName1)

		netName2 := "connect2" + stringid.GenerateRandomID()
		session = podmanTest.Podman([]string{"network", "create", netName2})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		defer podmanTest.removeNetwork(netName2)

		ctr := podmanTest.Podman([]string{"create", "--name", "test", "--network", netName1, ALPINE, "top"})
		ctr.WaitWithDefaultTimeout()
		Expect(ctr).Should(ExitCleanly())

		dis := podmanTest.Podman([]string{"network", "connect", netName2, "test"})
		dis.WaitWithDefaultTimeout()
		Expect(dis).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"container", "inspect", "test", "--format", "{{len .NetworkSettings.Networks}}"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(Equal("2"))

		start := podmanTest.Podman([]string{"start", "test"})
		start.WaitWithDefaultTimeout()
		Expect(start).Should(ExitCleanly())

		exec := podmanTest.Podman([]string{"exec", "test", "ip", "addr", "show", "eth0"})
		exec.WaitWithDefaultTimeout()
		Expect(exec).Should(ExitCleanly())

		exec = podmanTest.Podman([]string{"exec", "test", "ip", "addr", "show", "eth1"})
		exec.WaitWithDefaultTimeout()
		Expect(exec).Should(ExitCleanly())
	})

	It("podman network connect and run with network ID", func() {
		netName := "ID" + stringid.GenerateRandomID()
		session := podmanTest.Podman([]string{"network", "create", netName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		defer podmanTest.removeNetwork(netName)

		session = podmanTest.Podman([]string{"network", "ls", "--format", "{{.ID}}", "--filter", "name=" + netName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		netID := session.OutputToString()

		ctr := podmanTest.Podman([]string{"run", "-dt", "--name", "test", "--network", netID, "--network-alias", "somealias", ALPINE, "top"})
		ctr.WaitWithDefaultTimeout()
		Expect(ctr).Should(ExitCleanly())

		exec := podmanTest.Podman([]string{"exec", "test", "ip", "addr", "show", "eth0"})
		exec.WaitWithDefaultTimeout()
		Expect(exec).Should(ExitCleanly())

		// Create a second network
		newNetName := "ID2" + stringid.GenerateRandomID()
		session = podmanTest.Podman([]string{"network", "create", newNetName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		defer podmanTest.removeNetwork(newNetName)

		session = podmanTest.Podman([]string{"network", "ls", "--format", "{{.ID}}", "--filter", "name=" + newNetName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		newNetID := session.OutputToString()

		connect := podmanTest.Podman([]string{"network", "connect", "--alias", "secondalias", newNetID, "test"})
		connect.WaitWithDefaultTimeout()
		Expect(connect).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"container", "inspect", "test", "--format", "{{.NetworkSettings.Networks}}"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(ContainSubstring(netName))
		Expect(inspect.OutputToString()).To(ContainSubstring(newNetName))

		exec = podmanTest.Podman([]string{"exec", "test", "ip", "addr", "show", "eth1"})
		exec.WaitWithDefaultTimeout()
		Expect(exec).Should(ExitCleanly())
	})

	It("podman network disconnect when not running", func() {
		netName1 := "aliasTest" + stringid.GenerateRandomID()
		session := podmanTest.Podman([]string{"network", "create", netName1})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		defer podmanTest.removeNetwork(netName1)

		netName2 := "aliasTest" + stringid.GenerateRandomID()
		session2 := podmanTest.Podman([]string{"network", "create", netName2})
		session2.WaitWithDefaultTimeout()
		Expect(session2).Should(ExitCleanly())
		defer podmanTest.removeNetwork(netName2)

		ctr := podmanTest.Podman([]string{"create", "--name", "test", "--network", netName1 + "," + netName2, ALPINE, "top"})
		ctr.WaitWithDefaultTimeout()
		Expect(ctr).Should(ExitCleanly())

		dis := podmanTest.Podman([]string{"network", "disconnect", netName1, "test"})
		dis.WaitWithDefaultTimeout()
		Expect(dis).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"container", "inspect", "test", "--format", "{{len .NetworkSettings.Networks}}"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(Equal("1"))

		start := podmanTest.Podman([]string{"start", "test"})
		start.WaitWithDefaultTimeout()
		Expect(start).Should(ExitCleanly())

		exec := podmanTest.Podman([]string{"exec", "test", "ip", "addr", "show", "eth0"})
		exec.WaitWithDefaultTimeout()

		// because the network interface order is not guaranteed to be the same, we have to check both eth0 and eth1.
		// if eth0 did not exist, eth1 has to exist.
		var exitMatcher types.GomegaMatcher = ExitWithError(1, "ip: can't find device 'eth1'")
		if exec.ExitCode() > 0 {
			Expect(exec).To(ExitWithError(1, "ip: can't find device 'eth0'"))
			exitMatcher = ExitCleanly()
		}

		exec = podmanTest.Podman([]string{"exec", "test", "ip", "addr", "show", "eth1"})
		exec.WaitWithDefaultTimeout()
		Expect(exec).Should(exitMatcher)
	})

	It("podman network disconnect and run with network ID", func() {
		netName := "aliasTest" + stringid.GenerateRandomID()
		session := podmanTest.Podman([]string{"network", "create", netName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		defer podmanTest.removeNetwork(netName)

		session = podmanTest.Podman([]string{"network", "ls", "--format", "{{.ID}}", "--filter", "name=" + netName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		netID := session.OutputToString()

		ctr := podmanTest.Podman([]string{"run", "-dt", "--name", "test", "--network", netID, ALPINE, "top"})
		ctr.WaitWithDefaultTimeout()
		Expect(ctr).Should(ExitCleanly())

		exec := podmanTest.Podman([]string{"exec", "test", "ip", "addr", "show", "eth0"})
		exec.WaitWithDefaultTimeout()
		Expect(exec).Should(ExitCleanly())

		dis := podmanTest.Podman([]string{"network", "disconnect", netID, "test"})
		dis.WaitWithDefaultTimeout()
		Expect(dis).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"container", "inspect", "test", "--format", "{{len .NetworkSettings.Networks}}"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(Equal("0"))

		exec = podmanTest.Podman([]string{"exec", "test", "ip", "addr", "show", "eth0"})
		exec.WaitWithDefaultTimeout()
		Expect(exec).Should(ExitWithError(1, "ip: can't find device 'eth0'"))
	})
})
