package tmpdir

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/sirupsen/logrus"
	"go.podman.io/buildah/internal/ctxreader"
	"go.podman.io/buildah/internal/httpclient"
	"go.podman.io/buildah/internal/urlsource"
	"go.podman.io/storage/pkg/chrootarchive"
)

type URLOptions = httpclient.URLOptions

// ForURL checks if the passed-in string looks like a URL or "-".  If it is,
// ForURL creates a temporary directory, arranges for its contents to be the
// contents of that URL, and returns the temporary directory's path (for
// cleanup) and a relative subdirectory to the build context within it.
// Removal of the temporary directory is the responsibility of the caller.  If
// the string doesn't look like a URL or "-", ForURL returns empty strings and
// a nil error code.
func ForURL(ctx context.Context, dir, prefix, url string, options *URLOptions) (tempDir, relativeContextDir string, err error) {
	select {
	case <-ctx.Done():
		return "", "", ctx.Err()
	default:
	}

	if options == nil {
		options = &URLOptions{}
	}

	if !urlsource.IsHTTPOrHTTPS(url) &&
		!strings.HasPrefix(url, "git://") &&
		!strings.HasPrefix(url, "github.com/") &&
		url != "-" {
		return "", "", nil
	}
	tempDir, err = os.MkdirTemp(dir, prefix)
	if err != nil {
		return "", "", fmt.Errorf("creating temporary directory for %q: %w", url, err)
	}
	succeeded := false
	defer func() {
		if !succeeded {
			if err2 := os.RemoveAll(tempDir); err2 != nil {
				logrus.Errorf("error removing temporary directory %q: %v", tempDir, err2)
			}
		}
	}()

	downloadDir := filepath.Join(tempDir, "download")
	if err = os.MkdirAll(downloadDir, 0o700); err != nil {
		return "", "", fmt.Errorf("creating directory %q for %q: %w", downloadDir, url, err)
	}

	var contentSubdir string
	urlParsed, parseErr := neturl.Parse(url)
	if parseErr != nil {
		return "", "", fmt.Errorf("parsing url %q: %w", url, parseErr)
	}

	isGitURL := urlParsed.Scheme == "git" || strings.HasSuffix(urlParsed.Path, ".git")
	switch {
	case isGitURL:
		combinedOutput, gitSubDir, cloneErr := cloneToDirectory(ctx, url, downloadDir)
		if cloneErr != nil {
			return "", "", fmt.Errorf("cloning %q to %q:\n%s: %w", url, tempDir, string(combinedOutput), cloneErr)
		}
		contentSubdir = gitSubDir
	case urlsource.IsHTTPOrHTTPS(url):
		if err = downloadToDirectory(ctx, options, url, downloadDir); err != nil {
			return "", "", err
		}
	case strings.HasPrefix(url, "github.com/"):
		ghURL := url
		contentSubdir = path.Base(ghURL) + "-master"
		downloadURL := fmt.Sprintf("https://%s/archive/master.tar.gz", ghURL)
		logrus.Debugf("resolving url %q to %q", ghURL, downloadURL)
		if err = downloadToDirectory(ctx, options, downloadURL, downloadDir); err != nil {
			return "", "", err
		}
	case url == "-":
		if err = stdinToDirectory(ctx, downloadDir); err != nil {
			return "", "", err
		}
	}

	contextDir, err := securejoin.SecureJoin(downloadDir, contentSubdir)
	if err != nil {
		return "", "", fmt.Errorf("resolving subdirectory %q in %q: %w", contentSubdir, downloadDir, err)
	}
	relativeContextDir, err = filepath.Rel(tempDir, contextDir)
	if err != nil {
		return "", "", err
	}
	logrus.Debugf("Build context is at %q", contextDir)
	succeeded = true
	return tempDir, relativeContextDir, nil
}

// parseGitBuildContext parses git build context to `repo`, `sub-dir`
// `branch/commit`, accepts GitBuildContext in the format of
// `repourl.git[#[branch-or-commit]:subdir]`.
func parseGitBuildContext(url string) (string, string, string) {
	gitSubdir := ""
	gitBranch := ""
	gitBranchPart := strings.Split(url, "#")
	if len(gitBranchPart) > 1 {
		// check if string contains path to a subdir
		gitSubDirPart := strings.Split(gitBranchPart[1], ":")
		if len(gitSubDirPart) > 1 {
			gitSubdir = gitSubDirPart[1]
		}
		gitBranch = gitSubDirPart[0]
	}
	return gitBranchPart[0], gitSubdir, gitBranch
}

