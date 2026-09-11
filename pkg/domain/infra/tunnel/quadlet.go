package tunnel

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	"go.podman.io/podman/v6/pkg/bindings/quadlets"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

func (ic *ContainerEngine) QuadletExists(_ context.Context, name string) (*entities.BoolReport, error) {
	exists, err := quadlets.Exists(ic.ClientCtx, name, nil)
	if err != nil {
		return nil, err
	}
	return &entities.BoolReport{Value: exists}, nil
}

func (ic *ContainerEngine) QuadletInstall(_ context.Context, pathsOrURLs []string, opts entities.QuadletInstallOptions) (*entities.QuadletInstallReport, error) {
	options := new(quadlets.InstallOptions).
		WithReplace(opts.Replace).
		WithReloadSystemd(opts.ReloadSystemd)

	if opts.Application != "" {
		options = options.WithApplication(opts.Application)
	}

	files, cleanup, err := resolveInstallPaths(pathsOrURLs, opts.Application)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	filePaths := make([]string, len(files))
	fileNames := make([]string, len(files))
	for i, f := range files {
		filePaths[i] = f.localPath
		fileNames[i] = f.uploadName
	}

	return quadlets.Install(ic.ClientCtx, filePaths, fileNames, options)
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
		WithReloadSystemd(opts.ReloadSystemd).
		WithRecursive(opts.Recursive)

	return quadlets.Remove(ic.ClientCtx, names, options)
}

type installFile struct {
	localPath  string
	uploadName string
}

// resolveInstallPaths resolves pathsOrURLs into a list of local file paths
// paired with their upload names. URLs are downloaded to temp files.
// Directories are walked recursively, preserving relative paths.
func resolveInstallPaths(pathsOrURLs []string, application string) ([]installFile, func(), error) {
	var result []installFile
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
			result = append(result, installFile{
				localPath:  tmpFile,
				uploadName: filepath.Base(tmpFile),
			})

		default:
			info, err := os.Stat(arg)
			if err != nil {
				cleanup()
				return nil, nil, fmt.Errorf("cannot stat %s: %w", arg, err)
			}
			if info.IsDir() {
				if application == "" {
					cleanup()
					return nil, nil, fmt.Errorf("application name cannot be empty when installing from directory")
				}
				base := arg
				err := filepath.WalkDir(arg, func(path string, d os.DirEntry, err error) error {
					if err != nil {
						return err
					}
					if d.IsDir() {
						return nil
					}
					rel, relErr := filepath.Rel(base, path)
					if relErr != nil {
						return relErr
					}
					result = append(result, installFile{
						localPath:  path,
						uploadName: rel,
					})
					return nil
				})
				if err != nil {
					cleanup()
					return nil, nil, fmt.Errorf("walking directory %s: %w", arg, err)
				}
			} else {
				result = append(result, installFile{
					localPath:  arg,
					uploadName: filepath.Base(arg),
				})
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

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("downloading %s: %s", fileURL, resp.Status)
	}

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
	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		_, params, err := mime.ParseMediaType(cd)
		if err == nil {
			if filename := params["filename"]; filename != "" {
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
