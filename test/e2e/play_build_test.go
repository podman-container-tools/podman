//go:build !remote_testing && (linux || freebsd)

// build for play kube is not supported on remote yet.

package integration

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("Podman play kube with build", func() {
	testYAML := `
apiVersion: v1
kind: Pod
metadata:
  creationTimestamp: "2021-08-05T17:55:51Z"
  labels:
    app: foobar
  name: top_pod
spec:
  containers:
  - command:
    - top
    env:
    - name: PATH
      value: /usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
    - name: TERM
      value: xterm
    - name: container
      value: podman
    image: foobar
    name: foobar
    resources: {}
    securityContext:
      allowPrivilegeEscalation: true
      privileged: false
      readOnlyRootFilesystem: false
      seLinuxOptions: {}
    tty: true
    workingDir: /
  dnsConfig: {}
status: {}
`

	playBuildFile := `
FROM ` + CITEST_IMAGE + `
LABEL homer=dad
COPY copyfile /copyfile
`
	prebuiltImage := `
FROM ` + CITEST_IMAGE + `
LABEL marge=mom
`

	copyFile := `just a text file
`

	It("Check that image is built using Dockerfile", func() {
		// Setup
		yamlDir := filepath.Join(tempdir, RandomString(12))
		err := os.Mkdir(yamlDir, 0o755)
		Expect(err).ToNot(HaveOccurred(), "mkdir "+yamlDir)
		err = writeYaml(testYAML, filepath.Join(yamlDir, "top.yaml"))
		Expect(err).ToNot(HaveOccurred())
		app1Dir := filepath.Join(yamlDir, "foobar")
		err = os.Mkdir(app1Dir, 0o755)
		Expect(err).ToNot(HaveOccurred())
		err = writeYaml(playBuildFile, filepath.Join(app1Dir, "Dockerfile"))
		Expect(err).ToNot(HaveOccurred())
		// Write a file to be copied
		err = writeYaml(copyFile, filepath.Join(app1Dir, "copyfile"))
		Expect(err).ToNot(HaveOccurred())
		// Switch to temp dir and restore it afterwards
		cwd, err := os.Getwd()
		Expect(err).ToNot(HaveOccurred())
		Expect(os.Chdir(yamlDir)).To(Succeed())
		defer func() { Expect(os.Chdir(cwd)).To(Succeed()) }()

		session := podmanTest.Podman([]string{"kube", "play", "top.yaml"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		stdErrString := session.ErrorToString()
		Expect(stdErrString).To(ContainSubstring("Getting image source signatures"))
		Expect(stdErrString).To(ContainSubstring("Writing manifest to image destination"))

		exists := podmanTest.Podman([]string{"image", "exists", "foobar"})
		exists.WaitWithDefaultTimeout()
		Expect(exists).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"container", "inspect", "top_pod-foobar"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		inspectData := inspect.InspectContainerToJSON()
		Expect(inspectData).ToNot(BeEmpty())
		Expect(inspectData[0].Config.Labels).To(HaveKeyWithValue("homer", "dad"))
	})

	It("Check that image is built using Containerfile", func() {
		// Setup
		yamlDir := filepath.Join(tempdir, RandomString(12))
		err := os.Mkdir(yamlDir, 0o755)
		Expect(err).ToNot(HaveOccurred(), "mkdir "+yamlDir)
		err = writeYaml(testYAML, filepath.Join(yamlDir, "top.yaml"))
		Expect(err).ToNot(HaveOccurred())
		app1Dir := filepath.Join(yamlDir, "foobar")
		err = os.Mkdir(app1Dir, 0o755)
		Expect(err).ToNot(HaveOccurred())
		err = writeYaml(playBuildFile, filepath.Join(app1Dir, "Containerfile"))
		Expect(err).ToNot(HaveOccurred())
		// Write a file to be copied
		err = writeYaml(copyFile, filepath.Join(app1Dir, "copyfile"))
		Expect(err).ToNot(HaveOccurred())
		// Switch to temp dir and restore it afterwards
		cwd, err := os.Getwd()
		Expect(err).ToNot(HaveOccurred())
		Expect(os.Chdir(yamlDir)).To(Succeed())
		defer func() { Expect(os.Chdir(cwd)).To(Succeed()) }()

		session := podmanTest.Podman([]string{"kube", "play", "top.yaml"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		stdErrString := session.ErrorToString()
		Expect(stdErrString).To(ContainSubstring("Getting image source signatures"))
		Expect(stdErrString).To(ContainSubstring("Writing manifest to image destination"))

		exists := podmanTest.Podman([]string{"image", "exists", "foobar"})
		exists.WaitWithDefaultTimeout()
		Expect(exists).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"container", "inspect", "top_pod-foobar"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		inspectData := inspect.InspectContainerToJSON()
		Expect(inspectData).ToNot(BeEmpty())
		Expect(inspectData[0].Config.Labels).To(HaveKeyWithValue("homer", "dad"))
	})

	It("Do not build image if already in the local store", func() {
		// Setup
		yamlDir := filepath.Join(tempdir, RandomString(12))
		err := os.Mkdir(yamlDir, 0o755)
		Expect(err).ToNot(HaveOccurred(), "mkdir "+yamlDir)
		err = writeYaml(testYAML, filepath.Join(yamlDir, "top.yaml"))
		Expect(err).ToNot(HaveOccurred())

		// build an image called foobar but make sure it doesn't have
		// the same label as the yaml buildfile, so we can check that
		// the image is NOT rebuilt.
		err = writeYaml(prebuiltImage, filepath.Join(yamlDir, "Containerfile"))
		Expect(err).ToNot(HaveOccurred())

		app1Dir := filepath.Join(yamlDir, "foobar")
		err = os.Mkdir(app1Dir, 0o755)
		Expect(err).ToNot(HaveOccurred())
		err = writeYaml(playBuildFile, filepath.Join(app1Dir, "Containerfile"))
		Expect(err).ToNot(HaveOccurred())
		// Write a file to be copied
		err = writeYaml(copyFile, filepath.Join(app1Dir, "copyfile"))
		Expect(err).ToNot(HaveOccurred())

		// Switch to temp dir and restore it afterwards
		cwd, err := os.Getwd()
		Expect(err).ToNot(HaveOccurred())
		Expect(os.Chdir(yamlDir)).To(Succeed())
		defer func() { Expect(os.Chdir(cwd)).To(Succeed()) }()

		// Build the image into the local store
		build := podmanTest.Podman([]string{"build", "-t", "foobar", "-f", "Containerfile"})
		build.WaitWithDefaultTimeout()
		Expect(build).Should(ExitCleanly())

		session := podmanTest.Podman([]string{"kube", "play", "top.yaml"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"container", "inspect", "top_pod-foobar"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		inspectData := inspect.InspectContainerToJSON()
		Expect(inspectData).ToNot(BeEmpty())
		Expect(inspectData[0].Config.Labels).To(Not(HaveKey("homer")))
		Expect(inspectData[0].Config.Labels).To(HaveKeyWithValue("marge", "mom"))
	})

	It("Do not build image at all if --build=false", func() {
		// Setup
		yamlDir := filepath.Join(tempdir, RandomString(12))
		err := os.Mkdir(yamlDir, 0o755)
		Expect(err).ToNot(HaveOccurred(), "mkdir "+yamlDir)
		err = writeYaml(testYAML, filepath.Join(yamlDir, "top.yaml"))
		Expect(err).ToNot(HaveOccurred())

		// build an image called foobar but make sure it doesn't have
		// the same label as the yaml buildfile, so we can check that
		// the image is NOT rebuilt.
		err = writeYaml(prebuiltImage, filepath.Join(yamlDir, "Containerfile"))
		Expect(err).ToNot(HaveOccurred())

		app1Dir := filepath.Join(yamlDir, "foobar")
		err = os.Mkdir(app1Dir, 0o755)
		Expect(err).ToNot(HaveOccurred())
		err = writeYaml(playBuildFile, filepath.Join(app1Dir, "Containerfile"))
		Expect(err).ToNot(HaveOccurred())
		// Write a file to be copied
		err = writeYaml(copyFile, filepath.Join(app1Dir, "copyfile"))
		Expect(err).ToNot(HaveOccurred())

		// Switch to temp dir and restore it afterwards
		cwd, err := os.Getwd()
		Expect(err).ToNot(HaveOccurred())
		Expect(os.Chdir(yamlDir)).To(Succeed())
		defer func() { Expect(os.Chdir(cwd)).To(Succeed()) }()

		// Build the image into the local store
		build := podmanTest.Podman([]string{"build", "-t", "foobar", "-f", "Containerfile"})
		build.WaitWithDefaultTimeout()
		Expect(build).Should(ExitCleanly())

		session := podmanTest.Podman([]string{"kube", "play", "--build=false", "top.yaml"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"container", "inspect", "top_pod-foobar"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		inspectData := inspect.InspectContainerToJSON()
		Expect(inspectData).ToNot(BeEmpty())
		Expect(inspectData[0].Config.Labels).To(Not(HaveKey("homer")))
		Expect(inspectData[0].Config.Labels).To(HaveKeyWithValue("marge", "mom"))
	})

	It("--build should override image in store", func() {
		// Setup
		yamlDir := filepath.Join(tempdir, RandomString(12))
		err := os.Mkdir(yamlDir, 0o755)
		Expect(err).ToNot(HaveOccurred(), "os.Mkdir "+yamlDir)
		err = writeYaml(testYAML, filepath.Join(yamlDir, "top.yaml"))
		Expect(err).ToNot(HaveOccurred())

		// build an image called foobar but make sure it doesn't have
		// the same label as the yaml buildfile, so we can check that
		// the image is NOT rebuilt.
		err = writeYaml(prebuiltImage, filepath.Join(yamlDir, "Containerfile"))
		Expect(err).ToNot(HaveOccurred())

		app1Dir := filepath.Join(yamlDir, "foobar")
		err = os.Mkdir(app1Dir, 0o755)
		Expect(err).ToNot(HaveOccurred())
		err = writeYaml(playBuildFile, filepath.Join(app1Dir, "Containerfile"))
		Expect(err).ToNot(HaveOccurred())
		// Write a file to be copied
		err = writeYaml(copyFile, filepath.Join(app1Dir, "copyfile"))
		Expect(err).ToNot(HaveOccurred())

		// Switch to temp dir and restore it afterwards
		cwd, err := os.Getwd()
		Expect(err).ToNot(HaveOccurred())
		Expect(os.Chdir(yamlDir)).To(Succeed())
		defer func() { Expect(os.Chdir(cwd)).To(Succeed()) }()

		// Build the image into the local store
		build := podmanTest.Podman([]string{"build", "-t", "foobar", "-f", "Containerfile"})
		build.WaitWithDefaultTimeout()
		Expect(build).Should(ExitCleanly())

		session := podmanTest.Podman([]string{"kube", "play", "--build", "top.yaml"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		stdErrString := session.ErrorToString()
		Expect(stdErrString).To(ContainSubstring("Getting image source signatures"))
		Expect(stdErrString).To(ContainSubstring("Writing manifest to image destination"))

		inspect := podmanTest.Podman([]string{"container", "inspect", "top_pod-foobar"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		inspectData := inspect.InspectContainerToJSON()
		Expect(inspectData).ToNot(BeEmpty())
		Expect(inspectData[0].Config.Labels).To(HaveKeyWithValue("homer", "dad"))
		Expect(inspectData[0].Config.Labels).To(Not(HaveKey("marge")))
	})

	testYAMLForEnvExpand := `
apiVersion: v1
kind: Pod
metadata:
  creationTimestamp: "2021-08-05T17:55:51Z"
  labels:
    app: foobar
  name: echo_pod
spec:
  containers:
  - command:
    - /bin/sh
    - -c
    - 'echo paren$(FOO) brace${FOO} dollardollarparen$$(FOO) interp$(FOO)olate'
    env:
    - name: PATH
      value: /usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
    - name: TERM
      value: xterm
    - name: container
      value: podman
    - name: FOO
      value: BAR
    image: foobar
    name: foobar
    resources: {}
    securityContext:
      allowPrivilegeEscalation: true
      privileged: false
      readOnlyRootFilesystem: false
      seLinuxOptions: {}
    tty: true
    workingDir: /
  restartPolicy: Never
  dnsConfig: {}
status: {}
`
	It("Check that command is expanded", func() {
		// Setup
		yamlDir := filepath.Join(tempdir, RandomString(12))
		err := os.Mkdir(yamlDir, 0o755)
		Expect(err).ToNot(HaveOccurred(), "mkdir "+yamlDir)
		err = writeYaml(testYAMLForEnvExpand, filepath.Join(yamlDir, "echo.yaml"))
		Expect(err).ToNot(HaveOccurred())
		app1Dir := filepath.Join(yamlDir, "foobar")
		err = os.Mkdir(app1Dir, 0o755)
		Expect(err).ToNot(HaveOccurred())
		err = writeYaml(playBuildFile+`ENV FOO foo-from-buildfile
COPY FOO /bin/FOO
`, filepath.Join(app1Dir, "Containerfile"))
		Expect(err).ToNot(HaveOccurred())
		// Write a file to be copied
		err = writeYaml(copyFile, filepath.Join(app1Dir, "copyfile"))
		Expect(err).ToNot(HaveOccurred())

		// Shell expansion of $(FOO) in container needs executable FOO
		err = writeYaml(`#!/bin/sh
echo GOT-HERE
`, filepath.Join(app1Dir, "FOO"))
		Expect(err).ToNot(HaveOccurred())
		err = os.Chmod(filepath.Join(app1Dir, "FOO"), 0o555)
		Expect(err).ToNot(HaveOccurred(), "chmod FOO")

		os.Setenv("FOO", "make sure we use FOO from kube file, not env")
		defer os.Unsetenv("FOO")

		// Switch to temp dir and restore it afterwards
		cwd, err := os.Getwd()
		Expect(err).ToNot(HaveOccurred())
		Expect(os.Chdir(yamlDir)).To(Succeed())
		defer func() { Expect(os.Chdir(cwd)).To(Succeed()) }()

		session := podmanTest.Podman([]string{"kube", "play", "echo.yaml"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		stdErrString := session.ErrorToString()
		Expect(stdErrString).To(ContainSubstring("Getting image source signatures"))
		Expect(stdErrString).To(ContainSubstring("Writing manifest to image destination"))

		cid := "echo_pod-foobar"
		wait := podmanTest.Podman([]string{"wait", cid})
		wait.WaitWithDefaultTimeout()
		Expect(wait).To(ExitCleanly())

		logs := podmanTest.Podman([]string{"logs", cid})
		logs.WaitWithDefaultTimeout()
		Expect(logs).Should(ExitCleanly())
		Expect(logs.OutputToString()).To(Equal("parenBAR braceBAR dollardollarparenGOT-HERE interpBARolate"))

		inspect := podmanTest.Podman([]string{"container", "inspect", cid})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		inspectData := inspect.InspectContainerToJSON()
		Expect(inspectData).ToNot(BeEmpty())

		// dollar-paren are expanded by podman, never seen by container
		Expect(inspectData[0].Args).To(HaveLen(2))
		Expect(inspectData[0].Args[1]).To(Equal("echo parenBAR brace${FOO} dollardollarparen$(FOO) interpBARolate"))
	})
})
