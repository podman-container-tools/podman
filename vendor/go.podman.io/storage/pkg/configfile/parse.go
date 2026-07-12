package configfile

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"iter"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/sirupsen/logrus"
)

const _configPathName = "containers"

var (
	// systemConfigPath is the location for the default config files shipped by the distro/vendor.
	//
	// This can be overridden at build time with the following go linker flag:
	// -ldflags '-X go.podman.io/storage/pkg/configfile.systemConfigPath=$your_path'
	systemConfigPath = builtinSystemConfigPath

	// adminOverrideConfigPath is the location for admin local override config files.
	//
	// This can be overridden at build time with the following go linker flag:
	// -ldflags '-X go.podman.io/storage/pkg/configfile.adminOverrideConfigPath=$your_path'
	adminOverrideConfigPath = getAdminOverrideConfigPath()

	// ErrConfigFileNotFound is returned when ErrorIfNotFound is true and no config
	// file could be loaded.
	ErrConfigFileNotFound = errors.New("config file not found")
)

type File struct {
	// The name of the config file WITHOUT the extension (i.e. no .conf).
	// Must not be empty and must not contain the path separator.
	Name string

	// Extension is the file extension of the config file, i.e. "conf" or "yaml".
	// Must not be empty and must not contain the path separator.
	Extension string

	// EnvironmentName is the name of environment variable that can be set to specify the override.
	// If EnvironmentName is set, the variable with _OVERRIDE suffix is also checked for an override
	// unless DoNotLoadDropInFiles is set.
	// Optional.
	EnvironmentName string

	// RootForImplicitAbsolutePaths is the path to an alternate root
	// If not "", prefixed to any absolute paths used by default in the package.
	// NOTE: This does NOT affect paths starting by $HOME or environment variables paths.
	RootForImplicitAbsolutePaths string

	// CustomConfigFilePath is the path to a specific file that will be parsed as main file instead
	// of the default location files. Unlike the regular parsing logic if set this file must exists
	// or ErrNotExist will be returned. Note when just using this option without also
	// CustomConfigFileDropInDirectory it means the regular drop in directories are still searched
	// assuming DoNotLoadDropInFiles is not set.
	// This has higher priority over the EnvironmentName variable, so if set the env is ignored.
	// RootForImplicitAbsolutePaths will not be used for this path.
	// Optional.
	CustomConfigFilePath string

	// CustomConfigFileDropInDirectory is the path to a specific drop in directory that will be searched
	// instead of the default location. Note when just using this option without also
	// CustomConfigFilePath it means the regular main file location is still being read assuming
	// DoNotLoadMainFiles is not set.
	// This has higher priority over the EnvironmentName + "_OVERRIDE" variable, so if set the env is ignored.
	// RootForImplicitAbsolutePaths will not be used for this path.
	// Optional.
	CustomConfigFileDropInDirectory string

	// DoNotLoadMainFiles should be set if only the Drop In files should be loaded.
	DoNotLoadMainFiles bool

	// DoNotLoadDropInFiles should be set if only the main files should be loaded.
	// If DoNotLoadDropInFiles is set, the _OVERRIDE environment variable is ignored.
	DoNotLoadDropInFiles bool

	// DoNotUseExtensionForConfigName makes it so that the extension is only consulted for the drop in
	// file names but not the main config file name search path.
	DoNotUseExtensionForConfigName bool

	// UserId is the id of the user running this. Used to know where to search in the
	// different "rootful" and "rootless" drop in lookup paths.
	UserId int

	// Modules is a list of names of full paths which are loaded after all the other files.
	// Note the modules concept exists only for containers.conf.
	// For compatibility reasons this field is written to with the fully resolved paths
	// of each module as this is what podman expects today.
	Modules []string

	// ErrorIfNotFound is true if an error should be returned if no file is found.
	ErrorIfNotFound bool
}

// Item is a single config file that is being read once at a time and returned by the iterator from [Read].
type Item struct {
	// Reader is the reader from the file content. The Reader is only valid during
	Reader io.Reader
	// Name is the full filepath to the filename being read.
	Name string
}

