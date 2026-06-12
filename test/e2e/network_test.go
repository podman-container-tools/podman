//go:build linux || freebsd

package integration

import (
	"fmt"
	"net"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
	. "go.podman.io/podman/v6/test/utils"
	"go.podman.io/storage/pkg/stringid"
)

var _ = Describe("Podman network", func() {
	It("podman network list", func() {
		name, path := generateNetworkConfig(podmanTest)
		defer removeConf(path)

		session := podmanTest.Podman([]string{"network", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(name))
	})

	It("podman network list -q", func() {
		name, path := generateNetworkConfig(podmanTest)
		defer removeConf(path)

		session := podmanTest.Podman([]string{"network", "ls", "--quiet"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(name))
	})

	It("podman network list --filter success", func() {
		name, path := generateNetworkConfig(podmanTest)
		defer removeConf(path)

		session := podmanTest.Podman([]string{"network", "ls", "--filter", "driver=bridge"})
		session.WaitWithDefaultTimeout()
		// Cannot ExitCleanly(): "stat ~/.config/.../*.conflist: ENOENT"
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring(name))
	})

	It("podman network list --filter driver and name", func() {
		name, path := generateNetworkConfig(podmanTest)
		defer removeConf(path)

		session := podmanTest.Podman([]string{"network", "ls", "--filter", "driver=bridge", "--filter", "name=" + name})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(name))
	})

	It("podman network list --filter two names", func() {
		name1, path1 := generateNetworkConfig(podmanTest)
		defer removeConf(path1)

		name2, path2 := generateNetworkConfig(podmanTest)
		defer removeConf(path2)

		session := podmanTest.Podman([]string{"network", "ls", "--filter", "name=" + name1, "--filter", "name=" + name2})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(name1))
		Expect(session.OutputToString()).To(ContainSubstring(name2))
	})

	It("podman network list --filter labels", func() {
		net1 := "labelnet" + stringid.GenerateRandomID()
		label1 := "testlabel1=abc"
		label2 := "abcdef"
		session := podmanTest.Podman([]string{"network", "create", "--label", label1, net1})
		session.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(net1)
		Expect(session).Should(ExitCleanly())

		net2 := "labelnet" + stringid.GenerateRandomID()
		session = podmanTest.Podman([]string{"network", "create", "--label", label1, "--label", label2, net2})
		session.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(net2)
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"network", "ls", "--filter", "label=" + label1})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(net1))
		Expect(session.OutputToString()).To(ContainSubstring(net2))

		session = podmanTest.Podman([]string{"network", "ls", "--filter", "label=" + label1, "--filter", "label=" + label2})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).ToNot(ContainSubstring(net1))
		Expect(session.OutputToString()).To(ContainSubstring(net2))
	})

	It("podman network list --filter invalid value", func() {
		net := "net" + stringid.GenerateRandomID()
		session := podmanTest.Podman([]string{"network", "create", net})
		session.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(net)
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"network", "ls", "--filter", "namr=ab"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, `invalid filter "namr"`))
	})

	It("podman network list --filter failure", func() {
		name, path := generateNetworkConfig(podmanTest)
		defer removeConf(path)

		session := podmanTest.Podman([]string{"network", "ls", "--filter", "label=abc"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Not(ContainSubstring(name)))
	})

	It("podman network list --filter dangling", func() {
		name, path := generateNetworkConfig(podmanTest)
		defer removeConf(path)

		session := podmanTest.Podman([]string{"network", "ls", "--filter", "dangling=true"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(name))

		session = podmanTest.Podman([]string{"network", "ls", "--filter", "dangling=false"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).NotTo(ContainSubstring(name))

		session = podmanTest.Podman([]string{"network", "ls", "--filter", "dangling=foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, `invalid dangling filter value "foo"`))
	})

	It("podman network ID test", func() {
		net := "networkIDTest"
		session := podmanTest.Podman([]string{"network", "create", net})
		session.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(net)
		Expect(session).Should(ExitCleanly())

		// Get the network ID
		getid := podmanTest.Podman([]string{"network", "inspect", net, "--format", "{{.ID}}"})
		getid.WaitWithDefaultTimeout()
		Expect(getid).Should(ExitCleanly())
		netID := getid.OutputToString()

		// Tests Default Table Output
		session = podmanTest.Podman([]string{"network", "ls", "--filter", "id=" + netID})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		expectedTable := "NETWORK ID NAME DRIVER"
		Expect(session.OutputToString()).To(ContainSubstring(expectedTable))

		session = podmanTest.Podman([]string{"network", "ls", "--format", "{{.Name}} {{.ID}}", "--filter", "id=" + netID})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(net + " " + netID[:12]))

		session = podmanTest.Podman([]string{"network", "ls", "--format", "{{.Name}} {{.ID}}", "--filter", "id=" + netID[:50], "--no-trunc"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(net + " " + netID))

		session = podmanTest.Podman([]string{"network", "inspect", netID[:40]})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(net))

		session = podmanTest.Podman([]string{"network", "inspect", netID[1:]})
		session.WaitWithDefaultTimeout()
		expectMessage := fmt.Sprintf("network %s: ", netID[1:])
		// FIXME-someday: figure out why this part does not show up in remote
		if !IsRemote() {
			expectMessage += fmt.Sprintf("unable to find network with name or ID %s: ", netID[1:])
		}
		expectMessage += "network not found"
		Expect(session).Should(ExitWithError(125, expectMessage))

		session = podmanTest.Podman([]string{"network", "rm", netID})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	rmFunc := func(rm string) {
		It(fmt.Sprintf("podman network %s no args", rm), func() {
			session := podmanTest.Podman([]string{"network", rm})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitWithError(125, "requires at least 1 arg(s), only received 0"))
		})

		It(fmt.Sprintf("podman network %s", rm), func() {
			name, path := generateNetworkConfig(podmanTest)
			defer removeConf(path)

			session := podmanTest.Podman([]string{"network", "ls", "--quiet"})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.OutputToString()).To(ContainSubstring(name))

			rm := podmanTest.Podman([]string{"network", rm, name})
			rm.WaitWithDefaultTimeout()
			Expect(rm).Should(ExitCleanly())

			results := podmanTest.Podman([]string{"network", "ls", "--quiet"})
			results.WaitWithDefaultTimeout()
			Expect(results).Should(ExitCleanly())
			Expect(results.OutputToString()).To(Not(ContainSubstring(name)))
		})
	}

	rmFunc("rm")
	rmFunc("remove")

	It("podman network inspect no args", func() {
		session := podmanTest.Podman([]string{"network", "inspect"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(125, "requires at least 1 arg(s), only received 0"))
	})

	It("podman network inspect", func() {
		name, path := generateNetworkConfig(podmanTest)
		defer removeConf(path)

		expectedNetworks := []string{name}
		if !isRootless() {
			expectedNetworks = append(expectedNetworks, "podman")
		}
		session := podmanTest.Podman(append([]string{"network", "inspect"}, expectedNetworks...))
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(BeValidJSON())
	})

	It("podman network inspect", func() {
		name, path := generateNetworkConfig(podmanTest)
		defer removeConf(path)

		session := podmanTest.Podman([]string{"network", "inspect", name, "--format", "{{.Driver}}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("bridge"))
	})

	It("podman inspect container single network", func() {
		netName := "net-" + stringid.GenerateRandomID()
		network := podmanTest.Podman([]string{"network", "create", "--subnet", "10.50.50.0/24", netName})
		network.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(netName)
		Expect(network).Should(ExitCleanly())

		session := podmanTest.Podman([]string{"network", "inspect", netName, "--format", "{{.Id}}"})
		session.WaitWithDefaultTimeout()
		netID := session.OutputToString()

		ctrName := "testCtr"
		container := podmanTest.Podman([]string{"run", "-dt", "--network", netName, "--name", ctrName, ALPINE, "top"})
		container.WaitWithDefaultTimeout()
		Expect(container).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"inspect", ctrName})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		conData := inspect.InspectContainerToJSON()
		Expect(conData).To(HaveLen(1))
		Expect(conData[0].NetworkSettings.Networks).To(HaveLen(1))
		Expect(conData[0].NetworkSettings.Networks).To(HaveKey(netName))
		net := conData[0].NetworkSettings.Networks[netName]
		Expect(net).To(HaveField("NetworkID", netID))
		Expect(net).To(HaveField("IPPrefixLen", 24))
		Expect(net.IPAddress).To(HavePrefix("10.50.50."))

		// Necessary to ensure the network is removed cleanly
		rmAll := podmanTest.Podman([]string{"rm", "-t", "0", "-f", ctrName})
		rmAll.WaitWithDefaultTimeout()
		Expect(rmAll).Should(ExitCleanly())
	})

	It("podman run container host interface name", func() {
		Skip("FIXME: We need netavark >= v1.14 for host interface support")

		ctrName := "testCtr"
		vethName := "my_veth" + stringid.GenerateRandomID()[:8]
		container := podmanTest.Podman([]string{"run", "-dt", "--network", "bridge:host_interface_name=" + vethName, "--name", ctrName, ALPINE, "top"})
		container.WaitWithDefaultTimeout()
		Expect(container).Should(ExitCleanly())

		if !isRootless() {
			veth, err := net.InterfaceByName(vethName)
			Expect(err).ToNot(HaveOccurred())
			Expect(veth.Name).To(Equal(vethName))
		} else {
			session := podmanTest.Podman([]string{"unshare", "--rootless-netns", "ip", "link", "show", vethName})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.OutputToString()).To(ContainSubstring(vethName))
		}
	})

	It("podman inspect container two networks (container not running)", func() {
		netName1 := "net1-" + stringid.GenerateRandomID()
		network1 := podmanTest.Podman([]string{"network", "create", netName1})
		network1.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(netName1)
		Expect(network1).Should(ExitCleanly())

		netName2 := "net2-" + stringid.GenerateRandomID()
		network2 := podmanTest.Podman([]string{"network", "create", netName2})
		network2.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(netName2)
		Expect(network2).Should(ExitCleanly())

		session := podmanTest.Podman([]string{"network", "inspect", netName1, "--format", "{{.Id}}"})
		session.WaitWithDefaultTimeout()
		netID1 := session.OutputToString()

		session = podmanTest.Podman([]string{"network", "inspect", netName2, "--format", "{{.Id}}"})
		session.WaitWithDefaultTimeout()
		netID2 := session.OutputToString()

		ctrName := "testCtr"
		container := podmanTest.Podman([]string{"create", "--network", fmt.Sprintf("%s,%s", netName1, netName2), "--name", ctrName, ALPINE, "top"})
		container.WaitWithDefaultTimeout()
		Expect(container).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"inspect", ctrName})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		conData := inspect.InspectContainerToJSON()
		Expect(conData).To(HaveLen(1))
		Expect(conData[0].NetworkSettings.Networks).To(HaveLen(2))
		Expect(conData[0].NetworkSettings.Networks).To(HaveKey(netName1))
		Expect(conData[0].NetworkSettings.Networks).To(HaveKey(netName2))
		net1 := conData[0].NetworkSettings.Networks[netName1]
		Expect(net1).To(HaveField("NetworkID", netID1))
		net2 := conData[0].NetworkSettings.Networks[netName2]
		Expect(net2).To(HaveField("NetworkID", netID2))

		// Necessary to ensure the network is removed cleanly
		rmAll := podmanTest.Podman([]string{"rm", "-t", "0", "-f", ctrName})
		rmAll.WaitWithDefaultTimeout()
		Expect(rmAll).Should(ExitCleanly())
	})

	It("podman inspect container two networks", func() {
		netName1 := "net1-" + stringid.GenerateRandomID()
		network1 := podmanTest.Podman([]string{"network", "create", "--subnet", "10.50.51.0/25", netName1})
		network1.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(netName1)
		Expect(network1).Should(ExitCleanly())

		netName2 := "net2-" + stringid.GenerateRandomID()
		network2 := podmanTest.Podman([]string{"network", "create", "--subnet", "10.50.51.128/26", netName2})
		network2.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(netName2)
		Expect(network2).Should(ExitCleanly())

		session := podmanTest.Podman([]string{"network", "inspect", netName1, "--format", "{{.Id}}"})
		session.WaitWithDefaultTimeout()
		netID1 := session.OutputToString()

		session = podmanTest.Podman([]string{"network", "inspect", netName2, "--format", "{{.Id}}"})
		session.WaitWithDefaultTimeout()
		netID2 := session.OutputToString()

		ctrName := "testCtr"
		container := podmanTest.Podman([]string{"run", "-dt", "--network", fmt.Sprintf("%s,%s", netName1, netName2), "--name", ctrName, ALPINE, "top"})
		container.WaitWithDefaultTimeout()
		Expect(container).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"inspect", ctrName})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		conData := inspect.InspectContainerToJSON()
		Expect(conData).To(HaveLen(1))
		Expect(conData[0].NetworkSettings.Networks).To(HaveLen(2))
		Expect(conData[0].NetworkSettings.Networks).To(HaveKey(netName1))
		Expect(conData[0].NetworkSettings.Networks).To(HaveKey(netName2))
		net1 := conData[0].NetworkSettings.Networks[netName1]
		Expect(net1).To(HaveField("NetworkID", netID1))
		Expect(net1).To(HaveField("IPPrefixLen", 25))
		Expect(net1.IPAddress).To(HavePrefix("10.50.51."))
		net2 := conData[0].NetworkSettings.Networks[netName2]
		Expect(net2).To(HaveField("NetworkID", netID2))
		Expect(net2).To(HaveField("IPPrefixLen", 26))
		Expect(net2.IPAddress).To(HavePrefix("10.50.51."))

		// Necessary to ensure the network is removed cleanly
		rmAll := podmanTest.Podman([]string{"rm", "-t", "0", "-f", ctrName})
		rmAll.WaitWithDefaultTimeout()
		Expect(rmAll).Should(ExitCleanly())
	})

	It("podman network remove after disconnect when container initially created with the network", func() {
		container := "test"
		network := "foo" + stringid.GenerateRandomID()

		session := podmanTest.Podman([]string{"network", "create", network})
		session.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(network)
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--name", container, "--network", network, "-d", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"network", "disconnect", network, container})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"network", "rm", network})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman network remove bogus", func() {
		session := podmanTest.Podman([]string{"network", "rm", "bogus"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(1, "unable to find network with name or ID bogus: network not found"))
	})

	It("podman network remove --ignore bogus", func() {
		session := podmanTest.PodmanExitCleanly("network", "rm", "--ignore", "bogus")
		Expect(session.OutputToString()).To(Equal(""))
	})

	It("podman network remove --force with pod", func() {
		netName := "net-" + stringid.GenerateRandomID()
		session := podmanTest.Podman([]string{"network", "create", netName})
		session.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(netName)
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"pod", "create", "--network", netName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		podID := session.OutputToString()

		session = podmanTest.Podman([]string{"create", "--pod", podID, ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"network", "rm", netName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(2, fmt.Sprintf(`"%s" has associated containers with it. Use -f to forcibly delete containers and pods: network is being used`, netName)))

		session = podmanTest.Podman([]string{"network", "rm", "-t", "0", "--force", netName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// check if pod is deleted
		session = podmanTest.Podman([]string{"pod", "exists", podID})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(1, ""))

		// check if net is deleted
		session = podmanTest.Podman([]string{"network", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Not(ContainSubstring(netName)))
	})

	It("podman network remove with two networks", func() {
		netName1 := "net1-" + stringid.GenerateRandomID()
		session := podmanTest.Podman([]string{"network", "create", netName1})
		session.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(netName1)
		Expect(session).Should(ExitCleanly())

		netName2 := "net2-" + stringid.GenerateRandomID()
		session = podmanTest.Podman([]string{"network", "create", netName2})
		session.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(netName2)
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"network", "rm", netName1, netName2})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		lines := session.OutputToStringArray()
		Expect(lines[0]).To(Equal(netName1))
		Expect(lines[1]).To(Equal(netName2))
	})

	It("podman network with multiple aliases", func() {
		var worked bool
		netName := createNetworkName("aliasTest")
		session := podmanTest.Podman([]string{"network", "create", netName})
		session.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(netName)
		Expect(session).Should(ExitCleanly())

		interval := 250 * time.Millisecond
		for range 6 {
			n := podmanTest.Podman([]string{"network", "exists", netName})
			n.WaitWithDefaultTimeout()
			worked = n.ExitCode() == 0
			if worked {
				break
			}
			time.Sleep(interval)
			interval *= 2
		}

		top := podmanTest.Podman([]string{"run", "-dt", "--name=web", "--network=" + netName, "--network-alias=web1", "--network-alias=web2", NGINX_IMAGE})
		top.WaitWithDefaultTimeout()
		Expect(top).Should(ExitCleanly())
		interval = 250 * time.Millisecond
		// Wait for the nginx service to be running
		for range 6 {
			// Test curl against the container's name
			c1 := podmanTest.Podman([]string{"run", "--dns-search", "dns.podman", "--network=" + netName, NGINX_IMAGE, "curl", "web"})
			c1.WaitWithDefaultTimeout()
			worked = c1.ExitCode() == 0
			if worked {
				break
			}
			time.Sleep(interval)
			interval *= 2
		}
		Expect(worked).To(BeTrue(), "nginx came up")

		// Nginx is now running so no need to do a loop
		// Test against the first alias
		c2 := podmanTest.Podman([]string{"run", "--dns-search", "dns.podman", "--network=" + netName, NGINX_IMAGE, "curl", "-s", "web1"})
		c2.WaitWithDefaultTimeout()
		Expect(c2).Should(ExitCleanly())

		// Test against the second alias
		c3 := podmanTest.Podman([]string{"run", "--dns-search", "dns.podman", "--network=" + netName, NGINX_IMAGE, "curl", "-s", "web2"})
		c3.WaitWithDefaultTimeout()
		Expect(c3).Should(ExitCleanly())
	})

	It("podman network create/remove macvlan as driver (-d) no device name", func() {
		net := "macvlan" + stringid.GenerateRandomID()
		nc := podmanTest.Podman([]string{"network", "create", "-d", "macvlan", net})
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(net)
		Expect(nc).Should(ExitCleanly())

		results := podmanTest.InspectNetwork(net)
		Expect(results).To(HaveLen(1))
		result := results[0]
		Expect(result).To(HaveField("NetworkInterface", ""))
		Expect(result.IPAMOptions).To(HaveKeyWithValue("driver", "dhcp"))

		nc = podmanTest.Podman([]string{"network", "rm", net})
		nc.WaitWithDefaultTimeout()
		Expect(nc).Should(ExitCleanly())
	})

	for _, opt := range []string{"-o=parent=lo", "--interface-name=lo"} {
		It(fmt.Sprintf("podman network create/remove macvlan as driver (-d) with %s", opt), func() {
			net := "macvlan" + stringid.GenerateRandomID()
			nc := podmanTest.Podman([]string{"network", "create", "-d", "macvlan", opt, net})
			nc.WaitWithDefaultTimeout()
			defer podmanTest.removeNetwork(net)
			Expect(nc).Should(ExitCleanly())

			results := podmanTest.InspectNetwork(net)
			Expect(results).To(HaveLen(1))
			result := results[0]

			Expect(result).To(HaveField("Driver", "macvlan"))
			Expect(result).To(HaveField("NetworkInterface", "lo"))
			Expect(result.IPAMOptions).To(HaveKeyWithValue("driver", "dhcp"))
			Expect(result.Subnets).To(BeEmpty())

			nc = podmanTest.Podman([]string{"network", "rm", net})
			nc.WaitWithDefaultTimeout()
			Expect(nc).Should(ExitCleanly())
		})
	}

	It("podman network create/remove ipvlan as driver (-d) with device name", func() {
		net := "ipvlan" + stringid.GenerateRandomID()
		nc := podmanTest.Podman([]string{"network", "create", "-d", "ipvlan", "-o", "parent=lo", "--subnet", "10.0.2.0/24", net})
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(net)
		Expect(nc).Should(ExitCleanly())

		results := podmanTest.InspectNetwork(net)
		Expect(results).To(HaveLen(1))
		result := results[0]

		Expect(result).To(HaveField("Driver", "ipvlan"))
		Expect(result).To(HaveField("NetworkInterface", "lo"))
		Expect(result.IPAMOptions).To(HaveKeyWithValue("driver", "host-local"))
		Expect(result.Subnets).To(HaveLen(1))

		nc = podmanTest.Podman([]string{"network", "rm", net})
		nc.WaitWithDefaultTimeout()
		Expect(nc).Should(ExitCleanly())
	})

	It("podman network exists", func() {
		net := "net" + stringid.GenerateRandomID()
		session := podmanTest.Podman([]string{"network", "create", net})
		session.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(net)
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"network", "exists", net})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"network", "exists", stringid.GenerateRandomID()})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(1, ""))
	})

	It("podman network create macvlan with network info and options", func() {
		net := "macvlan" + stringid.GenerateRandomID()
		nc := podmanTest.Podman([]string{"network", "create", "-d", "macvlan", "-o", "parent=lo", "-o", "mtu=1500", "--gateway", "192.168.1.254", "--subnet", "192.168.1.0/24", net})
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(net)
		Expect(nc).Should(ExitCleanly())

		results := podmanTest.InspectNetwork(net)
		Expect(results).To(HaveLen(1))
		result := results[0]

		Expect(result.Options).To(HaveKeyWithValue("mtu", "1500"))
		Expect(result).To(HaveField("Driver", "macvlan"))
		Expect(result).To(HaveField("NetworkInterface", "lo"))
		Expect(result.IPAMOptions).To(HaveKeyWithValue("driver", "host-local"))

		Expect(result.Subnets).To(HaveLen(1))
		Expect(result.Subnets[0].Subnet.String()).To(Equal("192.168.1.0/24"))
		Expect(result.Subnets[0].Gateway.String()).To(Equal("192.168.1.254"))

		nc = podmanTest.Podman([]string{"network", "rm", net})
		nc.WaitWithDefaultTimeout()
		Expect(nc).Should(ExitCleanly())
	})

	It("podman network prune --filter", func() {
		useCustomNetworkDir(podmanTest, tempdir)
		net1 := "macvlan" + stringid.GenerateRandomID() + "net1"

		nc := podmanTest.Podman([]string{"network", "create", net1})
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(net1)
		Expect(nc).Should(ExitCleanly())

		list := podmanTest.Podman([]string{"network", "ls", "--format", "{{.Name}}"})
		list.WaitWithDefaultTimeout()
		Expect(list).Should(ExitCleanly())
		Expect(list.OutputToStringArray()).Should(HaveLen(2))

		Expect(list.OutputToStringArray()).Should(ContainElement(net1))
		Expect(list.OutputToStringArray()).Should(ContainElement("podman"))

		// -f needed only to skip y/n question
		prune := podmanTest.Podman([]string{"network", "prune", "-f", "--filter", "until=50"})
		prune.WaitWithDefaultTimeout()
		Expect(prune).Should(ExitCleanly())

		listAgain := podmanTest.Podman([]string{"network", "ls", "--format", "{{.Name}}"})
		listAgain.WaitWithDefaultTimeout()
		Expect(listAgain).Should(ExitCleanly())
		Expect(listAgain.OutputToStringArray()).Should(HaveLen(2))

		Expect(listAgain.OutputToStringArray()).Should(ContainElement(net1))
		Expect(listAgain.OutputToStringArray()).Should(ContainElement("podman"))

		// -f needed only to skip y/n question
		prune = podmanTest.Podman([]string{"network", "prune", "-f", "--filter", "until=5000000000000"})
		prune.WaitWithDefaultTimeout()
		Expect(prune).Should(ExitCleanly())

		listAgain = podmanTest.Podman([]string{"network", "ls", "--format", "{{.Name}}"})
		listAgain.WaitWithDefaultTimeout()
		Expect(listAgain).Should(ExitCleanly())
		Expect(listAgain.OutputToStringArray()).Should(HaveLen(1))

		Expect(listAgain.OutputToStringArray()).ShouldNot(ContainElement(net1))
		Expect(listAgain.OutputToStringArray()).Should(ContainElement("podman"))
	})

	It("podman network prune", Serial, func() {
		useCustomNetworkDir(podmanTest, tempdir)
		// Create two networks
		// Check they are there
		// Run a container on one of them
		// Network Prune
		// Check that one has been pruned, other remains
		net := "macvlan" + stringid.GenerateRandomID()
		net1 := net + "1"
		net2 := net + "2"
		nc := podmanTest.Podman([]string{"network", "create", net1})
		nc.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(net1)
		Expect(nc).Should(ExitCleanly())

		nc2 := podmanTest.Podman([]string{"network", "create", net2})
		nc2.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(net2)
		Expect(nc2).Should(ExitCleanly())

		list := podmanTest.Podman([]string{"network", "ls", "--format", "{{.Name}}"})
		list.WaitWithDefaultTimeout()
		Expect(list.OutputToStringArray()).Should(HaveLen(3))

		Expect(list.OutputToStringArray()).Should(ContainElement(net1))
		Expect(list.OutputToStringArray()).Should(ContainElement(net2))
		Expect(list.OutputToStringArray()).Should(ContainElement("podman"))

		session := podmanTest.Podman([]string{"run", "-dt", "--net", net2, ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		prune := podmanTest.Podman([]string{"network", "prune", "-f"})
		prune.WaitWithDefaultTimeout()
		Expect(prune).Should(ExitCleanly())

		listAgain := podmanTest.Podman([]string{"network", "ls", "--format", "{{.Name}}"})
		listAgain.WaitWithDefaultTimeout()
		Expect(listAgain).Should(ExitCleanly())
		Expect(listAgain.OutputToStringArray()).Should(HaveLen(2))

		Expect(listAgain.OutputToStringArray()).ShouldNot(ContainElement(net1))
		Expect(listAgain.OutputToStringArray()).Should(ContainElement(net2))
		Expect(listAgain.OutputToStringArray()).Should(ContainElement("podman"))
	})
})
