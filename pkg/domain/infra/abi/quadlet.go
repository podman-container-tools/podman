//go:build !remote && (linux || freebsd)

package abi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	"go.podman.io/common/pkg/config"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/rootless"
	"go.podman.io/podman/v6/pkg/systemd"
	systemdquadlet "go.podman.io/podman/v6/pkg/systemd/quadlet"
	"go.podman.io/storage/pkg/fileutils"
)

// Install one or more Quadlet files
func (ic *ContainerEngine) QuadletInstall(ctx context.Context, pathsOrURLs []string, options entities.QuadletInstallOptions) (*entities.QuadletInstallReport, error) {
	// Fail if quadlet files list is empty
	if len(pathsOrURLs) == 0 {
		return nil, fmt.Errorf("at least one quadlet file path needed")
	}

	// Fail if systemd isn't available to the current user
	conn, err := systemd.ConnectToDBUS()
	if err != nil {
		return nil, fmt.Errorf("connecting to systemd dbus: %w", err)
	}
	defer conn.Close()

	// Fail if the Quadlet binary cannot be found
	cfg, err := config.Default()
	if err != nil {
		return nil, fmt.Errorf("unable to load default config: %w", err)
	}
	quadletPath, err := cfg.FindHelperBinary("quadlet", true)
	if err != nil {
		return nil, fmt.Errorf("cannot stat Quadlet generator, Quadlet may not be installed: %w", err)
	}
	if quadletPath == "" {
		return nil, fmt.Errorf("unable to find `quadlet` binary, Quadlet may not be installed")
	}
	quadletStat, err := os.Stat(quadletPath)
	if err != nil {
		return nil, fmt.Errorf("cannot stat Quadlet generator, Quadlet may not be installed: %w", err)
	}
	if !quadletStat.Mode().IsRegular() || quadletStat.Mode()&0o100 == 0 {
		return nil, fmt.Errorf("no valid Quadlet binary installed to %q, unable to use Quadlet", quadletPath)
	}

	// Set installDir (quadlets target directory)
	installDir := systemdquadlet.GetInstallUnitDirPath(rootless.IsRootless())
	if len(options.Application) > 0 {
		// Prevent path traversal by validating the user input "Application"
		err := validateApplicationName(installDir, options.Application)
		if err != nil {
			return nil, fmt.Errorf("invalid application name: %w", err)
		}
		installDir = filepath.Join(installDir, options.Application)
	}
	logrus.Debugf("Going to install Quadlet to directory %s", installDir)
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		return nil, fmt.Errorf("unable to create Quadlet install path %s: %w", installDir, err)
	}

	type qpaths struct {
		src string
		dst string
	}
	var quadletPaths []qpaths
	var quadletURLs, nestedQuadletPaths []string
	firstArg := pathsOrURLs[0]
	switch {
	case isFolder(firstArg):
		if options.Application == "" {
			return nil, fmt.Errorf("application name cannot be empty when installing from directory")
		}
		nestedQuadletPaths, err = findNestedQuadlets(firstArg)
		if err != nil {
			return nil, fmt.Errorf("failed finding quadlet files in folder %q: %w", firstArg, err)
		}
	case isURL(firstArg):
		quadletURLs = append(quadletURLs, firstArg)
	default:
		quadletPaths = append(quadletPaths, qpaths{firstArg, filepath.Join(installDir, filepath.Base(firstArg))})
	}
	otherArgs := pathsOrURLs[1:]
	for _, pathOrURL := range otherArgs {
		if isURL(pathOrURL) {
			quadletURLs = append(quadletURLs, pathOrURL)
		} else {
			quadletPaths = append(quadletPaths, qpaths{pathOrURL, filepath.Join(installDir, filepath.Base(pathOrURL))})
		}
	}

	// Process the quadlet lists
	installReport := entities.QuadletInstallReport{
		InstalledQuadlets: make(map[string]string),
		QuadletErrors:     make(map[string]error),
	}
	for _, nestedPath := range nestedQuadletPaths {
		// `nestedQuadletPaths` are files under folder
		// `firstArg` or one of its subfolders. These files
		// need to be installed under folder `installDir`.
		// For example file `firstArg + "foo/bar"` needs
		// to be installed in `installDir + "foo/bar".
		// For this reason we need to get the relative
		// path ("foo/bar") and pass it to `installQuadlet`
		nestedPathRel, err := filepath.Rel(firstArg, nestedPath)
		if err != nil {
			installReport.QuadletErrors[nestedPath] = err
			continue
		}
		quadletPaths = append(quadletPaths, qpaths{nestedPath, filepath.Join(installDir, nestedPathRel)})
	}

	// Loop over the URLs
	for _, quadletURL := range quadletURLs {
		installedPath, err := installQuadletFromURL(ctx, ic, quadletURL, installDir, options.Replace)
		if err != nil {
			installReport.QuadletErrors[quadletURL] = err
			continue
		}
		installReport.InstalledQuadlets[quadletURL] = installedPath
	}
	// Loop over the paths
	for _, quadletPath := range quadletPaths {
		err = fileutils.Exists(quadletPath.src)
		if err != nil {
			installReport.QuadletErrors[quadletPath.src] = err
			continue
		}
		quadletExt := filepath.Ext(quadletPath.src)
		// Check if this file is a .quadlets file
		switch {
		case quadletExt == ".quadlets":
			// Parse the multi-quadlet file
			sections, err := parseMultiQuadletFile(quadletPath.src)
			if err != nil {
				installReport.QuadletErrors[quadletPath.src] = err
				continue
			}
			// The sections installation folder can be different
			// than the root `installDir`. For example if the .quadlets
			// file is part of an application and is in a subdirectory.
			sectionsDestDir := filepath.Dir(quadletPath.dst)
			// Install each quadlet section as a separate file
			for _, section := range sections {
				installedPath, err := installMultiQuadletSection(ctx, ic, section, sectionsDestDir, options.Replace)
				if err != nil {
					installReport.QuadletErrors[quadletPath.src] = fmt.Errorf("unable to create temporary file for quadlet section %s: %w", section.name, err)
					continue
				}
				// Record the installation (use a unique key for each section)
				sectionKey := fmt.Sprintf("%s#%s", quadletPath.src, filepath.Base(installedPath))
				installReport.InstalledQuadlets[sectionKey] = installedPath
			}
		case systemdquadlet.IsExtSupported(quadletPath.src) ||
			options.Application != "":
			// If quadletPath is a single file with a supported extension, or
			// if it isn't but it's part of an application, execute the original logic
			installedPath, err := ic.installQuadlet(ctx, quadletPath.src, quadletPath.dst, options.Replace)
			if err != nil {
				installReport.QuadletErrors[quadletPath.src] = err
				continue
			}
			installReport.InstalledQuadlets[quadletPath.src] = installedPath
		default:
			installReport.QuadletErrors[quadletPath.src] = fmt.Errorf("unsupported quadlet extension (%q)", quadletExt)
		}
	}

	// TODO: Should we still do this if the above validation errored?
	if options.ReloadSystemd {
		if err := conn.ReloadContext(ctx); err != nil {
			return &installReport, fmt.Errorf("reloading systemd: %w", err)
		}
	}

	return &installReport, nil
}