type SearchPaths struct {
	// MainFiles are the main config file paths, ordered from highest priority to lower ones.
	// For example: $HOME/..., then /etc/..., then /usr/...
	// Can be empty if there are no main files for the given config.
	MainFiles []string
	// DropInDirectories is the list of drop in directories read by this config file, again
	// ordered from highest priority to lower ones.
	// Can be empty if there are no drop in directories for the given config.
	DropInDirectories []string
	// ModuleDirectories is the list of module directories checked by this config file, again
	// ordered from highest priority to lower ones.
	// Will be empty if no modules were request for the given conf.
	ModuleDirectories []string
	// The file path from conf.EnvironmentName + "_OVERRIDE" env if it must be parsed for the given config.
	// Can be empty.
	ExtraOverrideFile string
}

func (f *File) getConfName() string {
	if f.DoNotUseExtensionForConfigName {
		return f.Name
	}
	return f.Name + "." + f.Extension
}

// GetSearchPaths returns the list of files which will be tried to be parsed.
// See the doc of [SearchPaths] for more information.
func GetSearchPaths(conf *File) (SearchPaths, error) {
	paths, _, err := getSearchPaths(conf)
	return paths, err
}

func getSearchPaths(conf *File) (SearchPaths, bool, error) {
	configFileName := conf.getConfName()

	// Note this can be empty which is a valid case and should be simply ignored then.
	defaultConfig := systemConfigPath
	if defaultConfig != "" {
		defaultConfig = filepath.Join(defaultConfig, configFileName)
		if conf.RootForImplicitAbsolutePaths != "" {
			defaultConfig = filepath.Join(conf.RootForImplicitAbsolutePaths, defaultConfig)
		}
	}

	// Same here this can be empty.
	overrideConfig := adminOverrideConfigPath
	if overrideConfig != "" {
		overrideConfig = filepath.Join(overrideConfig, configFileName)
		if conf.RootForImplicitAbsolutePaths != "" {
			overrideConfig = filepath.Join(conf.RootForImplicitAbsolutePaths, overrideConfig)
		}
	}

	// userConfig can be empty as well
	userConfig, err := UserConfigPath()
	if err != nil {
		return SearchPaths{}, false, err
	}
	if userConfig != "" {
		userConfig = filepath.Join(userConfig, configFileName)
	}

	// main files
	ignoreENOENT := true
	shouldLoadDropIns := true
	var mainFiles []string
	if !conf.DoNotLoadMainFiles {
		if conf.CustomConfigFilePath != "" {
			mainFiles = append(mainFiles, conf.CustomConfigFilePath)
			ignoreENOENT = false
			// Only consider the env if no custom path was explicitly set.
			// As this path often comes from cli options it is important it wins over the env value.
		} else if path := os.Getenv(conf.EnvironmentName); path != "" && conf.EnvironmentName != "" {
			mainFiles = append(mainFiles, path)
			ignoreENOENT = false
			// Also when the env is set skip the loading of drop in files, modules and _OVERRIDE env are still read though.
			shouldLoadDropIns = false
		} else {
			// default search paths
			if userConfig != "" {
				mainFiles = append(mainFiles, userConfig)
			}
			if overrideConfig != "" {
				mainFiles = append(mainFiles, overrideConfig)
			}
			if defaultConfig != "" {
				mainFiles = append(mainFiles, defaultConfig)
			}
		}
	}

	// drop in dirs
	var dropInDirs []string
	var extraOverrideFilePath string
	if !conf.DoNotLoadDropInFiles {
		if shouldLoadDropIns {
			if conf.CustomConfigFileDropInDirectory != "" {
				dropInDirs = append(dropInDirs, conf.CustomConfigFileDropInDirectory)
			} else {
				// default search paths
				dropInDirs = getDropInPaths(defaultConfig, overrideConfig, userConfig, "."+conf.Extension, conf.UserId)
			}
		}

		if conf.EnvironmentName != "" && conf.CustomConfigFileDropInDirectory == "" {
			if path := os.Getenv(conf.EnvironmentName + "_OVERRIDE"); path != "" {
				extraOverrideFilePath = path
			}
		}
	}

	// modules
	var modDirs []string
	if len(conf.Modules) > 0 {
		modDirs = moduleDirectories(defaultConfig, overrideConfig, userConfig)
	}

	return SearchPaths{
			MainFiles:         mainFiles,
			DropInDirectories: dropInDirs,
			ModuleDirectories: modDirs,
			ExtraOverrideFile: extraOverrideFilePath,
		},
		ignoreENOENT,
		nil
}

