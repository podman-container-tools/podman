//go:build linux || freebsd

package integration

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("Podman Info", func() {
	It("podman info --format json", func() {
		tests := []struct {
			input    string
			success  bool
			exitCode int
		}{
			{"json", true, 0},
			{" json", true, 0},
			{"json ", true, 0},
			{"  json   ", true, 0},
			{"{{json .}}", true, 0},
			{"{{ json .}}", true, 0},
			{"{{json .   }}", true, 0},
			{"  {{  json .    }}   ", true, 0},
			{"{{json }}", true, 0},
			{"{{json .", false, 125},
			{"json . }}", false, 0}, // without opening {{ template seen as string literal
		}
		for _, tt := range tests {
			session := podmanTest.Podman([]string{"info", "--format", tt.input})
			session.WaitWithDefaultTimeout()

			desc := fmt.Sprintf("JSON test(%q)", tt.input)
			Expect(session).Should(Exit(tt.exitCode), desc)
			Expect(session.IsJSONOutputValid()).To(Equal(tt.success), desc)
		}
	})

	It("podman info --format GO template", func() {
		podmanTest.PodmanExitCleanly("info", "--format", "{{.Store.GraphRoot}}")
	})

	It("podman info --format GO template", func() {
		session := podmanTest.PodmanExitCleanly("info", "--format", "{{.Registries}}")
		Expect(session.OutputToString()).To(ContainSubstring("registry"))
	})

	It("podman info --format GO template plugins", func() {
		session := podmanTest.PodmanExitCleanly("info", "--format", "{{.Plugins}}")
		Expect(session.OutputToString()).To(ContainSubstring("local"))
		Expect(session.OutputToString()).To(ContainSubstring("journald"))
		Expect(session.OutputToString()).To(ContainSubstring("bridge"))
	})

	It("podman info rootless storage path", func() {
		SkipIfNotRootless("test of rootless_storage_path is only meaningful as rootless")
		SkipIfRemote("Only tests storage on local client")
		configPath := filepath.Join(podmanTest.TempDir, ".config", "containers", "storage.conf")
		os.Setenv("CONTAINERS_STORAGE_CONF", configPath)
		defer func() {
			os.Unsetenv("CONTAINERS_STORAGE_CONF")
		}()
		err := os.RemoveAll(filepath.Dir(configPath))
		Expect(err).ToNot(HaveOccurred())

		err = os.MkdirAll(filepath.Dir(configPath), os.ModePerm)
		Expect(err).ToNot(HaveOccurred())

		rootlessStoragePath := `"/tmp/$HOME/$USER/$UID/storage"`
		driver := `"overlay"`
		storageConf := fmt.Appendf(nil, "[storage]\ndriver=%s\nrootless_storage_path=%s\n[storage.options]\n", driver, rootlessStoragePath)
		err = os.WriteFile(configPath, storageConf, os.ModePerm)
		Expect(err).ToNot(HaveOccurred())
		// Failures in this test are impossible to debug without breadcrumbs
		GinkgoWriter.Printf("CONTAINERS_STORAGE_CONF=%s:\n%s\n", configPath, storageConf)

		u, err := user.Current()
		Expect(err).ToNot(HaveOccurred())

		// Cannot use podmanTest.Podman() and test for storage path
		expect := filepath.Join("/tmp", os.Getenv("HOME"), u.Username, u.Uid, "storage")
		podmanPath := podmanTest.PodmanTest.PodmanBinary
		cmd := exec.Command(podmanPath, "info", "--format", "{{.Store.GraphRoot -}}")
		out, err := cmd.CombinedOutput()
		GinkgoWriter.Printf("Running: podman info --format {{.Store.GraphRoot -}}\nOutput: %s\n", string(out))
		Expect(err).ToNot(HaveOccurred(), "podman info")
		Expect(string(out)).To(Equal(expect), "output from podman info")
	})

	It("check RemoteSocket ", func() {
		session := podmanTest.PodmanExitCleanly("info", "--format", "{{.Host.RemoteSocket.Path}}")
		switch podmanTest.RemoteSocketScheme {
		case "unix":
			Expect(session.OutputToString()).To(MatchRegexp("/run/.*podman.*sock"))
		case "tcp":
			Expect(session.OutputToString()).To(MatchRegexp("tcp://127.0.0.1:.*"))
		}

		session = podmanTest.PodmanExitCleanly("info", "--format", "{{.Host.ServiceIsRemote}}")
		if podmanTest.RemoteTest {
			Expect(session.OutputToString()).To(Equal("true"))
		} else {
			Expect(session.OutputToString()).To(Equal("false"))
		}

		if IsRemote() {
			session = podmanTest.PodmanExitCleanly("info", "--format", "{{.Host.RemoteSocket.Exists}}")
			Expect(session.OutputToString()).To(Equal("true"))
		}
	})

	It("Podman info must contain cgroupControllers with RelevantControllers", func() {
		SkipIfRootless("Hard to tell which controllers are going to be enabled for rootless")
		session := podmanTest.PodmanExitCleanly("info", "--format", "{{.Host.CgroupControllers}}")
		Expect(session.OutputToString()).To(ContainSubstring("memory"))
		Expect(session.OutputToString()).To(ContainSubstring("pids"))
	})

	It("Podman info: check desired runtime", func() {
		// defined in .cirrus.yml
		want := os.Getenv("CI_DESIRED_RUNTIME")
		if want == "" {
			if os.Getenv("CIRRUS_CI") == "" {
				Skip("CI_DESIRED_RUNTIME is not set--this is OK because we're not running under Cirrus")
			}
			Fail("CIRRUS_CI is set, but CI_DESIRED_RUNTIME is not! See #14912")
		}
		session := podmanTest.PodmanExitCleanly("info", "--format", "{{.Host.OCIRuntime.Name}}")
		Expect(session.OutputToString()).To(Equal(want))
	})

	It("Podman info: check desired network backend", func() {
		session := podmanTest.PodmanExitCleanly("info", "--format", "{{.Host.NetworkBackend}}")
		Expect(session.OutputToString()).To(Equal("netavark"))

		session = podmanTest.PodmanExitCleanly("info", "--format", "{{.Host.NetworkBackendInfo.Backend}}")
		Expect(session.OutputToString()).To(Equal("netavark"))
	})

	It("Podman info: check default network from configuration", func() {
		configPath := filepath.Join(podmanTest.TempDir, "containers.conf")

		os.Setenv("CONTAINERS_CONF_OVERRIDE", configPath)

		customNetName := "my-custom-test-network"
		configContent := fmt.Sprintf("[network]\ndefault_network=%q\n", customNetName)

		err := os.WriteFile(configPath, []byte(configContent), os.ModePerm)
		Expect(err).ToNot(HaveOccurred())
		podmanTest.RestartRemoteService()

		session := podmanTest.PodmanExitCleanly("info", "--format", "{{.Host.NetworkBackendInfo.DefaultNetwork}}")
		Expect(session.OutputToString()).To(Equal(customNetName))
	})

	It("Podman info: check CDI spec dirs and devices from configuration", func() {
		cdiDir := filepath.Join(podmanTest.TempDir, "cdi")
		err := os.MkdirAll(cdiDir, os.ModePerm)
		Expect(err).ToNot(HaveOccurred())

		cdiSpec := []byte(`{
  "cdiVersion": "0.3.0",
  "kind": "vendor.com/device",
  "devices": [
    {
      "name": "myKmsg",
      "containerEdits": {
        "env": ["PODMAN_CDI_INFO_TEST=1"]
      }
    }
  ]
}`)
		err = os.WriteFile(filepath.Join(cdiDir, "device.json"), cdiSpec, os.ModePerm)
		Expect(err).ToNot(HaveOccurred())

		configPath := filepath.Join(podmanTest.TempDir, "containers.conf")
		configContent := fmt.Sprintf("[engine]\ncdi_spec_dirs = [%q]\n", cdiDir)
		err = os.WriteFile(configPath, []byte(configContent), os.ModePerm)
		Expect(err).ToNot(HaveOccurred())

		GinkgoT().Setenv("CONTAINERS_CONF_OVERRIDE", configPath)
		podmanTest.RestartRemoteService()

		session := podmanTest.PodmanExitCleanly("info", "--format", "{{.Host.CDISpecDirs}} {{.Host.DiscoveredDevices}}")

		Expect(session.OutputToString()).To(ContainSubstring(cdiDir))
		Expect(session.OutputToString()).To(ContainSubstring("vendor.com/device=myKmsg"))
	})

	It("Podman info: check desired storage driver", func() {
		// defined in .cirrus.yml
		want := os.Getenv("CI_DESIRED_STORAGE")
		if want == "" {
			if os.Getenv("CIRRUS_CI") == "" {
				Skip("CI_DESIRED_STORAGE is not set--this is OK because we're not running under Cirrus")
			}
			Fail("CIRRUS_CI is set, but CI_DESIRED_STORAGE is not! See #20161")
		}
		session := podmanTest.PodmanExitCleanly("info", "--format", "{{.Store.GraphDriverName}}")
		Expect(session.OutputToString()).To(Equal(want), ".Store.GraphDriverName from podman info")

		// Confirm desired setting of composefs
		if want == "overlay" {
			expect := "<no value>"
			if os.Getenv("CI_DESIRED_COMPOSEFS") != "" {
				expect = "true"
			}
			session = podmanTest.PodmanExitCleanly("info", "--format", `{{index .Store.GraphOptions "overlay.use_composefs"}}`)
			Expect(session.OutputToString()).To(Equal(expect), ".Store.GraphOptions -> overlay.use_composefs")
		}
	})

	It("Podman info: check lock count", Serial, func() {
		// This should not run on architectures and OSes that use the file locks backend.
		// Which, for now, is Linux + RISCV and FreeBSD, neither of which are in CI - so
		// no skips.
		info1 := podmanTest.PodmanExitCleanly("info", "--format", "{{ .Host.FreeLocks }}")
		free1, err := strconv.Atoi(info1.OutputToString())
		Expect(err).To(Not(HaveOccurred()))

		podmanTest.PodmanExitCleanly("create", ALPINE, "top")

		info2 := podmanTest.PodmanExitCleanly("info", "--format", "{{ .Host.FreeLocks }}")
		free2, err := strconv.Atoi(info2.OutputToString())
		Expect(err).To(Not(HaveOccurred()))

		// Effectively, we are checking that 1 lock has been taken.
		// We do this by comparing the number of locks after (plus 1), to the number of locks before.
		// Don't check absolute numbers because there is a decent chance of contamination, containers that were never removed properly, etc.
		Expect(free1).To(Equal(free2 + 1))
	})

	It("Podman info: check for client information when no system service", func() {
		// the output for this information is not really something we can marshall
		want := runtime.GOOS + "/" + runtime.GOARCH
		podmanTest.StopRemoteService()
		SkipIfNotRemote("Specifically testing a failed remote connection")
		info := podmanTest.Podman([]string{"info"})
		info.WaitWithDefaultTimeout()
		Expect(info.OutputToString()).To(ContainSubstring(want))
		Expect(info).ToNot(ExitCleanly())
		podmanTest.StartRemoteService() // Start service again so teardown runs clean
	})

	It("Podman info: check client information", func() {
		info := podmanTest.PodmanExitCleanly("info", "--format", "{{ .Client }}")
		// client info should only appear when using the remote client
		if IsRemote() {
			Expect(info.OutputToString()).ToNot(Equal("<nil>"))
		} else {
			Expect(info.OutputToString()).To(Equal("<nil>"))
		}
	})
})
