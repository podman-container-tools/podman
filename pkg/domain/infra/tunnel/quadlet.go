package tunnel

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	"go.podman.io/podman/v6/pkg/bindings/quadlets"
	"go.podman.io/podman/v6/pkg/domain/entities"
	systemdquadlet "go.podman.io/podman/v6/pkg/systemd/quadlet"
)

func (ic *ContainerEngine) QuadletExists(_ context.Context, name string) (*entities.BoolReport, error) {
	exists, err := quadlets.Exists(ic.ClientCtx, name, nil)
	if err != nil {
		return nil, err
	}
	return &entities.BoolReport{Value: exists}, nil
}

func (ic *ContainerEngine) QuadletInstall(_ context.Context, pathsOrURLs []string, opts entities.QuadletInstallOptions) (*entities.QuadletInstallReport, error) {
	options := new(quadlets.InstallOptions).WithReplace(opts.Replace).WithReloadSystemd(opts.ReloadSystemd)

	report := &entities.QuadletInstallReport{
		InstalledQuadlets: make(map[string]string),
		QuadletErrors:     make(map[string]error),
	}

	allFiles, cleanup, err := resolveInstallPaths(pathsOrURLs)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	// The API allows exactly one quadlet file per request, plus optional
	// non-quadlet asset files. Group the flat file list so each quadlet file
	// is bundled with any non-quadlet files that follow it.
	groups := groupByQuadletFile(allFiles)

	for _, group := range groups {
		installReport, err := quadlets.Install(ic.ClientCtx, group, options)
		if err != nil {
			report.QuadletErrors[group[0]] = err
			continue
		}
		for k, v := range installReport.InstalledQuadlets {
			report.InstalledQuadlets[k] = v
		}
		for k, v := range installReport.QuadletErrors {
			report.QuadletErrors[k] = v
		}
	}

	return report, nil
}

// groupByQuadletFile splits a flat file list into groups where each group
// contains exactly one quadlet file plus any non-quadlet asset files that
// follow it. Non-quadlet files that appear before the first quadlet are
// prepended to the first quadlet group.
func groupByQuadletFile(files []string) [][]string {
	var groups [][]string
	var pending []string

	for _, f := range files {
		if systemdquadlet.IsExtSupported(f) {
			if len(pending) > 0 && len(groups) > 0 {
				groups[len(groups)-1] = append(groups[len(groups)-1], pending...)
				pending = nil
			}
			if len(pending) > 0 {
				// Non-quadlet files before the first quadlet; will be
				// prepended once the first quadlet group is created.
				groups = append(groups, append(pending, f))
				pending = nil
			} else {
				groups = append(groups, []string{f})
			}
		} else {
			pending = append(pending, f)
		}
	}

	if len(pending) > 0 {
		if len(groups) > 0 {
			groups[len(groups)-1] = append(groups[len(groups)-1], pending...)
		} else {
			groups = append(groups, pending)
		}
	}

	return groups
}

func (ic *ContainerEngine) QuadletList(_ context.Context, opts entities.QuadletListOptions) ([]*entities.ListQuadlet, error) {
	options := new(quadlets.ListOptions)
	if len(opts.Filters) > 0 {
		filterMap := make(map[string][]string)
		for _, f := range opts.Filters {
			fname, filter, hasFilter := strings.Cut(f, "=")
			if hasFilter {
				filterMap[fname] = append(filterMap[fname], filter)
			}
		}
		options.Filters = filterMap
	}
	return quadlets.List(ic.ClientCtx, options)
}

func (ic *ContainerEngine) QuadletPrint(_ context.Context, quadlet string) (string, error) {
	return quadlets.Print(ic.ClientCtx, quadlet, nil)
}

func (ic *ContainerEngine) QuadletRemove(_ context.Context, names []string, opts entities.QuadletRemoveOptions) (*entities.QuadletRemoveReport, error) {
	options := new(quadlets.RemoveOptions).
		WithForce(opts.Force).
		WithAll(opts.All).
		WithIgnore(opts.Ignore).
		WithReloadSystemd(opts.ReloadSystemd)

	return quadlets.Remove(ic.ClientCtx, names, options)
}

// resolveInstallPaths resolves pathsOrURLs into a flat list of local file paths
// ready for upload. URLs are downloaded to temp files (in temp directories to
// preserve the original filename). Directories are expanded to their contained
// files. Returns the flat file list, a cleanup function that removes any temp
// dirs, and an error.
func resolveInstallPaths(pathsOrURLs []string) ([]string, func(), error) {
	var result []string
	var tempDirs []string
	cleanup := func() {
		for _, d := range tempDirs {
			if err := os.RemoveAll(d); err != nil {
				logrus.Warnf("Failed to remove temp dir %s: %v", d, err)
			}
		}
	}

	for _, arg := range pathsOrURLs {
		switch {
		case strings.HasPrefix(arg, "http://") || strings.HasPrefix(arg, "https://"):
			tmpFile, err := downloadToTemp(arg)
			if err != nil {
				cleanup()
				return nil, nil, fmt.Errorf("downloading %s: %w", arg, err)
			}
			tempDirs = append(tempDirs, filepath.Dir(tmpFile))
			result = append(result, tmpFile)

		default:
			info, err := os.Stat(arg)
			if err != nil {
				cleanup()
				return nil, nil, fmt.Errorf("cannot stat %s: %w", arg, err)
			}
			if info.IsDir() {
				entries, err := os.ReadDir(arg)
				if err != nil {
					cleanup()
					return nil, nil, fmt.Errorf("reading directory %s: %w", arg, err)
				}
				for _, entry := range entries {
					if !entry.IsDir() {
						result = append(result, filepath.Join(arg, entry.Name()))
					}
				}
			} else {
				result = append(result, arg)
			}
		}
	}

	return result, cleanup, nil
}

func downloadToTemp(fileURL string) (string, error) {
	resp, err := http.Get(fileURL) //nolint:gosec,noctx
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	filename := getFileNameFromResponse(resp, fileURL)

	tmpDir, err := os.MkdirTemp("", "quadlet-download-*")
	if err != nil {
		return "", err
	}

	tmpPath := filepath.Join(tmpDir, filename)
	tmpFile, err := os.Create(tmpPath)
	if err != nil {
		os.RemoveAll(tmpDir)
		return "", err
	}
	defer tmpFile.Close()

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		os.RemoveAll(tmpDir)
		return "", err
	}
	return tmpPath, nil
}

func getFileNameFromResponse(resp *http.Response, fileURL string) string {
	cd := resp.Header.Get("Content-Disposition")
	if cd != "" {
		const prefix = "filename="
		if idx := strings.Index(cd, prefix); idx != -1 {
			filename := cd[idx+len(prefix):]
			filename = strings.Trim(filename, "\"'")
			if filename != "" {
				return filename
			}
		}
	}
	u, err := url.Parse(fileURL)
	if err != nil {
		return "quadlet-download"
	}
	return path.Base(u.Path)
}
