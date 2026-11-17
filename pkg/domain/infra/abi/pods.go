//go:build !remote && (linux || freebsd)

package abi

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
	"go.podman.io/podman/v6/libpod"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/domain/entities"
	dfilters "go.podman.io/podman/v6/pkg/domain/filters"
	"go.podman.io/podman/v6/pkg/signal"
	"go.podman.io/podman/v6/pkg/specgen"
	"go.podman.io/podman/v6/pkg/specgen/generate"
)

// getPodsByContext returns a slice of pods. Note that all, latest and pods are
// mutually exclusive arguments.
func getPodsByContext(all, latest bool, pods []string, runtime *libpod.Runtime) ([]*libpod.Pod, error) {
	var outpods []*libpod.Pod
	if all {
		return runtime.GetAllPods()
	}
	if latest {
		p, err := runtime.GetLatestPod()
		if err != nil {
			return nil, err
		}
		outpods = append(outpods, p)
		return outpods, nil
	}
	var err error
	for _, p := range pods {
		pod, e := runtime.LookupPod(p)
		if e != nil {
			// Log all errors here, so callers don't need to.
			logrus.Debugf("Error looking up pod %q: %v", p, e)
			if err == nil {
				err = e
			}
		} else {
			outpods = append(outpods, pod)
		}
	}
	return outpods, err
}

func (ic *ContainerEngine) PodExists(_ context.Context, nameOrID string) (*entities.BoolReport, error) {
	_, err := ic.Libpod.LookupPod(nameOrID)
	if err != nil && !errors.Is(err, define.ErrNoSuchPod) {
		return nil, err
	}
	return &entities.BoolReport{Value: err == nil}, nil
}

func (ic *ContainerEngine) PodKill(ctx context.Context, namesOrIds []string, options entities.PodKillOptions) ([]*entities.PodKillReport, error) {
	reports := []*entities.PodKillReport{}
	sig, err := signal.ParseSignalNameOrNumber(options.Signal)
	if err != nil {
		return nil, err
	}
	pods, err := getPodsByContext(options.All, options.Latest, namesOrIds, ic.Libpod)
	if err != nil {
		return nil, err
	}

	for _, p := range pods {
		report := entities.PodKillReport{Id: p.ID()}
		conErrs, err := p.Kill(ctx, uint(sig))
		if err != nil && !errors.Is(err, define.ErrPodPartialFail) {
			report.Errs = []error{err}
			reports = append(reports, &report)
			continue
		}
		if len(conErrs) > 0 {
			for id, err := range conErrs {
				report.Errs = append(report.Errs, fmt.Errorf("killing container %s: %w", id, err))
			}
			reports = append(reports, &report)
			continue
		}
		reports = append(reports, &report)
	}
	return reports, nil
}

func (ic *ContainerEngine) PodLogs(ctx context.Context, nameOrID string, options entities.PodLogsOptions) error {
	// Implementation accepts slice
	podName := []string{nameOrID}
	pod, err := getPodsByContext(false, options.Latest, podName, ic.Libpod)
	if err != nil {
		return err
	}
	// Get pod containers
	podCtrs, err := pod[0].AllContainers()
	if err != nil {
		return err
	}

	ctrNames := []string{}
	// Check if `kubectl pod logs -c ctrname <podname>` alike command is used
	if options.ContainerName != "" {
		ctrFound := false
		for _, ctr := range podCtrs {
			if ctr.ID() == options.ContainerName || ctr.Name() == options.ContainerName {
				ctrNames = append(ctrNames, options.ContainerName)
				ctrFound = true
			}
		}
		if !ctrFound {
			return fmt.Errorf("container %s is not in pod %s: %w", options.ContainerName, nameOrID, define.ErrNoSuchCtr)
		}
	} else {
		// No container name specified select all containers
		for _, ctr := range podCtrs {
			ctrNames = append(ctrNames, ctr.Name())
		}
	}

	// PodLogsOptions are similar but contains few extra fields like ctrName
	// So cast other values as is so we can reuse the code
	containerLogsOpts := entities.PodLogsOptionsToContainerLogsOptions(options)

	return ic.ContainerLogs(ctx, ctrNames, containerLogsOpts)
}

func (ic *ContainerEngine) PodPause(ctx context.Context, namesOrIds []string, options entities.PodPauseOptions) ([]*entities.PodPauseReport, error) {
	reports := []*entities.PodPauseReport{}
	pods, err := getPodsByContext(options.All, options.Latest, namesOrIds, ic.Libpod)
	if err != nil {
		return nil, err
	}
	for _, p := range pods {
		report := entities.PodPauseReport{Id: p.ID()}
		errs, err := p.Pause(ctx)
		if err != nil && !errors.Is(err, define.ErrPodPartialFail) {
			report.Errs = []error{err}
			reports = append(reports, &report)
			continue
		}
		if len(errs) > 0 {
			for id, v := range errs {
				report.Errs = append(report.Errs, fmt.Errorf("pausing container %s: %w", id, v))
			}
			reports = append(reports, &report)
			continue
		}
		reports = append(reports, &report)
	}
	return reports, nil
}

