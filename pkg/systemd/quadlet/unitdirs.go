package quadlet

import (
	"errors"
	"io/fs"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"go.podman.io/podman/v6/pkg/logiface"
)

// This returns whether a file has an extension recognized as a valid Quadlet unit type.
func IsExtSupported(filename string) bool {
	ext := filepath.Ext(filename)
	_, ok := SupportedExtensions[ext]
	return ok
}

// This returns default install paths for .Quadlet files for `rootless` and `root` user.
// Defaults to `/etc/containers/systemd` for root user and `$XDG_CONFIG_HOME/containers/systemd`
// for rootless users.
func GetInstallUnitDirPath(rootless bool) string {
	if rootless {
		configDir, err := os.UserConfigDir()
		if err != nil {
			logiface.Errorf("Warning: %v", err)
			return ""
		}
		return path.Join(configDir, "containers/systemd")
	}
	return UnitDirAdmin
}

// This returns the directories where we read quadlet .container and .volumes from
// For system generators these are in /usr/share/containers/systemd (for distro files)
// and /etc/containers/systemd (for sysadmin files).
// For user generators these can live in $XDG_RUNTIME_DIR/containers/systemd, /etc/containers/systemd/users, /etc/containers/systemd/users/$UID, /usr/share/containers/systemd/users/${UID}, /usr/share/containers/systemd/users/ and $XDG_CONFIG_HOME/containers/systemd
func GetUnitDirs(rootless bool, recursive bool) []string {
	paths := NewSearchPaths(recursive)

	// Allow overriding source dir, this is mainly for the CI tests
	if getDirsFromEnv(paths) {
		return paths.sorted
	}

	resolvedUnitDirAdminUser := ResolveUnitDirAdminUser()
	userLevelFilter := GetUserLevelFilter(resolvedUnitDirAdminUser)

	if rootless {
		systemUserDirLevel := len(strings.Split(resolvedUnitDirAdminUser, string(os.PathSeparator)))
		nonNumericFilter := GetNonNumericFilter(resolvedUnitDirAdminUser, systemUserDirLevel)
		getRootlessDirs(paths, nonNumericFilter, userLevelFilter)
	} else {
		getRootDirs(paths, userLevelFilter)
	}
	return paths.sorted
}

type searchPaths struct {
	sorted []string
	// map to store paths so we can quickly check if we saw them already and not loop in case of symlinks
	visitedDirs map[string]struct{}

	recursive bool
}

func NewSearchPaths(recursive bool) *searchPaths {
	return &searchPaths{
		sorted:      make([]string, 0),
		visitedDirs: make(map[string]struct{}, 0),
		recursive:   recursive,
	}
}

func (s *searchPaths) Add(path string) {
	s.sorted = append(s.sorted, path)
	s.visitedDirs[path] = struct{}{}
}

func (s *searchPaths) GetSortedPaths() []string {
	return s.sorted
}

func (s *searchPaths) Visited(path string) bool {
	_, visited := s.visitedDirs[path]
	return visited
}

func getDirsFromEnv(paths *searchPaths) bool {
	unitDirsEnv := os.Getenv("QUADLET_UNIT_DIRS")
	if len(unitDirsEnv) == 0 {
		return false
	}

	for eachUnitDir := range strings.SplitSeq(unitDirsEnv, ":") {
		if !filepath.IsAbs(eachUnitDir) {
			logiface.Errorf("%s not a valid file path", eachUnitDir)
			break
		}
		AppendSubPaths(paths, eachUnitDir, false, nil)
	}
	return true
}

func AppendSubPaths(paths *searchPaths, path string, isUserFlag bool, filterPtr func(string, bool) bool) {
	resolvedPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			logiface.Debugf("Error occurred resolving path %q: %s", path, err)
		}
		// Despite the failure add the path to the list for logging purposes
		// This is the equivalent of adding the path when info==nil below
		paths.Add(path)
		return
	}

	if skipPath(paths, resolvedPath, isUserFlag, filterPtr) {
		return
	}

	// Add the current directory
	paths.Add(resolvedPath)

	if !paths.recursive {
		return
	}

	// Read the contents of the directory
	entries, err := os.ReadDir(resolvedPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			logiface.Debugf("Error occurred walking sub directories %q: %s", path, err)
		}
		return
	}

	// Recursively run through the contents of the directory
	for _, entry := range entries {
		fullPath := filepath.Join(resolvedPath, entry.Name())
		AppendSubPaths(paths, fullPath, isUserFlag, filterPtr)
	}
}

