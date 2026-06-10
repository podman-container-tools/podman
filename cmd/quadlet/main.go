package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"go.podman.io/podman/v6/pkg/logiface"
	"go.podman.io/podman/v6/pkg/systemd/parser"
	"go.podman.io/podman/v6/pkg/systemd/quadlet"
	"go.podman.io/podman/v6/version/rawversion"
)

// This commandline app is the systemd generator (system and user,
// decided by the name of the binary).

// Generators run at very early startup, so must work in a very
// limited environment (e.g. no /var, /home, or syslog).  See:
// https://www.freedesktop.org/software/systemd/man/systemd.generator.html#Notes%20about%20writing%20generators
// for more details.

var (
	verboseFlag bool // True if -v passed
	noKmsgFlag  bool
	isUserFlag  bool // True if run as quadlet-user-generator executable
	dryRunFlag  bool // True if -dryrun is used
	versionFlag bool // True if -version is used
)

var (
	// data saved between logToKmsg calls
	noKmsg   = false
	kmsgFile *os.File
)

var void struct{}

// We log directly to /dev/kmsg, because that is the only way to get information out
// of the generator into the system logs.
func logToKmsg(s string) bool {
	if noKmsg {
		return false
	}

	if kmsgFile == nil {
		f, err := os.OpenFile("/dev/kmsg", os.O_WRONLY, 0o644)
		if err != nil {
			noKmsg = true
			return false
		}
		kmsgFile = f
	}

	if _, err := kmsgFile.WriteString(s); err != nil {
		kmsgFile.Close()
		kmsgFile = nil
		return false
	}

	return true
}

func shouldLogToStderr(kmsgOK bool, dryRun bool, stderrIsTerminal bool) bool {
	return !kmsgOK || dryRun || stderrIsTerminal
}

// stderrIsTerminal reports whether stderr is a character device (e.g. a tty).
// Used as a lightweight isatty so quadlet logs to stderr when run interactively
// and to kmsg when run non-interactively (e.g. under systemd), without pulling
// golang.org/x/term (and its golang.org/x/sys dependency) into the generator
// binary.
func stderrIsTerminal() bool {
	fi, err := os.Stderr.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func Logf(format string, a ...any) {
	s := fmt.Sprintf(format, a...)
	line := fmt.Sprintf("quadlet-generator[%d]: %s", os.Getpid(), s)

	isTerminal := stderrIsTerminal()
	kmsgOK := false
	if !isTerminal {
		kmsgOK = logToKmsg(line)
	}
	if shouldLogToStderr(kmsgOK, dryRunFlag, isTerminal) {
		fmt.Fprintf(os.Stderr, "%s\n", line)
		os.Stderr.Sync()
	}
}

var debugEnabled = false

func enableDebug() {
	debugEnabled = true
}

func Debugf(format string, a ...any) {
	if debugEnabled {
		Logf(format, a...)
	}
}

var seen = make(map[string]struct{})

func loadUnitsFromDir(sourcePath string) ([]*parser.UnitFile, error) {
	var prevError error
	files, err := os.ReadDir(sourcePath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		return []*parser.UnitFile{}, nil
	}

	var units []*parser.UnitFile

	for _, file := range files {
		name := file.Name()
		if _, ok := seen[name]; !ok && quadlet.IsExtSupported(name) {
			path := path.Join(sourcePath, name)

			Debugf("Loading source unit file %s", path)

			if f, err := parser.ParseUnitFile(path); err != nil {
				err = fmt.Errorf("error loading %q, %w", path, err)
				if prevError == nil {
					prevError = err
				} else {
					prevError = fmt.Errorf("%s\n%s", prevError, err)
				}
			} else {
				seen[name] = void
				units = append(units, f)
			}
		}
	}

	return units, prevError
}

func loadUnitDropins(unit *parser.UnitFile, sourcePaths []string) error {
	var prevError error
	reportError := func(err error) {
		if prevError != nil {
			err = fmt.Errorf("%s\n%s", prevError, err)
		}
		prevError = err
	}

	unitDropinPaths := unit.GetUnitDropinPaths()
	dropinDirs := make([]string, 0, len(unitDropinPaths))
	for _, dropinPath := range unitDropinPaths {
		for _, sourcePath := range sourcePaths {
			dropinDirs = append(dropinDirs, path.Join(sourcePath, dropinPath))
		}
	}

	dropinPaths := make(map[string]string)
	for _, dropinDir := range dropinDirs {
		dropinFiles, err := os.ReadDir(dropinDir)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				reportError(fmt.Errorf("error reading directory %q, %w", dropinDir, err))
			}

			continue
		}

		for _, dropinFile := range dropinFiles {
			dropinName := dropinFile.Name()
			if filepath.Ext(dropinName) != ".conf" {
				continue // Only *.conf supported
			}

			if _, ok := dropinPaths[dropinName]; ok {
				continue // We already saw this name
			}

			dropinPaths[dropinName] = path.Join(dropinDir, dropinName)
		}
	}

	dropinFiles := make([]string, len(dropinPaths))
	i := 0
	for k := range dropinPaths {
		dropinFiles[i] = k
		i++
	}

	// Merge in alpha-numerical order
	sort.Strings(dropinFiles)

	for _, dropinFile := range dropinFiles {
		dropinPath := dropinPaths[dropinFile]

		Debugf("Loading source drop-in file %s", dropinPath)

		if f, err := parser.ParseUnitFile(dropinPath); err != nil {
			reportError(fmt.Errorf("error loading %q, %w", dropinPath, err))
		} else {
			unit.Merge(f)
		}
	}

	return prevError
}

