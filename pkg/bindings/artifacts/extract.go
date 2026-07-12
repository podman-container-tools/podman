package artifacts

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"

	"go.podman.io/podman/v6/pkg/bindings"
)

func Extract(ctx context.Context, artifactName string, target string, options *ExtractOptions) error {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return fmt.Errorf("getting client: %w", err)
	}

	if options == nil {
		options = new(ExtractOptions)
	}

	// Check if target is a directory to know if we can copy more than one blob
	targetIsDirectory := false
	stat, err := os.Stat(target)
	if err == nil {
		targetIsDirectory = stat.IsDir()
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("stat target %q failed: %w", target, err)
	}

	// If the target is not a directory, request API to return the blob without title.
	// If a blob has a malicious title it will be returned from the API without it
	// as the file will be written to the provided target
	if !targetIsDirectory {
		options.WithExcludeTitle(true)
	}

	params, err := options.ToParams()
	if err != nil {
		return fmt.Errorf("converting options to params: %w", err)
	}

	response, err := conn.DoRequest(ctx, nil, http.MethodGet, "/artifacts/%s/extract", params, nil, artifactName)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if !response.IsSuccess() {
		return response.Process(nil)
	}

	multipleBlobs := false
	tr := tar.NewReader(response.Body)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			return err
		}

		if !targetIsDirectory && multipleBlobs {
			return fmt.Errorf("the artifact consists of several blobs and the target %q is not a directory and neither digest or title was specified to only copy a single blob", target)
		}

		// If destination isn't a file, extract to target/filename
		fileTarget := target
		if targetIsDirectory {
			filename := header.Name
			// This matches the logic from generateArtifactBlobName().
			for i := range len(filename) {
				if os.IsPathSeparator(filename[i]) {
					return fmt.Errorf("invalid filename: %q cannot contain %c", filename, filename[i])
				}
			}
			fileTarget = filepath.Join(target, filename)
		}

		if header.Typeflag == tar.TypeReg {
			err = extractFile(tr, fileTarget)
			if err != nil {
				return err
			}
		}

		// Signal that the first blob has been extracted so we can return an error if more
		// than one blob are being extracted when target is not a directory.
		multipleBlobs = true
	}
	return nil
}

func extractFile(tr *tar.Reader, fileTarget string) (retErr error) {
	outFile, err := os.Create(fileTarget)
	if err != nil {
		return err
	}

	// Use an anonymous function to enable capturing the error from
	// outFile.Close() upon returning.
	defer func() {
		cErr := outFile.Close()
		if retErr == nil {
			retErr = cErr
		}
	}()

	_, err = io.Copy(outFile, tr)
	if err != nil {
		return fmt.Errorf("failed to extract blob to %s: %w", fileTarget, err)
	}
	return nil
}