// Read parses all config files with the specified options and returns an iterator which returns all files as Item in the right order.
// If an error is returned by the iterator then this must be treated as fatal error and must fail the config file parsing.
// Expected ENOENT errors are already ignored in this function and must not be handled again by callers.
// The given File options must not be nil and populated with valid options.
func Read(conf *File) iter.Seq2[*Item, error] {
	return func(yield func(*Item, error) bool) {
		paths, ignoreMainENOENT, err := getSearchPaths(conf)
		if err != nil {
			yield(nil, err)
			return
		}

		usedPaths := make([]string, 0, 8)
		foundAny := false

		yieldAndClose := func(f *os.File) bool {
			foundAny = true
			ok := yield(&Item{
				Reader: f,
				Name:   f.Name(),
			}, nil)
			// Once yield returns always close the file as the consumer should be done with it.
			if err := f.Close(); err != nil {
				if ok {
					// don't yield again if the previous yield returned false
					yield(nil, err)
				}
				return false
			}
			return ok
		}

		for _, path := range paths.MainFiles {
			if path == "" {
				continue
			}
			usedPaths = append(usedPaths, path)
			f, err := os.Open(path)
			if err != nil {
				// only ignore ErrNotExist when needed, all other errors get return to the caller via yield
				if ignoreMainENOENT && errors.Is(err, fs.ErrNotExist) {
					continue
				}
				yield(nil, err)
				return
			}

			if !yieldAndClose(f) {
				return
			}
			// we only read the first found file
			break
		}

		if len(paths.DropInDirectories) > 0 {
			suffix := "." + conf.Extension
			files, err := readDropInsFromPaths(paths.DropInDirectories, suffix)
			if err != nil {
				// return error via iterator
				yield(nil, err)
				return
			}
			for _, file := range files {
				usedPaths = append(usedPaths, file)
				f, err := os.Open(file)
				// always ignore ErrNotExist, all other errors get return to the caller via yield
				if err != nil {
					if errors.Is(err, fs.ErrNotExist) {
						continue
					}
					yield(nil, err)
					return
				}

				if !yieldAndClose(f) {
					return
				}
			}
		}

		if len(conf.Modules) > 0 {
			resolvedModules := make([]string, 0, len(conf.Modules))
			for _, module := range conf.Modules {
				f, err := resolveModule(module, paths.ModuleDirectories, &usedPaths)
				if err != nil {
					yield(nil, fmt.Errorf("could not resolve module: %w", err))
					return
				}
				resolvedModules = append(resolvedModules, f.Name())
				if !yieldAndClose(f) {
					return
				}
			}
			conf.Modules = resolvedModules
		}

		if paths.ExtraOverrideFile != "" {
			// The _OVERRIDE env must be appended after loading all files, even modules.
			usedPaths = append(usedPaths, paths.ExtraOverrideFile)
			f, err := os.Open(paths.ExtraOverrideFile)
			// Do not ignore ErrNotExist here, we want to hard error if users set a wrong path here.
			if err != nil {
				yield(nil, err)
				return
			}
			if !yieldAndClose(f) {
				return
			}
		}

		if conf.ErrorIfNotFound && !foundAny {
			yield(nil, fmt.Errorf("%w: no %s file found; searched paths: %q", ErrConfigFileNotFound, conf.getConfName(), usedPaths))
			return
		}
	}
}

const dropInSuffix = ".d"

