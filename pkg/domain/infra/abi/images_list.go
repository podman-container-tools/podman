//go:build !remote && (linux || freebsd)

package abi

import (
	"context"
	"fmt"
	"slices"

	"go.podman.io/common/libimage"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

func (ir *ImageEngine) List(ctx context.Context, opts entities.ImageListOptions) ([]*entities.ImageSummary, error) {
	listImagesOptions := &libimage.ListImagesOptions{
		Filters:     opts.Filter,
		SetListData: true,
	}
	if !opts.All && !slices.Contains(listImagesOptions.Filters, "intermediate=true") {
		// Filter intermediate images unless we want to list *all*.
		// NOTE: it's a positive filter, so `intermediate=false` means
		// to display non-intermediate images.
		listImagesOptions.Filters = append(listImagesOptions.Filters, "intermediate=false")
	}

	images, err := ir.Libpod.LibimageRuntime().ListImages(ctx, listImagesOptions)
	if err != nil {
		return nil, err
	}

	summaries := []*entities.ImageSummary{}
	for _, img := range images {
		summary, err := func() (*entities.ImageSummary, error) {
			repoDigests, err := img.RepoDigests()
			if err != nil {
				return nil, fmt.Errorf("getting repoDigests from image %q: %w", img.ID(), err)
			}

			if img.ListData.IsDangling == nil { // Sanity check
				return nil, fmt.Errorf("%w: ListData.IsDangling is nil but should not be", define.ErrInternal)
			}
			isDangling := *img.ListData.IsDangling
			parentID := ""
			if img.ListData.Parent != nil {
				parentID = img.ListData.Parent.ID()
			}

			s := &entities.ImageSummary{
				ID:          img.ID(),
				Created:     img.Created().Unix(),
				Dangling:    isDangling,
				Digest:      string(img.Digest()),
				RepoDigests: repoDigests,
				History:     img.NamesHistory(),
				Names:       img.Names(),
				ReadOnly:    img.IsReadOnly(),
				SharedSize:  0,
				RepoTags:    img.Names(), // may include tags and digests
				ParentId:    parentID,
			}
			if opts.ExtendedAttributes {
				iml, err := img.IsManifestList(ctx)
				if err != nil {
					return nil, err
				}
				s.IsManifestList = &iml
				if !iml {
					imgData, err := img.Inspect(ctx, nil)
					if err != nil {
						return nil, err
					}
					s.Arch = imgData.Architecture
					s.Os = imgData.Os
				}
			}
			s.Labels, err = img.Labels(ctx)
			if err != nil {
				return nil, fmt.Errorf("retrieving label for image %q: you may need to remove the image to resolve the error: %w", img.ID(), err)
			}

			ctnrs, err := img.Containers()
			if err != nil {
				return nil, fmt.Errorf("retrieving containers for image %q: you may need to remove the image to resolve the error: %w", img.ID(), err)
			}
			s.Containers = len(ctnrs)

			sz, err := img.Size()
			if err != nil {
				return nil, fmt.Errorf("retrieving size of image %q: you may need to remove the image to resolve the error: %w", img.ID(), err)
			}
			s.Size = sz
			// This is good enough for now, but has to be
			// replaced later with correct calculation logic
			s.VirtualSize = sz
			return s, nil
		}()
		if err != nil {
			if libimage.ErrorIsImageUnknown(err) {
				// The image may have been (partially) removed in the meantime
				continue
			}
			return nil, err
		}
		summaries = append(summaries, summary)
	}
	return summaries, nil
}