func installQuadletFromURL(ctx context.Context, ic *ContainerEngine, quadletURL string, installDir string, replace bool) (string, error) {
	r, err := http.Get(quadletURL)
	if err != nil {
		return "", fmt.Errorf("unable to download URL %s: %w", quadletURL, err)
	}
	defer r.Body.Close()
	quadletFileName, err := getFileName(r, quadletURL)
	if err != nil {
		return "", fmt.Errorf("unable to get file name from url %s: %w", quadletURL, err)
	}
	// It's a URL. Pull to temporary file.
	tmpFile, err := os.CreateTemp("", quadletFileName)
	if err != nil {
		return "", fmt.Errorf("unable to create temporary file to download URL %s: %w", quadletURL, err)
	}
	defer func() {
		tmpFile.Close()
		if err := os.Remove(tmpFile.Name()); err != nil {
			logrus.Errorf("unable to remove temporary file %q: %v", tmpFile.Name(), err)
		}
	}()
	_, err = io.Copy(tmpFile, r.Body)
	if err != nil {
		return "", fmt.Errorf("populating temporary file: %w", err)
	}
	return ic.installQuadlet(ctx, tmpFile.Name(), filepath.Join(installDir, quadletFileName), replace)
}

func installMultiQuadletSection(ctx context.Context, ic *ContainerEngine, section quadletSection, installDir string, replace bool) (string, error) {
	tmpFile, err := os.CreateTemp("", section.name+"*"+section.extension)
	if err != nil {
		return "", fmt.Errorf("unable to create temporary file for quadlet section %s: %w", section.name, err)
	}
	defer func() {
		tmpFile.Close()
		if err := os.Remove(tmpFile.Name()); err != nil {
			logrus.Errorf("unable to remove temporary file %q: %v", tmpFile.Name(), err)
		}
	}()

	// Write the quadlet content to the temporary file
	_, err = tmpFile.WriteString(section.content)
	if err != nil {
		return "", fmt.Errorf("unable to write quadlet section %s to temporary file: %w", section.name, err)
	}
	// Install the quadlet from the temporary file
	destName := section.name + section.extension
	return ic.installQuadlet(ctx, tmpFile.Name(), filepath.Join(installDir, destName), replace)
}

