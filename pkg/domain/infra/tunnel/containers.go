package tunnel

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"go.podman.io/common/pkg/config"
	"go.podman.io/image/v5/docker/reference"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/api/handlers"
	"go.podman.io/podman/v6/pkg/bindings"
	"go.podman.io/podman/v6/pkg/bindings/containers"
	"go.podman.io/podman/v6/pkg/bindings/images"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/domain/entities/reports"
	"go.podman.io/podman/v6/pkg/errorhandling"
	"go.podman.io/podman/v6/pkg/specgen"
	"go.podman.io/podman/v6/pkg/util"
	"go.podman.io/storage/types"
)

func (ic *ContainerEngine) ContainerRunlabel(_ context.Context, _ string, _ string, _ []string, _ entities.ContainerRunlabelOptions) error {
	return errors.New("not implemented")
}

func (ic *ContainerEngine) ContainerExists(_ context.Context, nameOrID string, options entities.ContainerExistsOptions) (*entities.BoolReport, error) {
	exists, err := containers.Exists(ic.ClientCtx, nameOrID, new(containers.ExistsOptions).WithExternal(options.External))
	return &entities.BoolReport{Value: exists}, err
}

func (ic *ContainerEngine) ContainerWait(ctx context.Context, namesOrIds []string, opts entities.WaitOptions) ([]entities.WaitReport, error) {
	responses := make([]entities.WaitReport, 0, len(namesOrIds))
	options := new(containers.WaitOptions).WithConditions(opts.Conditions).WithInterval(opts.Interval.String())

	if opts.ExitFirstMatch {
		waitChannel := make(chan entities.WaitReport, 1)
		waitFunction := func(ctx context.Context, nameOrId string, options *containers.WaitOptions, waitChannel chan<- entities.WaitReport) {
			response := entities.WaitReport{}
			exitCode, err := containers.Wait(ic.ClientCtx, nameOrId, options)
			if err != nil {
				if opts.Ignore && errorhandling.Contains(err, define.ErrNoSuchCtr) {
					response.ExitCode = -1
				} else {
					response.Error = err
				}
			} else {
				response.ExitCode = exitCode
			}

			select {
			case <-ctx.Done():
			case waitChannel <- response:
			}
		}

		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		for _, n := range namesOrIds {
			go waitFunction(ctx, n, options, waitChannel)
		}

		response := <-waitChannel
		responses = append(responses, response)
		return responses, nil
	}

	for _, n := range namesOrIds {
		response := entities.WaitReport{}
		exitCode, err := containers.Wait(ic.ClientCtx, n, options)
		if err != nil {
			if opts.Ignore && errorhandling.Contains(err, define.ErrNoSuchCtr) {
				response.ExitCode = -1
			} else {
				response.Error = err
			}
		} else {
			response.ExitCode = exitCode
		}
		responses = append(responses, response)
	}
	return responses, nil
}

func (ic *ContainerEngine) ContainerPause(_ context.Context, namesOrIds []string, options entities.PauseUnPauseOptions) ([]*entities.PauseUnpauseReport, error) {
	ctrs, rawInputs, err := getContainersAndInputByContext(ic.ClientCtx, options.All, false, namesOrIds, options.Filters)
	if err != nil {
		return nil, err
	}
	idToRawInput := map[string]string{}
	for i := range ctrs {
		idToRawInput[ctrs[i].ID] = rawInputs[i]
	}
	reports := make([]*entities.PauseUnpauseReport, 0, len(ctrs))
	for _, c := range ctrs {
		err := containers.Pause(ic.ClientCtx, c.ID, nil)
		if err != nil && options.All && strings.Contains(err.Error(), define.ErrCtrStateInvalid.Error()) {
			logrus.Debugf("Container %s is not running", c.ID)
			continue
		}
		reports = append(reports, &entities.PauseUnpauseReport{
			Id:       c.ID,
			Err:      err,
			RawInput: idToRawInput[c.ID],
		})
	}
	return reports, nil
}

func (ic *ContainerEngine) ContainerUnpause(_ context.Context, namesOrIds []string, options entities.PauseUnPauseOptions) ([]*entities.PauseUnpauseReport, error) {
	ctrs, rawInputs, err := getContainersAndInputByContext(ic.ClientCtx, options.All, false, namesOrIds, options.Filters)
	if err != nil {
		return nil, err
	}
	idToRawInput := map[string]string{}
	for i := range ctrs {
		idToRawInput[ctrs[i].ID] = rawInputs[i]
	}
	reports := make([]*entities.PauseUnpauseReport, 0, len(ctrs))
	for _, c := range ctrs {
		err := containers.Unpause(ic.ClientCtx, c.ID, nil)
		if err != nil && options.All && strings.Contains(err.Error(), define.ErrCtrStateInvalid.Error()) {
			logrus.Debugf("Container %s is not paused", c.ID)
			continue
		}
		reports = append(reports, &entities.PauseUnpauseReport{
			Id:       c.ID,
			Err:      err,
			RawInput: idToRawInput[c.ID],
		})
	}
	return reports, nil
}