func cloneToDirectory(ctx context.Context, url, dir string) ([]byte, string, error) {
	select {
	case <-ctx.Done():
		return nil, "", ctx.Err()
	default:
	}

	var cmd *exec.Cmd
	gitRepo, gitSubdir, gitRef := parseGitBuildContext(url)
	// init repo
	cmd = exec.CommandContext(ctx, "git", "init", dir)
	combinedOutput, err := cmd.CombinedOutput()
	if err != nil {
		// Return err.Error() instead of err as we want buildah to override error code with more predictable
		// value.
		return combinedOutput, gitSubdir, fmt.Errorf("failed while performing `git init`: %s", err.Error())
	}
	// add origin
	cmd = exec.CommandContext(ctx, "git", "remote", "add", "origin", gitRepo)
	cmd.Dir = dir
	combinedOutput, err = cmd.CombinedOutput()
	if err != nil {
		// Return err.Error() instead of err as we want buildah to override error code with more predictable
		// value.
		return combinedOutput, gitSubdir, fmt.Errorf("failed while performing `git remote add`: %s", err.Error())
	}

	logrus.Debugf("fetching repo %q and branch (or commit ID) %q to %q", gitRepo, gitRef, dir)
	args := []string{"fetch", "-u", "--depth=1", "origin", "--", gitRef}
	cmd = exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	combinedOutput, err = cmd.CombinedOutput()
	if err != nil {
		// Return err.Error() instead of err as we want buildah to override error code with more predictable
		// value.
		return combinedOutput, gitSubdir, fmt.Errorf("failed while performing `git fetch`: %s", err.Error())
	}

	cmd = exec.CommandContext(ctx, "git", "checkout", "FETCH_HEAD")
	cmd.Dir = dir
	combinedOutput, err = cmd.CombinedOutput()
	if err != nil {
		// Return err.Error() instead of err as we want buildah to override error code with more predictable
		// value.
		return combinedOutput, gitSubdir, fmt.Errorf("failed while performing `git checkout`: %s", err.Error())
	}
	return combinedOutput, gitSubdir, nil
}

func downloadToDirectory(ctx context.Context, options *URLOptions, url, dir string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	logrus.Debugf("extracting %q to %q", url, dir)
	httpClient, err := httpclient.ForURLOptions(*options)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("invalid response status %d", resp.StatusCode)
	}
	if resp.ContentLength == 0 {
		return fmt.Errorf("no contents in %q", url)
	}
	// Try to extract the response as a tar archive; if that fails,
	// assume it is a raw Dockerfile and write it as such.
	if err := chrootarchive.Untar(resp.Body, dir, nil); err != nil {
		req1, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		resp1, err := http.DefaultClient.Do(req1)
		if err != nil {
			return err
		}
		defer resp1.Body.Close()
		body, err := io.ReadAll(resp1.Body)
		if err != nil {
			return err
		}
		if err := writeFileInRoot(dir, "Dockerfile", body, 0o600); err != nil {
			return fmt.Errorf("failed to write %q to %q: %w", url, filepath.Join(dir, "Dockerfile"), err)
		}
	}
	return nil
}

func stdinToDirectory(ctx context.Context, dir string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	logrus.Debugf("extracting stdin to %q", dir)
	r := bufio.NewReader(os.Stdin)
	b, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("failed to read from stdin: %w", err)
	}
	// Try to extract the buffered input as a tar archive; if that fails,
	// assume it is a raw Dockerfile and write it as such.
	reader := bytes.NewReader(b)
	if err := chrootarchive.Untar(ctxreader.NewCancelableReader(ctx, reader), dir, nil); err != nil {
		if err := writeFileInRoot(dir, "Dockerfile", b, 0o600); err != nil {
			return fmt.Errorf("failed to write bytes to %q: %w", filepath.Join(dir, "Dockerfile"), err)
		}
	}
	return nil
}

// writeFileInRoot safely writes data to a file inside root, without following
// symlinks that escape the root directory.
func writeFileInRoot(root, name string, data []byte, perm os.FileMode) error { //nolint:unparam,nolintlint
	// Above:
	// unparam: 'name' currently only receives "Dockerfile" but will potentially support other files later
	// nolintlint: the unparam linter only triggers if there are ≥ 4 instances; we do have that
	// with --tests defaulting to true, but not with --tests=false.

	rootHandle, err := os.OpenRoot(root)
	if err != nil {
		return err
	}
	defer rootHandle.Close()

	if err := rootHandle.Remove(name); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	fileHandle, err := rootHandle.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, perm)
	if err != nil {
		return err
	}

	_, err = fileHandle.Write(data)
	if closeErr := fileHandle.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	return err
}