func isFolder(s string) bool {
	if isURL(s) {
		return false
	}
	info, err := os.Stat(s)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func isURL(s string) bool {
	return strings.HasPrefix(s, "http://") ||
		strings.HasPrefix(s, "https://")
}

func findNestedQuadlets(folderPath string) ([]string, error) {
	// If it's a directory, then read all files and add it to paths
	quadletPaths := make([]string, 0)
	err := filepath.WalkDir(folderPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("unable to read Quadlet dir %s: %w", path, err)
		}
		if !d.IsDir() {
			quadletPaths = append(quadletPaths, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return quadletPaths, nil
}

// Extracts file name from Content-Disposition or URL
func getFileName(resp *http.Response, fileURL string) (string, error) {
	// Try to get filename from Content-Disposition header
	cd := resp.Header.Get("Content-Disposition")
	if cd != "" {
		const prefix = "filename="
		if _, after, ok := strings.Cut(cd, prefix); ok {
			filename := after
			filename = strings.Trim(filename, "\"'")
			return filename, nil
		}
	}

	// Fallback: get filename from URL path
	u, err := url.Parse(fileURL)
	if err != nil {
		return "", err
	}
	return path.Base(u.Path), nil
}

// Install a single Quadlet from a path on local disk to the given install directory.
// Perform some minimal validation, but not much.
// We can't know about a lot of problems without running the Quadlet binary, which we
// only want to do once.
func (ic *ContainerEngine) installQuadlet(ctx context.Context, srcPath, destPath string, replace bool) (string, error) {
	select {
	case <-ctx.Done():
		return "", fmt.Errorf("context cancelled: %w", ctx.Err())
	default:
	}

	// First, validate that the source path exists and is a file
	_, err := os.Stat(srcPath)
	if err != nil {
		return "", fmt.Errorf("quadlet to install %q does not exist or cannot be read: %w", srcPath, err)
	}

	// Second, create the destPath folder as it may not exist yet
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return "", fmt.Errorf("unable to create Quadlet install path %s: %w", destPath, err)
	}

	var destFile *os.File
	var tempPath string
	if !replace {
		var err error
		// O_EXCL ensures we fail if the file already exists (avoids TOCTOU race)
		destFile, err = os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o644)
		if err != nil {
			if errors.Is(err, fs.ErrExist) {
				return "", fmt.Errorf("a Quadlet with name %s already exists, refusing to overwrite", filepath.Base(destPath))
			}
			return "", fmt.Errorf("unable to open file %s: %w", destPath, err)
		}
	} else {
		var err error
		destFile, err = os.CreateTemp(filepath.Dir(destPath), ".quadlet-install-*")
		if err != nil {
			return "", fmt.Errorf("unable to create temp file: %w", err)
		}
		tempPath = destFile.Name()
	}

	defer func() {
		if destFile != nil {
			destFile.Close()
		}
		if tempPath != "" {
			os.Remove(tempPath)
		}
	}()

	srcFile, err := os.Open(srcPath)
	if err != nil {
		return "", fmt.Errorf("unable to open file: %w", err)
	}
	defer srcFile.Close()

	err = fileutils.ReflinkOrCopy(srcFile, destFile)
	if err != nil {
		return "", fmt.Errorf("unable to copy file from %s to %s: %w", srcPath, destPath, err)
	}

	// Close before rename to flush writes; nil out to prevent double-close in defer
	if err := destFile.Close(); err != nil {
		return "", fmt.Errorf("unable to close file: %w", err)
	}
	destFile = nil

	if tempPath != "" {
		if err := os.Chmod(tempPath, 0o644); err != nil {
			return "", fmt.Errorf("unable to set permissions on temp file: %w", err)
		}

		if err := os.Rename(tempPath, destPath); err != nil {
			return "", fmt.Errorf("unable to rename temp file to %s: %w", destPath, err)
		}
		tempPath = ""
	}
	return destPath, nil
}

// quadletSection represents a single quadlet extracted from a multi-quadlet file
type quadletSection struct {
	content   string
	extension string
	name      string
}

// parseMultiQuadletFile parses a file that may contain multiple quadlets separated by "---"
// Returns a slice of quadletSection structs, each representing a separate quadlet
func parseMultiQuadletFile(filePath string) ([]quadletSection, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("unable to read file %s: %w", filePath, err)
	}

	// Split content by lines and reconstruct sections manually to handle "---" properly
	lines := strings.Split(string(content), "\n")
	var sections []string
	var currentSection strings.Builder

	for _, line := range lines {
		if strings.TrimSpace(line) == "---" {
			// Found separator, save current section and start new one
			if currentSection.Len() > 0 {
				sections = append(sections, currentSection.String())
				currentSection.Reset()
			}
		} else {
			currentSection.WriteString(line)
			currentSection.WriteString("\n")
		}
	}

	// Add the last section
	if currentSection.Len() > 0 {
		sections = append(sections, currentSection.String())
	}

	// Pre-allocate slice with capacity based on number of sections
	quadlets := make([]quadletSection, 0, len(sections))

	for i, section := range sections {
		// Trim whitespace from section
		section = strings.TrimSpace(section)
		if section == "" {
			continue // Skip empty sections
		}

		// Determine quadlet type from section content
		extension, err := detectQuadletType(section)
		if err != nil {
			return nil, fmt.Errorf("unable to detect quadlet type in section %d: %w", i+1, err)
		}

		fileName, err := extractFileNameFromSection(section)
		if err != nil {
			return nil, fmt.Errorf("section %d: %w", i+1, err)
		}
		name := fileName

		quadlets = append(quadlets, quadletSection{
			content:   section,
			extension: extension,
			name:      name,
		})
	}

	if len(quadlets) == 0 {
		return nil, fmt.Errorf("no valid quadlet sections found in file %s", filePath)
	}

	return quadlets, nil
}

