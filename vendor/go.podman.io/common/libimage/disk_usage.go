//go:build !remote

package libimage

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"go.podman.io/storage"
)

// ImageDiskUsage reports the total size of an image.  That is the size.
type ImageDiskUsage struct {
	// Number of containers using the image.
	Containers int
	// ID of the image.
	ID string
	// Repository of the image.
	Repository string
	// Tag of the image.
	Tag string
	// Created time stamp.
	Created time.Time
	// The amount of space that an image shares with another one (i.e. their common data).
	SharedSize int64
	// The the amount of space that is only used by a given image.
	UniqueSize int64
	// Sum of shared an unique size.
	Size int64
}

// hasSpaceLimits returns true if any of the space limits is set.
func (o *RemoveImagesOptions) hasSpaceLimits() bool {
	return o.ReservedSpace != 0 || o.MaxUsedSpace != 0 || o.MinFreeSpace != 0
}

// calculateKeepBytes returns how many bytes of the prunable images to keep, given
// their total size and the free space on the graph root's file system.
func calculateKeepBytes(totalSize, free int64, o *RemoveImagesOptions) int64 {
	// Try to keep as much as allowed
	keep := o.MaxUsedSpace

	// if we need to free up space, decrease it
	if excess := o.MinFreeSpace - free; excess > 0 {
		if keep == 0 {
			keep = totalSize - excess
		} else {
			keep = min(keep, totalSize-excess)
		}
	} else if o.MinFreeSpace != 0 && keep == 0 {
		// Only MinFreeSpace is set and it is already satisfied, so
		// there is nothing to remove
		keep = totalSize
	}

	return max(keep, o.ReservedSpace)
}

// trimToSpaceLimits narrows the prune candidates down to the oldest ones whose removal
// brings their combined size down to the limits in o.  Only the space that removing
// the candidates would free counts toward the limits.  Since containers/storage does
// not record when an image was last used, candidates are ordered by creation time.
// Unless NoPrune is set, removing a candidate may also remove it dangling parents, so
// more space than needed can be freed.
func (r *Runtime) trimToSpaceLimits(candidates []*Image, o *RemoveImagesOptions) ([]*Image, error) {
	if len(candidates) == 0 {
		return nil, nil
	}

	images, layers, err := r.getImagesAndLayers()
	if err != nil {
		return nil, err
	}

	layerMap := make(map[string]*storage.Layer, len(layers))
	for i := range layers {
		layer := &layers[i]
		if layer.UncompressedSize == -1 {
			size, err := r.store.DiffSize("", layer.ID)
			if err != nil {
				return nil, err
			}
			layer.UncompressedSize = size
		}
		layerMap[layer.ID] = layer
	}

	// Count how many images use each layer.
	layerCount := make(map[string]int)
	for _, img := range images {
		walkImageLayers(img, layerMap, func(layer *storage.Layer) {
			layerCount[layer.ID]++
		})
	}

	// The prunable size is what removing every candidate would free: layers
	// that no other image uses, plus each candidate's big data.
	scratch := maps.Clone(layerCount)
	var totalSize int64
	for _, img := range candidates {
		walkImageLayers(img, layerMap, func(layer *storage.Layer) {
			if scratch[layer.ID] > 0 {
				scratch[layer.ID]--
				if scratch[layer.ID] == 0 {
					totalSize += layer.UncompressedSize
				}
			}
		})
		for _, size := range img.storageImage.BigDataSizes {
			totalSize += size
		}
	}

	var free int64
	if o.MinFreeSpace != 0 {
		free, err = diskFree(r.store.GraphRoot())
		if err != nil {
			return nil, fmt.Errorf("determining free space of %q: %w", r.store.GraphRoot(), err)
		}
	}

	keep := calculateKeepBytes(totalSize, free, o)
	if totalSize <= keep {
		return nil, nil
	}

	slices.SortFunc(candidates, func(a, b *Image) int {
		return cmp.Or(a.Created().Compare(b.Created()), strings.Compare(a.ID(), b.ID()))
	})

	// Space is counted as freed when an image is selected. If removing it later
	// fails (it is in use), less space is freed than planned.
	var selected []*Image
	for _, img := range candidates {
		if totalSize <= keep {
			break
		}
		// Removing an image only frees the layers no other image uses
		walkImageLayers(img, layerMap, func(layer *storage.Layer) {
			if layerCount[layer.ID] > 0 {
				layerCount[layer.ID]--
				if layerCount[layer.ID] == 0 {
					totalSize -= layer.UncompressedSize
				}
			}
		})
		for _, size := range img.storageImage.BigDataSizes {
			totalSize -= size
		}
		selected = append(selected, img)
	}
	return selected, nil
}

