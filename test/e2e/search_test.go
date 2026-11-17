//go:build linux || freebsd

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"text/template"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.podman.io/podman/v6/pkg/domain/entities"
	. "go.podman.io/podman/v6/test/utils"
)

type endpoint struct {
	Host string
	Port string
}

func (e *endpoint) Address() string {
	return fmt.Sprintf("%s:%s", e.Host, e.Port)
}

var _ = Describe("Podman search", func() {
	const regFileContents = `
unqualified-search-registries = ["{{.Host}}:{{.Port}}"]
[[registry]]
location = "{{.Host}}:{{.Port}}"
insecure = true
`
	registryFileTmpl := template.Must(template.New("registryFile").Parse(regFileContents))

	const badRegFileContents = `
unqualified-search-registries = ["{{.Host}}:{{.Port}}"]
[[registry]]
location = "{{.Host}}:{{.Port}}"
insecure = false
`
	registryFileBadTmpl := template.Must(template.New("registryFileBad").Parse(badRegFileContents))

	mockFakeRegistryServerAsContainer := func(name string) endpoint {
		if runtime.GOARCH == "ppc64le" {
			Skip("No registry image for ppc64le")
		}
		port := GetPort()
		fakereg := podmanTest.Podman([]string{
			"run", "-d", "--name", name,
			"-p", fmt.Sprintf("%d:5000", port),
			REGISTRY_IMAGE, "/entrypoint.sh", "/etc/docker/registry/config.yml",
		})
		fakereg.WaitWithDefaultTimeout()
		Expect(fakereg).Should(ExitCleanly())

		if !WaitContainerReady(podmanTest, name, "listening on", 20, 1) {
			Fail("Cannot start docker registry on port %s", port)
		}
		ep := endpoint{Port: strconv.Itoa(port), Host: "localhost"}
		return ep
	}

	pushAlpineImageIntoMockRegistry := func(ep endpoint) string {
		err = podmanTest.RestoreArtifact(ALPINE)
		Expect(err).ToNot(HaveOccurred())
		image := fmt.Sprintf("%s/my-alpine", ep.Address())
		podmanTest.PodmanExitCleanly("push", "-q", "--tls-verify=false", "--remove-signatures", ALPINE, image)
		return image
	}

	Context("podman search with mock registry", func() {
		var registryAddress string
		var srv *http.Server
		var serverErr chan error

		BeforeEach(func() {
			registryAddress, srv, serverErr = CreateMockRegistryServer()
		})

		AfterEach(func() {
			CloseMockRegistryServer(srv, serverErr)
		})

		It("podman search", func() {
			search := podmanTest.PodmanExitCleanly("search", "--tls-verify=false", registryAddress+"/alpine")
			Expect(len(search.OutputToStringArray())).To(BeNumerically(">", 1))
			Expect(search.OutputToString()).To(ContainSubstring("alpine"))
		})

		It("podman search single registry flag", func() {
			search := podmanTest.PodmanExitCleanly("search", "--tls-verify=false", registryAddress+"/skopeo/stable:latest")
			Expect(search.OutputToString()).To(ContainSubstring(registryAddress + "/skopeo/stable"))
		})

		It("podman search image with description", func() {
			search := podmanTest.PodmanExitCleanly("search", "--tls-verify=false", registryAddress+"/podman/stable")
			output := string(search.Out.Contents())
			Expect(output).To(MatchRegexp(`(?m)NAME\s+DESCRIPTION$`))
			Expect(output).To(MatchRegexp(`(?m)/podman/stable\s+.*Podman Image`))
		})

		It("podman search image with --compatible", func() {
			search := podmanTest.PodmanExitCleanly("search", "--compatible", "--tls-verify=false", registryAddress+"/podman/stable")
			output := string(search.Out.Contents())
			Expect(output).To(MatchRegexp(`(?m)NAME\s+DESCRIPTION\s+STARS\s+OFFICIAL\s+AUTOMATED$`))
		})

		It("podman search format flag", func() {
			search := podmanTest.PodmanExitCleanly("search", "--format", "table {{.Index}} {{.Name}}", "--tls-verify=false", registryAddress+"/testdigest_v2s2")
			Expect(len(search.OutputToStringArray())).To(BeNumerically(">", 1))
			Expect(search.OutputToString()).To(ContainSubstring(registryAddress + "/libpod/testdigest_v2s2"))
		})

		It("podman search format json", func() {
			search := podmanTest.PodmanExitCleanly("search", "--format", "json", "--tls-verify=false", registryAddress+"/testdigest_v2s1")
			Expect(search.OutputToString()).To(BeValidJSON())
			Expect(search.OutputToString()).To(ContainSubstring(registryAddress + "/libpod/testdigest_v2s1"))
			Expect(search.OutputToString()).To(ContainSubstring("Test image used by buildah regression tests"))

			// Test for https://github.com/containers/podman/issues/11894
			contents := make([]entities.ImageSearchReport, 0)
			err := json.Unmarshal(search.Out.Contents(), &contents)
			Expect(err).ToNot(HaveOccurred())
			Expect(contents).ToNot(BeEmpty(), "No results from image search")
			for _, element := range contents {
				Expect(element.Description).ToNot(HaveSuffix("..."))
			}
		})

		It("podman search format json list tags", func() {
			search := podmanTest.PodmanExitCleanly("search", "--list-tags", "--format", "json", "--tls-verify=false", registryAddress+"/libpod/alpine:latest")
			Expect(search.OutputToString()).To(BeValidJSON())
			Expect(search.OutputToString()).To(ContainSubstring(registryAddress + "/libpod/alpine"))
			Expect(search.OutputToString()).To(ContainSubstring("3.10.2"))
			Expect(search.OutputToString()).To(ContainSubstring("3.2"))
		})

		// Test for https://github.com/containers/podman/issues/11894
		It("podman search no-trunc=false flag", func() {
			search := podmanTest.PodmanExitCleanly("search", "--no-trunc=false", "--tls-verify=false", registryAddress+"/alpine", "--format={{.Description}}")

			for _, line := range search.OutputToStringArray() {
				if len(line) > 44 {
					Expect(line).To(HaveSuffix("..."), line+" should have been truncated")
				}
			}
		})

		It("podman search limit flag", func() {
			search := podmanTest.PodmanExitCleanly("search", "--tls-verify=false", registryAddress+"/alpine")
			Expect(len(search.OutputToStringArray())).To(BeNumerically(">", 10))

			search = podmanTest.PodmanExitCleanly("search", "--limit", "3", "--tls-verify=false", registryAddress+"/alpine")
			search.WaitWithDefaultTimeout()
			Expect(search).Should(ExitCleanly())
			Expect(search.OutputToStringArray()).To(HaveLen(4))

			search = podmanTest.PodmanExitCleanly("search", "--limit", "10", "--tls-verify=false", registryAddress+"/alpine")
			Expect(search.OutputToStringArray()).To(HaveLen(11))
		})

		It("podman search with filter stars", func() {
			search := podmanTest.PodmanExitCleanly("search", "--filter", "stars=10", "--format", "{{.Stars}}", "--tls-verify=false", registryAddress+"/alpine")
			output := search.OutputToStringArray()
			for i := range output {
				Expect(strconv.Atoi(output[i])).To(BeNumerically(">=", 10))
			}
		})

		It("podman search with filter is-official", func() {
			search := podmanTest.PodmanExitCleanly("search", "--filter", "is-official", "--format", "{{.Official}}", "--tls-verify=false", registryAddress+"/alpine")
			output := search.OutputToStringArray()
			for i := range output {
				Expect(output[i]).To(Equal("[OK]"))
			}
		})

		It("podman search with filter is-automated", func() {
			search := podmanTest.PodmanExitCleanly("search", "--filter", "is-automated=false", "--format", "{{.Automated}}", "--tls-verify=false", registryAddress+"/alpine")
			output := search.OutputToStringArray()
			for i := range output {
				Expect(output[i]).To(Equal(""))
			}
		})

		It("podman search format list tags with custom", func() {
			search := podmanTest.PodmanExitCleanly("search", "--list-tags", "--format", "{{.Name}}", "--limit", "1", "--tls-verify=false", registryAddress+"/libpod/alpine")
			Expect(search.OutputToString()).To(Equal(registryAddress + "/libpod/alpine"))
		})

		It("podman search with wildcards", func() {
			search := podmanTest.PodmanExitCleanly("search", "--tls-verify=false", registryAddress+"/*alpine*")
			Expect(len(search.OutputToStringArray())).To(BeNumerically(">", 1))
			Expect(search.OutputToString()).To(ContainSubstring("alpine"))
		})

		It("podman search repository tags", func() {
			search := podmanTest.PodmanExitCleanly("search", "--list-tags", "--limit", "30", "--tls-verify=false", registryAddress+"/podman/stable")
			Expect(search.OutputToStringArray()).To(HaveLen(31))

			search = podmanTest.PodmanExitCleanly("search", "--list-tags", "--tls-verify=false", registryAddress+"/podman/stable")
			Expect(len(search.OutputToStringArray())).To(BeNumerically(">", 2))

			search = podmanTest.Podman([]string{"search", "--filter=is-official", "--list-tags", "--tls-verify=false", registryAddress + "/podman/stable"})
			search.WaitWithDefaultTimeout()
			Expect(search).To(ExitWithError(125, "filters are not applicable to list tags result"))

			// With trailing slash
			search = podmanTest.Podman([]string{"search", "--list-tags", "--tls-verify=false", registryAddress + "/podman/"})
			search.WaitWithDefaultTimeout()
			Expect(search).To(ExitWithError(125, `reference "podman/" must be a docker reference`))
			Expect(search.OutputToStringArray()).To(BeEmpty())

			// No trailing slash
			search = podmanTest.Podman([]string{"search", "--list-tags", "--tls-verify=false", registryAddress + "/podman"})
			search.WaitWithDefaultTimeout()
			Expect(search).To(ExitWithError(125, "getting repository tags: fetching tags list: StatusCode: 404"))
			Expect(search.OutputToStringArray()).To(BeEmpty())
		})

		It("podman search with limit over 100", func() {
			search := podmanTest.PodmanExitCleanly("search", "--limit", "100", "--tls-verify=false", registryAddress+"/podman")
			Expect(len(search.OutputToStringArray())).To(BeNumerically("<=", 101))
		})
	})

	Context("podman search with container-based registries", func() {
		var ep endpoint
		var image string
		var registryName string
		var port int64

		setupRegistryConfig := func(ep endpoint, registryName string, template *template.Template) {
			var buffer bytes.Buffer
			err := template.Execute(&buffer, ep)
			Expect(err).ToNot(HaveOccurred())
			podmanTest.setRegistriesConfigEnv(buffer.Bytes())
			err = os.WriteFile(fmt.Sprintf("%s/%s.conf", tempdir, registryName), buffer.Bytes(), 0o644)
			Expect(err).ToNot(HaveOccurred())
		}

		BeforeEach(func() {
			registryName = fmt.Sprintf("registry%d", GinkgoRandomSeed())
			ep = mockFakeRegistryServerAsContainer(registryName)
			image = pushAlpineImageIntoMockRegistry(ep)

			port, err = strconv.ParseInt(ep.Port, 10, 64)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			resetRegistriesConfigEnv()
			podmanTest.PodmanExitCleanly("rm", "-f", registryName)
		})

		It("podman search attempts HTTP if tls-verify flag is set false", func() {
			// if this test succeeded, there will be no output (there is no entry named fake/image:andtag in an empty registry)
			// and the exit code will be 0
			search := podmanTest.PodmanExitCleanly("search", fmt.Sprintf("%s/fake/image:andtag", ep.Address()), "--tls-verify=false")
			Expect(search.OutputToString()).Should(BeEmpty())
		})

		It("podman search in local registry", func() {
			search := podmanTest.PodmanExitCleanly("search", image, "--tls-verify=false")
			Expect(search.OutputToString()).ShouldNot(BeEmpty())

			// podman search v2 registry with empty query
			searchEmpty := podmanTest.PodmanExitCleanly("search", fmt.Sprintf("%s/", ep.Address()), "--tls-verify=false")
			Expect(searchEmpty.OutputToStringArray()).ToNot(BeEmpty())
			Expect(search.OutputToString()).To(ContainSubstring("my-alpine"))
		})

		It("podman search attempts HTTP if registry is in registries.insecure and force secure is false", func() {
			// registries.conf set up
			setupRegistryConfig(ep, registryName, registryFileTmpl)
			if IsRemote() {
				podmanTest.RestartRemoteService()
				defer podmanTest.RestartRemoteService()
			}

			search := podmanTest.PodmanExitCleanly("search", image)
			Expect(search.OutputToString()).To(ContainSubstring("my-alpine"))
		})

		It("podman search doesn't attempt HTTP if force secure is true", func() {
			setupRegistryConfig(ep, registryName, registryFileTmpl)
			if IsRemote() {
				podmanTest.RestartRemoteService()
				defer podmanTest.RestartRemoteService()
			}

			search := podmanTest.Podman([]string{"search", image, "--tls-verify=true"})
			search.WaitWithDefaultTimeout()

			Expect(search).Should(ExitWithError(125, fmt.Sprintf(`couldn't search registry "localhost:%d": pinging container registry localhost:%d: Get "https://localhost:%d/v2/": http: server gave HTTP response to HTTPS client`, port, port, port)))
			Expect(search.OutputToString()).Should(BeEmpty())
		})

		It("podman search doesn't attempt HTTP if registry is not listed as insecure", func() {
			setupRegistryConfig(ep, registryName, registryFileBadTmpl)
			if IsRemote() {
				podmanTest.RestartRemoteService()
				defer podmanTest.RestartRemoteService()
			}

			search := podmanTest.Podman([]string{"search", image})
			search.WaitWithDefaultTimeout()

			Expect(search).Should(ExitWithError(125, fmt.Sprintf(`couldn't search registry "localhost:%d": pinging container registry localhost:%d: Get "https://localhost:%d/v2/": http: server gave HTTP response to HTTPS client`, port, port, port)))
			Expect(search.OutputToString()).Should(BeEmpty())
		})
	})

	// search should fail with nonexistent authfile
	It("podman search fail with nonexistent --authfile", func() {
		search := podmanTest.Podman([]string{"search", "--authfile", "/tmp/nonexistent", ALPINE})
		search.WaitWithDefaultTimeout()
		Expect(search).To(ExitWithError(125, "credential file is not accessible: faccessat /tmp/nonexistent: no such file or directory"))
	})
})