func (ic *ContainerEngine) ContainerStop(_ context.Context, namesOrIds []string, opts entities.StopOptions) ([]*entities.StopReport, error) {
	ctrs, rawInputs, err := getContainersAndInputByContext(ic.ClientCtx, opts.All, opts.Ignore, namesOrIds, opts.Filters)
	if err != nil {
		return nil, err
	}
	idToRawInput := map[string]string{}
	for i := range ctrs {
		idToRawInput[ctrs[i].ID] = rawInputs[i]
	}
	options := new(containers.StopOptions).WithIgnore(opts.Ignore)
	if to := opts.Timeout; to != nil {
		options.WithTimeout(*to)
	}
	reports := []*entities.StopReport{}
	for _, c := range ctrs {
		report := entities.StopReport{
			Id:       c.ID,
			RawInput: idToRawInput[c.ID],
		}
		if err = containers.Stop(ic.ClientCtx, c.ID, options); err != nil {
			// These first two are considered non-fatal under the right conditions
			if strings.Contains(err.Error(), define.ErrCtrStopped.Error()) {
				logrus.Debugf("Container %s is already stopped", c.ID)
				reports = append(reports, &report)
				continue
			} else if opts.All && strings.Contains(err.Error(), define.ErrCtrStateInvalid.Error()) {
				logrus.Debugf("Container %s is not running, could not stop", c.ID)
				reports = append(reports, &report)
				continue
			}

			// TODO we need to associate errors returned by http with common
			// define.errors so that we can equity tests. this will allow output
			// to be the same as the native client
			report.Err = err
			reports = append(reports, &report)
			continue
		}
		reports = append(reports, &report)
	}
	return reports, nil
}

func (ic *ContainerEngine) ContainerStopService(_ context.Context, _ []string, _ entities.StopOptions) ([]*entities.StopReport, error) {
	return nil, fmt.Errorf("container stop as service is not supported when using remote connections")
}

func (ic *ContainerEngine) ContainerKill(_ context.Context, namesOrIds []string, opts entities.KillOptions) ([]*entities.KillReport, error) {
	ctrs, rawInputs, err := getContainersAndInputByContext(ic.ClientCtx, opts.All, false, namesOrIds, nil)
	if err != nil {
		return nil, err
	}
	idToRawInput := map[string]string{}
	for i := range ctrs {
		idToRawInput[ctrs[i].ID] = rawInputs[i]
	}
	options := new(containers.KillOptions).WithSignal(opts.Signal)
	reports := make([]*entities.KillReport, 0, len(ctrs))
	for _, c := range ctrs {
		err := containers.Kill(ic.ClientCtx, c.ID, options)
		if err != nil && opts.All && strings.Contains(err.Error(), define.ErrCtrStateInvalid.Error()) {
			logrus.Debugf("Container %s is not running", c.ID)
			continue
		}
		reports = append(reports, &entities.KillReport{
			Id:       c.ID,
			Err:      err,
			RawInput: idToRawInput[c.ID],
		})
	}
	return reports, nil
}

func (ic *ContainerEngine) ContainerRestart(_ context.Context, namesOrIds []string, opts entities.RestartOptions) ([]*entities.RestartReport, error) {
	reports := []*entities.RestartReport{}
	options := new(containers.RestartOptions)
	if to := opts.Timeout; to != nil {
		timeout := util.ConvertTimeout(int(*to))
		options.WithTimeout(int(timeout))
	}
	ctrs, rawInputs, err := getContainersAndInputByContext(ic.ClientCtx, opts.All, false, namesOrIds, opts.Filters)
	if err != nil {
		return nil, err
	}
	idToRawInput := map[string]string{}
	for i := range ctrs {
		idToRawInput[ctrs[i].ID] = rawInputs[i]
	}
	for _, c := range ctrs {
		if opts.Running && c.State != define.ContainerStateRunning.String() {
			continue
		}
		reports = append(reports, &entities.RestartReport{
			Id:       c.ID,
			Err:      containers.Restart(ic.ClientCtx, c.ID, options),
			RawInput: idToRawInput[c.ID],
		})
	}
	return reports, nil
}

func (ic *ContainerEngine) ContainerRm(_ context.Context, namesOrIds []string, opts entities.RmOptions) ([]*reports.RmReport, error) {
	// TODO there is no endpoint for container eviction.  Need to discuss
	options := new(containers.RemoveOptions).WithForce(opts.Force).WithVolumes(opts.Volumes).WithIgnore(opts.Ignore).WithDepend(opts.Depend)
	if opts.Timeout != nil {
		options = options.WithTimeout(*opts.Timeout)
	}

	toRemove := []string{}
	alreadyRemoved := make(map[string]bool) // Avoids trying to remove already removed containers
	idToRawInput := map[string]string{}

	if opts.All || len(opts.Filters) > 0 {
		ctrs, rawInputs, err := getContainersAndInputByContext(ic.ClientCtx, opts.All, opts.Ignore, nil, opts.Filters)
		if err != nil {
			return nil, err
		}
		if len(rawInputs) == len(ctrs) {
			for i := range ctrs {
				idToRawInput[ctrs[i].ID] = rawInputs[i]
			}
		}
		for _, c := range ctrs {
			toRemove = append(toRemove, c.ID)
		}
	} else {
		for _, ctr := range namesOrIds {
			// NOTE that we set ignore=true here to support
			// removing external containers (e.g., Buildah
			// containers). If we don't find the container,
			// we'll use the raw input provided by the user
			// instead of the ID. Since this can only happen
			// with external containers, it poses no threat
			// to the `alreadyRemoved` checks below.
			ctrs, rawInputs, err := getContainersAndInputByContext(ic.ClientCtx, false, true, []string{ctr}, opts.Filters)
			if err != nil {
				return nil, err
			}
			if len(rawInputs) == len(ctrs) {
				for i := range ctrs {
					idToRawInput[ctrs[i].ID] = rawInputs[i]
				}
			}
			id := ctr
			if len(ctrs) == 1 {
				id = ctrs[0].ID
			}
			toRemove = append(toRemove, id)
		}
	}

	rmReports := make([]*reports.RmReport, 0, len(toRemove))
	for _, rmCtr := range toRemove {
		if alreadyRemoved[rmCtr] {
			continue
		}
		if ctr, exist := idToRawInput[rmCtr]; exist {
			rmCtr = ctr
		}
		newReports, err := containers.Remove(ic.ClientCtx, rmCtr, options)
		if err != nil {
			rmReports = append(rmReports, &reports.RmReport{
				Id:       rmCtr,
				Err:      err,
				RawInput: idToRawInput[rmCtr],
			})
			continue
		}
		for i := range newReports {
			alreadyRemoved[newReports[i].Id] = true
			newReports[i].RawInput = idToRawInput[newReports[i].Id]
			rmReports = append(rmReports, newReports[i])
		}
	}
	return rmReports, nil
}