func (ic *ContainerEngine) PodUnpause(ctx context.Context, namesOrIds []string, options entities.PodunpauseOptions) ([]*entities.PodUnpauseReport, error) {
	reports := []*entities.PodUnpauseReport{}
	pods, err := getPodsByContext(options.All, options.Latest, namesOrIds, ic.Libpod)
	if err != nil {
		return nil, err
	}
	for _, p := range pods {
		status, err := p.GetPodStatus()
		if err != nil {
			return nil, err
		}
		// If the pod is not paused or degraded, there is no need to attempt an unpause on it
		if status != define.PodStatePaused && status != define.PodStateDegraded {
			continue
		}
		report := entities.PodUnpauseReport{Id: p.ID()}
		errs, err := p.Unpause(ctx)
		if err != nil && !errors.Is(err, define.ErrPodPartialFail) {
			report.Errs = []error{err}
			reports = append(reports, &report)
			continue
		}
		if len(errs) > 0 {
			for id, v := range errs {
				report.Errs = append(report.Errs, fmt.Errorf("unpausing container %s: %w", id, v))
			}
			reports = append(reports, &report)
			continue
		}
		reports = append(reports, &report)
	}
	return reports, nil
}

func (ic *ContainerEngine) PodStop(ctx context.Context, namesOrIds []string, options entities.PodStopOptions) ([]*entities.PodStopReport, error) {
	reports := []*entities.PodStopReport{}
	pods, err := getPodsByContext(options.All, options.Latest, namesOrIds, ic.Libpod)
	if err != nil && (!options.Ignore || !errors.Is(err, define.ErrNoSuchPod)) {
		return nil, err
	}
	for _, p := range pods {
		report := entities.PodStopReport{
			Id:       p.ID(),
			RawInput: p.Name(),
		}
		errs, err := p.StopWithTimeout(ctx, true, options.Timeout)
		if err != nil && !errors.Is(err, define.ErrPodPartialFail) {
			report.Errs = []error{err}
			reports = append(reports, &report)
			continue
		}
		if len(errs) > 0 {
			for id, v := range errs {
				report.Errs = append(report.Errs, fmt.Errorf("stopping container %s: %w", id, v))
			}
			reports = append(reports, &report)
			continue
		}
		reports = append(reports, &report)
	}
	return reports, nil
}

func (ic *ContainerEngine) PodRestart(ctx context.Context, namesOrIds []string, options entities.PodRestartOptions) ([]*entities.PodRestartReport, error) {
	reports := []*entities.PodRestartReport{}
	pods, err := getPodsByContext(options.All, options.Latest, namesOrIds, ic.Libpod)
	if err != nil {
		return nil, err
	}
	for _, p := range pods {
		report := entities.PodRestartReport{Id: p.ID()}
		errs, err := p.Restart(ctx)
		if err != nil && !errors.Is(err, define.ErrPodPartialFail) {
			report.Errs = []error{err}
			reports = append(reports, &report)
			continue
		}
		if len(errs) > 0 {
			for id, v := range errs {
				report.Errs = append(report.Errs, fmt.Errorf("restarting container %s: %w", id, v))
			}
			reports = append(reports, &report)
			continue
		}
		reports = append(reports, &report)
	}
	return reports, nil
}

func (ic *ContainerEngine) PodStart(ctx context.Context, namesOrIds []string, options entities.PodStartOptions) ([]*entities.PodStartReport, error) {
	reports := []*entities.PodStartReport{}
	pods, err := getPodsByContext(options.All, options.Latest, namesOrIds, ic.Libpod)
	if err != nil {
		return nil, err
	}

	for _, p := range pods {
		report := entities.PodStartReport{
			Id:       p.ID(),
			RawInput: p.Name(),
		}
		errs, err := p.Start(ctx)
		if err != nil && !errors.Is(err, define.ErrPodPartialFail) {
			report.Errs = []error{err}
			reports = append(reports, &report)
			continue
		}
		if len(errs) > 0 {
			for id, v := range errs {
				report.Errs = append(report.Errs, fmt.Errorf("starting container %s: %w", id, v))
			}
			reports = append(reports, &report)
			continue
		}
		reports = append(reports, &report)
	}
	return reports, nil
}

