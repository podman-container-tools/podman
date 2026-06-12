package e2e_test

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"go.podman.io/image/v5/types"
	"go.podman.io/podman/v6/pkg/machine/compression"
	"go.podman.io/podman/v6/pkg/machine/define"
	"go.podman.io/podman/v6/pkg/machine/ocipull"
)

func pullOCITestDisk(finalDir string, vmType define.VMType) error {
	imageCacheDir, err := define.NewMachineFile(finalDir, nil)
	if err != nil {
		return err
	}
	unusedFinalPath, err := imageCacheDir.AppendToNewVMFile(fmt.Sprintf("machinetest-%s", runtime.GOOS), nil)
	if err != nil {
		return err
	}
	dirs := define.MachineDirs{ImageCacheDir: imageCacheDir}

	var skipTlsVerify types.OptionalBool
	ociArtPull, err := ocipull.NewOCIArtifactPull(context.Background(), &dirs, "docker://quay.io/podman/machine-os", "e2emachine", vmType, unusedFinalPath, skipTlsVerify)
	if err != nil {
		return err
	}
	_, err = ociArtPull.GetNoCompress()
	if err != nil {
		return err
	}
	fp, originalName := ociArtPull.OriginalFileName()
	// Rename the download to something we recognize
	compressionExt := filepath.Ext(fp)
	fqImageName = filepath.Join(tmpDir, strings.TrimSuffix(originalName, compressionExt))
	suiteImageName = filepath.Base(fqImageName)
	compressedImage, err := define.NewMachineFile(fp, nil)
	if err != nil {
		return err
	}

	if err := os.Chmod(fp, 0o400); err != nil {
		return err
	}
	return compression.Decompress(compressedImage, fqImageName)
}
