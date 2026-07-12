//go:build linux || freebsd

package integration

import (
	"fmt"
	"sort"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
	"go.podman.io/storage/pkg/stringid"
)

var _ = Describe("Podman ps", func() {
	It("podman pod ps no pods", func() {
		session := podmanTest.Podman([]string{"pod", "ps"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman pod ps default", func() {
		_, ec, podid := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		result := podmanTest.Podman([]string{"pod", "ps"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToStringArray()).ShouldNot(BeEmpty())
	})

	It("podman pod ps quiet flag", func() {
		_, ec, podid := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		_, ec, _ = podmanTest.RunLsContainerInPod("", podid)
		Expect(ec).To(Equal(0))

		result := podmanTest.Podman([]string{"pod", "ps", "-q"})
		result.WaitWithDefaultTimeout()
		Expect(result).To(ExitCleanly())
		Expect(result.OutputToStringArray()).ShouldNot(BeEmpty())
		Expect(podid).To(ContainSubstring(result.OutputToStringArray()[0]))
	})

	It("podman pod ps no-trunc", func() {
		_, ec, podid := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		_, ec2, _ := podmanTest.RunLsContainerInPod("", podid)
		Expect(ec2).To(Equal(0))

		result := podmanTest.Podman([]string{"pod", "ps", "-q", "--no-trunc"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToStringArray()).ShouldNot(BeEmpty())
		Expect(podid).To(Equal(result.OutputToStringArray()[0]))
	})

	It("podman pod ps latest", func() {
		SkipIfRemote("--latest flag n/a")
		_, ec, podid1 := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		_, ec2, podid2 := podmanTest.CreatePod(nil)
		Expect(ec2).To(Equal(0))

		result := podmanTest.Podman([]string{"pod", "ps", "-q", "--no-trunc", "--latest"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToString()).To(ContainSubstring(podid2))
		Expect(result.OutputToString()).To(Not(ContainSubstring(podid1)))
	})

	It("podman pod ps id filter flag", func() {
		_, ec, podid := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		result := podmanTest.Podman([]string{"pod", "ps", "--filter", fmt.Sprintf("id=%s", podid)})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
	})

	It("podman pod ps --filter until", func() {
		name := "mypod"
		_, ec, _ := podmanTest.CreatePod(map[string][]string{"--name": {name}, "--label": {"test=with,comma"}})
		Expect(ec).To(Equal(0))

		result := podmanTest.Podman([]string{"pod", "ps", "--filter", "until=50"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToString()).To(Not(ContainSubstring(name)))

		result = podmanTest.Podman([]string{"pod", "ps", "--filter", "until=5000000000"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToString()).To(ContainSubstring(name))

		result = podmanTest.Podman([]string{"pod", "ps", "--filter", "label=test=with,comma"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToString()).To(ContainSubstring(name))
	})

	It("podman pod ps filter name regexp", func() {
		_, ec, podid := podmanTest.CreatePod(map[string][]string{"--name": {"mypod"}})
		Expect(ec).To(Equal(0))
		_, ec2, _ := podmanTest.CreatePod(map[string][]string{"--name": {"mypod1"}})
		Expect(ec2).To(Equal(0))

		result := podmanTest.Podman([]string{"pod", "ps", "-q", "--no-trunc", "--filter", "name=mypod"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())

		output := result.OutputToStringArray()
		Expect(output).To(HaveLen(2))

		result = podmanTest.Podman([]string{"pod", "ps", "-q", "--no-trunc", "--filter", "name=mypod$"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())

		output = result.OutputToStringArray()
		Expect(output).To(HaveLen(1))
		Expect(output[0]).To(Equal(podid))
	})

	It("podman pod ps mutually exclusive flags", func() {
		session := podmanTest.Podman([]string{"pod", "ps", "-q", "--format", "{{.ID}}"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, "quiet and format cannot be used together"))
	})

	It("podman pod ps --sort by name", func() {
		_, ec, _ := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		_, ec2, _ := podmanTest.CreatePod(nil)
		Expect(ec2).To(Equal(0))

		_, ec3, _ := podmanTest.CreatePod(nil)
		Expect(ec3).To(Equal(0))

		session := podmanTest.Podman([]string{"pod", "ps", "--sort=name", "--format", "{{.Name}}"})

		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		sortedArr := session.OutputToStringArray()

		Expect(sort.SliceIsSorted(sortedArr, func(i, j int) bool { return sortedArr[i] < sortedArr[j] })).To(BeTrue(), "slice is sorted")
	})

	It("podman pod ps --ctr-names", func() {
		_, ec, podid := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("test1", podid)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		_, ec, _ = podmanTest.RunLsContainerInPod("test2", podid)
		Expect(ec).To(Equal(0))

		session = podmanTest.Podman([]string{"pod", "ps", "--format={{.ContainerNames}}", "--ctr-names"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("test1"))
		Expect(session.OutputToString()).To(ContainSubstring("test2"))
	})

	It("podman pod ps filter ctr attributes", func() {
		_, ec, podid1 := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("test1", podid1)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		_, ec2, podid2 := podmanTest.CreatePod(nil)
		Expect(ec2).To(Equal(0))

		_, ec3, cid := podmanTest.RunLsContainerInPod("test2", podid2)
		Expect(ec3).To(Equal(0))

		session = podmanTest.Podman([]string{"pod", "ps", "-q", "--no-trunc", "--filter", "ctr-names=test1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(podid1))
		Expect(session.OutputToString()).To(Not(ContainSubstring(podid2)))

		session = podmanTest.Podman([]string{"pod", "ps", "-q", "--no-trunc", "--filter", "ctr-names=test", "--filter", "ctr-status=running"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(podid1))
		Expect(session.OutputToString()).To(Not(ContainSubstring(podid2)))

		session = podmanTest.Podman([]string{"pod", "ps", "-q", "--no-trunc", "--filter", fmt.Sprintf("ctr-ids=%s", cid)})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(podid2))
		Expect(session.OutputToString()).To(Not(ContainSubstring(podid1)))

		session = podmanTest.Podman([]string{"pod", "ps", "-q", "--no-trunc", "--filter", "ctr-ids=" + cid[:40], "--filter", "ctr-ids=" + cid + "$"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(podid2))
		Expect(session.OutputToString()).To(Not(ContainSubstring(podid1)))

		_, ec3, podid3 := podmanTest.CreatePod(nil)
		Expect(ec3).To(Equal(0))

		session = podmanTest.Podman([]string{"pod", "ps", "-q", "--no-trunc", "--filter", "ctr-number=1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(podid1))
		Expect(session.OutputToString()).To(ContainSubstring(podid2))
		Expect(session.OutputToString()).To(Not(ContainSubstring(podid3)))

		session = podmanTest.Podman([]string{"pod", "ps", "-q", "--no-trunc", "--filter", "ctr-number=1", "--filter", "ctr-number=0"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(podid1))
		Expect(session.OutputToString()).To(ContainSubstring(podid2))
		Expect(session.OutputToString()).To(ContainSubstring(podid3))

		session = podmanTest.Podman([]string{"pod", "ps", "-q", "--no-trunc", "--filter", "ctr-status=running"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(podid1))
		Expect(session.OutputToString()).To(Not(ContainSubstring(podid2)))
		Expect(session.OutputToString()).To(Not(ContainSubstring(podid3)))

		session = podmanTest.Podman([]string{"pod", "ps", "-q", "--no-trunc", "--filter", "ctr-status=exited"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(podid2))
		Expect(session.OutputToString()).To(Not(ContainSubstring(podid1)))
		Expect(session.OutputToString()).To(Not(ContainSubstring(podid3)))

		session = podmanTest.Podman([]string{"pod", "ps", "-q", "--no-trunc", "--filter", "ctr-status=exited", "--filter", "ctr-status=running"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(podid1))
		Expect(session.OutputToString()).To(ContainSubstring(podid2))
		Expect(session.OutputToString()).To(Not(ContainSubstring(podid3)))

		session = podmanTest.Podman([]string{"pod", "ps", "-q", "--no-trunc", "--filter", "ctr-status=created"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(BeEmpty())
	})

	It("podman pod ps filter labels", func() {
		s, _, podid1 := podmanTest.CreatePod(nil)
		Expect(s).To(ExitCleanly())

		s, _, podid2 := podmanTest.CreatePod(map[string][]string{
			"--label": {"app=myapp", "io.podman.test.key=irrelevant-value"},
		})
		Expect(s).To(ExitCleanly())

		s, _, podid3 := podmanTest.CreatePod(map[string][]string{"--label": {"app=test"}})
		Expect(s).To(ExitCleanly())

		session := podmanTest.Podman([]string{"pod", "ps", "--no-trunc", "--filter", "label=app", "--filter", "label=app=myapp"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitCleanly())
		Expect(session.OutputToString()).To(Not(ContainSubstring(podid1)))
		Expect(session.OutputToString()).To(ContainSubstring(podid2))
		Expect(session.OutputToString()).To(Not(ContainSubstring(podid3)))
	})

	It("podman pod ps filter network", func() {
		net := stringid.GenerateRandomID()
		session := podmanTest.Podman([]string{"network", "create", net})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		defer podmanTest.removeNetwork(net)

		session = podmanTest.Podman([]string{"pod", "create", "--network", net})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		podWithNet := session.OutputToString()

		session = podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		podWithoutNet := session.OutputToString()

		session = podmanTest.Podman([]string{"pod", "ps", "--no-trunc", "--filter", "network=" + net})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(podWithNet))
		Expect(session.OutputToString()).To(Not(ContainSubstring(podWithoutNet)))
	})

	It("podman pod ps --format networks", func() {
		session := podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"pod", "ps", "--format", "{{ .Networks }}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		if isRootless() {
			// rootless container don't have a network by default
			Expect(session.OutputToString()).To(Equal(""))
		} else {
			// default network name is podman
			Expect(session.OutputToString()).To(Equal("podman"))
		}

		net1 := stringid.GenerateRandomID()
		session = podmanTest.Podman([]string{"network", "create", net1})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		defer podmanTest.removeNetwork(net1)
		net2 := stringid.GenerateRandomID()
		session = podmanTest.Podman([]string{"network", "create", net2})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		defer podmanTest.removeNetwork(net2)

		session = podmanTest.Podman([]string{"pod", "create", "--network", net1 + "," + net2})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		pid := session.OutputToString()

		session = podmanTest.Podman([]string{"pod", "ps", "--format", "{{ .Networks }}", "--filter", "id=" + pid})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		// the output is not deterministic so check both possible orders
		Expect(session.OutputToString()).To(Or(Equal(net1+","+net2), Equal(net2+","+net1)))
	})

	It("pod no infra should ps", func() {
		session := podmanTest.Podman([]string{"pod", "create", "--infra=false"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		ps := podmanTest.Podman([]string{"pod", "ps"})
		ps.WaitWithDefaultTimeout()
		Expect(ps).Should(ExitCleanly())

		infra := podmanTest.Podman([]string{"pod", "ps", "--format", "{{.InfraId}}"})
		infra.WaitWithDefaultTimeout()
		Expect(infra.OutputToString()).To(BeEmpty())
	})

	It("podman pod ps format with labels", func() {
		_, ec, _ := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		_, ec1, _ := podmanTest.CreatePod(map[string][]string{"--label": {
			"io.podman.test.label=value1",
			"io.podman.test.key=irrelevant-value",
		}})
		Expect(ec1).To(Equal(0))

		session := podmanTest.Podman([]string{"pod", "ps", "--format", "{{.Labels}}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("value1"))
	})

	It("podman pod ps headers", func() {
		session := podmanTest.Podman([]string{"pod", "ps", "--ctr-ids", "--ctr-names", "--ctr-status", "--ns"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(MatchRegexp(`^POD ID\s+NAME\s+STATUS\s+CREATED\s+INFRA ID\s+IDS\s+NAMES\s+STATUS\s+CGROUP\s+NAMESPACES$`))
	})
})