func generateServiceFile(service *parser.UnitFile) error {
	Debugf("writing %q", service.Path)

	service.PrependComment("",
		fmt.Sprintf("Automatically generated by %s", os.Args[0]),
		"")

	f, err := os.Create(service.Path)
	if err != nil {
		return err
	}

	defer f.Close()

	err = service.Write(f)
	if err != nil {
		return err
	}

	err = f.Sync()
	if err != nil {
		return err
	}

	return nil
}

func gatherDependentSymlinks(service *parser.UnitFile, key, dir, filename string) []string {
	symlinks := make([]string, 0)
	groupBy := service.LookupAllStrv(quadlet.InstallGroup, key)
	for _, groupByUnit := range groupBy {
		// Only allow filenames, not paths
		if !strings.Contains(groupByUnit, "/") {
			symlinks = append(symlinks, fmt.Sprintf("%s.%s/%s", groupByUnit, dir, filename))
		}
	}
	return symlinks
}

// This parses the `Install` group of the unit file and creates the required
// symlinks to get systemd to start the newly generated file as needed.
// In a traditional setup this is done by "systemctl enable", but that doesn't
// work for auto-generated files like these.
func enableServiceFile(outputPath string, service *parser.UnitFile) {
	symlinks := make([]string, 0)

	aliases := service.LookupAllStrv(quadlet.InstallGroup, "Alias")
	for _, alias := range aliases {
		symlinks = append(symlinks, filepath.Clean(alias))
	}

	serviceFilename := service.Filename
	templateBase, templateInstance, isTemplate := service.GetTemplateParts()

	// For non-instantiated template service we only support installs if a
	// DefaultInstance is given. Otherwise we ignore the Install group, but
	// it is still useful when instantiating the unit via a symlink.
	if isTemplate && templateInstance == "" {
		if defaultInstance, ok := service.Lookup(quadlet.InstallGroup, "DefaultInstance"); ok {
			serviceFilename = templateBase + "@" + defaultInstance + filepath.Ext(serviceFilename)
		} else {
			serviceFilename = ""
		}
	}

	if serviceFilename != "" {
		symlinks = append(symlinks, gatherDependentSymlinks(service, "WantedBy", "wants", serviceFilename)...)
		symlinks = append(symlinks, gatherDependentSymlinks(service, "RequiredBy", "requires", serviceFilename)...)
		symlinks = append(symlinks, gatherDependentSymlinks(service, "UpheldBy", "upholds", serviceFilename)...)
	}

	for _, symlinkRel := range symlinks {
		target, err := filepath.Rel(path.Dir(symlinkRel), service.Filename)
		if err != nil {
			Logf("Can't create symlink %s: %s", symlinkRel, err)
			continue
		}
		symlinkPath := path.Join(outputPath, symlinkRel)

		symlinkDir := path.Dir(symlinkPath)
		err = os.MkdirAll(symlinkDir, os.ModePerm)
		if err != nil {
			Logf("Can't create dir %s: %s", symlinkDir, err)
			continue
		}

		Debugf("Creating symlink %s -> %s", symlinkPath, target)
		_ = os.Remove(symlinkPath) // overwrite existing symlinks
		err = os.Symlink(target, symlinkPath)
		if err != nil {
			Logf("Failed creating symlink %s: %s", symlinkPath, err)
		}
	}
}

