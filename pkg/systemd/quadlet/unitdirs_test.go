//go:build linux

package quadlet

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnitDirs(t *testing.T) {
	u, err := user.Current()
	assert.NoError(t, err)
	uidInt, err := strconv.Atoi(u.Uid)
	assert.NoError(t, err)

	if os.Getenv("_UNSHARED") != "true" {
		unitDirs := GetUnitDirs(false, true)

		resolvedUnitDirAdminUser := ResolveUnitDirAdminUser()
		userLevelFilter := GetUserLevelFilter(resolvedUnitDirAdminUser)
		rootfulPaths := NewSearchPaths(true)
		AppendSubPaths(rootfulPaths, UnitDirTemp, false, userLevelFilter)
		AppendSubPaths(rootfulPaths, UnitDirAdmin, false, userLevelFilter)
		AppendSubPaths(rootfulPaths, UnitDirDistro, false, userLevelFilter)
		assert.Equal(t, rootfulPaths.GetSortedPaths(), unitDirs, "rootful unit dirs should match")

		configDir, err := os.UserConfigDir()
		assert.NoError(t, err)

		rootlessPaths := NewSearchPaths(true)

		systemUserDirLevel := len(strings.Split(resolvedUnitDirAdminUser, string(os.PathSeparator)))
		nonNumericFilter := GetNonNumericFilter(resolvedUnitDirAdminUser, systemUserDirLevel)

		runtimeDir, found := os.LookupEnv("XDG_RUNTIME_DIR")
		if found {
			AppendSubPaths(rootlessPaths, path.Join(runtimeDir, "containers/systemd"), false, nil)
		}
		AppendSubPaths(rootlessPaths, path.Join(configDir, "containers/systemd"), false, nil)
		AppendSubPaths(rootlessPaths, filepath.Join(UnitDirAdmin, "users"), true, nonNumericFilter)
		AppendSubPaths(rootlessPaths, filepath.Join(UnitDirAdmin, "users", u.Uid), true, userLevelFilter)
		AppendSubPaths(rootlessPaths, filepath.Join(UnitDirDistro, "users"), true, nonNumericFilter)
		AppendSubPaths(rootlessPaths, filepath.Join(UnitDirDistro, "users", u.Uid), true, userLevelFilter)

		unitDirs = GetUnitDirs(true, true)
		assert.Equal(t, rootlessPaths.GetSortedPaths(), unitDirs, "rootless unit dirs should match")

		// Test that relative path returns an empty list
		t.Setenv("QUADLET_UNIT_DIRS", "./relative/path")
		unitDirs = GetUnitDirs(false, true)
		assert.Equal(t, []string{}, unitDirs)

		name := t.TempDir()
		t.Setenv("QUADLET_UNIT_DIRS", name)
		unitDirs = GetUnitDirs(false, true)
		assert.Equal(t, []string{name}, unitDirs, "rootful should use environment variable")

		unitDirs = GetUnitDirs(true, true)
		assert.Equal(t, []string{name}, unitDirs, "rootless should use environment variable")

		symLinkTestBaseDir := t.TempDir()

		actualDir := filepath.Join(symLinkTestBaseDir, "actual")
		err = os.Mkdir(actualDir, 0o755)
		assert.NoError(t, err)
		innerDir := filepath.Join(actualDir, "inner")
		err = os.Mkdir(innerDir, 0o755)
		assert.NoError(t, err)
		symlink := filepath.Join(symLinkTestBaseDir, "symlink")
		err = os.Symlink(actualDir, symlink)
		assert.NoError(t, err)
		t.Setenv("QUADLET_UNIT_DIRS", symlink)
		unitDirs = GetUnitDirs(true, true)
		assert.Equal(t, []string{actualDir, innerDir}, unitDirs, "directory resolution should follow symlink")

		// Make a more elborate test with the following structure:
		// <BASE>/linkToDir - real directory to link to
		// <BASE>/linkToDir/a - real directory
		// <BASE>/linkToDir/b - link to <BASE>/unitDir/b/a should be ignored
		// <BASE>/linkToDir/c - link to <BASE>/unitDir should be ignored
		// <BASE>/unitDir - start from here
		// <BASE>/unitDir/a - real directory
		// <BASE>/unitDir/a/a - real directory
		// <BASE>/unitDir/a/a/a - real directory
		// <BASE>/unitDir/b/a - real directory
		// <BASE>/unitDir/b/b - link to <BASE>/unitDir/a/a should be ignored
		// <BASE>/unitDir/c - link to <BASE>/linkToDir
		createDir := func(path, name string, dirs []string) (string, []string) {
			dirName := filepath.Join(path, name)
			assert.NotContains(t, dirs, dirName)
			err = os.Mkdir(dirName, 0o755)
			assert.NoError(t, err)
			dirs = append(dirs, dirName)
			return dirName, dirs
		}

		linkDir := func(path, name, target string) {
			linkName := filepath.Join(path, name)
			err = os.Symlink(target, linkName)
			assert.NoError(t, err)
		}

		symLinkRecursiveTestBaseDir := t.TempDir()

		expectedDirs := make([]string, 0)
		// Create <BASE>/unitDir
		unitsDirPath, expectedDirs := createDir(symLinkRecursiveTestBaseDir, "unitsDir", expectedDirs)
		// Create <BASE>/unitDir/a
		aDirPath, expectedDirs := createDir(unitsDirPath, "a", expectedDirs)
		// Create <BASE>/unitDir/a/a
		aaDirPath, expectedDirs := createDir(aDirPath, "a", expectedDirs)
		// Create <BASE>/unitDir/a/a/a
		_, expectedDirs = createDir(aaDirPath, "a", expectedDirs)
		// Create <BASE>/unitDir/a/b
		_, expectedDirs = createDir(aDirPath, "b", expectedDirs)
		// Create <BASE>/unitDir/b
		bDirPath, expectedDirs := createDir(unitsDirPath, "b", expectedDirs)
		// Create <BASE>/unitDir/b/a
		baDirPath, expectedDirs := createDir(bDirPath, "a", expectedDirs)
		// Create <BASE>/linkToDir
		linkToDirPath, expectedDirs := createDir(symLinkRecursiveTestBaseDir, "linkToDir", expectedDirs)
		// Create <BASE>/linkToDir/a
		_, expectedDirs = createDir(linkToDirPath, "a", expectedDirs)

		// Link <BASE>/unitDir/b/b to <BASE>/unitDir/a/a
		linkDir(bDirPath, "b", aaDirPath)
		// Link <BASE>/linkToDir/b to <BASE>/unitDir/b/a
		linkDir(linkToDirPath, "b", baDirPath)
		// Link <BASE>/linkToDir/c to <BASE>/unitDir
		linkDir(linkToDirPath, "c", unitsDirPath)
		// Link <BASE>/unitDir/c to <BASE>/linkToDir
		linkDir(unitsDirPath, "c", linkToDirPath)

		t.Setenv("QUADLET_UNIT_DIRS", unitsDirPath)
		unitDirs = GetUnitDirs(true, true)
		assert.Equal(t, expectedDirs, unitDirs, "directory resolution should follow symlink")
		// remove the temporary directory at the end of the program
		defer os.RemoveAll(symLinkTestBaseDir)

		// because chroot is only available for root,
		// unshare the namespace and map user to root
		c := exec.Command("/proc/self/exe", os.Args[1:]...)
		c.Stdin = os.Stdin
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		c.SysProcAttr = &syscall.SysProcAttr{
			Cloneflags: syscall.CLONE_NEWUSER,
			UidMappings: []syscall.SysProcIDMap{
				{
					ContainerID: 0,
					HostID:      uidInt,
					Size:        1,
				},
			},
		}
		c.Env = append(os.Environ(), "_UNSHARED=true")
		err = c.Run()
		assert.NoError(t, err)
	} else {
		fmt.Println(os.Args)

		symLinkTestBaseDir := t.TempDir()
		rootF, err := os.Open("/")
		assert.NoError(t, err)
		defer rootF.Close()
		defer func() {
			err := rootF.Chdir() //nolint:usetesting // we switch in a defer and don't want to switch back
			assert.NoError(t, err)
			err = syscall.Chroot(".")
			assert.NoError(t, err)
		}()
		err = syscall.Chroot(symLinkTestBaseDir)
		assert.NoError(t, err)

		for _, v := range []string{UnitDirAdmin, UnitDirDistro} {
			err = os.MkdirAll(v, 0o755)
			assert.NoError(t, err)
			err = os.RemoveAll(v)
			assert.NoError(t, err)
		}

		createDir := func(path, name string) string {
			dirName := filepath.Join(path, name)
			err = os.Mkdir(dirName, 0o755)
			assert.NoError(t, err)
			return dirName
		}

		linkDir := func(path, name, target string) {
			linkName := filepath.Join(path, name)
			err = os.Symlink(target, linkName)
			assert.NoError(t, err)
		}

		systemdDir := createDir("/", "systemd")
		userDir := createDir("/", "users")
		linkDir(systemdDir, "users", userDir)
		linkDir(UnitDirAdmin, "", systemdDir)

		uidDir := createDir(userDir, u.Uid)
		uidDir2 := createDir(userDir, strconv.Itoa(uidInt+1))
		userInternalDir := createDir(userDir, "internal")

		distroSystemdDir := createDir("/", "systemd2")
		distroUserDir := createDir("/", "users2")
		linkDir(distroSystemdDir, "users", distroUserDir)
		linkDir(UnitDirDistro, "", distroSystemdDir)

		distroUidDir := createDir(distroUserDir, u.Uid)
		distroUidDir2 := createDir(distroUserDir, strconv.Itoa(uidInt+1))
		distroInternalDir := createDir(distroUserDir, "internal")

		// Make sure QUADLET_UNIT_DIRS is not set
		t.Setenv("QUADLET_UNIT_DIRS", "")
		// Test Rootful
		unitDirs := GetUnitDirs(false, true)
		assert.NotContains(t, unitDirs, userDir, "rootful should not contain rootless")
		assert.NotContains(t, unitDirs, userInternalDir, "rootful should not contain rootless")
		assert.NotContains(t, unitDirs, distroUserDir, "rootful should not contain distro rootless")
		assert.NotContains(t, unitDirs, distroInternalDir, "rootful should not contain distro rootless")

		// Test Rootless
		unitDirs = GetUnitDirs(true, true)
		assert.NotContains(t, unitDirs, uidDir2, "rootless should not contain other users'")
		assert.Contains(t, unitDirs, userInternalDir, "rootless should contain sub-directories of users dir")
		assert.Contains(t, unitDirs, uidDir, "rootless should contain the directory for its UID")
		assert.NotContains(t, unitDirs, distroUidDir2, "rootless should not contain other distro users' dirs")
		assert.Contains(t, unitDirs, distroInternalDir, "rootless should contain sub-directories of distro users dir")
		assert.Contains(t, unitDirs, distroUidDir, "rootless should contain the distro directory for its UID")
	}
}
