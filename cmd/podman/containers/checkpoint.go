package containers

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/utils"
	"go.podman.io/podman/v6/cmd/podman/validate"
	"go.podman.io/podman/v6/pkg/criu"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/rootless"
	"go.podman.io/storage/pkg/archive"
)

var (
	checkpointDescription = `
   podman container checkpoint

   Checkpoints one or more running containers. The container name or ID can be used.
`
	checkpointCommand = &cobra.Command{
		Use:   "checkpoint [options] CONTAINER [CONTAINER...]",
		Short: "Checkpoint one or more containers",
		Long:  checkpointDescription,
		RunE:  checkpoint,
		Args: func(cmd *cobra.Command, args []string) error {
			return validate.CheckAllLatestAndIDFile(cmd, args, false, "")
		},
		ValidArgsFunction: common.AutocompleteContainersRunning,
		Example: `podman container checkpoint --keep ctrID
podman container checkpoint --all
podman container checkpoint --leave-running ctrID`,
	}
)

var checkpointOptions entities.CheckpointOptions

type checkpointStatistics struct {
	PodmanDuration      int64                        `json:"podman_checkpoint_duration"`
	ContainerStatistics []*entities.CheckpointReport `json:"container_statistics"`
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: checkpointCommand,
		Parent:  containerCmd,
	})
	flags := checkpointCommand.Flags()
	flags.BoolVarP(&checkpointOptions.Keep, "keep", "k", false, "Keep all temporary checkpoint files")
	flags.BoolVarP(&checkpointOptions.LeaveRunning, "leave-running", "R", false, "Leave the container running after writing checkpoint to disk")
	flags.BoolVar(&checkpointOptions.TCPEstablished, "tcp-established", false, "Checkpoint a container with established TCP connections")
	flags.BoolVar(&checkpointOptions.FileLocks, "file-locks", false, "Checkpoint a container with file locks")
	flags.BoolVarP(&checkpointOptions.All, "all", "a", false, "Checkpoint all running containers")

	exportFlagName := "export"
	flags.StringVarP(&checkpointOptions.Export, exportFlagName, "e", "", "Export the checkpoint image to a tar.gz")
	_ = checkpointCommand.RegisterFlagCompletionFunc(exportFlagName, completion.AutocompleteDefault)

	flags.BoolVar(&checkpointOptions.IgnoreRootFS, "ignore-rootfs", false, "Do not include root file-system changes when exporting")
	flags.BoolVar(&checkpointOptions.IgnoreVolumes, "ignore-volumes", false, "Do not export volumes associated with container")
	flags.BoolVarP(&checkpointOptions.PreCheckPoint, "pre-checkpoint", "P", false, "Dump container's memory information only, leave the container running")
	flags.BoolVar(&checkpointOptions.WithPrevious, "with-previous", false, "Checkpoint container with pre-checkpoint images")

	createImageFlagName := "create-image"
	flags.StringVarP(&checkpointOptions.CreateImage, createImageFlagName, "", "", "Create checkpoint image with specified name")
	_ = checkpointCommand.RegisterFlagCompletionFunc(createImageFlagName, completion.AutocompleteNone)

	flags.StringP("compress", "c", "zstd", "Select compression algorithm (gzip, none, zstd) for checkpoint archive.")
	_ = checkpointCommand.RegisterFlagCompletionFunc("compress", common.AutocompleteCheckpointCompressType)

	flags.BoolVar(
		&checkpointOptions.PrintStats,
		"print-stats",
		false,
		"Display checkpoint statistics",
	)

	validate.AddLatestFlag(checkpointCommand, &checkpointOptions.Latest)
}

func checkpoint(cmd *cobra.Command, args []string) error {
	var errs utils.OutputErrors
	args = utils.RemoveSlash(args)
	podmanStart := time.Now()
	if cmd.Flags().Changed("compress") {
		if checkpointOptions.Export == "" {
			return errors.New("--compress can only be used with --export")
		}
		compress, _ := cmd.Flags().GetString("compress")
		switch strings.ToLower(compress) {
		case "none":
			checkpointOptions.Compression = archive.Uncompressed
		case "gzip":
			checkpointOptions.Compression = archive.Gzip
		case "zstd":
			checkpointOptions.Compression = archive.Zstd
		default:
			return fmt.Errorf("selected compression algorithm (%q) not supported. Please select one from: gzip, none, zstd", compress)
		}
	} else {
		checkpointOptions.Compression = archive.Zstd
	}
	if rootless.IsRootless() {
		return errors.New("checkpointing a container requires root")
	}
	if checkpointOptions.Export == "" && checkpointOptions.IgnoreRootFS {
		return errors.New("--ignore-rootfs can only be used with --export")
	}
	if checkpointOptions.Export == "" && checkpointOptions.IgnoreVolumes {
		return errors.New("--ignore-volumes can only be used with --export")
	}
	if checkpointOptions.WithPrevious && checkpointOptions.PreCheckPoint {
		return errors.New("--with-previous can not be used with --pre-checkpoint")
	}
	if (checkpointOptions.WithPrevious || checkpointOptions.PreCheckPoint) && !criu.MemTrack() {
		return errors.New("system (architecture/kernel/CRIU) does not support memory tracking")
	}
	responses, err := registry.ContainerEngine().ContainerCheckpoint(context.Background(), args, checkpointOptions)
	if err != nil {
		return err
	}
	podmanFinished := time.Now()

	var statistics checkpointStatistics

	for _, r := range responses {
		switch {
		case r.Err != nil:
			errs = append(errs, r.Err)
		case checkpointOptions.PrintStats:
			statistics.ContainerStatistics = append(statistics.ContainerStatistics, r)
		case r.RawInput != "":
			fmt.Println(r.RawInput)
		default:
			fmt.Println(r.Id)
		}
	}

	if checkpointOptions.PrintStats {
		statistics.PodmanDuration = podmanFinished.Sub(podmanStart).Microseconds()
		j, err := json.MarshalIndent(statistics, "", "    ")
		if err != nil {
			return err
		}
		fmt.Println(string(j))
	}

	return errs.PrintErrors()
}