func (ic *ContainerEngine) ContainerPrune(_ context.Context, opts entities.ContainerPruneOptions) ([]*reports.PruneReport, error) {
	options := new(containers.PruneOptions).WithFilters(opts.Filters)
	return containers.Prune(ic.ClientCtx, options)
}

func (ic *ContainerEngine) ContainerInspect(_ context.Context, namesOrIds []string, opts entities.InspectOptions) ([]*entities.ContainerInspectReport, []error, error) {
	var (
		reports = make([]*entities.ContainerInspectReport, 0, len(namesOrIds))
		errs    = []error{}
	)
	options := new(containers.InspectOptions).WithSize(opts.Size)
	for _, name := range namesOrIds {
		inspect, err := containers.Inspect(ic.ClientCtx, name, options)
		if err != nil {
			errModel, ok := err.(*errorhandling.ErrorModel)
			if !ok {
				return nil, nil, err
			}
			if errModel.ResponseCode == 404 {
				errs = append(errs, fmt.Errorf("no such container %q", name))
				continue
			}
			return nil, nil, err
		}
		reports = append(reports, &entities.ContainerInspectReport{InspectContainerData: inspect})
	}
	return reports, errs, nil
}

func (ic *ContainerEngine) ContainerTop(_ context.Context, opts entities.TopOptions) (*entities.StringSliceReport, error) {
	switch {
	case opts.Latest:
		return nil, errors.New("latest is not supported")
	case opts.NameOrID == "":
		return nil, errors.New("NameOrID must be specified")
	}
	options := new(containers.TopOptions).WithDescriptors(opts.Descriptors)
	topOutput, err := containers.Top(ic.ClientCtx, opts.NameOrID, options)
	if err != nil {
		return nil, err
	}
	return &entities.StringSliceReport{Value: topOutput}, nil
}

func (ic *ContainerEngine) ContainerCommit(_ context.Context, nameOrID string, opts entities.CommitOptions) (*entities.CommitReport, error) {
	var (
		repo string
		tag  = "latest"
	)
	if len(opts.ImageName) > 0 {
		ref, err := reference.Parse(opts.ImageName)
		if err != nil {
			return nil, fmt.Errorf("parsing reference %q: %w", opts.ImageName, err)
		}
		if t, ok := ref.(reference.Tagged); ok {
			tag = t.Tag()
		}
		if r, ok := ref.(reference.Named); ok {
			repo = r.Name()
		}
		if len(repo) < 1 {
			return nil, fmt.Errorf("invalid image name %q", opts.ImageName)
		}
	}
	var changes []string
	if len(opts.Changes) > 0 {
		changes = util.DecodeChanges(opts.Changes)
	}
	var configReader io.Reader
	if len(opts.Config) > 0 {
		configReader = bytes.NewReader(opts.Config)
	}
	options := new(containers.CommitOptions).WithAuthor(opts.Author).WithChanges(changes).WithComment(opts.Message).WithConfig(configReader).WithSquash(opts.Squash).WithStream(!opts.Quiet)
	options.WithFormat(opts.Format).WithPause(opts.Pause).WithRepo(repo).WithTag(tag)
	response, err := containers.Commit(ic.ClientCtx, nameOrID, options)
	if err != nil {
		return nil, err
	}
	return &entities.CommitReport{Id: response.ID}, nil
}

func (ic *ContainerEngine) ContainerExport(_ context.Context, nameOrID string, options entities.ContainerExportOptions) error {
	return containers.Export(ic.ClientCtx, nameOrID, options.Output, nil)
}

