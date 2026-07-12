//go:build linux || freebsd

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
)

// Without Ordered, tests flake with "Getting key identity" (#18358)
var _ = Describe("Podman trust", Ordered, func() {
	BeforeEach(func() {
		SkipIfRemote("podman-remote does not support image trust")
	})

	It("podman image trust show", func() {
		session := podmanTest.Podman([]string{"image", "trust", "show", "-n", "--registrypath", filepath.Join(INTEGRATION_ROOT, "test"), "--signature-policy", filepath.Join(INTEGRATION_ROOT, "test/policy.json")})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		outArray := session.OutputToStringArray()
		Expect(outArray).To(HaveLen(3))

		// Repository order is not guaranteed. So, check that
		// all expected lines appear in output; we also check total number of lines, so that handles all of them.
		Expect(string(session.Out.Contents())).To(MatchRegexp(`(?m)^all\s+default\s+accept\s*$`))
		Expect(string(session.Out.Contents())).To(MatchRegexp(`(?m)^repository\s+docker.io/library/hello-world\s+reject\s*$`))
		Expect(string(session.Out.Contents())).To(MatchRegexp(`(?m)^repository\s+registry.access.redhat.com\s+signed\s+security@redhat.com, security@redhat.com\s+https://access.redhat.com/webassets/docker/content/sigstore\s*$`))
	})

	It("podman image trust set", func() {
		policyJSON := filepath.Join(podmanTest.TempDir, "trust_set_test.json")
		session := podmanTest.Podman([]string{"image", "trust", "set", "--signature-policy", policyJSON, "-t", "accept", "default"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		var teststruct map[string][]map[string]string
		policyContent, err := os.ReadFile(policyJSON)
		if err != nil {
			os.Exit(1)
		}
		err = json.Unmarshal(policyContent, &teststruct)
		if err != nil {
			os.Exit(1)
		}
		Expect(teststruct["default"][0]).To(HaveKeyWithValue("type", "insecureAcceptAnything"))
	})

	It("podman image trust show --json", func() {
		session := podmanTest.Podman([]string{"image", "trust", "show", "--registrypath", filepath.Join(INTEGRATION_ROOT, "test"), "--signature-policy", filepath.Join(INTEGRATION_ROOT, "test/policy.json"), "--json"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(BeValidJSON())
		var teststruct []map[string]string
		err = json.Unmarshal(session.Out.Contents(), &teststruct)
		Expect(err).ToNot(HaveOccurred())
		Expect(teststruct).To(HaveLen(3))
		// To ease comparison, group the unordered array of repos by repo (and we expect only one entry by repo, so order within groups doesn’t matter)
		repoMap := map[string][]map[string]string{}
		for _, e := range teststruct {
			key := e["name"]
			repoMap[key] = append(repoMap[key], e)
		}
		Expect(repoMap).To(Equal(map[string][]map[string]string{
			"* (default)": {{
				"type":      "accept",
				"transport": "all",
				"name":      "* (default)",
				"repo_name": "default",
			}},
			"docker.io/library/hello-world": {{
				"transport": "repository",
				"name":      "docker.io/library/hello-world",
				"repo_name": "docker.io/library/hello-world",
				"type":      "reject",
			}},
			"registry.access.redhat.com": {{
				"transport": "repository",
				"name":      "registry.access.redhat.com",
				"repo_name": "registry.access.redhat.com",
				"sigstore":  "https://access.redhat.com/webassets/docker/content/sigstore",
				"type":      "signed",
				"gpg_id":    "security@redhat.com, security@redhat.com",
			}},
		}))
	})

	It("podman image trust show --raw", func() {
		session := podmanTest.Podman([]string{"image", "trust", "show", "--signature-policy", filepath.Join(INTEGRATION_ROOT, "test/policy.json"), "--raw"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		contents, err := os.ReadFile(filepath.Join(INTEGRATION_ROOT, "test/policy.json"))
		Expect(err).ShouldNot(HaveOccurred())
		Expect(session.OutputToString()).To(BeValidJSON())
		Expect(string(session.Out.Contents())).To(Equal(string(contents) + "\n"))
	})
})
