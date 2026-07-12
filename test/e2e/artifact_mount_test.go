//go:build linux || freebsd

package integration

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("Podman artifact mount", func() {
	It("podman artifact mount single blob", func() {
		podmanTest.PodmanExitCleanly("artifact", "pull", ARTIFACT_SINGLE)

		const artifactContent = "mRuO9ykak1Q2j"

		tests := []struct {
			name          string
			mountOpts     string
			containerFile string
		}{
			{
				name:          "single artifact mount",
				mountOpts:     "dst=/test",
				containerFile: "/test",
			},
			{
				name:          "single artifact mount on existing file",
				mountOpts:     "dst=/etc/os-release",
				containerFile: "/etc/os-release",
			},
			{
				name:          "single artifact mount with title",
				mountOpts:     "dst=/tmp,title=testfile",
				containerFile: "/tmp/testfile",
			},
			{
				name:          "single artifact mount with digest",
				mountOpts:     "dst=/data,digest=sha256:e9510923578af3632946ecf5ae479c1b5f08b47464e707b5cbab9819272a9752",
				containerFile: "/data",
			},
			{
				name:          "single artifact mount with name",
				mountOpts:     "dst=/tmp,name=abcd",
				containerFile: "/tmp/abcd",
			},
		}

		for _, tt := range tests {
			By(tt.name)
			// FIXME: we need https://github.com/containers/container-selinux/pull/360 to fix the selinux access problem, until then disable it.
			session := podmanTest.PodmanExitCleanly("run", "--security-opt=label=disable", "--rm", "--mount", "type=artifact,src="+ARTIFACT_SINGLE+","+tt.mountOpts, ALPINE, "cat", tt.containerFile)
			Expect(session.OutputToString()).To(Equal(artifactContent))
		}
	})

	It("podman artifact mount multi blob", func() {
		podmanTest.PodmanExitCleanly("artifact", "pull", ARTIFACT_MULTI)
		podmanTest.PodmanExitCleanly("artifact", "pull", ARTIFACT_MULTI_NO_TITLE)

		const (
			artifactContent1 = "xuHWedtC0ADST"
			artifactContent2 = "tAyZczFlgFsi4"
		)

		type expectedFiles struct {
			file    string
			content string
		}

		tests := []struct {
			name           string
			mountOpts      string
			containerFiles []expectedFiles
		}{
			{
				name:      "multi blob with title",
				mountOpts: "src=" + ARTIFACT_MULTI + ",dst=/test",
				containerFiles: []expectedFiles{
					{
						file:    "/test/test1",
						content: artifactContent1,
					},
					{
						file:    "/test/test2",
						content: artifactContent2,
					},
				},
			},
			{
				name:      "multi blob without title",
				mountOpts: "src=" + ARTIFACT_MULTI_NO_TITLE + ",dst=/test",
				containerFiles: []expectedFiles{
					{
						file:    "/test/sha256-8257bba28b9d19ac353c4b713b470860278857767935ef7e139afd596cb1bb2d",
						content: artifactContent1,
					},
					{
						file:    "/test/sha256-63700c54129c6daaafe3a20850079f82d6d658d69de73d6158d81f920c6fbdd7",
						content: artifactContent2,
					},
				},
			},
			{
				name:      "multi blob filter by title on non existing file",
				mountOpts: "src=" + ARTIFACT_MULTI + ",dst=/test,title=test2",
				containerFiles: []expectedFiles{
					{
						file:    "/test",
						content: artifactContent2,
					},
				},
			},
			{
				name:      "multi blob filter by title on existing file",
				mountOpts: "src=" + ARTIFACT_MULTI + ",dst=/tmp,title=test2",
				containerFiles: []expectedFiles{
					{
						file:    "/tmp/test2",
						content: artifactContent2,
					},
				},
			},
			{
				name:      "multi blob filter by digest",
				mountOpts: "src=" + ARTIFACT_MULTI + ",dst=/tmp,digest=sha256:8257bba28b9d19ac353c4b713b470860278857767935ef7e139afd596cb1bb2d",
				containerFiles: []expectedFiles{
					{
						file:    "/tmp/sha256-8257bba28b9d19ac353c4b713b470860278857767935ef7e139afd596cb1bb2d",
						content: artifactContent1,
					},
				},
			},
			{
				name:      "multi blob with name",
				mountOpts: "src=" + ARTIFACT_MULTI + ",dst=/test,name=myname",
				containerFiles: []expectedFiles{
					{
						file:    "/test/myname-0",
						content: artifactContent1,
					},
					{
						file:    "/test/myname-1",
						content: artifactContent2,
					},
				},
			},
		}
		for _, tt := range tests {
			By(tt.name)
			// FIXME: we need https://github.com/containers/container-selinux/pull/360 to fix the selinux access problem, until then disable it.
			args := []string{"run", "--security-opt=label=disable", "--rm", "--mount", "type=artifact," + tt.mountOpts, ALPINE, "cat"}
			for _, f := range tt.containerFiles {
				args = append(args, f.file)
			}
			session := podmanTest.PodmanExitCleanly(args...)
			outs := session.OutputToStringArray()
			Expect(outs).To(HaveLen(len(tt.containerFiles)))
			for i, f := range tt.containerFiles {
				Expect(outs[i]).To(Equal(f.content))
			}
		}
	})

	It("podman artifact mount remove while in use", func() {
		ctrName := "ctr1"
		artifactName := "localhost/test"
		artifactFileName := "somefile"

		artifactFile := filepath.Join(podmanTest.TempDir, artifactFileName)
		err := os.WriteFile(artifactFile, []byte("hello world\n"), 0o644)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("artifact", "add", artifactName, artifactFile)

		// FIXME: we need https://github.com/containers/container-selinux/pull/360 to fix the selinux access problem, until then disable it.
		podmanTest.PodmanExitCleanly("run", "--security-opt=label=disable", "--name", ctrName, "-d", "--mount", "type=artifact,src="+artifactName+",dst=/tmp", ALPINE, "sleep", "100")

		podmanTest.PodmanExitCleanly("artifact", "rm", artifactName)

		// file must sill be readable after artifact removal
		session := podmanTest.PodmanExitCleanly("exec", ctrName, "cat", "/tmp/"+artifactFileName)
		Expect(session.OutputToString()).To(Equal("hello world"))

		// restart will fail if artifact does not exist
		session = podmanTest.Podman([]string{"restart", "-t0", ctrName})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, artifactName+":latest: artifact does not exist"))

		// create a artifact with the same name again and add another file to ensure it picks up the changes
		artifactFile2Name := "otherfile"
		artifactFile2 := filepath.Join(podmanTest.TempDir, artifactFile2Name)
		err = os.WriteFile(artifactFile2, []byte("second file"), 0o644)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("artifact", "add", artifactName, artifactFile, artifactFile2)
		podmanTest.PodmanExitCleanly("start", ctrName)

		session = podmanTest.PodmanExitCleanly("exec", ctrName, "cat", "/tmp/"+artifactFileName, "/tmp/"+artifactFile2Name)
		Expect(session.OutputToString()).To(Equal("hello world second file"))
	})

	It("podman artifact mount dest conflict", func() {
		tests := []struct {
			name  string
			mount string
		}{
			{
				name:  "bind mount --volume",
				mount: "--volume=/tmp:/test",
			},
			{
				name:  "overlay mount",
				mount: "--volume=/tmp:/test:O",
			},
			{
				name:  "named volume",
				mount: "--volume=abc:/test:O",
			},
			{
				name:  "bind mount --mount type=bind",
				mount: "--mount=type=bind,src=/tmp,dst=/test",
			},
			{
				name:  "image mount",
				mount: "--mount=type=bind,src=someimage,dst=/test",
			},
			{
				name:  "tmpfs mount",
				mount: "--tmpfs=/test",
			},
			{
				name:  "artifact mount",
				mount: "--mount=type=artifact,src=abc,dst=/test",
			},
		}

		for _, tt := range tests {
			By(tt.name)
			session := podmanTest.Podman([]string{"run", "--rm", "--mount", "type=artifact,src=someartifact,dst=/test", tt.mount, ALPINE})
			session.WaitWithDefaultTimeout()
			Expect(session).To(ExitWithError(125, "/test: duplicate mount destination"))
		}
	})
})
