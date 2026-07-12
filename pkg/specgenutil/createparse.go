package specgenutil

import (
	"errors"

	"go.podman.io/common/pkg/config"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

// validate determines if the flags and values given by the user are valid. things checked
// by validate must not need any state information on the flag (i.e. changed)
func validate(c *entities.ContainerCreateOptions) error {
	var ()
	if c.Rm && (c.Restart != "" && c.Restart != "no" && c.Restart != "on-failure") {
		return errors.New(`the --rm option conflicts with --restart, when the restartPolicy is not "" and "no"`)
	}

	if _, err := config.ParsePullPolicy(c.Pull); err != nil {
		return err
	}

	return config.ValidateImageVolumeMode(c.ImageVolume)
}