func (ic *ContainerEngine) ContainerCheckpoint(_ context.Context, namesOrIds []string, opts entities.CheckpointOptions) ([]*entities.CheckpointReport, error) {
	var (
		err          error
		ctrs         []entities.ListContainer
		rawInputs    []string
		idToRawInput = map[string]string{}
	)
	options := new(containers.CheckpointOptions)
	options.WithFileLocks(opts.FileLocks)
	options.WithIgnoreRootfs(opts.IgnoreRootFS)
	options.WithKeep(opts.Keep)
	options.WithExport(opts.Export)
	options.WithCreateImage(opts.CreateImage)
	options.WithTCPEstablished(opts.TCPEstablished)
	options.WithPrintStats(opts.PrintStats)
	options.WithPreCheckpoint(opts.PreCheckPoint)
	options.WithLeaveRunning(opts.LeaveRunning)
	options.WithWithPrevious(opts.WithPrevious)

	if opts.All {
		allCtrs, err := getContainersByContext(ic.ClientCtx, true, false, []string{})
		if err != nil {
			return nil, err
		}
		// narrow the list to running only
		for _, c := range allCtrs {
			if c.State == define.ContainerStateRunning.String() {
				ctrs = append(ctrs, c)
			}
		}
	} else {
		ctrs, rawInputs, err = getContainersAndInputByContext(ic.ClientCtx, false, false, namesOrIds, nil)
		if err != nil {
			return nil, err
		}
		if len(rawInputs) == len(ctrs) {
			for i := range ctrs {
				idToRawInput[ctrs[i].ID] = rawInputs[i]
			}
		}
	}
	reports := make([]*entities.CheckpointReport, 0, len(ctrs))
	for _, c := range ctrs {
		report, err := containers.Checkpoint(ic.ClientCtx, c.ID, options)
		if err != nil {
			reports = append(reports, &entities.CheckpointReport{Id: c.ID, Err: err})
		} else {
			report.RawInput = idToRawInput[c.ID]
			reports = append(reports, report)
		}
	}
	return reports, nil
}

func (ic *ContainerEngine) ContainerRestore(_ context.Context, namesOrIds []string, opts entities.RestoreOptions) ([]*entities.RestoreReport, error) {
	if opts.ImportPrevious != "" {
		return nil, fmt.Errorf("--import-previous is not supported on the remote client")
	}

	var (
		ids          []string
		idToRawInput = map[string]string{}
	)
	options := new(containers.RestoreOptions)
	options.WithFileLocks(opts.FileLocks)
	options.WithIgnoreRootfs(opts.IgnoreRootFS)
	options.WithIgnoreVolumes(opts.IgnoreVolumes)
	options.WithIgnoreStaticIP(opts.IgnoreStaticIP)
	options.WithIgnoreStaticMAC(opts.IgnoreStaticMAC)
	options.WithKeep(opts.Keep)
	options.WithName(opts.Name)
	options.WithTCPEstablished(opts.TCPEstablished)
	options.WithTCPClose(opts.TCPClose)
	options.WithPod(opts.Pod)
	options.WithPrintStats(opts.PrintStats)
	options.WithPublishPorts(opts.PublishPorts)

	if opts.Import != "" {
		options.WithImportArchive(opts.Import)
		report, err := containers.Restore(ic.ClientCtx, "", options)
		return []*entities.RestoreReport{report}, err
	}
	if opts.All {
		allCtrs, err := getContainersByContext(ic.ClientCtx, true, false, []string{})
		if err != nil {
			return nil, err
		}
		// narrow the list to exited only
		for _, c := range allCtrs {
			if c.State == define.ContainerStateExited.String() {
				ids = append(ids, c.ID)
			}
		}
	} else {
		getImageOptions := new(images.GetOptions).WithSize(false)
		hostInfo, err := ic.Info(context.Background())
		if err != nil {
			return nil, err
		}

		for _, nameOrID := range namesOrIds {
			ctrData, _, err := ic.ContainerInspect(ic.ClientCtx, []string{nameOrID}, entities.InspectOptions{})
			if err == nil && len(ctrData) > 0 {
				ids = append(ids, ctrData[0].ID)
				idToRawInput[ctrData[0].ID] = nameOrID
			} else {
				// If container was not found, check if this is a checkpoint image
				inspectReport, err := images.GetImage(ic.ClientCtx, nameOrID, getImageOptions)
				if err != nil {
					return nil, fmt.Errorf("no such container or image: %s", nameOrID)
				}
				checkpointRuntimeName, found := inspectReport.Annotations[define.CheckpointAnnotationRuntimeName]
				if !found {
					return nil, fmt.Errorf("image is not a checkpoint: %s", nameOrID)
				}
				if hostInfo.Host.OCIRuntime.Name != checkpointRuntimeName {
					return nil, fmt.Errorf("container image \"%s\" requires runtime: \"%s\"", nameOrID, checkpointRuntimeName)
				}
				ids = append(ids, inspectReport.ID)
			}
		}
	}
	reports := make([]*entities.RestoreReport, 0, len(ids))
	for _, id := range ids {
		report, err := containers.Restore(ic.ClientCtx, id, options)
		if err != nil {
			reports = append(reports, &entities.RestoreReport{Id: id, Err: err})
		}
		report.RawInput = idToRawInput[report.Id]
		reports = append(reports, report)
	}
	return reports, nil
}

func (ic *ContainerEngine) ContainerCreate(_ context.Context, s *specgen.SpecGenerator) (*entities.ContainerCreateReport, error) {
	response, err := containers.CreateWithSpec(ic.ClientCtx, s, nil)
	if err != nil {
		return nil, err
	}
	for _, w := range response.Warnings {
		fmt.Fprintf(os.Stderr, "%s\n", w)
	}
	return &entities.ContainerCreateReport{Id: response.ID}, nil
}

