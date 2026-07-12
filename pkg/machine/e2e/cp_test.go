package e2e_test

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("run cp commands", func() {
	It("podman cp", func() {
		const (
			file            = "foo.txt"
			directory       = "foo-dir"
			fileInDirectory = "bar.txt"
			stdinFile       = "file.txt"
			stdinDirectory  = "stdin-dir"
			containerName   = "podman-cp-test"
		)

		sourceDir := GinkgoT().TempDir()
		destinationDir := GinkgoT().TempDir()

		f, err := os.Create(filepath.Join(sourceDir, file))
		Expect(err).ToNot(HaveOccurred())
		err = f.Close()
		Expect(err).ToNot(HaveOccurred())

		// Get the file stat to check permissions later
		sourceFileStat, err := os.Stat(filepath.Join(sourceDir, file))
		Expect(err).ToNot(HaveOccurred())

		err = os.MkdirAll(filepath.Join(sourceDir, directory), 0o755)
		Expect(err).ToNot(HaveOccurred())

		// Get the directory stat to check permissions later
		sourceDirStat, err := os.Stat(filepath.Join(sourceDir, directory))
		Expect(err).ToNot(HaveOccurred())

		f, err = os.Create(filepath.Join(sourceDir, directory, fileInDirectory))
		Expect(err).ToNot(HaveOccurred())
		err = f.Close()
		Expect(err).ToNot(HaveOccurred())

		// Get the file in directory stat to check permissions later
		sourceFileInDirStat, err := os.Stat(filepath.Join(sourceDir, directory, fileInDirectory))
		Expect(err).ToNot(HaveOccurred())

		name := randomString()
		i := new(initMachine)
		session, err := mb.setName(name).setCmd(i.withImage(mb.imagePath).withNow()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		bm := basicMachine{}
		newImgs, err := mb.setCmd(bm.withPodmanCommand([]string{"pull", TESTIMAGE})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(newImgs).To(Exit(0))
		Expect(newImgs.outputToStringSlice()).To(HaveLen(1))

		createAlp, err := mb.setCmd(bm.withPodmanCommand([]string{"create", "--name", containerName, TESTIMAGE, "top"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(createAlp).To(Exit(0))
		Expect(createAlp.outputToStringSlice()).To(HaveLen(1))

		containerID := createAlp.outputToStringSlice()[0]

		// Create a second container to test copying between containers
		// This container is named "C" to also test that Windows prefers
		// to treat C:\ as a local file path instead of a container name
		createAlp, err = mb.setCmd(bm.withPodmanCommand([]string{"create", "--name", "C", TESTIMAGE, "top"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(createAlp).To(Exit(0))
		Expect(createAlp.outputToStringSlice()).To(HaveLen(1))

		destinationContainerID := createAlp.outputToStringSlice()[0]

		By("copy from host to container by id")
		// Copy a single file into the container
		cpFile, err := mb.setCmd(bm.withPodmanCommand([]string{"cp", filepath.Join(sourceDir, file), containerID + ":/tmp/"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(cpFile).To(Exit(0))

		// Copy a directory into the container
		cpDir, err := mb.setCmd(bm.withPodmanCommand([]string{"cp", filepath.Join(sourceDir, directory), containerID + ":/tmp"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(cpDir).To(Exit(0))

		start, err := mb.setCmd(bm.withPodmanCommand([]string{"start", containerID})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(start).To(Exit(0))

		// Check the single file is created with the appropriate mode, uid, gid
		exec, err := mb.setCmd(bm.withPodmanCommand([]string{"exec", containerID, "stat", "-c", "%a %u %g", path.Join("/tmp", file)})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(exec).To(Exit(0))
		Expect(exec.outputToString()).To(Equal(fmt.Sprintf("%o %d %d", sourceFileStat.Mode().Perm(), 0, 0)))

		// Check the directory is created with the appropriate mode, uid, gid
		exec, err = mb.setCmd(bm.withPodmanCommand([]string{"exec", containerID, "stat", "-c", "%a %u %g", path.Join("/tmp", directory)})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(exec).To(Exit(0))
		Expect(exec.outputToString()).To(Equal(fmt.Sprintf("%o %d %d", sourceDirStat.Mode().Perm(), 0, 0)))

		// Check the file in the directory is created with the appropriate mode, uid, gid
		exec, err = mb.setCmd(bm.withPodmanCommand([]string{"exec", containerID, "stat", "-c", "%a %u %g", path.Join("/tmp", directory, fileInDirectory)})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(exec).To(Exit(0))
		Expect(exec.outputToString()).To(Equal(fmt.Sprintf("%o %d %d", sourceFileInDirStat.Mode().Perm(), 0, 0)))

		By("copy from host to container by name")
		// Copy a single renamed file into the container
		cpFile, err = mb.setCmd(bm.withPodmanCommand([]string{"cp", filepath.Join(sourceDir, file), containerName + ":/tmp/rename.txt"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(cpFile).To(Exit(0))

		// Check the single file is created with the appropriate mode, uid, gid
		exec, err = mb.setCmd(bm.withPodmanCommand([]string{"exec", containerID, "stat", "-c", "%a %u %g", "/tmp/rename.txt"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(exec).To(Exit(0))
		Expect(exec.outputToString()).To(Equal(fmt.Sprintf("%o %d %d", sourceFileStat.Mode().Perm(), 0, 0)))

		By("copy from container to host")
		// Copy the file back from the container to the host
		cpFile, err = mb.setCmd(bm.withPodmanCommand([]string{"cp", containerID + ":" + path.Join("/tmp", file), destinationDir + string(os.PathSeparator)})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(cpFile).To(Exit(0))

		// Get the file stat of the copied file to compare against the original
		destinationFileStat, err := os.Stat(filepath.Join(destinationDir, file))
		Expect(err).ToNot(HaveOccurred())
		Expect(destinationFileStat.Mode()).To(Equal(sourceFileStat.Mode()))
		// Compare the modification time of the file in the container and the host (with second level precision)
		Expect(destinationFileStat.ModTime()).To(BeTemporally("~", sourceFileStat.ModTime(), time.Second))

		// Copy a directory back from the container to the host
		cpDir, err = mb.setCmd(bm.withPodmanCommand([]string{"cp", containerID + ":" + path.Join("/tmp", directory), destinationDir + string(os.PathSeparator)})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(cpDir).To(Exit(0))

		// Get the stat of the copied directory to compare against the original
		destinationDirStat, err := os.Stat(filepath.Join(destinationDir, directory))
		Expect(err).ToNot(HaveOccurred())
		Expect(destinationDirStat.Mode()).To(Equal(sourceDirStat.Mode()))
		// Compare the modification time of the folder in the container and the host (with second level precision)
		Expect(destinationDirStat.ModTime()).To(BeTemporally("~", sourceDirStat.ModTime(), time.Second))

		// Get the stat of the copied file in the directory to compare against the original
		destinationFileInDirStat, err := os.Stat(filepath.Join(sourceDir, directory, fileInDirectory))
		Expect(err).ToNot(HaveOccurred())
		Expect(destinationFileInDirStat.Mode()).To(Equal(sourceFileInDirStat.Mode()))
		// Compare the modification time of the file in the container and the host (with second level precision)
		Expect(destinationFileInDirStat.ModTime()).To(BeTemporally("~", sourceFileInDirStat.ModTime(), time.Second))

		By("copy stdin to container")
		now := time.Now()
		tarBuffer := &bytes.Buffer{}
		tw := tar.NewWriter(tarBuffer)

		// Write a directory header to the tar
		err = tw.WriteHeader(&tar.Header{
			Name:       stdinDirectory,
			Mode:       int64(0o640 | fs.ModeDir),
			Gid:        1000,
			ModTime:    now,
			ChangeTime: now,
			AccessTime: now,
			Typeflag:   tar.TypeDir,
		})
		Expect(err).ToNot(HaveOccurred())

		// Write a file header to the tar
		err = tw.WriteHeader(&tar.Header{
			Name:       path.Join(stdinDirectory, stdinFile),
			Mode:       0o755,
			Uid:        1000,
			ModTime:    now,
			ChangeTime: now,
			AccessTime: now,
		})
		Expect(err).ToNot(HaveOccurred())

		err = tw.Close()
		Expect(err).ToNot(HaveOccurred())

		// Testing stdin copy with archive mode disabled (ownership will be determined by the tar file)
		cpTar, err := mb.setCmd(bm.withPodmanCommand([]string{"cp", "-a=false", "-", containerID + ":/tmp"})).setStdin(tarBuffer).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(cpTar).To(Exit(0))

		// Check the directory is created with the appropriate mode, uid, gid
		exec, err = mb.setCmd(bm.withPodmanCommand([]string{"exec", containerID, "stat", "-c", "%a %u %g", "/tmp/stdin-dir"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(exec).To(Exit(0))
		Expect(exec.outputToString()).To(Equal("640 0 1000"))

		// Check the file is created with the appropriate mode, uid, gid
		exec, err = mb.setCmd(bm.withPodmanCommand([]string{"exec", containerID, "stat", "-c", "%a %u %g", "/tmp/stdin-dir/file.txt"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(exec).To(Exit(0))
		Expect(exec.outputToString()).To(Equal("755 1000 0"))

		By("copy from container to container")
		// Copy the file from the first container to the second container (with renaming)
		cpFile, err = mb.setCmd(bm.withPodmanCommand([]string{"cp", containerID + ":" + path.Join("/tmp", file), destinationContainerID + ":" + path.Join("/tmp", "destination.txt")})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(cpFile).To(Exit(0))

		start, err = mb.setCmd(bm.withPodmanCommand([]string{"start", destinationContainerID})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(start).To(Exit(0))

		// Check the single file is created with the appropriate mode, uid, gid
		exec, err = mb.setCmd(bm.withPodmanCommand([]string{"exec", destinationContainerID, "stat", "-c", "%a %u %g", path.Join("/tmp", "destination.txt")})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(exec).To(Exit(0))
		Expect(exec.outputToString()).To(Equal(fmt.Sprintf("%o %d %d", sourceFileStat.Mode().Perm(), 0, 0)))
	})

	It("podman machine cp", func() {
		// HOST FILE SYSTEM
		// ~/<ginkgo_tmp>
		//   * foo.txt
		//   - foo-dir
		//     * bar.txt
		//
		//   * guest-foo.txt
		//   - guest-foo-dir
		//     * bar.txt

		// GUEST FILE SYSTEM
		// ~/
		//   * foo.txt
		//   - foo-dir
		//     * bar.txt

		var (
			file     = "foo.txt"
			filePath = filepath.Join(GinkgoT().TempDir(), file)

			directory     = "foo-dir"
			directoryPath = filepath.Join(GinkgoT().TempDir(), directory)

			fileInDirectory     = "bar.txt"
			fileInDirectoryPath = filepath.Join(directoryPath, fileInDirectory)

			guestToHostFile = "guest-foo.txt"
			guestToHostDir  = "guest-foo-dir"
		)

		f, err := os.Create(filePath)
		Expect(err).ToNot(HaveOccurred())
		err = f.Close()
		Expect(err).ToNot(HaveOccurred())

		err = os.MkdirAll(directoryPath, 0o755)
		Expect(err).ToNot(HaveOccurred())

		f, err = os.Create(fileInDirectoryPath)
		Expect(err).ToNot(HaveOccurred())
		err = f.Close()
		Expect(err).ToNot(HaveOccurred())

		name := randomString()
		initMachine := initMachine{}
		cp := cpMachine{}
		sshMachine := sshMachine{}

		By("host file to guest")
		session, err := mb.setName(name).setCmd(initMachine.withImage(mb.imagePath).withNow()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		// copy the file into the guest
		session, err = mb.setCmd(cp.withQuiet().withSrc(filePath).withDest(name + ":~/" + file)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		// verify the guest has the file
		session, err = mb.setName(name).setCmd(sshMachine.withSSHCommand([]string{"ls"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))
		Expect(session.outputToString()).To(Equal(file))

		// try to copy the file to a location in the guest where permission will get denied
		session, err = mb.setCmd(cp.withQuiet().withSrc(filePath).withDest(name + ":/etc/tmp.txt")).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(125))
		Expect(session.errorToString()).To(ContainSubstring("scp: dest open \"/etc/tmp.txt\": Permission denied"))

		By("host directory to guest")
		// copy contents into the guest
		session, err = mb.setCmd(cp.withQuiet().withSrc(directoryPath).withDest(name + ":~/" + directory)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		// verify the content is in the guest
		session, err = mb.setName(name).setCmd(sshMachine.withSSHCommand([]string{"ls", directory})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))
		Expect(session.outputToString()).To(Equal(fileInDirectory))

		By("guest file to host")
		// copy contents to the host
		guestToHostFilePath := filepath.Join(GinkgoT().TempDir(), guestToHostFile)
		session, err = mb.setCmd(cp.withQuiet().withSrc(name + ":~/" + file).withDest(guestToHostFilePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))
		// check the contents are on the host
		Expect(guestToHostFilePath).To(BeARegularFile())

		// this test can only be run on Unix systems. Windows does not
		// strictly enforce read-only permissions specified by `os.MkdirAll()`,
		// so we cannot easily make a read-only directory.
		if runtime.GOOS != "windows" {
			// try to copy the file to a location on the host where permission will get denied
			hostDirPath := filepath.Join(GinkgoT().TempDir(), "test-guest-copy-dir")
			err = os.MkdirAll(hostDirPath, 0o444)
			Expect(err).ToNot(HaveOccurred())
			hostFileInDirPath := filepath.Join(hostDirPath, file)
			session, err = mb.setCmd(cp.withQuiet().withSrc(name + ":~/" + file).withDest(hostFileInDirPath)).run()
			Expect(err).ToNot(HaveOccurred())
			Expect(session).To(Exit(125))
			Expect(session.errorToString()).To(ContainSubstring(fmt.Sprintf("scp: open local \"%s\": Permission denied", hostFileInDirPath)))
		}

		By("guest directory to host")
		// copy contents to the host
		guestToHostDirPath := filepath.Join(GinkgoT().TempDir(), guestToHostDir)
		session, err = mb.setCmd(cp.withQuiet().withSrc(name + ":~/" + directory).withDest(guestToHostDirPath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))
		// check the contents are on the host
		Expect(filepath.Join(guestToHostDirPath, fileInDirectory)).To(BeARegularFile())

		By("attempt copying file to a new directory")
		// copy the file to a guest directory
		session, err = mb.setCmd(cp.withQuiet().withSrc(filePath).withDest(name + ":~/directory/")).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(125))
		Expect(session.errorToString()).To(ContainSubstring("scp: dest open \"directory/\": Failure"))

		// try copying a guest file to a host directory
		hostDirPath := filepath.Join(GinkgoT().TempDir(), "directory") + string(filepath.Separator)
		session, err = mb.setCmd(cp.withQuiet().withSrc(name + ":~/" + file).withDest(hostDirPath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(125))
		switch runtime.GOOS {
		case "windows":
			hostDirPath = filepath.ToSlash(hostDirPath)
			Expect(session.errorToString()).To(Or(ContainSubstring(fmt.Sprintf("scp: open local \"%s\": No such file or directory", hostDirPath)),
				ContainSubstring(fmt.Sprintf("scp: open local \"%s\": Unknown error", hostDirPath))))
		case "darwin":
			Expect(session.errorToString()).To(ContainSubstring(fmt.Sprintf("scp: open local \"%s\": No such file or directory", hostDirPath)))
		case "linux":
			Expect(session.errorToString()).To(ContainSubstring(fmt.Sprintf("scp: open local \"%s\": Is a directory", hostDirPath)))
		}

		By("attempt copying directory to a file")
		// try copying a local directory to a guest file
		session, err = mb.setCmd(cp.withQuiet().withSrc(GinkgoT().TempDir() + string(filepath.Separator)).withDest(name + ":~/" + directory + "/" + fileInDirectory + "/")).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(125))
		Expect(session.errorToString()).To(ContainSubstring("/foo-dir/bar.txt\" exists but is not a directory"))

		// try copying the guest directory to a local file
		session, err = mb.setCmd(cp.withQuiet().withSrc(name + ":~/" + directory + "/").withDest(filePath + string(filepath.Separator))).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(125))

		hostLocalFilePath := filePath + string(filepath.Separator)
		switch runtime.GOOS {
		case "windows":
			hostLocalFilePath = filepath.ToSlash(hostLocalFilePath)
			Expect(session.errorToString()).To(ContainSubstring(fmt.Sprintf("scp: open local \"%s\": No such file or directory", hostLocalFilePath+fileInDirectory)))
		case "darwin":
			Expect(session.errorToString()).To(ContainSubstring(fmt.Sprintf("scp: mkdir %s: Not a directory", hostLocalFilePath)))
		case "linux":
			Expect(session.errorToString()).To(ContainSubstring(fmt.Sprintf("scp: open local \"%s\": Not a directory", hostLocalFilePath+fileInDirectory)))
		}
	})
})