func skipPath(paths *searchPaths, path string, isUserFlag bool, filterPtr func(string, bool) bool) bool {
	// If the path is already in the map no need to read it again
	if paths.Visited(path) {
		return true
	}

	// Don't traverse drop-in directories
	if strings.HasSuffix(path, ".d") {
		return true
	}

	// Check if the directory should be filtered out
	if filterPtr != nil && !filterPtr(path, isUserFlag) {
		return true
	}

	stat, err := os.Stat(path)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			logiface.Debugf("Error occurred resolving path %q: %s", path, err)
		}
		return true
	}

	// Not a directory nothing to add
	return !stat.IsDir()
}

func ResolveUnitDirAdminUser() string {
	unitDirAdminUser := filepath.Join(UnitDirAdmin, "users")
	var err error
	var resolvedUnitDirAdminUser string
	if resolvedUnitDirAdminUser, err = filepath.EvalSymlinks(unitDirAdminUser); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			logiface.Debugf("Error occurred resolving path %q: %s", unitDirAdminUser, err)
		}
		resolvedUnitDirAdminUser = unitDirAdminUser
	}
	return resolvedUnitDirAdminUser
}

func GetUserLevelFilter(resolvedUnitDirAdminUser string) func(string, bool) bool {
	return func(_path string, isUserFlag bool) bool {
		// if quadlet generator is run rootless, do not recurse other user sub dirs
		// if quadlet generator is run as root, ignore users sub dirs
		if strings.HasPrefix(_path, resolvedUnitDirAdminUser) {
			if isUserFlag {
				return true
			}
		} else {
			return true
		}
		return false
	}
}

func GetNonNumericFilter(resolvedUnitDirAdminUser string, systemUserDirLevel int) func(string, bool) bool {
	return func(path string, _ bool) bool {
		// when running in rootless, recursive walk directories that are non numeric
		// ignore sub dirs under the `users` directory which correspond to a user id
		if strings.HasPrefix(path, resolvedUnitDirAdminUser) {
			listDirUserPathLevels := strings.Split(path, string(os.PathSeparator))
			// Make sure to add the base directory
			if len(listDirUserPathLevels) == systemUserDirLevel {
				return true
			}
			if len(listDirUserPathLevels) > systemUserDirLevel {
				if !(regexp.MustCompile(`^[0-9]*$`).MatchString(listDirUserPathLevels[systemUserDirLevel])) {
					return true
				}
			}
		} else {
			return true
		}
		return false
	}
}

func getRootlessDirs(paths *searchPaths, nonNumericFilter, userLevelFilter func(string, bool) bool) {
	runtimeDir, found := os.LookupEnv("XDG_RUNTIME_DIR")
	if found {
		AppendSubPaths(paths, path.Join(runtimeDir, "containers/systemd"), false, nil)
	}

	configDir, err := os.UserConfigDir()
	if err != nil {
		logiface.Errorf("Warning: %v", err)
		return
	}
	AppendSubPaths(paths, path.Join(configDir, "containers/systemd"), false, nil)

	basePaths := []string{UnitDirAdmin, UnitDirDistro}
	u, err := user.Current()
	if err == nil {
		for _, basePath := range basePaths {
			AppendSubPaths(paths, filepath.Join(basePath, "users"), true, nonNumericFilter)
			AppendSubPaths(paths, filepath.Join(basePath, "users", u.Uid), true, userLevelFilter)
		}
	} else {
		logiface.Errorf("Warning: %v", err)
		// Add the base directories even if the UID was not found
		for _, basePath := range basePaths {
			paths.Add(filepath.Join(basePath, "users"))
		}
	}
}

func getRootDirs(paths *searchPaths, userLevelFilter func(string, bool) bool) {
	AppendSubPaths(paths, UnitDirTemp, false, userLevelFilter)
	AppendSubPaths(paths, UnitDirAdmin, false, userLevelFilter)
	AppendSubPaths(paths, UnitDirDistro, false, userLevelFilter)
}
