//go:build linux || freebsd

package integration

import (
	"encoding/json"
	"regexp"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman buildx inspect", func() {
	It("podman buildx inspect", func() {
		session := podmanTest.PodmanExitCleanly("buildx", "inspect")
		out := session.OutputToString()

		session_bootstrap := podmanTest.PodmanExitCleanly("buildx", "inspect", "--bootstrap")
		out_bootstrap := session_bootstrap.OutputToString()
		Expect(out_bootstrap).To(Equal(out), "Output of 'podman buildx inspect' and 'podman buildx inspect --bootstrap' should be the same")

		emuInfo := podmanTest.PodmanExitCleanly("info", "--format", "{{json .Host.EmulatedArchitectures}}")
		var emuArchs []string
		Expect(json.Unmarshal([]byte(emuInfo.OutputToString()), &emuArchs)).To(Succeed())

		nativeInfo := podmanTest.PodmanExitCleanly("info", "--format", "{{.Host.OS}}/{{.Host.Arch}}")
		nativePlat := strings.TrimSpace(nativeInfo.OutputToString())
		Expect(nativePlat).ToNot(BeEmpty())

		expected := append(emuArchs, nativePlat)

		for _, p := range expected {
			re := regexp.MustCompile(`(?s)Platforms:.*\b` + regexp.QuoteMeta(p) + `\b`)
			Expect(out).To(MatchRegexp(re.String()), "missing %q in:\n%s", p, out)
		}
	})
})