func isImageID(imageName string) bool {
	// All sha25:... names are assumed by podman to be fully specified
	if strings.HasPrefix(imageName, "sha256:") {
		return true
	}

	// However, podman also accepts image ids as pure hex strings,
	// but only those of length 64 are unambiguous image ids
	if len(imageName) != 64 {
		return false
	}

	for _, c := range imageName {
		if !unicode.Is(unicode.Hex_Digit, c) {
			return false
		}
	}

	return true
}

func isUnambiguousName(imageName string) bool {
	// Fully specified image ids are unambiguous
	if isImageID(imageName) {
		return true
	}

	// Otherwise we require a fully qualified name
	before, _, ok := strings.Cut(imageName, "/")
	if !ok {
		// No domain or path, not fully qualified
		return false
	}

	// What is before the first slash can be a domain or a path
	domain := before

	// If its a domain (has dot or port or is "localhost") it is considered fq
	if strings.ContainsAny(domain, ".:") || domain == "localhost" {
		return true
	}

	return false
}

// warns if input is an ambiguous name, i.e. a partial image id or a short
// name (i.e. is missing a registry)
//
// Examples:
//   - short names: "image:tag", "library/fedora"
//   - fully qualified names: "quay.io/image", "localhost/image:tag",
//     "server.org:5000/lib/image", "sha256:..."
//
// We implement a simple version of this from scratch here to avoid
// a huge dependency in the generator just for a warning.
func warnIfAmbiguousName(unit *parser.UnitFile, group string) {
	imageName, ok := unit.Lookup(group, quadlet.KeyImage)
	if !ok {
		return
	}
	if strings.HasSuffix(imageName, ".build") || strings.HasSuffix(imageName, ".image") {
		return
	}
	if !isUnambiguousName(imageName) {
		Logf("Warning: %s specifies the image \"%s\" which not a fully qualified image name. This is not ideal for performance and security reasons. See the podman-pull manpage discussion of short-name-aliases.conf for details.", unit.Filename, imageName)
	}
}

// Warns if the unit has any properties defined in the Service group that are known to cause issues.
// We want to warn instead of erroring to avoid breaking any existing users' units,
// or to allow users to use these properties if they know what they are doing.
// We implement this here instead of in quadlet.initServiceUnitFile to avoid
// having to refactor a large amount of code in the generator just for a warning.
func warnIfUnsupportedServiceKeys(unit *parser.UnitFile) {
	for _, key := range quadlet.UnsupportedServiceKeys {
		_, hasKey := unit.Lookup(quadlet.ServiceGroup, key)
		if hasKey {
			Logf("Warning: using key %s in the Service group is not supported - use at your own risk", key)
		}
	}
}