// extractFileNameFromSection extracts the FileName from a comment in the quadlet section
// The comment must be in the format: # FileName=my-name
func extractFileNameFromSection(content string) (string, error) {
	lines := strings.SplitSeq(content, "\n")
	for line := range lines {
		line = strings.TrimSpace(line)
		// Look for comment lines starting with #
		if strings.HasPrefix(line, "#") {
			// Remove the # and trim whitespace
			commentContent := strings.TrimSpace(line[1:])
			// Check if it's a FileName directive
			if strings.HasPrefix(commentContent, "FileName=") {
				fileName := strings.TrimSpace(commentContent[9:]) // Remove "FileName="
				if fileName == "" {
					return "", fmt.Errorf("FileName comment found but no filename specified")
				}
				// Validate filename (basic validation - no path separators)
				if strings.ContainsAny(fileName, "/\\") {
					return "", fmt.Errorf("FileName '%s' cannot contain path separators", fileName)
				}
				return fileName, nil
			}
		}
	}
	return "", fmt.Errorf("missing required '# FileName=<name>' comment at the beginning of quadlet section")
}

// detectQuadletType analyzes the content of a quadlet section to determine its type
// Returns the appropriate file extension (.container, .volume, .network, etc.)
func detectQuadletType(content string) (string, error) {
	// Look for section headers like [Container], [Volume], [Network], etc.
	lines := strings.SplitSeq(content, "\n")
	for line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			sectionName := strings.ToLower(strings.Trim(line, "[]"))
			expected := "." + sectionName
			if systemdquadlet.IsExtSupported("a" + expected) {
				return expected, nil
			}
		}
	}
	return "", fmt.Errorf("no recognized quadlet section found (expected [Container], [Volume], [Network], [Kube], [Image], [Build], or [Pod])")
}