// DiskUsage calculates the disk usage for each image in the local containers
// storage.  Note that a single image may yield multiple usage reports, one for
// each repository tag.
func (r *Runtime) DiskUsage(ctx context.Context) ([]ImageDiskUsage, int64, error) {
	images, layers, err := r.getImagesAndLayers()
	if err != nil {
		return nil, -1, err
	}

	var totalSize int64
	layerMap := make(map[string]*storage.Layer)
	for _, layer := range layers {
		layerMap[layer.ID] = &layer
		if layer.UncompressedSize == -1 {
			// size is unknown, we must manually diff the layer size which
			// can be quite slow as it might have to walk all files
			size, err := r.store.DiffSize("", layer.ID)
			if err != nil {
				return nil, -1, err
			}
			// cache the size now
			layer.UncompressedSize = size
		}
		// count the total layer size here so we know we only count each layer once
		totalSize += layer.UncompressedSize
	}

	// First walk all images to count how often each layer is used.
	// This is done so we know if the size for an image is shared between
	// images that use the same layer or unique.
	layerCount := make(map[string]int)
	for _, image := range images {
		walkImageLayers(image, layerMap, func(layer *storage.Layer) {
			// Increment the count for each layer visit
			layerCount[layer.ID] += 1
		})
	}

	// Now that we actually have all the info walk again to add the sizes.
	var allUsages []ImageDiskUsage
	for _, image := range images {
		usages, err := diskUsageForImage(ctx, image, layerMap, layerCount, &totalSize)
		if err != nil {
			// The image can be removed by a parallel process between the
			// listing above and reading it here. That is not a corruption,
			// skip it instead of failing the whole disk usage.
			if errors.Is(err, storage.ErrImageUnknown) {
				continue
			}
			return nil, -1, err
		}
		allUsages = append(allUsages, usages...)
	}
	return allUsages, totalSize, err
}

// diskUsageForImage returns the disk-usage baseistics for the specified image.
func diskUsageForImage(ctx context.Context, image *Image, layerMap map[string]*storage.Layer, layerCount map[string]int, totalSize *int64) ([]ImageDiskUsage, error) {
	if err := image.isCorrupted(ctx, ""); err != nil {
		return nil, err
	}

	base := ImageDiskUsage{
		ID:         image.ID(),
		Created:    image.Created(),
		Repository: "<none>",
		Tag:        "<none>",
	}

	walkImageLayers(image, layerMap, func(layer *storage.Layer) {
		// If the layer used by more than one image it shares its size
		if layerCount[layer.ID] > 1 {
			base.SharedSize += layer.UncompressedSize
		} else {
			base.UniqueSize += layer.UncompressedSize
		}
	})

	// Count the image specific big data as well.
	// store.BigDataSize() is not used intentionally, it is slower (has to take
	// locks) and can fail.
	// BigDataSizes is always correctly populated on new stores since c/storage
	// commit a7d7fe8c9a (2016). It should be safe to assume that no such old
	// store+image exist now so we don't bother. Worst case we report a few
	// bytes to little.
	for _, size := range image.storageImage.BigDataSizes {
		base.UniqueSize += size
		*totalSize += size
	}

	base.Size = base.SharedSize + base.UniqueSize

	// Number of containers using the image.
	containers, err := image.Containers()
	if err != nil {
		return nil, err
	}
	base.Containers = len(containers)

	repoTags, err := image.NamedRepoTags()
	if err != nil {
		return nil, err
	}

	if len(repoTags) == 0 {
		return []ImageDiskUsage{base}, nil
	}

	pairs, err := ToNameTagPairs(repoTags)
	if err != nil {
		return nil, err
	}

	results := make([]ImageDiskUsage, len(pairs))
	for i, pair := range pairs {
		res := base
		res.Repository = pair.Name
		res.Tag = pair.Tag
		results[i] = res
	}

	return results, nil
}

// walkImageLayers walks all layers in an image and calls the given function for each layer.
func walkImageLayers(image *Image, layerMap map[string]*storage.Layer, f func(layer *storage.Layer)) {
	visited := make(map[string]struct{})
	// Layers are walked recursively until it has no parent which means we reached the end.
	// We must account for the fact that an image might have several top layers when id mappings are used.
	layers := append([]string{image.storageImage.TopLayer}, image.storageImage.MappedTopLayers...)
	for _, layerID := range layers {
		for layerID != "" {
			layer := layerMap[layerID]
			if layer == nil {
				logrus.Errorf("Local Storage is corrupt, layer %q missing from the storage", layerID)
				break
			}
			if _, ok := visited[layerID]; ok {
				// We have seen this layer before. Break here to
				// a) Do not count the same layer twice that was shared between
				//    the TopLayer and MappedTopLayers layer chain.
				// b) Prevent infinite loops, should not happen per c/storage
				//    design but it is good to be safer.
				break
			}
			visited[layerID] = struct{}{}

			f(layer)
			// Set the layer for the next iteration, parent is empty if we reach the end.
			layerID = layer.Parent
		}
	}
}
