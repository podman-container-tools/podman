//go:build linux || freebsd

package integration

import (
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.podman.io/podman/v6/cmd/podman/system"
	. "go.podman.io/podman/v6/test/utils"
	"go.podman.io/storage/pkg/stringid"
)

var _ = Describe("Podman events", func() {
	// For most, all, of these tests we do not "live" test following a log because it may make a fragile test
	// system more complex.  Instead we run the "events" and then verify that the events are processed correctly.
	// Perhaps a future version of this test would put events in a go func and send output back over a channel
	// while events occur.

	It("podman events", func() {
		_, ec, _ := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))
		result := podmanTest.Podman([]string{"events", "--stream=false"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
	})

	It("podman events with an event filter", func() {
		_, ec, _ := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))
		result := podmanTest.Podman([]string{"events", "--stream=false", "--filter", "event=start"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToStringArray()).ToNot(BeEmpty(), "Number of events")
		date := time.Now().Format("2006-01-02")
		Expect(result.OutputToStringArray()).To(ContainElement(HavePrefix(date)), "event log has correct timestamp")
	})
	It("podman events with a volume filter", func() {
		_, ec, vname := podmanTest.CreateVolume(nil)
		Expect(ec).To(Equal(0))

		// Run two event commands - one with the full volume name and the second with the prefix
		result := podmanTest.Podman([]string{"events", "--stream=false", "--filter", fmt.Sprintf("volume=%s", vname)})
		resultPrefix := podmanTest.Podman([]string{"events", "--stream=false", "--filter", fmt.Sprintf("volume=%s", vname[:5])})

		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		events := result.OutputToStringArray()
		Expect(events).To(HaveLen(1), "number of events")
		Expect(events[0]).To(ContainSubstring(vname), "event log includes volume name")

		resultPrefix.WaitWithDefaultTimeout()
		Expect(resultPrefix).Should(ExitCleanly())
		events = resultPrefix.OutputToStringArray()
		Expect(events).To(HaveLen(1), "number of events")
		Expect(events[0]).To(ContainSubstring(vname), "event log includes volume name")
	})

	It("podman events with an event filter and container=cid", func() {
		_, ec, cid := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))
		_, ec2, cid2 := podmanTest.RunLsContainer("")
		Expect(ec2).To(Equal(0))
		time.Sleep(5 * time.Second)
		result := podmanTest.Podman([]string{"events", "--stream=false", "--filter", "event=start", "--filter", fmt.Sprintf("container=%s", cid)})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		events := result.OutputToStringArray()
		Expect(events).To(HaveLen(1), "number of events")
		Expect(events[0]).To(ContainSubstring(cid), "event log includes CID")
		Expect(events[0]).To(Not(ContainSubstring(cid2)), "event log does not include second CID")
	})

	It("podman events with a type and filter container=id", func() {
		_, ec, cid := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))
		result := podmanTest.Podman([]string{"events", "--stream=false", "--filter", "type=pod", "--filter", fmt.Sprintf("container=%s", cid)})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToStringArray()).To(BeEmpty())
	})

	It("podman events with a type", func() {
		setup := podmanTest.Podman([]string{"run", "-dt", "--pod", "new:foobarpod", ALPINE, "top"})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(ExitCleanly())

		podmanTest.StopPod("foobarpod")

		result := podmanTest.Podman([]string{"events", "--stream=false", "--filter", "type=pod", "--filter", "pod=foobarpod"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		events := result.OutputToStringArray()
		GinkgoWriter.Println(events)
		Expect(len(events)).To(BeNumerically(">=", 2), "Number of events")
		Expect(events).To(ContainElement(ContainSubstring(" pod create ")))
		Expect(events).To(ContainElement(ContainSubstring(" pod stop ")))
		Expect(events).To(ContainElement(ContainSubstring("name=foobarpod")))
	})

	It("podman events --since", func() {
		_, ec, _ := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))
		result := podmanTest.Podman([]string{"events", "--stream=false", "--since", "1m"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
	})

	It("podman events --until", func() {
		_, ec, _ := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))
		result := podmanTest.Podman([]string{"events", "--stream=false", "--until", "1h"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
	})

	It("podman events format", func() {
		start := time.Now()
		ctrName := "testCtr"
		_, ec, _ := podmanTest.RunLsContainer(ctrName)
		end := time.Now()
		Expect(ec).To(Equal(0))

		test := podmanTest.Podman([]string{"events", "--stream=false", "--format", "json"})
		test.WaitWithDefaultTimeout()
		Expect(test).To(ExitCleanly())

		jsonArr := test.OutputToStringArray()
		Expect(test.OutputToStringArray()).ShouldNot(BeEmpty())

		event := system.Event{}
		err := json.Unmarshal([]byte(jsonArr[0]), &event)
		Expect(err).ToNot(HaveOccurred())

		test = podmanTest.Podman([]string{
			"events",
			"--stream=false",
			"--since", strconv.FormatInt(start.Unix(), 10),
			"--filter", fmt.Sprintf("container=%s", ctrName),
			"--format", "{{json .}}",
		})

		test.WaitWithDefaultTimeout()
		Expect(test).To(ExitCleanly())

		jsonArr = test.OutputToStringArray()
		Expect(test.OutputToStringArray()).ShouldNot(BeEmpty())

		event = system.Event{}
		err = json.Unmarshal([]byte(jsonArr[0]), &event)
		Expect(err).ToNot(HaveOccurred())

		Expect(event.Time).To(BeNumerically(">=", start.Unix()))
		Expect(event.Time).To(BeNumerically("<=", end.Unix()))
		Expect(event.TimeNano).To(BeNumerically(">=", start.UnixNano()))
		Expect(event.TimeNano).To(BeNumerically("<=", end.UnixNano()))
		Expect(time.Unix(0, event.TimeNano).Unix()).To(BeEquivalentTo(event.Time))

		test = podmanTest.Podman([]string{"events", "--stream=false", "--filter=type=container", "--format", "ID: {{.ID}}"})
		test.WaitWithDefaultTimeout()
		Expect(test).To(ExitCleanly())
		arr := test.OutputToStringArray()
		Expect(len(arr)).To(BeNumerically(">", 1))
		Expect(arr[0]).To(MatchRegexp("ID: [a-fA-F0-9]{64}"))
	})

	It("podman events --until future", func() {
		name1 := stringid.GenerateRandomID()
		name2 := stringid.GenerateRandomID()
		name3 := stringid.GenerateRandomID()
		session := podmanTest.Podman([]string{"create", "--name", name1, ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer GinkgoRecover()
			defer wg.Done()

			// wait 2 seconds to be sure events is running
			time.Sleep(time.Second * 2)
			session = podmanTest.Podman([]string{"create", "--name", name2, ALPINE})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			session = podmanTest.Podman([]string{"create", "--name", name3, ALPINE})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
		}()

		// unix timestamp in 10 seconds
		until := time.Now().Add(time.Second * 10).Unix()
		result := podmanTest.Podman([]string{"events", "--since", "30s", "--until", strconv.FormatInt(until, 10)})
		result.Wait(11)
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToString()).To(ContainSubstring(name1))
		Expect(result.OutputToString()).To(ContainSubstring(name2))
		Expect(result.OutputToString()).To(ContainSubstring(name3))

		// string duration in 10 seconds
		untilT := time.Now().Add(time.Second * 9)
		result = podmanTest.Podman([]string{"events", "--since", "30s", "--until", "10s"})
		result.Wait(11)
		Expect(result).Should(ExitCleanly())
		tEnd := time.Now()
		outDur := tEnd.Sub(untilT)
		Expect(outDur.Seconds()).To(BeNumerically(">", 0), "duration")
		Expect(result.OutputToString()).To(ContainSubstring(name1))
		Expect(result.OutputToString()).To(ContainSubstring(name2))
		Expect(result.OutputToString()).To(ContainSubstring(name3))

		wg.Wait()
	})

	It("podman events pod creation", func() {
		create := podmanTest.Podman([]string{"pod", "create", "--infra=false", "--name", "foobarpod"})
		create.WaitWithDefaultTimeout()
		Expect(create).Should(ExitCleanly())
		id := create.OutputToString()
		result := podmanTest.Podman([]string{"events", "--stream=false", "--filter", "pod=" + id})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToStringArray()).To(HaveLen(1))
		Expect(result.OutputToString()).To(ContainSubstring("create"))

		ctrName := "testCtr"
		run := podmanTest.Podman([]string{"create", "--pod", id, "--name", ctrName, ALPINE, "top"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(ExitCleanly())

		result2 := podmanTest.Podman([]string{"events", "--stream=false", "--filter", fmt.Sprintf("container=%s", ctrName), "--since", "30s"})
		result2.WaitWithDefaultTimeout()
		Expect(result2).Should(ExitCleanly())
		Expect(result2.OutputToString()).To(ContainSubstring(fmt.Sprintf("pod_id=%s", id)))
	})

	It("podman events network connection", func() {
		network := stringid.GenerateRandomID()
		networkDriver := "bridge"
		result := podmanTest.Podman([]string{"create", "--network", networkDriver, ALPINE, "top"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		ctrID := result.OutputToString()

		result = podmanTest.Podman([]string{"network", "create", network})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())

		result = podmanTest.Podman([]string{"network", "connect", network, ctrID})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())

		result = podmanTest.Podman([]string{"network", "disconnect", network, ctrID})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())

		result = podmanTest.Podman([]string{"network", "rm", network})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())

		result = podmanTest.Podman([]string{"events", "--stream=false", "--since", "30s"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())

		eventDetails := fmt.Sprintf(" %s (container=%s, name=%s)", ctrID, ctrID, network)
		networkCreateRemoveDetails := fmt.Sprintf("(name=%s, type=%s)", network, networkDriver)
		// Workaround for #23634, event order not guaranteed when remote.
		// Although the issue is closed, the bug is a real one. It seems
		// unlikely ever to be fixed.
		if IsRemote() {
			lines := result.OutputToString()
			Expect(lines).To(ContainSubstring("network connect" + eventDetails))
			Expect(lines).To(ContainSubstring("network disconnect" + eventDetails))
			Expect(lines).To(MatchRegexp(" network connect .* network disconnect "))
		} else {
			lines := result.OutputToStringArray()
			Expect(lines).To(HaveLen(7))
			Expect(lines[3]).To(And(ContainSubstring("network create"), ContainSubstring(networkCreateRemoveDetails)))
			Expect(lines[4]).To(ContainSubstring("network connect" + eventDetails))
			Expect(lines[5]).To(ContainSubstring("network disconnect" + eventDetails))
			Expect(lines[6]).To(And(ContainSubstring("network remove"), ContainSubstring(networkCreateRemoveDetails)))
		}
	})

	It("podman events health_status generated", func() {
		session := podmanTest.Podman([]string{"run", "--name", "test-hc", "-dt", "--health-cmd", "echo working", "busybox"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		for i := range 5 {
			hc := podmanTest.Podman([]string{"healthcheck", "run", "test-hc"})
			hc.WaitWithDefaultTimeout()
			exitCode := hc.ExitCode()
			if exitCode == 0 || i == 4 {
				break
			}
			time.Sleep(1 * time.Second)
		}

		result := podmanTest.Podman([]string{"events", "--stream=false", "--filter", "event=health_status", "--since", "1m"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToStringArray()).ToNot(BeEmpty(), "Number of health_status events")
	})

	It("podman events for artifacts", func() {
		artifactFile, err := createArtifactFile(1024)
		Expect(err).ToNot(HaveOccurred())

		lock, port, err := setupRegistry(nil)
		if err == nil {
			defer lock.Unlock()
		}
		Expect(err).ToNot(HaveOccurred())

		artifactName := fmt.Sprintf("localhost:%s/test/events-artifact-remote:latest", port)
		add := podmanTest.Podman([]string{"artifact", "add", artifactName, artifactFile})
		add.WaitWithDefaultTimeout()
		Expect(add).Should(ExitCleanly())

		push := podmanTest.Podman([]string{"artifact", "push", "-q", "--tls-verify=false", artifactName})
		push.WaitWithDefaultTimeout()
		Expect(push).Should(ExitCleanly())

		rm := podmanTest.Podman([]string{"artifact", "rm", artifactName})
		rm.WaitWithDefaultTimeout()
		Expect(rm).Should(ExitCleanly())

		pull := podmanTest.Podman([]string{"artifact", "pull", "-q", "--tls-verify=false", artifactName})
		pull.WaitWithDefaultTimeout()
		Expect(pull).Should(ExitCleanly())

		var events []string
		Eventually(func() int {
			result := podmanTest.Podman([]string{"events", "--stream=false", "--filter", "type=artifact", "--filter", "artifact=" + artifactName})
			result.WaitWithDefaultTimeout()
			Expect(result).Should(ExitCleanly())
			events = result.OutputToStringArray()
			return len(events)
		}, defaultWaitTimeout, 2).Should(BeNumerically("==", 4), "number of artifact events")

		Expect(events).To(HaveLen(4), "number of artifact events")
		Expect(events[0]).To(And(ContainSubstring("artifact create"), ContainSubstring(artifactName)), "event log includes artifact create")
		Expect(events[1]).To(And(ContainSubstring("artifact push"), ContainSubstring(artifactName)), "event log includes artifact push")
		Expect(events[2]).To(And(ContainSubstring("artifact remove"), ContainSubstring(artifactName)), "event log includes artifact remove")
		Expect(events[3]).To(And(ContainSubstring("artifact pull"), ContainSubstring(artifactName)), "event log includes artifact pull")
	})
})