func (ic *ContainerEngine) QuadletList(ctx context.Context, options entities.QuadletListOptions) ([]*entities.ListQuadlet, error) {
	// Is systemd available to the current user?
	// We cannot proceed if not.
	conn, err := systemd.ConnectToDBUS()
	if err != nil {
		return nil, fmt.Errorf("connecting to systemd dbus: %w", err)
	}
	defer conn.Close()

	// Create filter functions
	filterFunc, err := generateQuadletFilters(options.Filters)
	if err != nil {
		return nil, fmt.Errorf("cannot use filters: %w", err)
	}

	reports, err := getAllQuadlets(ctx, conn)
	if err != nil {
		return nil, fmt.Errorf("cannot get quadlets: %w", err)
	}

	finalReports := make([]*entities.ListQuadlet, 0, len(reports))
	for _, report := range reports {
		include := filterFunc(report)
		if include {
			finalReports = append(finalReports, report)
		}
	}

	return finalReports, nil
}

// QuadletExists checks whether a quadlet with the given name exists.
func (ic *ContainerEngine) QuadletExists(_ context.Context, name string) (*entities.BoolReport, error) {
	_, err := getQuadletPathByName(name)
	if err != nil && !errors.Is(err, define.ErrNoSuchQuadlet) {
		return nil, err
	}
	return &entities.BoolReport{Value: err == nil}, nil
}

// Retrieve path to a Quadlet file given full name including extension
func getQuadletPathByName(name string) (string, error) {
	// Check if we were given a valid extension
	if !systemdquadlet.IsExtSupported(name) {
		return "", fmt.Errorf("%q is not a supported quadlet file type", filepath.Ext(name))
	}

	quadletDirs := systemdquadlet.GetUnitDirs(rootless.IsRootless(), true)
	for _, dir := range quadletDirs {
		testPath := filepath.Join(dir, name)
		if _, err := os.Stat(testPath); err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				return "", fmt.Errorf("cannot stat quadlet at path %q: %w", testPath, err)
			}
			continue
		}
		return testPath, nil
	}
	return "", fmt.Errorf("could not locate quadlet %q in any supported quadlet directory: %w", name, define.ErrNoSuchQuadlet)
}

func (ic *ContainerEngine) QuadletPrint(_ context.Context, quadlet string) (string, error) {
	quadletPath, err := getQuadletPathByName(quadlet)
	if err != nil {
		return "", err
	}

	contents, err := os.ReadFile(quadletPath)
	if err != nil {
		return "", fmt.Errorf("reading quadlet %q contents: %w", quadletPath, err)
	}

	return string(contents), nil
}