func (ic *ContainerEngine) ContainerLogs(_ context.Context, nameOrIDs []string, opts entities.ContainerLogsOptions) error {
	since := opts.Since.Format(time.RFC3339)
	until := opts.Until.Format(time.RFC3339)
	tail := strconv.FormatInt(opts.Tail, 10)
	stdout := opts.StdoutWriter != nil
	stderr := opts.StderrWriter != nil
	options := new(containers.LogOptions).WithFollow(opts.Follow).WithSince(since).WithUntil(until).WithStderr(stderr)
	options.WithStdout(stdout).WithTail(tail).WithTimestamps(opts.Timestamps)

	var err error
	stdoutCh := make(chan string)
	stderrCh := make(chan string)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		err = containers.Logs(ic.ClientCtx, nameOrIDs[0], options, stdoutCh, stderrCh)
		cancel()
	}()

	for {
		select {
		case <-ctx.Done():
			return err
		case line := <-stdoutCh:
			if opts.StdoutWriter != nil {
				_, _ = io.WriteString(opts.StdoutWriter, line)
			}
		case line := <-stderrCh:
			if opts.StderrWriter != nil {
				_, _ = io.WriteString(opts.StderrWriter, line)
			}
		}
	}
}

func (ic *ContainerEngine) ContainerAttach(ctx context.Context, nameOrID string, opts entities.AttachOptions) error {
	ctrs, err := getContainersByContext(ic.ClientCtx, false, false, []string{nameOrID})
	if err != nil {
		return err
	}
	ctr := ctrs[0]
	if ctr.State != define.ContainerStateRunning.String() {
		return fmt.Errorf("you can only attach to running containers")
	}
	options := new(containers.AttachOptions).WithStream(true).WithDetachKeys(opts.DetachKeys)
	if opts.SigProxy {
		remoteProxySignals(ctr.ID, func(signal string) error {
			killOpts := entities.KillOptions{All: false, Latest: false, Signal: signal}
			_, err := ic.ContainerKill(ctx, []string{ctr.ID}, killOpts)
			return err
		})
	}
	return containers.Attach(ic.ClientCtx, nameOrID, opts.Stdin, opts.Stdout, opts.Stderr, nil, options)
}