func (ic *ContainerEngine) PodRm(ctx context.Context, namesOrIds []string, options entities.PodRmOptions) ([]*entities.PodRmReport, error) {
	pods, err := getPodsByContext(options.All, options.Latest, namesOrIds, ic.Libpod)
	if err != nil && (!options.Ignore || !errors.Is(err, define.ErrNoSuchPod)) {
		return nil, err
	}
	reports := make([]*entities.PodRmReport, 0, len(pods))
	for _, p := range pods {
		report := entities.PodRmReport{Id: p.ID()}
		ctrs, err := ic.Libpod.RemovePod(ctx, p, true, options.Force, options.Timeout)
		if err != nil {
			report.Err = err
		}
		report.RemovedCtrs = ctrs
		reports = append(reports, &report)
	}
	return reports, nil
}

func (ic *ContainerEngine) PodPrune(ctx context.Context, _ entities.PodPruneOptions) ([]*entities.PodPruneReport, error) {
	return ic.prunePodHelper(ctx)
}

func (ic *ContainerEngine) prunePodHelper(ctx context.Context) ([]*entities.PodPruneReport, error) {
	response, err := ic.Libpod.PrunePods(ctx)
	if err != nil {
		return nil, err
	}
	reports := make([]*entities.PodPruneReport, 0, len(response))
	for k, v := range response {
		reports = append(reports, &entities.PodPruneReport{
			Err: v,
			Id:  k,
		})
	}
	return reports, nil
}

func (ic *ContainerEngine) PodCreate(_ context.Context, specg entities.PodSpec) (*entities.PodCreateReport, error) {
	pod, err := generate.MakePod(&specg, ic.Libpod)
	if err != nil {
		return nil, err
	}
	return &entities.PodCreateReport{Id: pod.ID()}, nil
}

func (ic *ContainerEngine) PodClone(ctx context.Context, podClone entities.PodCloneOptions) (*entities.PodCloneReport, error) {
	spec := specgen.NewPodSpecGenerator()
	p, err := generate.PodConfigToSpec(ic.Libpod, spec, &podClone.InfraOptions, podClone.ID)
	if err != nil {
		return nil, err
	}

	if len(podClone.CreateOpts.Name) > 0 {
		spec.Name = podClone.CreateOpts.Name
	} else {
		n := p.Name()
		_, err := ic.Libpod.LookupPod(n + "-clone")
		if err == nil {
			n += "-clone"
		}
		switch {
		case strings.Contains(n, "-clone"): // meaning this name is taken!
			ind := strings.Index(n, "-clone") + 6
			num, err := strconv.Atoi(n[ind:])
			if num == 0 && err != nil { // meaning invalid
				_, err = ic.Libpod.LookupPod(n + "1")
				if err != nil {
					spec.Name = n + "1"
					break
				}
			} else { // else we already have a number
				n = n[0:ind]
			}
			err = nil
			count := num
			for err == nil { // until we cannot find a pod w/ this name, increment num and try again
				count++
				tempN := n + strconv.Itoa(count)
				_, err = ic.Libpod.LookupPod(tempN)
			}
			n += strconv.Itoa(count)
			spec.Name = n
		default:
			spec.Name = p.Name() + "-clone"
		}
	}

	podSpec := entities.PodSpec{PodSpecGen: *spec}
	pod, err := generate.MakePod(&podSpec, ic.Libpod)
	if err != nil {
		return nil, err
	}

	ctrs, err := p.AllContainers()
	if err != nil {
		return nil, err
	}
	for _, ctr := range ctrs {
		if ctr.IsInfra() {
			continue // already copied infra
		}

		podClone.PerContainerOptions.Pod = pod.ID()
		_, err := ic.ContainerClone(ctx, entities.ContainerCloneOptions{ID: ctr.ID(), CreateOpts: podClone.PerContainerOptions})
		if err != nil {
			return nil, err
		}
	}

	if podClone.Destroy {
		var timeout *uint
		_, err = ic.Libpod.RemovePod(ctx, p, true, true, timeout)
		if err != nil {
			// TODO: Possibly should handle case where containers
			// failed to remove - maybe compact the errors into a
			// multierror and return that?
			return &entities.PodCloneReport{Id: pod.ID()}, err
		}
	}

	if podClone.Start {
		_, err := ic.PodStart(ctx, []string{pod.ID()}, entities.PodStartOptions{})
		if err != nil {
			return &entities.PodCloneReport{Id: pod.ID()}, err
		}
	}

	return &entities.PodCloneReport{Id: pod.ID()}, nil
}

