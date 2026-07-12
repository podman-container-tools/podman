package ocipull

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/opencontainers/go-digest"
	specV1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
	"go.podman.io/image/v5/image"
	"go.podman.io/image/v5/manifest"
	"go.podman.io/image/v5/oci/layout"
	"go.podman.io/image/v5/types"
)

func GetLocalBlob(ctx context.Context, path string) (*types.BlobInfo, error) {
	ociRef, err := layout.ParseReference(path)
	if err != nil {
		return nil, err
	}
	img, err := ociRef.NewImage(ctx, &types.SystemContext{})
	if err != nil {
		return nil, err
	}

	b, _, err := img.Manifest(ctx)
	if err != nil {
		return nil, err
	}

	localManifest := specV1.Manifest{}
	if err := json.Unmarshal(b, &localManifest); err != nil {
		return nil, err
	}
	blobs := img.LayerInfos()
	if len(blobs) != 1 {
		return nil, errors.New("invalid disk image")
	}
	fmt.Println(blobs[0].Digest.Hex())
	return &blobs[0], nil
}

func GetDiskArtifactReference(ctx context.Context, imgSrc types.ImageSource, opts *DiskArtifactOpts) (digest.Digest, error) {
	rawMannyFest, mannyType, err := image.UnparsedInstance(imgSrc, nil).Manifest(ctx)
	if err != nil {
		return "", err
	}

	if !manifest.MIMETypeIsMultiImage(mannyType) { // if not true
		return "", fmt.Errorf("wrong manifest type for disk artifact: %s", mannyType)
	}

	mannyFestList, err := manifest.ListFromBlob(rawMannyFest, mannyType)
	if err != nil {
		return "", fmt.Errorf("failed to parse manifest list from blob: %q", err)
	}

	var artifactDigest digest.Digest
	for _, d := range mannyFestList.Instances() {
		bar, err := mannyFestList.Instance(d)
		if err != nil {
			return "", err
		}
		val, ok := bar.ReadOnly.Annotations["disktype"]
		if !ok { // quick exit, no type match
			continue
		}
		// wrong arch
		if bar.ReadOnly.Platform.Architecture != opts.arch {
			continue
		}
		// wrong os
		if bar.ReadOnly.Platform.OS != opts.os {
			continue
		}
		// wrong disktype
		if val != opts.diskType {
			continue
		}

		// ok, we have a match
		artifactDigest = d
		logrus.Debugf("found image in digest: %q", artifactDigest.String())
		break
	}
	if artifactDigest == "" {
		return "", fmt.Errorf("no valid disk artifact found")
	}
	v1RawMannyfest, _, err := image.UnparsedInstance(imgSrc, &artifactDigest).Manifest(ctx)
	if err != nil {
		return "", err
	}
	v1MannyFest := specV1.Manifest{}
	if err := json.Unmarshal(v1RawMannyfest, &v1MannyFest); err != nil {
		return "", err
	}
	if layerLen := len(v1MannyFest.Layers); layerLen > 1 {
		return "", fmt.Errorf("podman-machine images should only have 1 layer: %d found", layerLen)
	}

	// podman-machine-images should have a original file name
	// stored in the annotations under org.opencontainers.image.title
	// i.e. fedora-coreos-39.20240128.2.2-qemu.x86_64.qcow2.xz
	originalFileName, ok := v1MannyFest.Layers[0].Annotations[specV1.AnnotationTitle]
	if !ok {
		return "", fmt.Errorf("unable to determine original artifact name: missing required annotation '%s'", specV1.AnnotationTitle)
	}
	logrus.Debugf("original artifact file name: %s", originalFileName)
	return artifactDigest, err
}