func generateUnitsInfoMap(units []*parser.UnitFile) map[string]*quadlet.UnitInfo {
	unitsInfoMap := make(map[string]*quadlet.UnitInfo)
	for _, unit := range units {
		var serviceName string
		var containers []string
		var resourceName string
		var err error

		serviceName, err = quadlet.GetUnitServiceName(unit)
		if err != nil {
			Logf("Error obtaining service name: %v", err)
		}

		switch {
		case strings.HasSuffix(unit.Filename, ".container"):
			// Prefill resourceNames for .container files. This solves network reusing.
			resourceName = quadlet.GetContainerResourceName(unit)
		case strings.HasSuffix(unit.Filename, ".build"):
			// Prefill resourceNames for .build files. This is significantly less complex than
			// pre-computing all resourceNames for all Quadlet types (which is rather complex for a few
			// types), but still breaks the dependency cycle between .volume and .build ([Volume] can
			// have Image=some.build, and [Build] can have Volume=some.volume:/some-volume)
			resourceName = quadlet.GetBuiltImageName(unit)
		case strings.HasSuffix(unit.Filename, ".artifact"):
			serviceName = quadlet.GetArtifactServiceName(unit)
		case strings.HasSuffix(unit.Filename, ".pod"):
			containers = make([]string, 0)
			// Prefill resourceNames for .pod files.
			// This is requires for referencing the pod from .container files
			resourceName = quadlet.GetPodResourceName(unit)
		case strings.HasSuffix(unit.Filename, ".volume"), strings.HasSuffix(unit.Filename, ".kube"), strings.HasSuffix(unit.Filename, ".network"), strings.HasSuffix(unit.Filename, ".image"):
			// Do nothing for these case.
		default:
			Logf("Unsupported file type %q", unit.Filename)
			continue
		}

		unitsInfoMap[unit.Filename] = &quadlet.UnitInfo{
			ServiceName:       serviceName,
			ContainersToStart: containers,
			ResourceName:      resourceName,
		}
	}

	return unitsInfoMap
}

// quadletLogger implements the logiface.Logger interface using quadlet's custom logging
type quadletLogger struct{}

func (l quadletLogger) Errorf(format string, args ...any) {
	Logf(format, args...)
}

func (l quadletLogger) Debugf(format string, args ...any) {
	Debugf(format, args...)
}

func main() {
	// Initialize logiface with quadlet's custom logger
	logiface.SetLogger(quadletLogger{})

	if processErred := process(); processErred {
		Logf("processing encountered some errors")
		os.Exit(1)
	}
	os.Exit(0)
}