// QuadletRemove removes one or more Quadlet files or applications and reloads systemd daemon as needed. The function returns a `QuadletRemoveReport`
// containing the removal status for each quadlet file or application, or returns an error if the entire function fails.
func (ic *ContainerEngine) QuadletRemove(ctx context.Context, quadlets []string, options entities.QuadletRemoveOptions) (*entities.QuadletRemoveReport, error) {
	report := entities.QuadletRemoveReport{
		Errors:  make(map[string]error),
		Removed: []string{},
	}

	// Is systemd available to the current user?
	// We cannot proceed if not.
	conn, err := systemd.ConnectToDBUS()
	if err != nil {
		return nil, fmt.Errorf("connecting to systemd dbus: %w", err)
	}
	defer conn.Close()

	// Get all units (aka Quadlets)
	units, err := getAllQuadlets(ctx, conn)
	if err != nil {
		return nil, fmt.Errorf("cannot get quadlets: %w", err)
	}

	if len(quadlets) == 0 && !options.All {
		return nil, errors.New("must provide at least 1 quadlet to remove")
	}

	// Group units by application
	// Map application -> quadlets
	applications := make(map[string][]*entities.ListQuadlet)
	for _, unit := range units {
		if len(unit.App) > 0 {
			applications[unit.App] = append(applications[unit.App], unit)
		}
	}

	if options.All {
		// Add all units not part of an Application
		for _, unit := range units {
			if len(unit.App) == 0 {
				quadlets = append(quadlets, unit.Name)
			}
		}
		// Add all application if recursive is true
		if options.Recursive {
			for application := range applications {
				quadlets = append(quadlets, application)
			}
		}
	}

	// Create a map filename -> quadlet
	files := make(map[string]*entities.ListQuadlet, len(units))
	for _, unit := range units {
		files[unit.Name] = unit
	}

	// Iterate over the list of quadlets to remove
	for _, quadlet := range quadlets {
		var (
			applicationName string
			isUnit          bool
		)

		// Check if the parameter passed is a valid unit name
		// Otherwise it must be the name of an application
		isUnit = systemdquadlet.IsExtSupported(quadlet)

		if isUnit {
			unit := files[quadlet]
			if unit == nil {
				if options.Ignore {
					report.Removed = append(report.Removed, quadlet)
				} else {
					report.Errors[quadlet] = fmt.Errorf("no such quadlet")
				}
				continue
			}

			applicationName = unit.App

			// If the unit isn't part of an application remove it
			if applicationName == "" {
				err := removeQuadlet(ctx, conn, unit, options.Force)
				if err != nil {
					report.Errors[quadlet] = err
				} else {
					report.Removed = append(report.Removed, unit.Name)
				}
				continue
			}
		}

		if applicationName == "" {
			applicationName = quadlet
		}

		// delete an application
		if len(applications[applicationName]) == 0 {
			return nil, fmt.Errorf("no such application %q", applicationName)
		}

		if !options.Recursive {
			return nil, fmt.Errorf("refusing to remove application %q: recursive option is not set", applicationName)
		}

		removeFailed := false
		for _, unit := range applications[applicationName] {
			err := removeQuadlet(ctx, conn, unit, options.Force)
			if err != nil {
				removeFailed = true
				// Use applicationName rather than unit.Name as
				// the `report.Errors` key because function
				// `RemoveQuadlet` uses it to look for errors
				// of a specific application removal.
				report.Errors[applicationName] = err
			} else {
				report.Removed = append(report.Removed, unit.Name)
			}
		}

		// clean up application folder when no error
		if !removeFailed {
			appPath, err := getApplicationPath(applicationName)
			if err != nil {
				report.Errors[applicationName] = err
			} else {
				err = os.RemoveAll(appPath)
				if err != nil {
					report.Errors[applicationName] = err
				}
			}
		}
	}

	// Reload systemd, if necessary/requested.
	if options.ReloadSystemd {
		if err := conn.ReloadContext(ctx); err != nil {
			return &report, fmt.Errorf("reloading systemd: %w", err)
		}
	}

	return &report, nil
}