func getDropInPaths(defaultConfig, overrideConfig, userConfig, suffix string, uid int) []string {
	paths := make([]string, 0, 7)

	if userConfig != "" {
		// the $HOME config only has one .d path not the rootful/rootless ones.
		paths = append(paths, userConfig+dropInSuffix)
	}
	if overrideConfig != "" {
		paths = append(paths, getDropInPathsUnderMain(overrideConfig, suffix, uid)...)
	}
	if defaultConfig != "" {
		paths = append(paths, getDropInPathsUnderMain(defaultConfig, suffix, uid)...)
	}

	return paths
}

func readDropInsFromPaths(paths []string, suffix string) ([]string, error) {
	dropInMap := make(map[string]string)

	for _, path := range slices.Backward(paths) {
		entries, err := os.ReadDir(path)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return nil, err
		}
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), suffix) {
				dropInMap[entry.Name()] = filepath.Join(path, entry.Name())
			}
		}
	}

	sortedNames := slices.Sorted(maps.Keys(dropInMap))
	files := make([]string, 0, len(sortedNames))
	for _, file := range sortedNames {
		files = append(files, dropInMap[file])
	}
	return files, nil
}

func getDropInPathsUnderMain(mainPath, suffix string, uid int) []string {
	paths := make([]string, 0, 3)

	// Extra condition for windows where uid is always -1 per os.Getuid().
	// In this case it makes no sense to give the rootful path, just ignore
	// the special drop in locations.
	if uid >= 0 {
		rootless := uid > 0
		var specialName string
		if rootless {
			specialName = "rootless"
		} else {
			specialName = "rootful"
		}
		// insert the name after the main config name but before the extension if it has one.
		specialPrefix, cut := strings.CutSuffix(mainPath, suffix)
		specialPath := specialPrefix + "." + specialName
		if cut {
			specialPath += suffix
		}
		specialPath += dropInSuffix
		if rootless {
			paths = append(paths, filepath.Join(specialPath, strconv.Itoa(uid)))
		}
		paths = append(paths, specialPath)
	}

	paths = append(paths, mainPath+dropInSuffix)
	return paths
}

func moduleDirectories(defaultConfig, overrideConfig, userConfig string) []string {
	const moduleSuffix = ".modules"
	modules := make([]string, 0, 3)
	if userConfig != "" {
		modules = append(modules, userConfig+moduleSuffix)
	}
	if overrideConfig != "" {
		modules = append(modules, overrideConfig+moduleSuffix)
	}
	if defaultConfig != "" {
		modules = append(modules, defaultConfig+moduleSuffix)
	}
	return modules
}

// Resolve the specified path to a module.
func resolveModule(path string, dirs []string, usedPaths *[]string) (*os.File, error) {
	if filepath.IsAbs(path) {
		if usedPaths != nil {
			*usedPaths = append(*usedPaths, path)
		}
		return os.Open(path)
	}

	// Collect all errors to avoid suppressing important errors (e.g.,
	// permission errors).
	var multiErr error
	for _, d := range dirs {
		candidate := filepath.Join(d, path)
		if usedPaths != nil {
			*usedPaths = append(*usedPaths, candidate)
		}

		f, err := os.Open(candidate)
		if err == nil {
			return f, nil
		}
		multiErr = errors.Join(multiErr, err)
	}
	return nil, multiErr
}

// ParseTOML parses the given config according to the rules in by [Read].
// Note the given configStruct must be a pointer to a struct that describes
// the toml config fields and is modified in place.
// If an error is returned the struct should not be used.
func ParseTOML(configStruct any, conf *File) error {
	for item, err := range Read(conf) {
		if err != nil {
			return err
		}
		meta, err := toml.NewDecoder(item.Reader).Decode(configStruct)
		if err != nil {
			return fmt.Errorf("decode configuration %q: %w", item.Name, err)
		}
		keys := meta.Undecoded()
		if len(keys) > 0 {
			logrus.Debugf("Failed to decode the keys %q from %q", keys, item.Name)
		}

		logrus.Debugf("Read config file %q", item.Name)
		// This prints large potentially large structs so keep it to trace level only.
		// It can however be useful to figure out which setting come from which file.
		logrus.Tracef("Merged new config: %+v", configStruct)
	}
	return nil
}