func makeExecConfig(options entities.ExecOptions) *handlers.ExecCreateConfig {
	env := make([]string, 0, len(options.Envs))
	for k, v := range options.Envs {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	createConfig := new(handlers.ExecCreateConfig)
	createConfig.User = options.User
	createConfig.Privileged = options.Privileged
	createConfig.Tty = options.Tty
	createConfig.AttachStdin = options.Interactive
	createConfig.AttachStdout = true
	createConfig.AttachStderr = true
	createConfig.DetachKeys = options.DetachKeys
	createConfig.Env = env
	createConfig.WorkingDir = options.WorkDir
	createConfig.Cmd = options.Cmd
	// Size the exec terminal at creation, mirroring the local path, so the
	// service does not have to rely on the asynchronous resize that follows
	// attach. ConsoleSize is ordered [height, width].
	if options.Tty && options.ConsoleSize != nil {
		createConfig.ConsoleSize = &[2]uint{uint(options.ConsoleSize.Height), uint(options.ConsoleSize.Width)}
	}

	return createConfig
}

func (ic *ContainerEngine) ContainerExec(_ context.Context, nameOrID string, options entities.ExecOptions, streams define.AttachStreams) (exitCode int, retErr error) {
	createConfig := makeExecConfig(options)

	sessionID, err := containers.ExecCreate(ic.ClientCtx, nameOrID, createConfig)
	if err != nil {
		return 125, err
	}
	defer func() {
		if err := containers.ExecRemove(ic.ClientCtx, sessionID, nil); err != nil {
			apiErr := new(bindings.APIVersionError)
			if errors.As(err, &apiErr) {
				// if the API is to old do not throw an error
				return
			}
			if retErr == nil {
				exitCode = -1
				retErr = err
			}
		}
	}()
	startAndAttachOptions := new(containers.ExecStartAndAttachOptions)
	startAndAttachOptions.WithOutputStream(streams.OutputStream).WithErrorStream(streams.ErrorStream)
	if streams.InputStream != nil {
		startAndAttachOptions.WithInputStream(*streams.InputStream)
	}
	startAndAttachOptions.WithAttachError(streams.AttachError).WithAttachOutput(streams.AttachOutput).WithAttachInput(streams.AttachInput)
	if err := containers.ExecStartAndAttach(ic.ClientCtx, sessionID, startAndAttachOptions); err != nil {
		return 125, err
	}

	inspectOut, err := containers.ExecInspect(ic.ClientCtx, sessionID, nil)
	if err != nil {
		return 125, err
	}

	return inspectOut.ExitCode, nil
}

func (ic *ContainerEngine) ContainerExecNoSession(_ context.Context, _ string, _ entities.ExecOptions, _ define.AttachStreams) (int, error) {
	return 0, errors.New("--no-session is not supported for the remote client")
}

func (ic *ContainerEngine) ContainerExecDetached(_ context.Context, nameOrID string, options entities.ExecOptions) (retSessionID string, retErr error) {
	createConfig := makeExecConfig(options)

	sessionID, err := containers.ExecCreate(ic.ClientCtx, nameOrID, createConfig)
	if err != nil {
		return "", err
	}
	defer func() {
		if retErr != nil {
			_ = containers.ExecRemove(ic.ClientCtx, sessionID, nil)
		}
	}()

	if err := containers.ExecStart(ic.ClientCtx, sessionID, nil); err != nil {
		return "", err
	}

	return sessionID, nil
}

func startAndAttach(ic *ContainerEngine, name string, detachKeys *string, sigProxy bool, input, output, errput *os.File) (int, error) {
	if output == nil && errput == nil {
		fmt.Printf("%s\n", name)
	}
	attachErr := make(chan error)
	attachReady := make(chan bool)
	options := new(containers.AttachOptions).WithStream(true)
	if dk := detachKeys; dk != nil {
		options.WithDetachKeys(*dk)
	}
	if sigProxy {
		remoteProxySignals(name, func(signal string) error {
			killOpts := entities.KillOptions{All: false, Latest: false, Signal: signal}
			_, err := ic.ContainerKill(ic.ClientCtx, []string{name}, killOpts)
			return err
		})
	}

	go func() {
		err := containers.Attach(ic.ClientCtx, name, input, output, errput, attachReady, options)
		attachErr <- err
		close(attachErr)
	}()
	// Wait for the attach to actually happen before starting
	// the container.

	cancelCtx, cancel := context.WithCancel(ic.ClientCtx)
	defer cancel()
	var code int
	select {
	case <-attachReady:
		startOptions := new(containers.StartOptions)
		if dk := detachKeys; dk != nil {
			startOptions.WithDetachKeys(*dk)
		}
		if err := containers.Start(ic.ClientCtx, name, startOptions); err != nil {
			return -1, err
		}

		// Call wait immediately after start to avoid racing against container removal when it was created with --rm.
		// It must be run in a separate goroutine to so we do not block when attach returns early, i.e. user
		// detaches in which case wait would not return.
		waitChan := make(chan error)
		go func() {
			defer close(waitChan)

			exitCode, err := containers.Wait(cancelCtx, name, nil)
			if err != nil {
				waitChan <- fmt.Errorf("wait for container: %w", err)
				return
			}
			code = int(exitCode)
		}()

		select {
		case err := <-waitChan:
			if err != nil {
				return -1, err
			}
		case err := <-attachErr:
			if err != nil {
				return -1, err
			}
			// also wait for the wait to be complete in this case
			err = <-waitChan
			if err != nil {
				return -1, err
			}
		}
	case err := <-attachErr:
		return -1, err
	}
	// If attachReady happens first, wait for containers.Attach to complete
	return code, <-attachErr
}

func logIfRmError(id string, err error, reports []*reports.RmReport) {
	logError := func(id string, err error) {
		if errorhandling.Contains(err, define.ErrNoSuchCtr) ||
			errorhandling.Contains(err, define.ErrCtrRemoved) ||
			errorhandling.Contains(err, types.ErrLayerUnknown) {
			logrus.Debugf("Container %s does not exist: %v", id, err)
		} else {
			logrus.Errorf("Removing container %s: %v", id, err)
		}
	}
	if err != nil {
		logError(id, err)
	} else {
		for _, report := range reports {
			if report.Err != nil {
				logError(report.Id, report.Err)
			}
		}
	}
}

func (ic *ContainerEngine) ContainerStart(_ context.Context, namesOrIds []string, options entities.ContainerStartOptions) ([]*entities.ContainerStartReport, error) {
	reports := []*entities.ContainerStartReport{}
	exitCode := define.ExecErrorCodeGeneric
	ctrs, rawInputs, err := getContainersAndInputByContext(ic.ClientCtx, options.All, len(options.Filters) > 0, namesOrIds, options.Filters)
	if err != nil {
		return nil, err
	}
	idToRawInput := map[string]string{}
	if len(rawInputs) == len(ctrs) {
		for i := range ctrs {
			idToRawInput[ctrs[i].ID] = rawInputs[i]
		}
	}
	removeOptions := new(containers.RemoveOptions).WithVolumes(true).WithForce(false)
	removeContainer := func(id, CIDFile string) {
		if CIDFile != "" {
			if err := os.Remove(CIDFile); err != nil && !errors.Is(err, os.ErrNotExist) {
				logrus.Warnf("Cleaning up CID file: %s", err)
			}
		}
		reports, err := containers.Remove(ic.ClientCtx, id, removeOptions)
		logIfRmError(id, err, reports)
	}

	// There can only be one container if attach was used
	for _, ctr := range ctrs {
		name := ctr.ID
		report := entities.ContainerStartReport{
			Id:       name,
			RawInput: idToRawInput[name],
			ExitCode: exitCode,
		}
		ctrRunning := ctr.State == define.ContainerStateRunning.String()
		if options.Attach {
			code, err := startAndAttach(ic, name, &options.DetachKeys, options.SigProxy, options.Stdin, options.Stdout, options.Stderr)
			if err == define.ErrDetach {
				// User manually detached
				// Exit cleanly immediately
				reports = append(reports, &report)
				return reports, nil
			}
			if ctrRunning {
				reports = append(reports, &report)
				return reports, nil
			}

			if err != nil {
				if ctr.AutoRemove {
					removeContainer(ctr.ID, ctr.CIDFile)
				}
				report.ExitCode = define.ExitCode(report.Err)
				report.Err = err
				reports = append(reports, &report)
				return reports, fmt.Errorf("unable to start container %s: %w", name, report.Err)
			}
			if ctr.AutoRemove {
				// Defer the removal, so we can return early if needed and
				// de-spaghetti the code.
				defer removeContainer(ctr.ID, ctr.CIDFile)
			}

			report.ExitCode = code
			reports = append(reports, &report)
			return reports, nil
		}
		// Start the container if it's not running already.
		if !ctrRunning {
			err = containers.Start(ic.ClientCtx, name, new(containers.StartOptions).WithDetachKeys(options.DetachKeys))
			if err != nil {
				if ctr.AutoRemove {
					rmOptions := new(containers.RemoveOptions).WithForce(false).WithVolumes(true)
					reports, err := containers.Remove(ic.ClientCtx, ctr.ID, rmOptions)
					logIfRmError(ctr.ID, err, reports)
				}
				report.Err = fmt.Errorf("unable to start container %q: %w", name, err)
				report.ExitCode = define.ExitCode(err)
				reports = append(reports, &report)
				continue
			}
			report.ExitCode = 0
			reports = append(reports, &report)
		}
	}
	return reports, nil
}

func (ic *ContainerEngine) ContainerList(_ context.Context, opts entities.ContainerListOptions) ([]entities.ListContainer, error) {
	options := new(containers.ListOptions).WithFilters(opts.Filters).WithAll(opts.All).WithLast(opts.Last)
	options.WithNamespace(opts.Namespace).WithSize(opts.Size).WithSync(opts.Sync).WithExternal(opts.External)
	return containers.List(ic.ClientCtx, options)
}

func (ic *ContainerEngine) ContainerListExternal(_ context.Context) ([]entities.ListContainer, error) {
	options := new(containers.ListOptions).WithAll(true)
	options.WithNamespace(true).WithSize(true).WithSync(true).WithExternal(true)
	return containers.List(ic.ClientCtx, options)
}

func (ic *ContainerEngine) ContainerRun(ctx context.Context, opts entities.ContainerRunOptions) (*entities.ContainerRunReport, error) {
	if opts.Spec != nil && !reflect.ValueOf(opts.Spec).IsNil() && opts.Spec.RawImageName != "" {
		// If this is a checkpoint image, restore it.
		getImageOptions := new(images.GetOptions).WithSize(false)
		inspectReport, err := images.GetImage(ic.ClientCtx, opts.Spec.RawImageName, getImageOptions)
		if err != nil {
			return nil, fmt.Errorf("no such container or image: %s", opts.Spec.RawImageName)
		}
		if inspectReport != nil {
			_, isCheckpointImage := inspectReport.Annotations[define.CheckpointAnnotationRuntimeName]
			if isCheckpointImage {
				restoreOptions := new(containers.RestoreOptions)
				restoreOptions.WithName(opts.Spec.Name)
				restoreOptions.WithPod(opts.Spec.Pod)

				restoreReport, err := containers.Restore(ic.ClientCtx, inspectReport.ID, restoreOptions)
				if err != nil {
					return nil, err
				}
				runReport := entities.ContainerRunReport{Id: restoreReport.Id}
				return &runReport, nil
			}
		}
	}

	con, err := containers.CreateWithSpec(ic.ClientCtx, opts.Spec, nil)
	if err != nil {
		return nil, err
	}
	for _, w := range con.Warnings {
		fmt.Fprintf(os.Stderr, "%s\n", w)
	}
	removeContainer := func(id, CIDFile string, force bool) {
		if CIDFile != "" {
			if err := os.Remove(CIDFile); err != nil && !errors.Is(err, os.ErrNotExist) {
				logrus.Warnf("Cleaning up CID file: %s", err)
			}
		}

		removeOptions := new(containers.RemoveOptions).WithVolumes(true).WithForce(force)
		reports, err := containers.Remove(ic.ClientCtx, id, removeOptions)
		logIfRmError(id, err, reports)
	}

	if opts.CIDFile != "" {
		if err := util.CreateIDFile(opts.CIDFile, con.ID); err != nil {
			// If you fail to create CIDFile then remove the container
			removeContainer(con.ID, opts.CIDFile, true)
			return nil, err
		}
	}

	report := entities.ContainerRunReport{Id: con.ID}

	if opts.Detach {
		// Detach and return early
		err := containers.Start(ic.ClientCtx, con.ID, new(containers.StartOptions).WithRecursive(true))
		if err != nil {
			report.ExitCode = define.ExitCode(err)
			if opts.Rm {
				removeContainer(con.ID, opts.CIDFile, true)
			}
		}
		return &report, err
	}

	// Attach
	if opts.SigProxy {
		remoteProxySignals(con.ID, func(signal string) error {
			killOpts := entities.KillOptions{All: false, Latest: false, Signal: signal}
			_, err := ic.ContainerKill(ctx, []string{con.ID}, killOpts)
			return err
		})
	}

	code, err := startAndAttach(ic, con.ID, &opts.DetachKeys, opts.SigProxy, opts.InputStream, opts.OutputStream, opts.ErrorStream)
	if err != nil {
		if err == define.ErrDetach {
			return &report, nil
		}

		report.ExitCode = define.ExitCode(err)
		if opts.Rm {
			removeContainer(con.ID, opts.CIDFile, false)
		}
		return &report, err
	}

	if opts.Rm {
		removeContainer(con.ID, opts.CIDFile, false)
	}

	report.ExitCode = code
	return &report, nil
}

func (ic *ContainerEngine) Diff(_ context.Context, namesOrIDs []string, opts entities.DiffOptions) (*entities.DiffReport, error) {
	var base string
	options := new(containers.DiffOptions).WithDiffType(opts.Type.String())
	if len(namesOrIDs) > 0 {
		base = namesOrIDs[0]
		if len(namesOrIDs) > 1 {
			options.WithParent(namesOrIDs[1])
		}
	} else {
		return nil, errors.New("no arguments for diff")
	}
	changes, err := containers.Diff(ic.ClientCtx, base, options)
	return &entities.DiffReport{Changes: changes}, err
}

func (ic *ContainerEngine) ContainerCleanup(_ context.Context, _ []string, _ entities.ContainerCleanupOptions) ([]*entities.ContainerCleanupReport, error) {
	return nil, errors.New("not implemented")
}

func (ic *ContainerEngine) ContainerInit(_ context.Context, namesOrIds []string, options entities.ContainerInitOptions) ([]*entities.ContainerInitReport, error) {
	ctrs, rawInputs, err := getContainersAndInputByContext(ic.ClientCtx, options.All, false, namesOrIds, nil)
	if err != nil {
		return nil, err
	}
	idToRawInput := map[string]string{}
	if len(rawInputs) == len(ctrs) {
		for i := range ctrs {
			idToRawInput[ctrs[i].ID] = rawInputs[i]
		}
	}
	reports := make([]*entities.ContainerInitReport, 0, len(ctrs))
	for _, c := range ctrs {
		err := containers.ContainerInit(ic.ClientCtx, c.ID, nil)
		// When using all, it is NOT considered an error if a container
		// has already been init'd.
		if err != nil && options.All && strings.Contains(err.Error(), define.ErrCtrStateInvalid.Error()) {
			err = nil
		}
		reports = append(reports, &entities.ContainerInitReport{
			Err:      err,
			RawInput: idToRawInput[c.ID],
			Id:       c.ID,
		})
	}
	return reports, nil
}

func (ic *ContainerEngine) ContainerMount(_ context.Context, _ []string, _ entities.ContainerMountOptions) ([]*entities.ContainerMountReport, error) {
	return nil, errors.New("mounting containers is not supported for remote clients")
}

func (ic *ContainerEngine) ContainerUnmount(_ context.Context, _ []string, _ entities.ContainerUnmountOptions) ([]*entities.ContainerUnmountReport, error) {
	return nil, errors.New("unmounting containers is not supported for remote clients")
}

func (ic *ContainerEngine) Config(_ context.Context) (*config.Config, error) {
	return config.Default()
}

func (ic *ContainerEngine) ContainerPort(_ context.Context, nameOrID string, options entities.ContainerPortOptions) ([]*entities.ContainerPortReport, error) {
	var (
		reports    = []*entities.ContainerPortReport{}
		namesOrIds = []string{}
	)
	if len(nameOrID) > 0 {
		namesOrIds = append(namesOrIds, nameOrID)
	}
	ctrs, err := getContainersByContext(ic.ClientCtx, options.All, false, namesOrIds)
	if err != nil {
		return nil, err
	}
	for _, con := range ctrs {
		if con.State != define.ContainerStateRunning.String() {
			continue
		}
		if len(con.Ports) > 0 {
			reports = append(reports, &entities.ContainerPortReport{
				Id:    con.ID,
				Ports: con.Ports,
			})
		}
	}
	return reports, nil
}

func (ic *ContainerEngine) ContainerCopyFromArchive(_ context.Context, nameOrID, path string, reader io.Reader, options entities.CopyOptions) (entities.ContainerCopyFunc, error) {
	copyOptions := new(containers.CopyOptions).WithChown(options.Chown).WithRename(options.Rename).WithNoOverwriteDirNonDir(options.NoOverwriteDirNonDir)
	return containers.CopyFromArchiveWithOptions(ic.ClientCtx, nameOrID, path, reader, copyOptions)
}

func (ic *ContainerEngine) ContainerCopyToArchive(_ context.Context, nameOrID string, path string, writer io.Writer) (entities.ContainerCopyFunc, error) {
	return containers.CopyToArchive(ic.ClientCtx, nameOrID, path, writer)
}

func (ic *ContainerEngine) ContainerStat(_ context.Context, nameOrID string, path string) (*entities.ContainerStatReport, error) {
	return containers.Stat(ic.ClientCtx, nameOrID, path)
}

// Shutdown Libpod engine.
func (ic *ContainerEngine) Shutdown(_ context.Context) {
}

func (ic *ContainerEngine) ContainerStats(_ context.Context, namesOrIds []string, options entities.ContainerStatsOptions) (statsChan chan entities.ContainerStatsReport, err error) {
	if options.Latest {
		return nil, errors.New("latest is not supported for the remote client")
	}
	return containers.Stats(ic.ClientCtx, namesOrIds, new(containers.StatsOptions).WithStream(options.Stream).WithInterval(options.Interval).WithAll(options.All))
}

// ContainerRename renames the given container.
func (ic *ContainerEngine) ContainerRename(_ context.Context, nameOrID string, opts entities.ContainerRenameOptions) error {
	return containers.Rename(ic.ClientCtx, nameOrID, new(containers.RenameOptions).WithName(opts.NewName))
}

func (ic *ContainerEngine) ContainerClone(_ context.Context, _ entities.ContainerCloneOptions) (*entities.ContainerCreateReport, error) {
	return nil, errors.New("cloning a container is not supported on the remote client")
}

// ContainerUpdate finds and updates the given container's cgroup config with the specified options
func (ic *ContainerEngine) ContainerUpdate(_ context.Context, updateOptions *entities.ContainerUpdateOptions) (string, error) {
	updateOptions.ProcessSpecgen()
	return containers.Update(ic.ClientCtx, updateOptions)
}