func process() bool {
	var processErred bool

	prgname := path.Base(os.Args[0])
	isUserFlag = strings.Contains(prgname, "user")

	flag.Parse()

	if versionFlag {
		fmt.Printf("%s\n", rawversion.RawVersion)
		return processErred
	}

	if verboseFlag || dryRunFlag {
		enableDebug()
	}

	if noKmsgFlag || dryRunFlag {
		noKmsg = true
	}

	reportError := func(err error) {
		Logf("%s", err.Error())
		processErred = true
	}

	if !dryRunFlag && flag.NArg() < 1 {
		reportError(errors.New("missing output directory argument"))
		return processErred
	}

	var outputPath string

	if !dryRunFlag {
		outputPath = flag.Arg(0)

		Debugf("Starting quadlet-generator, output to: %s", outputPath)
	}

	sourcePathsMap := quadlet.GetUnitDirs(isUserFlag, true)

	var units []*parser.UnitFile
	for _, d := range sourcePathsMap {
		if result, err := loadUnitsFromDir(d); err != nil {
			reportError(err)
		} else {
			units = append(units, result...)
		}
	}

	if len(units) == 0 {
		// containers/podman/issues/17374: exit cleanly but log that we
		// had nothing to do
		Debugf("No files parsed from %s", sourcePathsMap)
		return processErred
	}

	for _, unit := range units {
		if err := loadUnitDropins(unit, sourcePathsMap); err != nil {
			reportError(err)
		}
	}

	if !dryRunFlag {
		err := os.MkdirAll(outputPath, os.ModePerm)
		if err != nil {
			reportError(err)
			return processErred
		}
	}

	// Sort unit files according to potential inter-dependencies, with Volume and Network units
	// taking precedence over all others.
	sort.Slice(units, func(i, j int) bool {
		getOrder := func(i int) int {
			ext := filepath.Ext(units[i].Filename)
			order, ok := quadlet.SupportedExtensions[ext]
			if !ok {
				return 0
			}
			return order
		}
		return getOrder(i) < getOrder(j)
	})

	// Generate the PodsInfoMap to allow containers to link to their pods and add themselves to the pod's containers list
	unitsInfoMap := generateUnitsInfoMap(units)

	for _, unit := range units {
		var service *parser.UnitFile
		var warnings, err error

		warnIfUnsupportedServiceKeys(unit)

		switch {
		case strings.HasSuffix(unit.Filename, ".container"):
			warnIfAmbiguousName(unit, quadlet.ContainerGroup)
			service, warnings, err = quadlet.ConvertContainer(unit, unitsInfoMap, isUserFlag)
		case strings.HasSuffix(unit.Filename, ".volume"):
			warnIfAmbiguousName(unit, quadlet.VolumeGroup)
			service, warnings, err = quadlet.ConvertVolume(unit, unitsInfoMap, isUserFlag)
		case strings.HasSuffix(unit.Filename, ".kube"):
			service, err = quadlet.ConvertKube(unit, unitsInfoMap, isUserFlag)
		case strings.HasSuffix(unit.Filename, ".network"):
			service, warnings, err = quadlet.ConvertNetwork(unit, unitsInfoMap, isUserFlag)
		case strings.HasSuffix(unit.Filename, ".image"):
			warnIfAmbiguousName(unit, quadlet.ImageGroup)
			service, err = quadlet.ConvertImage(unit, unitsInfoMap, isUserFlag)
		case strings.HasSuffix(unit.Filename, ".build"):
			service, warnings, err = quadlet.ConvertBuild(unit, unitsInfoMap, isUserFlag)
		case strings.HasSuffix(unit.Filename, ".artifact"):
			warnIfAmbiguousName(unit, quadlet.ArtifactGroup)
			service, err = quadlet.ConvertArtifact(unit, unitsInfoMap, isUserFlag)
		case strings.HasSuffix(unit.Filename, ".pod"):
			service, warnings, err = quadlet.ConvertPod(unit, unitsInfoMap, isUserFlag)
		default:
			Logf("Unsupported file type %q", unit.Filename)
			continue
		}

		if warnings != nil {
			Logf("%s", warnings.Error())
		}

		if err != nil {
			reportError(fmt.Errorf("converting %q: %w", unit.Filename, err))
			continue
		}

		service.Path = path.Join(outputPath, service.Filename)

		if dryRunFlag {
			data, err := service.ToString()
			if err != nil {
				reportError(fmt.Errorf("parsing %s: %w", service.Path, err))
				continue
			}
			fmt.Printf("---%s---\n%s\n", service.Path, data)
			continue
		}
		if err := generateServiceFile(service); err != nil {
			reportError(fmt.Errorf("generating service file %s: %w", service.Path, err))
		}
		enableServiceFile(outputPath, service)
	}
	return processErred
}

func init() {
	flag.BoolVar(&verboseFlag, "v", false, "Print debug information")
	flag.BoolVar(&noKmsgFlag, "no-kmsg-log", false, "Don't log to kmsg")
	flag.BoolVar(&isUserFlag, "user", false, "Run as systemd user")
	flag.BoolVar(&dryRunFlag, "dryrun", false, "Run in dryrun mode printing debug information")
	flag.BoolVar(&versionFlag, "version", false, "Print version information and exit")
}
