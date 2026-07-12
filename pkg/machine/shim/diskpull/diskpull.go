package diskpull

import (
	"context"
	"strings"

	"go.podman.io/image/v5/types"
	"go.podman.io/podman/v6/pkg/machine/define"
	"go.podman.io/podman/v6/pkg/machine/ocipull"
	"go.podman.io/podman/v6/pkg/machine/stdpull"
)

func GetDisk(userInputPath string, dirs *define.MachineDirs, imagePath *define.VMFile, vmType define.VMType, name string, skipTlsVerify types.OptionalBool) error {
	var (
		err    error
		mydisk ocipull.Disker
	)

	if userInputPath == "" || strings.HasPrefix(userInputPath, "docker://") {
		mydisk, err = ocipull.NewOCIArtifactPull(context.Background(), dirs, userInputPath, name, vmType, imagePath, skipTlsVerify)
	} else {
		if strings.HasPrefix(userInputPath, "http") {
			// TODO probably should use tempdir instead of datadir
			mydisk, err = stdpull.NewDiskFromURL(userInputPath, imagePath, dirs.DataDir, nil, false)
		} else {
			mydisk, err = stdpull.NewStdDiskPull(userInputPath, imagePath)
		}
	}
	if err != nil {
		return err
	}

	return mydisk.Get()
}
