//go:build linux || freebsd

package integration

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
	. "go.podman.io/podman/v6/test/utils"
	"go.podman.io/storage/pkg/stringid"
)

var _ = Describe("Podman secret", func() {
	AfterEach(func() {
		podmanTest.CleanupSecrets()
	})

	It("podman secret create", func() {
		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := os.WriteFile(secretFilePath, []byte("mysecret"), 0o755)
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"secret", "create", "-d", "file", "--driver-opts", "opt1=val", "a", secretFilePath})
		session.WaitWithDefaultTimeout()
		secrID := session.OutputToString()
		Expect(session).Should(ExitCleanly())

		result := podmanTest.Podman([]string{"events", "--stream=false"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		events := result.OutputToStringArray()
		Expect(events).ToNot(BeEmpty(), "Number of events")
		Expect(events).To(ContainElement(ContainSubstring(" secret create %s", secrID)))

		inspect := podmanTest.Podman([]string{"secret", "inspect", "--format", "{{.ID}}", secrID})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(Equal(secrID))
		inspect = podmanTest.Podman([]string{"secret", "inspect", "-f", "{{.Spec.Driver.Options}}", secrID})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(ContainSubstring("opt1:val"))

		session = podmanTest.Podman([]string{"secret", "create", "-d", "file", "--driver-opts", "opt1=val1", "a", secretFilePath})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(125, "Error: a: secret name in use"))

		session = podmanTest.Podman([]string{"secret", "create", "-d", "file", "--driver-opts", "opt1=val1", "--replace", "a", secretFilePath})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Not(Equal(secrID)))

		inspect = podmanTest.Podman([]string{"secret", "inspect", "-f", "{{.Spec.Driver.Options}}", secrID})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).To(ExitWithError(125, fmt.Sprintf("Error: inspecting secret: no secret with name or id %q: no such secret", secrID)))

		inspect = podmanTest.Podman([]string{"secret", "inspect", "-f", "{{.Spec.Driver.Options}}", "a"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(ContainSubstring("opt1:val1"))
	})

	It("podman secret create with --ignore", func() {
		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := os.WriteFile(secretFilePath, []byte("mysecret"), 0o755)
		Expect(err).ToNot(HaveOccurred())

		// First create a secret
		session := podmanTest.Podman([]string{"secret", "create", "ignore-test", secretFilePath})
		session.WaitWithDefaultTimeout()
		secrID := session.OutputToString()
		Expect(session).Should(ExitCleanly())

		// Try to create the same secret again without --ignore, should fail
		session = podmanTest.Podman([]string{"secret", "create", "ignore-test", secretFilePath})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(125, "Error: ignore-test: secret name in use"))

		// Try to create the same secret again with --ignore, should succeed and return existing ID
		session = podmanTest.Podman([]string{"secret", "create", "--ignore", "ignore-test", secretFilePath})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal(secrID))

		// Try to use both --ignore and --replace, should fail
		session = podmanTest.Podman([]string{"secret", "create", "--ignore", "--replace", "ignore-test", secretFilePath})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(125, "Error: cannot use --ignore and --replace flags together"))
	})

	It("podman secret create bad name should fail", func() {
		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := os.WriteFile(secretFilePath, []byte("mysecret"), 0o755)
		Expect(err).ToNot(HaveOccurred())

		badName := "foo/bar"
		session := podmanTest.Podman([]string{"secret", "create", badName, secretFilePath})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, fmt.Sprintf("Error: secret name %q can not include '=', '/', ',', or the '\\0' (NULL) and be between 1 and 253 characters: invalid secret name", badName)))

		badName = "foo=bar"
		session = podmanTest.Podman([]string{"secret", "create", badName, secretFilePath})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, fmt.Sprintf("Error: secret name %q can not include '=', '/', ',', or the '\\0' (NULL) and be between 1 and 253 characters: invalid secret name", badName)))
	})

	It("podman secret inspect", func() {
		random := stringid.GenerateRandomID()
		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := os.WriteFile(secretFilePath, []byte(random), 0o755)
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"secret", "create", "a", secretFilePath})
		session.WaitWithDefaultTimeout()
		secrID := session.OutputToString()
		Expect(session).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"secret", "inspect", secrID})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(BeValidJSON())

		inspect = podmanTest.Podman([]string{"secret", "inspect", "--format", "{{ .SecretData }}", secrID})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(Equal(""))

		inspect = podmanTest.Podman([]string{"secret", "inspect", "--showsecret", "--format", "{{ .SecretData }}", secrID})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(Equal(random))
	})

	It("podman secret inspect with --format", func() {
		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := os.WriteFile(secretFilePath, []byte("mysecret"), 0o755)
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"secret", "create", "a", secretFilePath})
		session.WaitWithDefaultTimeout()
		secrID := session.OutputToString()
		Expect(session).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"secret", "inspect", "--format", "{{.ID}}", secrID})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(Equal(secrID))
	})

	It("podman secret inspect with --pretty", func() {
		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := os.WriteFile(secretFilePath, []byte("mysecret"), 0o755)
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"secret", "create", "a", secretFilePath})
		session.WaitWithDefaultTimeout()
		secrID := session.OutputToString()
		Expect(session).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"secret", "inspect", "--pretty", secrID})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(ContainSubstring("Name:"))
		Expect(inspect.OutputToString()).To(ContainSubstring(secrID))
	})

	It("podman secret inspect multiple secrets", func() {
		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := os.WriteFile(secretFilePath, []byte("mysecret"), 0o755)
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"secret", "create", "a", secretFilePath})
		session.WaitWithDefaultTimeout()
		secrID := session.OutputToString()
		Expect(session).Should(ExitCleanly())

		session2 := podmanTest.Podman([]string{"secret", "create", "b", secretFilePath})
		session2.WaitWithDefaultTimeout()
		secrID2 := session2.OutputToString()
		Expect(session2).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"secret", "inspect", secrID, secrID2})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(BeValidJSON())
	})

	It("podman secret inspect bogus", func() {
		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := os.WriteFile(secretFilePath, []byte("mysecret"), 0o755)
		Expect(err).ToNot(HaveOccurred())

		inspect := podmanTest.Podman([]string{"secret", "inspect", "bogus"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).To(ExitWithError(125, `inspecting secret: no secret with name or id "bogus": no such secret`))
	})

	It("podman secret ls", func() {
		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := os.WriteFile(secretFilePath, []byte("mysecret"), 0o755)
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"secret", "create", "a", secretFilePath})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		list := podmanTest.Podman([]string{"secret", "ls"})
		list.WaitWithDefaultTimeout()
		Expect(list).Should(ExitCleanly())
		Expect(list.OutputToStringArray()).To(HaveLen(2))
	})

	It("podman secret ls --quiet", func() {
		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := os.WriteFile(secretFilePath, []byte("mysecret"), 0o755)
		Expect(err).ToNot(HaveOccurred())

		secretName := "a"

		session := podmanTest.Podman([]string{"secret", "create", secretName, secretFilePath})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		secretID := session.OutputToString()

		list := podmanTest.Podman([]string{"secret", "ls", "-q"})
		list.WaitWithDefaultTimeout()
		Expect(list).Should(ExitCleanly())
		Expect(list.OutputToString()).To(Equal(secretID))

		list = podmanTest.Podman([]string{"secret", "ls", "--quiet"})
		list.WaitWithDefaultTimeout()
		Expect(list).Should(ExitCleanly())
		Expect(list.OutputToString()).To(Equal(secretID))

		// Prefer format over quiet
		list = podmanTest.Podman([]string{"secret", "ls", "-q", "--format", "{{.Name}}"})
		list.WaitWithDefaultTimeout()
		Expect(list).Should(ExitCleanly())
		Expect(list.OutputToString()).To(Equal(secretName))
	})

	It("podman secret ls with filters", func() {
		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := os.WriteFile(secretFilePath, []byte("mysecret"), 0o755)
		Expect(err).ToNot(HaveOccurred())

		secret1 := "Secret1"
		secret2 := "Secret2"

		session := podmanTest.Podman([]string{"secret", "ls", "-n"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal(""))

		session = podmanTest.Podman([]string{"secret", "ls", "--noheading"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal(""))

		session = podmanTest.Podman([]string{"secret", "create", secret1, secretFilePath})
		session.WaitWithDefaultTimeout()
		secrID1 := session.OutputToString()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"secret", "create", secret2, secretFilePath})
		session.WaitWithDefaultTimeout()
		secrID2 := session.OutputToString()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"secret", "create", "Secret3", secretFilePath})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		list := podmanTest.Podman([]string{"secret", "ls", "--filter", fmt.Sprintf("name=%s", secret1)})
		list.WaitWithDefaultTimeout()
		Expect(list).Should(ExitCleanly())
		Expect(list.OutputToStringArray()).To(HaveLen(2))
		Expect(list.OutputToStringArray()[1]).To(ContainSubstring(secret1))

		list = podmanTest.Podman([]string{"secret", "ls", "--filter", fmt.Sprintf("name=%s", secret2)})
		list.WaitWithDefaultTimeout()
		Expect(list).Should(ExitCleanly())
		Expect(list.OutputToStringArray()).To(HaveLen(2))
		Expect(list.OutputToStringArray()[1]).To(ContainSubstring(secret2))

		list = podmanTest.Podman([]string{"secret", "ls", "--filter", fmt.Sprintf("id=%s", secrID1)})
		list.WaitWithDefaultTimeout()
		Expect(list).Should(ExitCleanly())
		Expect(list.OutputToStringArray()).To(HaveLen(2))
		Expect(list.OutputToStringArray()[1]).To(ContainSubstring(secrID1))

		list = podmanTest.Podman([]string{"secret", "ls", "--filter", fmt.Sprintf("id=%s", secrID2)})
		list.WaitWithDefaultTimeout()
		Expect(list).Should(ExitCleanly())
		Expect(list.OutputToStringArray()).To(HaveLen(2))
		Expect(list.OutputToStringArray()[1]).To(ContainSubstring(secrID2))

		list = podmanTest.Podman([]string{"secret", "ls", "--filter", fmt.Sprintf("name=%s", secret1), "--filter", fmt.Sprintf("name=%s", secret2)})
		list.WaitWithDefaultTimeout()
		Expect(list).Should(ExitCleanly())
		Expect(list.OutputToStringArray()).To(HaveLen(3))
		Expect(list.OutputToString()).To(ContainSubstring(secret1))
		Expect(list.OutputToString()).To(ContainSubstring(secret2))
	})

	It("podman secret ls with Go template", func() {
		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := os.WriteFile(secretFilePath, []byte("mysecret"), 0o755)
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"secret", "create", "a", secretFilePath})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		list := podmanTest.Podman([]string{"secret", "ls", "--format", "table {{.Name}}"})
		list.WaitWithDefaultTimeout()

		Expect(list).Should(ExitCleanly())
		Expect(list.OutputToStringArray()).To(HaveLen(2), list.OutputToString())
	})

	It("podman secret ls with valid format fields Driver/CreatedAt", func() {
		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := os.WriteFile(secretFilePath, []byte("mysecret"), 0o755)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("secret", "create", "fmt", secretFilePath)

		// .Driver should be available on SecretListReport
		list := podmanTest.PodmanExitCleanly("secret", "ls", "--format", "{{.Driver}}")
		Expect(list.OutputToString()).ToNot(BeEmpty())

		// .CreatedAt should be available and human-readable (e.g., "X ago")
		list = podmanTest.PodmanExitCleanly("secret", "ls", "--format", "{{.CreatedAt}}")
		Expect(list.OutputToString()).To(ContainSubstring("ago"))
	})

	It("podman secret ls with invalid Spec.* format should error", func() {
		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := os.WriteFile(secretFilePath, []byte("mysecret"), 0o755)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("secret", "create", "fmt2", secretFilePath)

		bad := podmanTest.Podman([]string{"secret", "ls", "--format", "{{.Spec.Name}}"})
		bad.WaitWithDefaultTimeout()
		Expect(bad).To(Not(ExitCleanly()))
		Expect(bad.ErrorToString()).To(ContainSubstring("can't evaluate field Spec"))
	})

	It("podman secret rm", func() {
		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := os.WriteFile(secretFilePath, []byte("mysecret"), 0o755)
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"secret", "create", "a", secretFilePath})
		session.WaitWithDefaultTimeout()
		secrID := session.OutputToString()
		Expect(session).Should(ExitCleanly())

		removed := podmanTest.Podman([]string{"secret", "rm", "a"})
		removed.WaitWithDefaultTimeout()
		Expect(removed).Should(ExitCleanly())
		Expect(removed.OutputToString()).To(Equal(secrID))

		result := podmanTest.Podman([]string{"events", "--stream=false"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		events := result.OutputToStringArray()
		Expect(events).ToNot(BeEmpty(), "Number of events")
		Expect(events).To(ContainElement(ContainSubstring(" secret remove %s", secrID)))

		session = podmanTest.Podman([]string{"secret", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(1))
	})

	It("podman secret rm --all", func() {
		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := os.WriteFile(secretFilePath, []byte("mysecret"), 0o755)
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"secret", "create", "a", secretFilePath})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		session = podmanTest.Podman([]string{"secret", "create", "b", secretFilePath})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		removed := podmanTest.Podman([]string{"secret", "rm", "-a"})
		removed.WaitWithDefaultTimeout()
		Expect(removed).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"secret", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(1))
	})

	It("podman secret rm --ignore", func() {
		remove := podmanTest.Podman([]string{"secret", "rm", "non-existent-secret"})
		remove.WaitWithDefaultTimeout()
		Expect(remove).Should(Not(ExitCleanly()))
		Expect(remove.ErrorToString()).To(Equal("Error: no secret with name or id \"non-existent-secret\": no such secret"))

		ignoreRm := podmanTest.Podman([]string{"secret", "rm", "--ignore", "non-existent-secret"})
		ignoreRm.WaitWithDefaultTimeout()
		Expect(ignoreRm).Should(ExitCleanly())
		Expect(ignoreRm.ErrorToString()).To(BeEmpty())
	})

	It("podman secret creates from environment variable", func() {
		// no env variable set, should fail
		session := podmanTest.Podman([]string{"secret", "create", "--env", "a", "MYENVVAR"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, "cannot create store secret data: environment variable MYENVVAR is not set"))

		os.Setenv("MYENVVAR", "somedata")
		if IsRemote() {
			podmanTest.RestartRemoteService()
		}

		session = podmanTest.Podman([]string{"secret", "create", "--env", "a", "MYENVVAR"})
		session.WaitWithDefaultTimeout()
		secrID := session.OutputToString()
		Expect(session).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"secret", "inspect", "--format", "{{.ID}}", secrID})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(Equal(secrID))
	})

	It("podman secret with labels", func() {
		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := os.WriteFile(secretFilePath, []byte("mysecret"), 0o755)
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"secret", "create", "--label", "foo=bar", "a", secretFilePath})
		session.WaitWithDefaultTimeout()
		secrID := session.OutputToString()
		Expect(session).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"secret", "inspect", "--format", "{{.Spec.Labels}}", secrID})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(ContainSubstring("foo:bar"))

		session = podmanTest.Podman([]string{"secret", "create", "--label", "foo=bar", "--label", "a:b", "b", secretFilePath})
		session.WaitWithDefaultTimeout()
		secrID = session.OutputToString()
		Expect(session).Should(ExitCleanly())

		inspect = podmanTest.Podman([]string{"secret", "inspect", "--format", "{{.Spec.Labels}}", secrID})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(ContainSubstring("foo:bar"))
		Expect(inspect.OutputToString()).To(ContainSubstring("a:b"))

		session = podmanTest.Podman([]string{"secret", "create", "c", secretFilePath})
		session.WaitWithDefaultTimeout()
		secrID = session.OutputToString()
		Expect(session).Should(ExitCleanly())

		inspect = podmanTest.Podman([]string{"secret", "inspect", "--format", "{{.Spec.Labels}}", secrID})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(Equal("map[]"))
	})

	It("podman secret exists should return true if secret exists", func() {
		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := os.WriteFile(secretFilePath, []byte("mysecret"), 0o755)
		Expect(err).ToNot(HaveOccurred())

		secretName := "does_exist"

		session := podmanTest.Podman([]string{"secret", "create", secretName, secretFilePath})
		session.WaitWithDefaultTimeout()
		secretID := session.OutputToString()
		Expect(session).Should(ExitCleanly())

		exists := podmanTest.Podman([]string{"secret", "exists", secretName})
		exists.WaitWithDefaultTimeout()
		Expect(exists).Should(ExitCleanly())

		exists = podmanTest.Podman([]string{"secret", "exists", secretID})
		exists.WaitWithDefaultTimeout()
		Expect(exists).Should(ExitCleanly())
	})

	It("podman secret exists should return false if secret does not exist", func() {
		secretName := "does_not_exist"

		exists := podmanTest.Podman([]string{"secret", "exists", secretName})
		exists.WaitWithDefaultTimeout()
		Expect(exists).Should(ExitWithError(1, ""))
	})

	It("podman secret create with non-default driver does not inherit default driver opts", func() {
		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := os.WriteFile(secretFilePath, []byte("mysecret"), 0o755)
		Expect(err).ToNot(HaveOccurred())

		// Create a secret with driver=shell but no --driver-opts.
		// Before the fix, the file driver's default "path" opt bled
		// into the shell driver, causing "invalid shell driver option".
		// After the fix, the shell driver correctly receives no
		// foreign opts and fails only because its required commands
		// (store, lookup, list, delete) are not configured.
		session := podmanTest.Podman([]string{
			"secret", "create", "-d", "shell",
			"shell-secret", secretFilePath,
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(125, "missing config value"))
	})

	It("podman secret create from stdin", func() {
		secretData := "mysecretdata"
		secretName := "stdin-secret-" + stringid.GenerateRandomID()

		args := []string{"secret", "create", secretName, "-"}
		podmanOptions := podmanTest.MakeOptions(args, PodmanExecOptions{})
		cmd := exec.Command(podmanTest.PodmanBinary, podmanOptions...)
		cmd.Stdin = strings.NewReader(secretData)
		session, err := Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).ToNot(HaveOccurred())
		podmanSession := &PodmanSession{Session: session}
		podmanSession.WaitWithDefaultTimeout()
		Expect(podmanSession).Should(ExitCleanly())
		secrID := podmanSession.OutputToString()

		inspect := podmanTest.Podman([]string{"secret", "inspect", "--showsecret", "--format", "{{.SecretData}}", secrID})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(Equal(secretData))
	})
})
