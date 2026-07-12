package farm

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/hashicorp/go-multierror"
	"github.com/sirupsen/logrus"
	"go.podman.io/image/v5/docker"
	"go.podman.io/image/v5/types"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

type listBuilderOptions struct {
	cleanup       bool
	iidFile       string
	iidFileRaw    string
	authfile      string
	skipTLSVerify *bool
}

type listLocal struct {
	listName    string
	localEngine entities.ImageEngine
	options     listBuilderOptions
}

// newManifestListBuilder returns a manifest list builder which saves a
// manifest list and images to local storage.
func newManifestListBuilder(listName string, localEngine entities.ImageEngine, options listBuilderOptions) *listLocal {
	return &listLocal{
		listName:    listName,
		options:     options,
		localEngine: localEngine,
	}
}

// Build retrieves images from the build reports and assembles them into a
// manifest list in local container storage.
func (l *listLocal) build(ctx context.Context, images map[entities.BuildReport]entities.ImageEngine) (string, error) {
	// Set skipTLSVerify based on whether it was changed by the caller
	skipTLSVerify := types.OptionalBoolUndefined
	if l.options.skipTLSVerify != nil {
		skipTLSVerify = types.NewOptionalBool(*l.options.skipTLSVerify)
	}

	exists, err := l.localEngine.ManifestExists(ctx, l.listName)
	if err != nil {
		return "", err
	}
	// Create list if it doesn't exist
	if !exists.Value {
		_, err = l.localEngine.ManifestCreate(ctx, l.listName, []string{}, entities.ManifestCreateOptions{SkipTLSVerify: skipTLSVerify})
		if err != nil {
			return "", fmt.Errorf("creating manifest list %q: %w", l.listName, err)
		}
	}

	// Push the images to the registry given by the user
	var (
		pushGroup multierror.Group
		refsMutex sync.Mutex
	)
	refs := []string{}
	for image, engine := range images {
		pushGroup.Go(func() error {
			logrus.Infof("pushing image %s", image.ID)
			defer logrus.Infof("pushed image %s", image.ID)
			// Push the image to the registry
			report, err := engine.Push(ctx, image.ID, l.listName+docker.UnknownDigestSuffix, entities.ImagePushOptions{Authfile: l.options.authfile, Quiet: false, SkipTLSVerify: skipTLSVerify})
			if err != nil {
				return fmt.Errorf("pushing image %q to registry: %w", image, err)
			}
			refsMutex.Lock()
			defer refsMutex.Unlock()
			refs = append(refs, "docker://"+l.listName+"@"+report.ManifestDigest)
			return nil
		})
	}
	pushErrors := pushGroup.Wait()
	err = pushErrors.ErrorOrNil()
	if err != nil {
		return "", fmt.Errorf("building: %w", err)
	}

	if l.options.cleanup {
		var rmGroup multierror.Group
		for image, engine := range images {
			if engine.FarmNodeName(ctx) == entities.LocalFarmImageBuilderName {
				continue
			}
			rmGroup.Go(func() error {
				_, err := engine.Remove(ctx, []string{image.ID}, entities.ImageRemoveOptions{})
				if len(err) > 0 {
					return err[0]
				}
				return nil
			})
		}
		rmErrors := rmGroup.Wait()
		if rmErrors != nil {
			if err = rmErrors.ErrorOrNil(); err != nil {
				return "", fmt.Errorf("removing intermediate images: %w", err)
			}
		}
	}

	// Clear the list in the event it already existed
	if exists.Value {
		_, err = l.localEngine.ManifestListClear(ctx, l.listName)
		if err != nil {
			return "", fmt.Errorf("error clearing list %q", l.listName)
		}
	}

	// Add the images to the list
	listID, err := l.localEngine.ManifestAdd(ctx, l.listName, refs, entities.ManifestAddOptions{Authfile: l.options.authfile, SkipTLSVerify: skipTLSVerify})
	if err != nil {
		return "", fmt.Errorf("adding images %q to list: %w", refs, err)
	}
	_, err = l.localEngine.ManifestPush(ctx, l.listName, l.listName, entities.ImagePushOptions{Authfile: l.options.authfile, SkipTLSVerify: skipTLSVerify})
	if err != nil {
		return "", err
	}

	// Write the manifest list's ID file if we're expected to
	if l.options.iidFile != "" {
		if err := os.WriteFile(l.options.iidFile, []byte("sha256:"+listID), 0o644); err != nil {
			return "", err
		}
	}
	if l.options.iidFileRaw != "" {
		if err := os.WriteFile(l.options.iidFileRaw, []byte(listID), 0o644); err != nil {
			return "", err
		}
	}

	return l.listName, nil
}