func (ic *ContainerEngine) PodTop(_ context.Context, options entities.PodTopOptions) (*entities.StringSliceReport, error) {
	var (
		pod *libpod.Pod
		err error
	)

	// Look up the pod.
	if options.Latest {
		pod, err = ic.Libpod.GetLatestPod()
	} else {
		pod, err = ic.Libpod.LookupPod(options.NameOrID)
	}
	if err != nil {
		return nil, fmt.Errorf("unable to look up requested container: %w", err)
	}

	// Run Top.
	report := &entities.StringSliceReport{}
	report.Value, err = pod.GetPodPidInformation(options.Descriptors)
	return report, err
}

func (ic *ContainerEngine) listPodReportFromPod(p *libpod.Pod) (*entities.ListPodsReport, error) {
	status, err := p.GetPodStatus()
	if err != nil {
		return nil, err
	}
	cons, err := p.AllContainers()
	if err != nil {
		return nil, err
	}
	lpcs := make([]*entities.ListPodContainer, len(cons))
	for i, c := range cons {
		state, err := c.State()
		if err != nil {
			return nil, err
		}
		restartCount, err := c.RestartCount()
		if err != nil {
			return nil, err
		}
		lpcs[i] = &entities.ListPodContainer{
			Id:           c.ID(),
			Names:        c.Name(),
			Status:       state.String(),
			RestartCount: restartCount,
		}
	}
	infraID, err := p.InfraContainerID()
	if err != nil {
		return nil, err
	}
	networks := []string{}
	if len(infraID) > 0 {
		infra, err := p.InfraContainer()
		if err != nil {
			return nil, err
		}
		networks, err = infra.Networks()
		if err != nil {
			return nil, err
		}
	}
	return &entities.ListPodsReport{
		Cgroup:     p.CgroupParent(),
		Containers: lpcs,
		Created:    p.CreatedTime(),
		Id:         p.ID(),
		InfraId:    infraID,
		Name:       p.Name(),
		Namespace:  p.Namespace(),
		Networks:   networks,
		Status:     status,
		Labels:     p.Labels(),
	}, nil
}

func (ic *ContainerEngine) PodPs(_ context.Context, options entities.PodPSOptions) ([]*entities.ListPodsReport, error) {
	var (
		err error
		pds = []*libpod.Pod{}
	)

	filters := make([]libpod.PodFilter, 0, len(options.Filters))
	for k, v := range options.Filters {
		f, err := dfilters.GeneratePodFilterFunc(k, v, ic.Libpod)
		if err != nil {
			return nil, err
		}
		filters = append(filters, f)
	}
	if options.Latest {
		pod, err := ic.Libpod.GetLatestPod()
		if err != nil {
			return nil, err
		}
		pds = append(pds, pod)
	} else {
		pds, err = ic.Libpod.Pods(filters...)
		if err != nil {
			return nil, err
		}
	}

	reports := make([]*entities.ListPodsReport, 0, len(pds))
	for _, p := range pds {
		r, err := ic.listPodReportFromPod(p)
		if err != nil {
			if errors.Is(err, define.ErrNoSuchPod) || errors.Is(err, define.ErrNoSuchCtr) {
				continue
			}
			return nil, err
		}
		reports = append(reports, r)
	}
	return reports, nil
}

func (ic *ContainerEngine) PodInspect(_ context.Context, nameOrIDs []string, options entities.InspectOptions) ([]*entities.PodInspectReport, []error, error) {
	if options.Latest {
		pod, err := ic.Libpod.GetLatestPod()
		if err != nil {
			return nil, nil, err
		}
		inspect, err := pod.Inspect()
		if err != nil {
			return nil, nil, err
		}

		return []*entities.PodInspectReport{
			{
				InspectPodData: inspect,
			},
		}, nil, nil
	}

	var errs []error
	podReport := make([]*entities.PodInspectReport, 0, len(nameOrIDs))
	for _, name := range nameOrIDs {
		pod, err := ic.Libpod.LookupPod(name)
		if err != nil {
			// ErrNoSuchPod is non-fatal, other errors will be
			// treated as fatal.
			if errors.Is(err, define.ErrNoSuchPod) {
				errs = append(errs, fmt.Errorf("no such pod %s", name))
				continue
			}
			return nil, nil, err
		}

		inspect, err := pod.Inspect()
		if err != nil {
			// ErrNoSuchPod is non-fatal, other errors will be
			// treated as fatal.
			if errors.Is(err, define.ErrNoSuchPod) {
				errs = append(errs, fmt.Errorf("no such pod %s", name))
				continue
			}
			return nil, nil, err
		}
		podReport = append(podReport, &entities.PodInspectReport{InspectPodData: inspect})
	}
	return podReport, errs, nil
}
