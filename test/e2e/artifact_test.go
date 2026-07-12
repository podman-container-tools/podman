//go:build linux || freebsd

package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	imgspec "github.com/opencontainers/image-spec/specs-go/v1"
	. "go.podman.io/podman/v6/test/utils"
	"go.podman.io/podman/v6/utils"
)

const (
	ARTIFACT_SINGLE         = "quay.io/libpod/testartifact:20250206-single"
	ARTIFACT_MULTI          = "quay.io/libpod/testartifact:20250206-multi"
	ARTIFACT_MULTI_NO_TITLE = "quay.io/libpod/testartifact:20250206-multi-no-title"
	ARTIFACT_EVIL           = "quay.io/libpod/testartifact:20250206-evil"
)

var _ = Describe("Podman artifact", func() {
	It("podman artifact ls", func() {
		artifact1File, err := createArtifactFile(4192)
		Expect(err).ToNot(HaveOccurred())
		artifact1Name := "localhost/test/artifact1"
		add1 := podmanTest.PodmanExitCleanly("artifact", "add", artifact1Name, artifact1File)

		artifact2File, err := createArtifactFile(10240)
		Expect(err).ToNot(HaveOccurred())
		artifact2Name := "localhost/test/artifact2"
		podmanTest.PodmanExitCleanly("artifact", "add", artifact2Name, artifact2File)

		// Should be three items in the list
		listSession := podmanTest.PodmanExitCleanly("artifact", "ls")
		Expect(listSession.OutputToStringArray()).To(HaveLen(3))

		// --format should work
		listFormatSession := podmanTest.PodmanExitCleanly("artifact", "ls", "--format", "{{.Repository}}")
		output := listFormatSession.OutputToStringArray()

		// There should be only 2 "lines" because the header should not be output
		Expect(output).To(HaveLen(2))

		// Make sure the names are what we expect
		Expect(output).To(ContainElement(artifact1Name))
		Expect(output).To(ContainElement(artifact2Name))

		// Check default digest length (should be 12)
		defaultFormatSession := podmanTest.PodmanExitCleanly("artifact", "ls", "--format", "{{.Digest}}")
		defaultOutput := defaultFormatSession.OutputToStringArray()[0]
		Expect(defaultOutput).To(HaveLen(12))

		// Check with --no-trunc and verify the len of the digest is the same as the len what was returned when the artifact
		// was added
		noTruncSession := podmanTest.PodmanExitCleanly("artifact", "ls", "--no-trunc", "--format", "{{.Digest}}")
		truncOutput := noTruncSession.OutputToStringArray()[0]
		Expect(truncOutput).To(HaveLen(len(add1.OutputToString())))

		// check with --noheading and verify the header is not present through a line count AND substring match
		noHeaderSession := podmanTest.PodmanExitCleanly("artifact", "ls", "--noheading")
		noHeaderOutput := noHeaderSession.OutputToStringArray()
		Expect(noHeaderOutput).To(HaveLen(2))
		Expect(noHeaderOutput).ToNot(ContainElement("REPOSITORY"))

		// Check if .VirtualSize is reported correctly
		virtualSizeFormatSession := podmanTest.PodmanExitCleanly("artifact", "ls", "--format", "{{.VirtualSize}}")
		virtualSizes := virtualSizeFormatSession.OutputToStringArray()

		// Should list 2 lines (without the header)
		Expect(virtualSizes).To(HaveLen(2))

		// Verify if the virtual size values are present in the output
		Expect(virtualSizes).To(ContainElement("4192"))
		Expect(virtualSizes).To(ContainElement("10240"))

		// Check if .Created is reported correctly
		createdFormatSession := podmanTest.PodmanExitCleanly("artifact", "ls", "--format", "{{.Created}}")
		created := createdFormatSession.OutputToStringArray()

		Expect(created).To(HaveLen(2))

		// Assuming the test runs less than a minute
		humanReadableDurationRegexp := `^(Less than a second|1 second|\d+ seconds) ago$`
		Expect(created).To(ContainElements(MatchRegexp(humanReadableDurationRegexp), MatchRegexp(humanReadableDurationRegexp)))

		// Check if .CreatedAt is reported correctly
		createdAtFormatSession := podmanTest.PodmanExitCleanly("artifact", "ls", "--format", "{{.CreatedAt}}")
		createdAt := createdAtFormatSession.OutputToStringArray()

		Expect(createdAt).To(HaveLen(2))

		// Verify the timestamp format looks like "2025-10-23 12:34:56.789 +0000 UTC"
		timestampRegexp := `^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}(\.\d+)? \+\d{4} UTC$`
		Expect(createdAt).To(ContainElements(MatchRegexp(timestampRegexp), MatchRegexp(timestampRegexp)))
	})

	It("podman artifact simple add", func() {
		artifact1File, err := createArtifactFile(1024)
		Expect(err).ToNot(HaveOccurred())

		artifact1Name := "localhost/test/artifact1"
		podmanTest.PodmanExitCleanly("artifact", "add", artifact1Name, artifact1File)

		a := podmanTest.InspectArtifact(artifact1Name)

		Expect(a.Name).To(Equal(artifact1Name + ":latest"))

		// Adding an artifact with an existing name should fail
		addAgain := podmanTest.Podman([]string{"artifact", "add", artifact1Name, artifact1File})
		addAgain.WaitWithDefaultTimeout()
		Expect(addAgain).Should(ExitWithError(125, fmt.Sprintf("Error: %s: artifact already exists", artifact1Name+":latest")))
	})

	It("podman artifact add with options", func() {
		yamlType := "text/yaml"
		artifact1Name := "localhost/test/artifact1:latest"
		artifact2Name := "localhost/test/artifact2:latest"
		artifact3Name := "localhost/test/artifact3:latest"
		artifact1File, err := createArtifactFile(1024)
		Expect(err).ToNot(HaveOccurred())
		artifact2File, err := createArtifactFile(1024)
		Expect(err).ToNot(HaveOccurred())

		artifactType := "octet/foobar"
		annotation1 := "color=blue"
		annotation2 := "flavor=lemon"

		podmanTest.PodmanExitCleanly("artifact", "add", "--file-type", yamlType, "--type", artifactType, "--annotation", annotation1, "--annotation", annotation2, artifact1Name, artifact1File)

		a := podmanTest.InspectArtifact(artifact1Name)
		Expect(a.Name).To(Equal(artifact1Name))
		Expect(a.Manifest.ArtifactType).To(Equal(artifactType))
		Expect(a.Manifest.Layers[0].Annotations["color"]).To(Equal("blue"))
		Expect(a.Manifest.Layers[0].Annotations["flavor"]).To(Equal("lemon"))
		Expect(a.Manifest.Layers[0].MediaType).To(Equal(yamlType))

		title := RandomString(12)
		annotation3 := fmt.Sprintf("%s=%s", imgspec.AnnotationTitle, title)
		podmanTest.PodmanExitCleanly("artifact", "add", "--annotation", annotation3, artifact2Name, artifact1File)
		a = podmanTest.InspectArtifact(artifact2Name)
		Expect(a.Manifest.Layers[0].Annotations[imgspec.AnnotationTitle]).To(Equal(title))

		failSession := podmanTest.Podman([]string{"artifact", "add", "--append", "--annotation", annotation3, artifact2Name, artifact2File})
		failSession.WaitWithDefaultTimeout()
		Expect(failSession).Should(ExitWithError(125, "Error: duplicate layers org.opencontainers.image.title labels within an artifact not allowed"))

		title = RandomString(12)
		annotation3 = fmt.Sprintf("%s=%s", imgspec.AnnotationTitle, title)
		podmanTest.PodmanExitCleanly("artifact", "add", "--append", "--annotation", annotation3, artifact2Name, artifact2File)
		a = podmanTest.InspectArtifact(artifact2Name)
		Expect(a.Manifest.Layers[1].Annotations[imgspec.AnnotationTitle]).To(Equal(title))

		failSession = podmanTest.Podman([]string{"artifact", "add", "--annotation", annotation3, artifact3Name, artifact1File, artifact2File})
		failSession.WaitWithDefaultTimeout()
		Expect(failSession).Should(ExitWithError(125, "Error: duplicate layers org.opencontainers.image.title labels within an artifact not allowed"))
	})

	It("podman artifact add multiple", func() {
		artifact1File1, err := createArtifactFile(1024)
		Expect(err).ToNot(HaveOccurred())
		artifact1File2, err := createArtifactFile(8192)
		Expect(err).ToNot(HaveOccurred())

		artifact1Name := "localhost/test/artifact1:123"

		podmanTest.PodmanExitCleanly("artifact", "add", artifact1Name, artifact1File1, artifact1File2)

		a := podmanTest.InspectArtifact(artifact1Name)
		Expect(a.Name).To(Equal(artifact1Name))

		Expect(a.Manifest.Layers).To(HaveLen(2))
	})

	It("podman artifact push and pull", func() {
		// Before starting a registry, try to pull a bogus image from a bogus registry
		// using retry-delay
		retrySession := podmanTest.Podman([]string{"artifact", "pull", "--retry", "1", "--retry-delay", "100ms", "127.0.0.1/mybadimagename"})
		retrySession.WaitWithDefaultTimeout()
		Expect(retrySession).Should(ExitWithError(125, "connect: connection refused"))

		// TODO: This can be removed once Artifact API supports streaming
		if !IsRemote() {
			Expect(retrySession.ErrorToString()).To(ContainSubstring("retrying in 100ms ..."))
		}

		artifact1File, err := createArtifactFile(1024)
		Expect(err).ToNot(HaveOccurred())

		lock, port, err := setupRegistry(nil)
		if err == nil {
			defer lock.Unlock()
		}
		Expect(err).ToNot(HaveOccurred())

		artifact1Name := fmt.Sprintf("localhost:%s/test/artifact1:mytag", port)
		podmanTest.PodmanExitCleanly("artifact", "add", artifact1Name, artifact1File)

		podmanTest.PodmanExitCleanly("artifact", "push", "-q", "--tls-verify=false", artifact1Name)

		podmanTest.PodmanExitCleanly("artifact", "rm", artifact1Name)

		podmanTest.PodmanExitCleanly("artifact", "pull", "--tls-verify=false", artifact1Name)

		a := podmanTest.InspectArtifact(artifact1Name)

		Expect(a.Name).To(Equal(artifact1Name))
	})

	It("podman artifact push with authorization", func() {
		portNo, err := utils.GetRandomPort()
		Expect(err).ToNot(HaveOccurred())
		port := strconv.Itoa(portNo)

		lock := GetPortLock(port)
		defer lock.Unlock()

		artifact1File, err := createArtifactFile(1024)
		Expect(err).ToNot(HaveOccurred())
		artifact1Name := fmt.Sprintf("localhost:%s/test/artifact1", port)
		podmanTest.PodmanExitCleanly("artifact", "add", artifact1Name, artifact1File)

		authPath := filepath.Join(podmanTest.TempDir, "auth")
		err = os.Mkdir(authPath, os.ModePerm)
		Expect(err).ToNot(HaveOccurred())
		htpasswd := SystemExec("htpasswd", []string{"-Bbn", "podmantest", "test"})
		htpasswd.WaitWithDefaultTimeout()
		Expect(htpasswd).Should(ExitCleanly())

		f, err := os.Create(filepath.Join(authPath, "htpasswd"))
		Expect(err).ToNot(HaveOccurred())
		defer f.Close()
		_, err = f.WriteString(htpasswd.OutputToString())
		Expect(err).ToNot(HaveOccurred())
		err = f.Sync()
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("run", "-d", "-p", port+":5000", "--name", "artifact-creds-registry", "-v",
			strings.Join([]string{authPath, "/auth", "z"}, ":"), "-e", "REGISTRY_AUTH=htpasswd", "-e",
			"REGISTRY_AUTH_HTPASSWD_REALM=Registry Realm", "-e", "REGISTRY_AUTH_HTPASSWD_PATH=/auth/htpasswd",
			REGISTRY_IMAGE)
		Expect(WaitContainerReady(podmanTest, "artifact-creds-registry", "listening on", 20, 1)).To(BeTrue(), "registry container ready")

		push := podmanTest.Podman([]string{"artifact", "push", "--tls-verify=false", "--creds=podmantest:wrongpasswd", artifact1Name})
		push.WaitWithDefaultTimeout()
		Expect(push).To(ExitWithError(125, "/artifact1: authentication required"))

		podmanTest.PodmanExitCleanly("artifact", "push", "-q", "--tls-verify=false", "--creds=podmantest:test", artifact1Name)
	})

	It("podman artifact push and pull with authfile", func() {
		portNo, err := utils.GetRandomPort()
		Expect(err).ToNot(HaveOccurred())
		port := strconv.Itoa(portNo)

		lock := GetPortLock(port)
		defer lock.Unlock()

		artifact1File, err := createArtifactFile(1024)
		Expect(err).ToNot(HaveOccurred())
		artifact1Name := fmt.Sprintf("localhost:%s/test/artifact1:latest", port)
		podmanTest.PodmanExitCleanly("artifact", "add", artifact1Name, artifact1File)

		authPath := filepath.Join(podmanTest.TempDir, "auth")
		err = os.Mkdir(authPath, os.ModePerm)
		Expect(err).ToNot(HaveOccurred())
		htpasswd := SystemExec("htpasswd", []string{"-Bbn", "podmantest", "test"})
		htpasswd.WaitWithDefaultTimeout()
		Expect(htpasswd).Should(ExitCleanly())

		f, err := os.Create(filepath.Join(authPath, "htpasswd"))
		Expect(err).ToNot(HaveOccurred())
		defer f.Close()
		_, err = f.WriteString(htpasswd.OutputToString())
		Expect(err).ToNot(HaveOccurred())
		err = f.Sync()
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("run", "-d", "-p", port+":5000", "--name", "artifact-authfile-registry", "-v",
			strings.Join([]string{authPath, "/auth", "z"}, ":"), "-e", "REGISTRY_AUTH=htpasswd", "-e",
			"REGISTRY_AUTH_HTPASSWD_REALM=Registry Realm", "-e", "REGISTRY_AUTH_HTPASSWD_PATH=/auth/htpasswd",
			REGISTRY_IMAGE)
		Expect(WaitContainerReady(podmanTest, "artifact-authfile-registry", "listening on", 20, 1)).To(BeTrue(), "registry container ready")

		authFile := filepath.Join(podmanTest.TempDir, "auth.json")
		server := fmt.Sprintf("localhost:%s", port)
		podmanTest.PodmanExitCleanly("login", "--username", "podmantest", "--password", "test", "--authfile", authFile, "--tls-verify=false", server)

		pushFail := podmanTest.Podman([]string{"artifact", "push", "-q", "--authfile", "/tmp/nonexistent", "--tls-verify=false", artifact1Name})
		pushFail.WaitWithDefaultTimeout()
		Expect(pushFail).To(ExitWithError(125, "credential file is not accessible"))

		podmanTest.PodmanExitCleanly("artifact", "push", "-q", "--authfile", authFile, "--tls-verify=false", artifact1Name)

		podmanTest.PodmanExitCleanly("artifact", "rm", artifact1Name)
		podmanTest.PodmanExitCleanly("artifact", "pull", "--authfile", authFile, "--tls-verify=false", artifact1Name)
		inspectedArtifact := podmanTest.InspectArtifact(artifact1Name)
		Expect(inspectedArtifact.Name).To(Equal(artifact1Name))
	})

	It("podman artifact remove", func() {
		// Trying to remove an image that does not exist should fail
		rmFail := podmanTest.Podman([]string{"artifact", "rm", "foobar"})
		rmFail.WaitWithDefaultTimeout()
		Expect(rmFail).Should(ExitWithError(125, fmt.Sprintf("Error: %s: artifact does not exist", "foobar")))

		// Add an artifact to remove later
		artifact1File, err := createArtifactFile(4192)
		Expect(err).ToNot(HaveOccurred())
		artifact1Name := "localhost/test/artifact1:latest"
		addArtifact1 := podmanTest.PodmanExitCleanly("artifact", "add", artifact1Name, artifact1File)

		// Removing that artifact should work
		rmWorks := podmanTest.PodmanExitCleanly("artifact", "rm", artifact1Name)
		// The digests printed by removal should be the same as the digest that was added
		Expect(rmWorks.OutputToString()).To(ContainSubstring(addArtifact1.OutputToString()))

		// Inspecting that the removed artifact should fail
		inspectArtifact := podmanTest.Podman([]string{"artifact", "inspect", artifact1Name})
		inspectArtifact.WaitWithDefaultTimeout()
		Expect(inspectArtifact).Should(ExitWithError(125, fmt.Sprintf("Error: %s: artifact does not exist", artifact1Name)))

		// Add some artifacts back in
		artifact2File, err := createArtifactFile(8096)
		Expect(err).ToNot(HaveOccurred())
		artifact2Name := "localhost/test/artifact2"
		podmanTest.PodmanExitCleanly("artifact", "add", artifact2Name, artifact2File)
		podmanTest.PodmanExitCleanly("artifact", "add", artifact1Name, artifact1File)

		// Using -a and an arg should trigger an error
		failArgs := podmanTest.Podman([]string{"artifact", "rm", "-a", artifact1Name})
		failArgs.WaitWithDefaultTimeout()
		Expect(failArgs).Should(ExitWithError(125, "Error: when using the --all switch, you may not pass any artifact names or digests"))

		// No args is an error
		failNoArgs := podmanTest.Podman([]string{"artifact", "rm"})
		failNoArgs.WaitWithDefaultTimeout()
		Expect(failNoArgs).Should(ExitWithError(125, "Error: at least one artifact name or digest must be specified"))

		// Remove all
		podmanTest.PodmanExitCleanly("artifact", "rm", "-a")

		// There should be no artifacts in the store
		rmAll := podmanTest.PodmanExitCleanly("artifact", "ls", "--noheading")
		Expect(rmAll.OutputToString()).To(BeEmpty())

		// Trying to remove an artifact that does not exist should pass with -i
		podmanTest.PodmanExitCleanly("artifact", "rm", "-i", "foobar")

		// Add an artifact to test remove with --ignore flag
		artifact3File, err := createArtifactFile(4192)
		Expect(err).ToNot(HaveOccurred())
		artifact3Name := "localhost/test/artifact3"
		_ = podmanTest.PodmanExitCleanly("artifact", "add", artifact3Name, artifact3File)

		// Trying to remove an existing artifact should also pass with -i
		podmanTest.PodmanExitCleanly("artifact", "rm", "-i", artifact3Name)

		// There should be no artifacts in the store at this point
		rmAll = podmanTest.PodmanExitCleanly("artifact", "ls", "--noheading")
		Expect(rmAll.OutputToString()).To(BeEmpty())
	})

	It("podman artifact inspect with full or partial digest", func() {
		artifact1File, err := createArtifactFile(4192)
		Expect(err).ToNot(HaveOccurred())
		artifact1Name := "localhost/test/artifact1"
		addArtifact1 := podmanTest.PodmanExitCleanly("artifact", "add", artifact1Name, artifact1File)

		artifactDigest := addArtifact1.OutputToString()

		podmanTest.PodmanExitCleanly("artifact", "inspect", artifactDigest)
		podmanTest.PodmanExitCleanly("artifact", "inspect", artifactDigest[:12])
	})

	It("podman artifact extract single", func() {
		podmanTest.PodmanExitCleanly("artifact", "pull", ARTIFACT_SINGLE)

		const (
			artifactContent = "mRuO9ykak1Q2j\n"
			artifactDigest  = "sha256:e9510923578af3632946ecf5ae479c1b5f08b47464e707b5cbab9819272a9752"
			artifactTitle   = "testfile"
		)

		path := filepath.Join(podmanTest.TempDir, "testfile")
		// Extract to non existing file
		podmanTest.PodmanExitCleanly("artifact", "extract", ARTIFACT_SINGLE, path)
		Expect(readFileToString(path)).To(Equal(artifactContent))

		// Extract to existing file will overwrite file
		path = filepath.Join(podmanTest.TempDir, "abcd")
		f, err := os.Create(path)
		Expect(err).ToNot(HaveOccurred())
		f.Close()
		podmanTest.PodmanExitCleanly("artifact", "extract", ARTIFACT_SINGLE, path)
		Expect(readFileToString(path)).To(Equal(artifactContent))

		tests := []struct {
			name      string
			filename  string
			extraArgs []string
		}{
			{
				name:     "extract to dir",
				filename: artifactTitle,
			},
			{
				name:      "extract to dir by digest",
				filename:  digestToFilename(artifactDigest),
				extraArgs: []string{"--digest", artifactDigest},
			},
			{
				name:      "extract to dir by title",
				filename:  artifactTitle,
				extraArgs: []string{"--title", artifactTitle},
			},
		}

		for _, tt := range tests {
			By(tt.name)
			dir := makeTempDirInDir(podmanTest.TempDir)
			args := append([]string{"artifact", "extract"}, tt.extraArgs...)
			args = append(args, ARTIFACT_SINGLE, dir)
			podmanTest.PodmanExitCleanly(args...)
			Expect(readFileToString(filepath.Join(dir, tt.filename))).To(Equal(artifactContent))
		}

		// invalid digest
		session := podmanTest.Podman([]string{"artifact", "extract", "--digest", "blah", ARTIFACT_SINGLE, podmanTest.TempDir})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, `no blob with the digest "blah"`))

		// invalid title
		session = podmanTest.Podman([]string{"artifact", "extract", "--title", "abcd", ARTIFACT_SINGLE, podmanTest.TempDir})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, `no blob with the title "abcd"`))
	})

	It("podman artifact extract multi", func() {
		podmanTest.PodmanExitCleanly("artifact", "pull", ARTIFACT_MULTI)
		podmanTest.PodmanExitCleanly("artifact", "pull", ARTIFACT_MULTI_NO_TITLE)

		const (
			artifactContent1 = "xuHWedtC0ADST\n"
			artifactDigest1  = "sha256:8257bba28b9d19ac353c4b713b470860278857767935ef7e139afd596cb1bb2d"
			artifactTitle1   = "test1"
			artifactContent2 = "tAyZczFlgFsi4\n"
			artifactDigest2  = "sha256:63700c54129c6daaafe3a20850079f82d6d658d69de73d6158d81f920c6fbdd7"
			artifactTitle2   = "test2"
		)

		type expect struct {
			filename string
			content  string
		}
		tests := []struct {
			name      string
			image     string
			extraArgs []string
			expect    []expect
		}{
			{
				name:  "extract multi blob to dir",
				image: ARTIFACT_MULTI,
				expect: []expect{
					{filename: artifactTitle1, content: artifactContent1},
					{filename: artifactTitle2, content: artifactContent2},
				},
			},
			{
				name:  "extract multi blob to dir without title",
				image: ARTIFACT_MULTI_NO_TITLE,
				expect: []expect{
					{filename: digestToFilename(artifactDigest1), content: artifactContent1},
					{filename: digestToFilename(artifactDigest2), content: artifactContent2},
				},
			},
			{
				name:      "extract multi blob to dir with --title",
				image:     ARTIFACT_MULTI,
				extraArgs: []string{"--title", artifactTitle1},
				expect: []expect{
					{filename: artifactTitle1, content: artifactContent1},
				},
			},
			{
				name:      "extract multi blob to dir with --digest",
				image:     ARTIFACT_MULTI,
				extraArgs: []string{"--digest", artifactDigest2},
				expect: []expect{
					{filename: digestToFilename(artifactDigest2), content: artifactContent2},
				},
			},
		}

		for _, tt := range tests {
			By(tt.name)
			dir := makeTempDirInDir(podmanTest.TempDir)
			args := append([]string{"artifact", "extract"}, tt.extraArgs...)
			args = append(args, tt.image, dir)
			podmanTest.PodmanExitCleanly(args...)
			files, err := os.ReadDir(dir)
			Expect(err).ToNot(HaveOccurred())
			Expect(files).To(HaveLen(len(tt.expect)))
			for _, expect := range tt.expect {
				Expect(readFileToString(filepath.Join(dir, expect.filename))).To(Equal(expect.content))
			}
		}
	})

	It("podman artifact extract evil", func() {
		path := filepath.Join(podmanTest.TempDir, "testfile")
		podmanTest.PodmanExitCleanly("artifact", "pull", ARTIFACT_EVIL)

		const (
			artifactContent = "RM5eA27F9psa2\n"
			artifactDigest  = "sha256:4c29da41ff27fcbf273653bcfba58ed69efa4aefec7b6c486262711cb1dfd050"
		)

		// Extract to file is fine as we are not using the malicious title
		podmanTest.PodmanExitCleanly("artifact", "extract", ARTIFACT_EVIL, path)
		Expect(readFileToString(path)).To(Equal(artifactContent))

		// This must fail for security reasons we do not allow a title with /
		session := podmanTest.Podman([]string{"artifact", "extract", ARTIFACT_EVIL, podmanTest.TempDir})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, `invalid name: "../../../../tmp/evil" cannot contain /`))

		// Extracting by digest should be fine too
		podmanTest.PodmanExitCleanly("artifact", "extract", "--digest", artifactDigest, ARTIFACT_EVIL, podmanTest.TempDir)
		Expect(readFileToString(filepath.Join(podmanTest.TempDir, digestToFilename(artifactDigest)))).To(Equal(artifactContent))
	})

	It("podman artifact simple add --append", func() {
		artifact1File, err := createArtifactFile(1024)
		Expect(err).ToNot(HaveOccurred())

		artifact2File, err := createArtifactFile(2048)
		Expect(err).ToNot(HaveOccurred())

		artifact3File, err := createArtifactFile(4192)
		Expect(err).ToNot(HaveOccurred())

		layersNames := map[string]int{
			filepath.Base(artifact1File): 0,
			filepath.Base(artifact2File): 0,
			filepath.Base(artifact3File): 0,
		}

		artifact1Name := "localhost/test/artifact1:latest"
		podmanTest.PodmanExitCleanly("artifact", "add", artifact1Name, artifact1File)

		_ = podmanTest.InspectArtifact(artifact1Name)

		podmanTest.PodmanExitCleanly("artifact", "add", "--append", artifact1Name, artifact2File)

		a := podmanTest.InspectArtifact(artifact1Name)

		Expect(a.Manifest.Layers).To(HaveLen(2))

		annotation1 := "color=blue"
		podmanTest.PodmanExitCleanly("artifact", "add", "--append", "--annotation", annotation1, artifact1Name, artifact3File)

		a = podmanTest.InspectArtifact(artifact1Name)
		Expect(a.Name).To(Equal(artifact1Name))
		Expect(a.Manifest.Layers).To(HaveLen(3))

		for _, l := range a.Manifest.Layers {
			layersNames[l.Annotations[imgspec.AnnotationTitle]] += 1

			if l.Annotations[imgspec.AnnotationTitle] == filepath.Base(artifact3File) {
				Expect(l.Annotations["color"]).To(Equal("blue"))
			} else {
				Expect(l.Annotations).To(HaveLen(1))
			}
		}
		for _, count := range layersNames {
			Expect(count).To(Equal(1))
		}

		listSession := podmanTest.PodmanExitCleanly([]string{"artifact", "ls"}...)
		Expect(listSession.OutputToStringArray()).To(HaveLen(2))
	})

	It("podman artifact add --append multiple", func() {
		artifact1File, err := createArtifactFile(1024)
		Expect(err).ToNot(HaveOccurred())

		artifact2File, err := createArtifactFile(2048)
		Expect(err).ToNot(HaveOccurred())

		artifact3File, err := createArtifactFile(4192)
		Expect(err).ToNot(HaveOccurred())

		artifact1Name := "localhost/test/artifact1"
		podmanTest.PodmanExitCleanly("artifact", "add", artifact1Name, artifact1File)

		podmanTest.PodmanExitCleanly("artifact", "add", "--append", artifact1Name, artifact2File, artifact3File)

		a := podmanTest.InspectArtifact(artifact1Name)

		Expect(a.Manifest.Layers).To(HaveLen(3))
	})

	It("podman artifact add --append modified file", func() {
		artifact1File, err := createArtifactFile(1024)
		Expect(err).ToNot(HaveOccurred())

		artifact1Name := "localhost/test/artifact1"
		podmanTest.PodmanExitCleanly("artifact", "add", artifact1Name, artifact1File)

		f, err := os.OpenFile(artifact1File, os.O_APPEND|os.O_WRONLY, 0o644)
		Expect(err).ToNot(HaveOccurred())
		_, err = f.WriteString("This is modification.")
		Expect(err).ToNot(HaveOccurred())
		f.Close()

		appendFail := podmanTest.Podman([]string{"artifact", "add", "--append", artifact1Name, artifact1File})
		appendFail.WaitWithDefaultTimeout()
		//  Error: PJYjLgoU: file already exists in artifact
		Expect(appendFail).Should(ExitWithError(125, fmt.Sprintf("Error: %s: file already exists in artifact", filepath.Base(artifact1File))))

		a := podmanTest.InspectArtifact(artifact1Name)

		Expect(a.Manifest.Layers).To(HaveLen(1))
		Expect(a.TotalSizeBytes()).To(Equal(int64(1024)))
	})

	It("podman artifact add file already exists in artifact", func() {
		artifact1File, err := createArtifactFile(2048)
		Expect(err).ToNot(HaveOccurred())

		artifact1Name := "localhost/test/artifact1:latest"

		addFail := podmanTest.Podman([]string{"artifact", "add", artifact1Name, artifact1File, artifact1File})
		addFail.WaitWithDefaultTimeout()
		Expect(addFail).Should(ExitWithError(125, fmt.Sprintf("Error: %s: file already exists in artifact", filepath.Base(artifact1File))))

		inspectFail := podmanTest.Podman([]string{"artifact", "inspect", artifact1Name})
		inspectFail.WaitWithDefaultTimeout()
		//   Error: localhost/test/artifact1:latest: artifact does not exist
		Expect(inspectFail).Should(ExitWithError(125, fmt.Sprintf("Error: %s: artifact does not exist", artifact1Name)))
	})

	It("podman artifact add --append file already exists in artifact", func() {
		artifact1File, err := createArtifactFile(2048)
		Expect(err).ToNot(HaveOccurred())

		artifact1Name := "localhost/test/artifact1"
		podmanTest.PodmanExitCleanly("artifact", "add", artifact1Name, artifact1File)

		appendFail := podmanTest.Podman([]string{"artifact", "add", "--append", artifact1Name, artifact1File})
		appendFail.WaitWithDefaultTimeout()
		Expect(appendFail).Should(ExitWithError(125, fmt.Sprintf("Error: %s: file already exists in artifact", filepath.Base(artifact1File))))
		a := podmanTest.InspectArtifact(artifact1Name)

		Expect(a.Manifest.Layers).To(HaveLen(1))
		Expect(a.TotalSizeBytes()).To(Equal(int64(2048)))
	})

	It("podman artifact add with --append and --type", func() {
		artifact1Name := "localhost/test/artifact1:latest"
		artifact1File, err := createArtifactFile(1024)
		Expect(err).ToNot(HaveOccurred())

		artifact2File, err := createArtifactFile(2048)
		Expect(err).ToNot(HaveOccurred())

		artifact3File, err := createArtifactFile(4192)
		Expect(err).ToNot(HaveOccurred())

		artifactType := "octet/foobar"

		podmanTest.PodmanExitCleanly("artifact", "add", "--type", artifactType, artifact1Name, artifact1File)

		a := podmanTest.InspectArtifact(artifact1Name)
		Expect(a.Name).To(Equal(artifact1Name))
		Expect(a.Manifest.ArtifactType).To(Equal(artifactType))

		podmanTest.PodmanExitCleanly("artifact", "add", "--append", artifact1Name, artifact2File)

		a = podmanTest.InspectArtifact(artifact1Name)
		Expect(a.Name).To(Equal(artifact1Name))
		Expect(a.Manifest.ArtifactType).To(Equal(artifactType))
		Expect(a.Manifest.Layers).To(HaveLen(2))

		failSession := podmanTest.Podman([]string{"artifact", "add", "--type", artifactType, "--append", artifact1Name, artifact3File})
		failSession.WaitWithDefaultTimeout()
		Expect(failSession).Should(ExitWithError(125, "Error: append option is not compatible with type option"))
	})

	It("podman artifact inspect shows created date", func() {
		artifact1File, err := createArtifactFile(1024)
		Expect(err).ToNot(HaveOccurred())
		artifact2File, err := createArtifactFile(2048)
		Expect(err).ToNot(HaveOccurred())

		artifact1Name := "localhost/test/artifact1:latest"

		// Add artifact
		podmanTest.PodmanExitCleanly("artifact", "add", artifact1Name, artifact1File)

		// Inspect artifact
		a := podmanTest.InspectArtifact(artifact1Name)
		Expect(a.Name).To(Equal(artifact1Name))

		// Check that created annotation exists and is in valid Unix nanosecond format
		createdStr, exists := a.Manifest.Annotations["org.opencontainers.image.created"]
		Expect(exists).To(BeTrue(), "Should have org.opencontainers.image.created annotation")

		// podman artifact append preserves original created date
		// Wait a moment to ensure timestamps would be different
		time.Sleep(100 * time.Millisecond)

		// Append to the artifact
		podmanTest.PodmanExitCleanly("artifact", "add", "--append", artifact1Name, artifact2File)

		// Check that created timestamp is unchanged
		a = podmanTest.InspectArtifact(artifact1Name)
		currentCreated := a.Manifest.Annotations["org.opencontainers.image.created"]
		Expect(currentCreated).To(Equal(createdStr), "Created timestamp should not change when appending")

		// Verify we have 2 layers
		Expect(a.Manifest.Layers).To(HaveLen(2))
	})

	It("podman artifact inspect with --format", func() {
		artifact1File, err := createArtifactFile(4192)
		Expect(err).ToNot(HaveOccurred())
		artifact1Name := "localhost/test/artifact1:latest"
		addArtifact1 := podmanTest.PodmanExitCleanly("artifact", "add", artifact1Name, artifact1File)

		artifactDigest := addArtifact1.OutputToString()

		session := podmanTest.PodmanExitCleanly("artifact", "inspect", artifactDigest, "--format", "{{.Digest}}")
		Expect(session.OutputToString()).To(Equal("sha256:" + artifactDigest))
		session = podmanTest.PodmanExitCleanly("artifact", "inspect", artifactDigest[:12], "-f", "{{.Name}}")
		Expect(session.OutputToString()).To(Equal(artifact1Name))
	})
})

func digestToFilename(digest string) string {
	return strings.ReplaceAll(digest, ":", "-")
}

func readFileToString(path string) string {
	GinkgoHelper()
	b, err := os.ReadFile(path)
	Expect(err).ToNot(HaveOccurred())
	return string(b)
}
