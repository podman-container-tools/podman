//go:build linux || freebsd

package integration

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	. "github.com/onsi/gomega/gexec"
	. "go.podman.io/podman/v6/test/utils"
	"go.podman.io/storage/pkg/stringid"
)

func isEventBackendJournald(podmanTest *PodmanTestIntegration) bool {
	if !podmanTest.RemoteTest {
		// If not remote test, '--events-backend' is set to 'file' or 'none'
		return false
	}
	info := podmanTest.Podman([]string{"info", "--format", "{{.Host.EventLogger}}"})
	info.WaitWithDefaultTimeout()
	return info.OutputToString() == "journald"
}

var _ = Describe("Podman logs", func() {
	It("podman logs on not existent container", func() {
		results := podmanTest.Podman([]string{"logs", "notexist"})
		results.WaitWithDefaultTimeout()
		Expect(results).To(ExitWithError(125, `no container with name or ID "notexist" found: no such container`))
	})

	for _, log := range []string{"k8s-file", "journald", "json-file"} {

		// Flake prevention: journalctl makes no timeliness guarantees
		logTimeout := time.Millisecond
		if log == "journald" {
			logTimeout = time.Second
		}

		skipIfJournaldInContainer := func() {
			if log == "journald" {
				SkipIfJournaldUnavailable()
			}
		}

		It("all lines: "+log, func() {
			skipIfJournaldInContainer()

			logc := podmanTest.Podman([]string{"run", "--log-driver", log, "-dt", ALPINE, "sh", "-c", "echo podman; echo podman; echo podman"})
			logc.WaitWithDefaultTimeout()
			Expect(logc).To(ExitCleanly())
			cid := logc.OutputToString()

			results := podmanTest.Podman([]string{"wait", cid})
			results.WaitWithDefaultTimeout()
			Expect(results).To(ExitCleanly())

			Eventually(func(g Gomega) {
				results = podmanTest.Podman([]string{"logs", cid})
				results.WaitWithDefaultTimeout()
				g.Expect(results).To(ExitCleanly())
				g.Expect(results.OutputToStringArray()).To(HaveLen(3))
				g.Expect(results.OutputToString()).To(Equal("podman podman podman"))
			}).WithTimeout(logTimeout).Should(Succeed())
		})

		It("tail two lines: "+log, func() {
			skipIfJournaldInContainer()

			logc := podmanTest.Podman([]string{"run", "--log-driver", log, "-dt", ALPINE, "sh", "-c", "echo podman; echo podman; echo podman"})
			logc.WaitWithDefaultTimeout()
			Expect(logc).To(ExitCleanly())
			cid := logc.OutputToString()

			wait := podmanTest.Podman([]string{"wait", cid})
			wait.WaitWithDefaultTimeout()
			Expect(wait).To(ExitCleanly())

			Eventually(func(g Gomega) {
				results := podmanTest.Podman([]string{"logs", "--tail", "2", cid})
				results.WaitWithDefaultTimeout()
				g.Expect(results).To(ExitCleanly())
				g.Expect(results.OutputToStringArray()).To(HaveLen(2))
				g.Expect(results.OutputToString()).To(Equal("podman podman"))
			}).WithTimeout(logTimeout).Should(Succeed())
		})

		It("tail zero lines: "+log, func() {
			skipIfJournaldInContainer()

			logc := podmanTest.Podman([]string{"run", "--log-driver", log, "-dt", ALPINE, "sh", "-c", "echo podman; echo podman; echo podman"})
			logc.WaitWithDefaultTimeout()
			Expect(logc).To(ExitCleanly())
			cid := logc.OutputToString()

			wait := podmanTest.Podman([]string{"wait", cid})
			wait.WaitWithDefaultTimeout()
			Expect(wait).To(ExitCleanly())

			time.Sleep(logTimeout)
			results := podmanTest.Podman([]string{"logs", "--tail", "0", cid})
			results.WaitWithDefaultTimeout()
			Expect(results).To(ExitCleanly())
			Expect(results.OutputToStringArray()).To(BeEmpty())
		})

		It("tail 99 lines: "+log, func() {
			skipIfJournaldInContainer()

			name := "test1"
			logc := podmanTest.Podman([]string{"run", "--name", name, "--log-driver", log, ALPINE, "sh", "-c", "echo podman; echo podman; echo podman"})
			logc.WaitWithDefaultTimeout()
			Expect(logc).To(ExitCleanly())

			wait := podmanTest.Podman([]string{"wait", name})
			wait.WaitWithDefaultTimeout()
			Expect(wait).To(ExitCleanly())

			Eventually(func(g Gomega) {
				// Options after name should work as well
				// https://github.com/containers/podman/issues/25653
				results := podmanTest.Podman([]string{"logs", name, "--tail", "99"})
				results.WaitWithDefaultTimeout()
				g.Expect(results).To(ExitCleanly())
				g.Expect(results.OutputToStringArray()).To(HaveLen(3))
			}).WithTimeout(logTimeout).Should(Succeed())
		})

		It("tail 800 lines: "+log, func() {
			skipIfJournaldInContainer()

			// we match 800 line array here, make sure to print all lines when assertion fails.
			// There is something weird going on (https://github.com/containers/podman/issues/18501)
			// and only the normal output log does not seem to be enough to figure out why it flakes.
			oldLength := format.MaxLength
			// unlimited matcher output
			format.MaxLength = 0
			defer func() {
				format.MaxLength = oldLength
			}()

			// this uses -d so that we do not have 1000 unnecessary lines printed in every test log
			logc := podmanTest.Podman([]string{"run", "--log-driver", log, "-d", ALPINE, "sh", "-c", "i=1; while [ \"$i\" -ne 1000 ]; do echo \"line $i\"; i=$((i + 1)); done"})
			logc.WaitWithDefaultTimeout()
			Expect(logc).To(ExitCleanly())
			cid := logc.OutputToString()

			// make sure we wait for the container to finish writing its output
			wait := podmanTest.Podman([]string{"wait", cid})
			wait.WaitWithDefaultTimeout()
			Expect(wait).To(ExitCleanly())

			Eventually(func(g Gomega) {
				results := podmanTest.Podman([]string{"logs", "--tail", "800", cid})
				results.WaitWithDefaultTimeout()
				g.Expect(results).To(ExitCleanly())
				g.Expect(results.OutputToStringArray()).To(HaveLen(800))
			}).WithTimeout(logTimeout).Should(Succeed())
		})

		It("tail 2 lines with timestamps: "+log, func() {
			skipIfJournaldInContainer()

			logc := podmanTest.Podman([]string{"run", "--log-driver", log, "-dt", ALPINE, "sh", "-c", "echo podman; echo podman; echo podman"})
			logc.WaitWithDefaultTimeout()
			Expect(logc).To(ExitCleanly())
			cid := logc.OutputToString()

			wait := podmanTest.Podman([]string{"wait", cid})
			wait.WaitWithDefaultTimeout()
			Expect(wait).To(ExitCleanly())

			Eventually(func(g Gomega) {
				results := podmanTest.Podman([]string{"logs", "--tail", "2", "-t", cid})
				results.WaitWithDefaultTimeout()
				g.Expect(results).To(ExitCleanly())
				g.Expect(results.OutputToStringArray()).To(HaveLen(2))
			}).WithTimeout(logTimeout).Should(Succeed())
		})

		It("timestamps with nanosecond precision: "+log, func() {
			skipIfJournaldInContainer()

			logc := podmanTest.Podman([]string{"run", "--log-driver", log, "-dt", ALPINE, "echo", "test message"})
			logc.WaitWithDefaultTimeout()
			Expect(logc).To(ExitCleanly())
			cid := logc.OutputToString()

			wait := podmanTest.Podman([]string{"wait", cid})
			wait.WaitWithDefaultTimeout()
			Expect(wait).To(ExitCleanly())

			Eventually(func(g Gomega) {
				results := podmanTest.Podman([]string{"logs", "-t", cid})
				results.WaitWithDefaultTimeout()
				g.Expect(results).To(ExitCleanly())
				// LogTimeFormat: 2006-01-02T15:04:05.000000000Z07:00
				// Verify timestamp contains nanoseconds (exactly 9 digits, zero-padded)
				// Timezone can be 'Z' (UTC) or '+/-HH:MM'
				g.Expect(results.OutputToString()).To(MatchRegexp(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{9}(Z|[+-]\d{2}:\d{2})\s+test message`))
			}).WithTimeout(logTimeout).Should(Succeed())
		})

		It("since time 2017-08-07: "+log, func() {
			skipIfJournaldInContainer()

			logc := podmanTest.Podman([]string{"run", "--log-driver", log, "-dt", ALPINE, "sh", "-c", "echo podman; echo podman; echo podman"})
			logc.WaitWithDefaultTimeout()
			Expect(logc).To(ExitCleanly())
			cid := logc.OutputToString()

			wait := podmanTest.Podman([]string{"wait", cid})
			wait.WaitWithDefaultTimeout()
			Expect(wait).To(ExitCleanly())

			Eventually(func(g Gomega) {
				results := podmanTest.Podman([]string{"logs", "--since", "2017-08-07T10:10:09.056611202-04:00", cid})
				results.WaitWithDefaultTimeout()
				g.Expect(results).To(ExitCleanly())
				g.Expect(results.OutputToStringArray()).To(HaveLen(3))
			}).WithTimeout(logTimeout).Should(Succeed())
		})

		It("since duration 10m: "+log, func() {
			skipIfJournaldInContainer()

			logc := podmanTest.Podman([]string{"run", "--log-driver", log, "-dt", ALPINE, "sh", "-c", "echo podman; echo podman; echo podman"})
			logc.WaitWithDefaultTimeout()
			Expect(logc).To(ExitCleanly())
			cid := logc.OutputToString()

			wait := podmanTest.Podman([]string{"wait", cid})
			wait.WaitWithDefaultTimeout()
			Expect(wait).To(ExitCleanly())

			Eventually(func(g Gomega) {
				results := podmanTest.Podman([]string{"logs", "--since", "10m", cid})
				results.WaitWithDefaultTimeout()
				g.Expect(results).To(ExitCleanly())
				g.Expect(results.OutputToStringArray()).To(HaveLen(3))
			}).WithTimeout(logTimeout).Should(Succeed())
		})

		It("until duration 10m: "+log, func() {
			skipIfJournaldInContainer()

			logc := podmanTest.Podman([]string{"run", "--log-driver", log, "-dt", ALPINE, "sh", "-c", "echo podman; echo podman; echo podman"})
			logc.WaitWithDefaultTimeout()
			Expect(logc).To(ExitCleanly())
			cid := logc.OutputToString()

			wait := podmanTest.Podman([]string{"wait", cid})
			wait.WaitWithDefaultTimeout()
			Expect(wait).To(ExitCleanly())

			Eventually(func(g Gomega) {
				results := podmanTest.Podman([]string{"logs", "--until", "10m", cid})
				results.WaitWithDefaultTimeout()
				g.Expect(results).To(ExitCleanly())
				g.Expect(results.OutputToStringArray()).To(HaveLen(3))
			}).WithTimeout(logTimeout).Should(Succeed())
		})

		It("until time NOW: "+log, func() {
			skipIfJournaldInContainer()

			logc := podmanTest.Podman([]string{"run", "--log-driver", log, "-dt", ALPINE, "sh", "-c", "echo podman; echo podman; echo podman"})
			logc.WaitWithDefaultTimeout()
			Expect(logc).To(ExitCleanly())
			cid := logc.OutputToString()

			wait := podmanTest.Podman([]string{"wait", cid})
			wait.WaitWithDefaultTimeout()
			Expect(wait).To(ExitCleanly())

			Eventually(func(g Gomega) {
				now := time.Now()
				now = now.Add(time.Minute * 1)
				nowS := now.Format(time.RFC3339)
				results := podmanTest.Podman([]string{"logs", "--until", nowS, cid})
				results.WaitWithDefaultTimeout()
				g.Expect(results).To(ExitCleanly())
				g.Expect(results.OutputToStringArray()).To(HaveLen(3))
			}).WithTimeout(logTimeout).Should(Succeed())
		})

		It("latest and container name should fail: "+log, func() {
			skipIfJournaldInContainer()

			results := podmanTest.Podman([]string{"logs", "-l", "foobar"})
			results.WaitWithDefaultTimeout()
			if IsRemote() {
				Expect(results).To(ExitWithError(125, "unknown shorthand flag: 'l' in -l"))
			} else {
				Expect(results).To(ExitWithError(125, "--latest and containers cannot be used together"))
			}
		})

		It("two containers showing short container IDs: "+log, func() {
			skipIfJournaldInContainer()
			SkipIfRemote("podman-remote logs does not support showing two containers at the same time")

			log1 := podmanTest.Podman([]string{"run", "--log-driver", log, "-dt", ALPINE, "sh", "-c", "echo podman; echo podman; echo podman"})
			log1.WaitWithDefaultTimeout()
			Expect(log1).Should(ExitCleanly())
			cid1 := log1.OutputToString()

			log2 := podmanTest.Podman([]string{"run", "--log-driver", log, "-dt", ALPINE, "sh", "-c", "echo podman; echo podman; echo podman"})
			log2.WaitWithDefaultTimeout()
			Expect(log2).Should(ExitCleanly())
			cid2 := log2.OutputToString()

			wait := podmanTest.Podman([]string{"wait", cid1, cid2})
			wait.WaitWithDefaultTimeout()
			Expect(wait).To(ExitCleanly())

			results := podmanTest.Podman([]string{"logs", cid1, cid2})
			results.WaitWithDefaultTimeout()
			Expect(results).Should(ExitCleanly())

			output := results.OutputToStringArray()
			Expect(output).To(HaveLen(6))
			Expect(output[0]).To(Or(ContainSubstring(cid1[:12]), ContainSubstring(cid2[:12])))
		})

		It("podman logs on a created container should result in 0 exit code: "+log, func() {
			skipIfJournaldInContainer()

			session := podmanTest.Podman([]string{"create", "--log-driver", log, "--name", "log", ALPINE})
			session.WaitWithDefaultTimeout()
			Expect(session).To(ExitCleanly())

			results := podmanTest.Podman([]string{"logs", "log"})
			results.WaitWithDefaultTimeout()
			Expect(results).To(ExitCleanly())
		})

		It("streaming output: "+log, func() {
			skipIfJournaldInContainer()

			containerName := "logs-f"

			logc := podmanTest.Podman([]string{"run", "--log-driver", log, "--name", containerName, "-dt", ALPINE, "sh", "-c", "echo podman-1; sleep 1; echo podman-2"})
			logc.WaitWithDefaultTimeout()
			Expect(logc).To(ExitCleanly())

			results := podmanTest.Podman([]string{"logs", "-f", containerName})
			results.WaitWithDefaultTimeout()

			if log == "journald" && !isEventBackendJournald(podmanTest) {
				// --follow + journald log-driver is only supported with journald events-backend(PR #10431)
				Expect(results).To(ExitWithError(125, "using --follow with the journald --log-driver but without the journald --events-backend"))
				return
			}

			Expect(results).To(ExitCleanly())

			Expect(results.OutputToString()).To(ContainSubstring("podman-1"))
			Expect(results.OutputToString()).To(ContainSubstring("podman-2"))

			// Container should now be terminatING or terminatED, but we
			// have no guarantee of which: 'logs -f' does not necessarily
			// wait for cleanup. Run 'inspect' and accept either state.
			inspect := podmanTest.Podman([]string{"container", "inspect", "--format", "{{.State.Status}}", containerName})
			inspect.WaitWithDefaultTimeout()
			if inspect.ExitCode() == 0 {
				Expect(inspect.OutputToString()).To(Equal("exited"))
				// TODO: add 2-second wait loop to confirm cleanup
			} else {
				Expect(inspect.ErrorToString()).To(ContainSubstring("no such container"))
			}

			results = podmanTest.Podman([]string{"rm", "--time", "0", "-f", containerName})
			results.WaitWithDefaultTimeout()
			Expect(results).To(ExitCleanly())
		})

		It("follow output stopped container: "+log, func() {
			skipIfJournaldInContainer()

			containerName := "logs-f"

			logc := podmanTest.Podman([]string{"run", "--log-driver", log, "--name", containerName, ALPINE, "true"})
			logc.WaitWithDefaultTimeout()
			Expect(logc).To(ExitCleanly())

			results := podmanTest.Podman([]string{"logs", "-f", containerName})
			results.WaitWithDefaultTimeout()
			if log == "journald" && !isEventBackendJournald(podmanTest) {
				// --follow + journald log-driver is only supported with journald events-backend(PR #10431)
				Expect(results).To(ExitWithError(125, "using --follow with the journald --log-driver but without the journald --events-backend"))
				return
			}
			Expect(results).To(ExitCleanly())
		})

		It("using container with container log-size: "+log, func() {
			skipIfJournaldInContainer()

			logc := podmanTest.Podman([]string{"run", "--log-driver", log, "--log-opt=max-size=10k", "-d", ALPINE, "echo", "podman podman podman"})
			logc.WaitWithDefaultTimeout()
			Expect(logc).To(ExitCleanly())
			cid := logc.OutputToString()

			wait := podmanTest.Podman([]string{"wait", cid})
			wait.WaitWithDefaultTimeout()
			Expect(wait).To(ExitCleanly())

			inspect := podmanTest.Podman([]string{"container", "inspect", "--format", "{{.HostConfig.LogConfig.Size}}", cid})
			inspect.WaitWithDefaultTimeout()
			Expect(inspect).To(ExitCleanly())
			Expect(inspect.OutputToString()).To(Equal("10kB"))

			Eventually(func(g Gomega) {
				results := podmanTest.Podman([]string{"logs", cid})
				results.WaitWithDefaultTimeout()
				g.Expect(results).To(ExitCleanly())
				g.Expect(results.OutputToString()).To(Equal("podman podman podman"))
			}).WithTimeout(logTimeout).Should(Succeed())
		})

		It("Make sure logs match expected length: "+log, func() {
			skipIfJournaldInContainer()

			logc := podmanTest.Podman([]string{"run", "--log-driver", log, "--name", "test", ALPINE, "sh", "-c", "echo 1; echo 2"})
			logc.WaitWithDefaultTimeout()
			Expect(logc).To(ExitCleanly())

			wait := podmanTest.Podman([]string{"wait", "test"})
			wait.WaitWithDefaultTimeout()
			Expect(wait).To(ExitCleanly())

			Eventually(func(g Gomega) {
				results := podmanTest.Podman([]string{"logs", "test"})
				results.WaitWithDefaultTimeout()
				g.Expect(results).To(ExitCleanly())
				outlines := results.OutputToStringArray()
				g.Expect(outlines).To(HaveLen(2))
				g.Expect(outlines[0]).To(Equal("1"))
				g.Expect(outlines[1]).To(Equal("2"))
			}).WithTimeout(logTimeout).Should(Succeed())
		})

		It("podman logs test stdout and stderr: "+log, func() {
			skipIfJournaldInContainer()

			cname := "log-test"
			logc := podmanTest.Podman([]string{"run", "--log-driver", log, "--name", cname, ALPINE, "sh", "-c", "echo stdout; echo stderr >&2"})
			logc.WaitWithDefaultTimeout()
			Expect(logc).To(Exit(0))

			wait := podmanTest.Podman([]string{"wait", cname})
			wait.WaitWithDefaultTimeout()
			Expect(wait).To(ExitCleanly())

			Eventually(func(g Gomega) {
				results := podmanTest.Podman([]string{"logs", cname})
				results.WaitWithDefaultTimeout()
				g.Expect(results).To(Exit(0))
				g.Expect(results.OutputToString()).To(Equal("stdout"))
				g.Expect(results.ErrorToString()).To(Equal("stderr"))
			}).WithTimeout(logTimeout).Should(Succeed())
		})

		It("podman logs partial log lines: "+log, func() {
			skipIfJournaldInContainer()

			cname := "log-test"
			content := stringid.GenerateRandomID()
			// use printf to print no extra newline
			logc := podmanTest.Podman([]string{"run", "--log-driver", log, "--name", cname, ALPINE, "printf", content})
			logc.WaitWithDefaultTimeout()
			Expect(logc).To(ExitCleanly())
			// Important: do not use OutputToString(), this will remove the trailing newline from the output.
			// However this test must make sure that there is no such extra newline.
			Expect(string(logc.Out.Contents())).To(Equal(content))

			Eventually(func(g Gomega) {
				logs := podmanTest.Podman([]string{"logs", cname})
				logs.WaitWithDefaultTimeout()
				g.Expect(logs).To(ExitCleanly())
				// see comment above
				g.Expect(string(logs.Out.Contents())).To(Equal(content))
			}).WithTimeout(logTimeout).Should(Succeed())
		})

		It("podman pod logs -l with newer container created: "+log, func() {
			skipIfJournaldInContainer()
			SkipIfRemote("no -l in remote")

			podName := "testPod"
			containerName1 := "container1"
			containerName2 := "container2"
			containerName3 := "container3"

			testPod := podmanTest.Podman([]string{"pod", "create", fmt.Sprintf("--name=%s", podName)})
			testPod.WaitWithDefaultTimeout()
			Expect(testPod).To(ExitCleanly())

			log1 := podmanTest.Podman([]string{"run", "--log-driver", log, "--name", containerName1, "--pod", podName, BB, "echo", "log1"})
			log1.WaitWithDefaultTimeout()
			Expect(log1).To(ExitCleanly())

			log2 := podmanTest.Podman([]string{"run", "--log-driver", log, "--name", containerName2, "--pod", podName, BB, "echo", "log2"})
			log2.WaitWithDefaultTimeout()
			Expect(log2).To(ExitCleanly())

			ctr := podmanTest.Podman([]string{"run", "--log-driver", log, "--name", containerName3, BB, "date"})
			ctr.WaitWithDefaultTimeout()
			Expect(ctr).To(ExitCleanly())

			Eventually(func(g Gomega) {
				results := podmanTest.Podman([]string{"pod", "logs", "-l"})
				results.WaitWithDefaultTimeout()
				g.Expect(results).To(ExitCleanly())
				podOutput := results.OutputToString()

				results = podmanTest.Podman([]string{"logs", "-l"})
				results.WaitWithDefaultTimeout()
				g.Expect(results).To(ExitCleanly())
				ctrOutput := results.OutputToString()

				g.Expect(podOutput).ToNot(Equal(ctrOutput))
			}).WithTimeout(logTimeout).Should(Succeed())
		})

		It("podman pod logs -l: "+log, func() {
			skipIfJournaldInContainer()
			SkipIfRemote("no -l in remote")

			podName := "testPod"
			containerName1 := "container1"
			containerName2 := "container2"

			testPod := podmanTest.Podman([]string{"pod", "create", fmt.Sprintf("--name=%s", podName)})
			testPod.WaitWithDefaultTimeout()
			Expect(testPod).To(ExitCleanly())

			log1 := podmanTest.Podman([]string{"run", "--log-driver", log, "--name", containerName1, "--pod", podName, BB, "echo", "log1"})
			log1.WaitWithDefaultTimeout()
			Expect(log1).To(ExitCleanly())

			log2 := podmanTest.Podman([]string{"run", "--log-driver", log, "--name", containerName2, "--pod", podName, BB, "echo", "log2"})
			log2.WaitWithDefaultTimeout()
			Expect(log2).To(ExitCleanly())

			Eventually(func(g Gomega) {
				results := podmanTest.Podman([]string{"pod", "logs", "-l"})
				results.WaitWithDefaultTimeout()
				g.Expect(results).To(ExitCleanly())
				output := results.OutputToString()
				g.Expect(output).To(ContainSubstring("log1"))
				g.Expect(output).To(ContainSubstring("log2"))
			}).WithTimeout(logTimeout).Should(Succeed())
		})
	}

	It("using journald for container with container tag", func() {
		SkipIfJournaldUnavailable()
		logc := podmanTest.Podman([]string{"run", "--log-driver", "journald", "--log-opt=tag={{.ImageName}},withcomma", "-d", ALPINE, "sh", "-c", "echo podman; sleep 0.1; echo podman; sleep 0.1; echo podman"})
		logc.WaitWithDefaultTimeout()
		Expect(logc).To(ExitCleanly())
		cid := logc.OutputToString()

		wait := podmanTest.Podman([]string{"wait", cid})
		wait.WaitWithDefaultTimeout()
		Expect(wait).To(ExitCleanly())

		Eventually(func(g Gomega) {
			cmd := exec.Command("journalctl", "--no-pager", "-o", "json", "--output-fields=CONTAINER_TAG", fmt.Sprintf("CONTAINER_ID_FULL=%s", cid))
			out, err := cmd.CombinedOutput()
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(string(out)).To(ContainSubstring(ALPINE + ",withcomma"))
		}).Should(Succeed())
	})

	It("using journald container name", func() {
		SkipIfJournaldUnavailable()
		containerName := "inside-journal"
		logc := podmanTest.Podman([]string{"run", "--log-driver", "journald", "-d", "--name", containerName, ALPINE, "sh", "-c", "echo podman; sleep 0.1; echo podman; sleep 0.1; echo podman"})
		logc.WaitWithDefaultTimeout()
		Expect(logc).To(ExitCleanly())
		cid := logc.OutputToString()

		wait := podmanTest.Podman([]string{"wait", cid})
		wait.WaitWithDefaultTimeout()
		Expect(wait).To(ExitCleanly())

		Eventually(func(g Gomega) {
			cmd := exec.Command("journalctl", "--no-pager", "-o", "json", "--output-fields=CONTAINER_NAME", fmt.Sprintf("CONTAINER_ID_FULL=%s", cid))
			out, err := cmd.CombinedOutput()
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(string(out)).To(ContainSubstring(containerName))
		}).Should(Succeed())
	})

	It("using journald labels", func() {
		SkipIfJournaldUnavailable()
		SkipIfConmonVersionLessThan("2.2.0")
		containerName := "inside-journal"
		logc := podmanTest.Podman([]string{"run", "--log-driver", "journald", "--log-opt", "label=CONTAINER_LABEL=LabelValue", "-d", "--name", containerName, ALPINE, "sh", "-c", "echo podman; sleep 0.1; echo podman; sleep 0.1; echo podman"})
		logc.WaitWithDefaultTimeout()
		Expect(logc).To(ExitCleanly())
		cid := logc.OutputToString()

		wait := podmanTest.Podman([]string{"wait", cid})
		wait.WaitWithDefaultTimeout()
		Expect(wait).To(ExitCleanly())

		Eventually(func(g Gomega) {
			cmd := exec.Command("journalctl", "--no-pager", "-o", "json", "--output-fields=CONTAINER_LABEL", fmt.Sprintf("CONTAINER_ID_FULL=%s", cid))
			out, err := cmd.CombinedOutput()
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(string(out)).To(ContainSubstring("LabelValue"))
		}).Should(Succeed())
	})

	It("using promoted journald labels", func() {
  	      SkipIfJournaldUnavailable()
              SkipIfConmonVersionLessThan("2.2.0")

              containerName := "inside-journal-promoted"

              logc := podmanTest.Podman([]string{
                "run",
                "--log-driver", "journald",
                "--label", "team=payments",
                "--label", "env=prod",
                "--log-opt", "labels=team,env",
                "-d",
                "--name", containerName,
                ALPINE,
                "sh", "-c", "echo podman; sleep 0.1; echo podman; sleep 0.1; echo podman",
              })

              logc.WaitWithDefaultTimeout()
              Expect(logc).To(ExitCleanly())
              cid := logc.OutputToString()

              wait := podmanTest.Podman([]string{"wait", cid})
              wait.WaitWithDefaultTimeout()
              Expect(wait).To(ExitCleanly())

              Eventually(func(g Gomega) {
                cmd := exec.Command(
                        "journalctl",
                        "--no-pager",
                        "-o", "json",
                        "--output-fields=TEAM,ENV",
                        fmt.Sprintf("CONTAINER_ID_FULL=%s", cid),
                )

                out, err := cmd.CombinedOutput()
                g.Expect(err).ToNot(HaveOccurred())

                output := string(out)
                g.Expect(output).To(ContainSubstring("payments"))
                g.Expect(output).To(ContainSubstring("prod"))
        	}).Should(Succeed())
	})

	It("podman logs with log-driver=none errors", func() {
		ctrName := "logsctr"
		logc := podmanTest.Podman([]string{"run", "--name", ctrName, "-d", "--log-driver", "none", ALPINE, "top"})
		logc.WaitWithDefaultTimeout()
		Expect(logc).To(ExitCleanly())

		logs := podmanTest.Podman([]string{"logs", "-f", ctrName})
		logs.WaitWithDefaultTimeout()
		Expect(logs).To(ExitWithError(125, "this container is using the 'none' log driver, cannot read logs: this container is not logging output"))
	})

	It("podman logs with non ASCII log tag fails without correct LANG", func() {
		SkipIfJournaldUnavailable()
		// need to set the LANG to something that does not support german umlaute to trigger the failure case
		cleanup := setLangEnv("C")
		if IsRemote() {
			podmanTest.RestartRemoteService()
		}
		defer cleanup()
		logc := podmanTest.Podman([]string{"run", "--log-driver", "journald", "--log-opt", "tag=äöüß", ALPINE, "echo", "podman"})
		logc.WaitWithDefaultTimeout()
		Expect(logc).To(ExitWithError(126, "conmon failed: exit status 1"))
		if !IsRemote() {
			Expect(logc.ErrorToString()).To(ContainSubstring("conmon: option parsing failed: Invalid byte sequence in conversion input"))
		}
	})

	It("podman logs with non ASCII log tag succeeds with proper env", func() {
		SkipIfJournaldUnavailable()
		cleanup := setLangEnv("en_US.UTF-8")
		if IsRemote() {
			podmanTest.RestartRemoteService()
		}
		defer cleanup()
		logc := podmanTest.Podman([]string{"run", "--log-driver", "journald", "--log-opt", "tag=äöüß", ALPINE, "echo", "podman"})
		logc.WaitWithDefaultTimeout()
		Expect(logc).To(ExitCleanly())
		Expect(logc.OutputToString()).To(Equal("podman"))
	})

	It("log tag with non-journald driver fails", func() {
		logc := podmanTest.Podman([]string{"run", "--log-driver", "k8s-file", "--log-opt", "tag=mytag", ALPINE, "true"})
		logc.WaitWithDefaultTimeout()
		Expect(logc).To(ExitWithError(125, "log tags can only be used with the journald log driver"))
	})

	It("podman pod logs with container names", func() {
		SkipIfRemote("Remote can only process one container at a time")
		podName := "testPod"
		containerName1 := "container1"
		containerName2 := "container2"

		testPod := podmanTest.Podman([]string{"pod", "create", fmt.Sprintf("--name=%s", podName)})
		testPod.WaitWithDefaultTimeout()
		Expect(testPod).To(ExitCleanly())

		log1 := podmanTest.Podman([]string{"run", "--name", containerName1, "--pod", podName, BB, "echo", "log1"})
		log1.WaitWithDefaultTimeout()
		Expect(log1).To(ExitCleanly())

		log2 := podmanTest.Podman([]string{"run", "--name", containerName2, "--pod", podName, BB, "echo", "log2"})
		log2.WaitWithDefaultTimeout()
		Expect(log2).To(ExitCleanly())

		Eventually(func(g Gomega) {
			results := podmanTest.Podman([]string{"pod", "logs", "--names", podName})
			results.WaitWithDefaultTimeout()
			g.Expect(results).To(ExitCleanly())

			output := results.OutputToStringArray()
			g.Expect(output).To(HaveLen(2))
			g.Expect(output).To(ContainElement(ContainSubstring(containerName1)))
			g.Expect(output).To(ContainElement(ContainSubstring(containerName2)))
		}).Should(Succeed())
	})
	It("podman pod logs with different colors", func() {
		SkipIfRemote("Remote can only process one container at a time")
		podName := "testPod"
		containerName1 := "container1"
		containerName2 := "container2"
		testPod := podmanTest.Podman([]string{"pod", "create", fmt.Sprintf("--name=%s", podName)})
		testPod.WaitWithDefaultTimeout()
		Expect(testPod).To(ExitCleanly())
		log1 := podmanTest.Podman([]string{"run", "--name", containerName1, "--pod", podName, BB, "echo", "log1"})
		log1.WaitWithDefaultTimeout()
		Expect(log1).To(ExitCleanly())
		log2 := podmanTest.Podman([]string{"run", "--name", containerName2, "--pod", podName, BB, "echo", "log2"})
		log2.WaitWithDefaultTimeout()
		Expect(log2).To(ExitCleanly())

		Eventually(func(g Gomega) {
			results := podmanTest.Podman([]string{"pod", "logs", "--color", podName})
			results.WaitWithDefaultTimeout()
			g.Expect(results).To(ExitCleanly())
			output := results.OutputToStringArray()
			g.Expect(output).To(HaveLen(2))
			g.Expect(output[0]).To(MatchRegexp(`\x1b\[3[0-9a-z ]+\x1b\[0m`))
			g.Expect(output[1]).To(MatchRegexp(`\x1b\[3[0-9a-z ]+\x1b\[0m`))
		}).Should(Succeed())
	})
})

func setLangEnv(lang string) func() {
	oldLang, okLang := os.LookupEnv("LANG")
	oldLcAll, okLc := os.LookupEnv("LC_ALL")
	err := os.Setenv("LANG", lang)
	Expect(err).ToNot(HaveOccurred())
	err = os.Setenv("LC_ALL", "")
	Expect(err).ToNot(HaveOccurred())

	return func() {
		if okLang {
			os.Setenv("LANG", oldLang)
		} else {
			os.Unsetenv("LANG")
		}
		if okLc {
			os.Setenv("LC_ALL", oldLcAll)
		} else {
			os.Unsetenv("LC_ALL")
		}
	}
}
