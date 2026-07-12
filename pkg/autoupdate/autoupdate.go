//go:build !remote && (linux || freebsd)

package autoupdate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"

	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/sirupsen/logrus"
	"go.podman.io/common/libimage"
	"go.podman.io/common/pkg/config"
	"go.podman.io/image/v5/docker"
	"go.podman.io/podman/v6/libpod"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/libpod/events"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/systemd"
	systemdDefine "go.podman.io/podman/v6/pkg/systemd/define"
)

// Policy represents an auto-update policy.
type Policy string

const (
	// PolicyDefault is the default policy denoting no auto updates.
	PolicyDefault Policy = "disabled"
	// PolicyRegistryImage is the policy to update as soon as there's a new image found.
	PolicyRegistryImage = "registry"
	// PolicyLocalImage is the policy to run auto-update based on a local image
	PolicyLocalImage = "local"
)

// Map for easy lookups of supported policies.
var supportedPolicies = map[string]Policy{
	"":                          PolicyDefault,
	string(PolicyDefault):       PolicyDefault,
	"image":                     PolicyRegistryImage, // Deprecated in favor of PolicyRegistryImage
	string(PolicyRegistryImage): PolicyRegistryImage,
	string(PolicyLocalImage):    PolicyLocalImage,
}

// updater includes shared state for auto-updating one or more containers.
type updater struct {
	conn             *dbus.Conn                  // DBUS connection
	options          *entities.AutoUpdateOptions // User-specified options
	unitToTasks      map[string][]*task          // Keeps track of tasks per unit
	updatedRawImages map[string]bool             // Keeps track of updated images
	runtime          *libpod.Runtime             // The libpod runtime
}

const (
	statusFailed     = "failed"      // The update has failed
	statusUpdated    = "true"        // The update succeeded
	statusNotUpdated = "false"       // No update was needed
	statusPending    = "pending"     // The update is pending (see options.DryRun)
	statusRolledBack = "rolled back" // Rollback after a failed update
)

// task includes data and state for updating a container
type task struct {
	authfile     string            // Container-specific authfile
	auto         *updater          // Reverse pointer to the updater
	container    *libpod.Container // Container to update
	policy       Policy            // Update policy
	image        *libimage.Image   // Original image before the update
	rawImageName string            // The container's raw image name
	status       string            // Auto-update status
	unit         string            // Name of the systemd unit
}

// LookupPolicy looks up the corresponding Policy for the specified
// string. If none is found, an errors is returned including the list of
// supported policies.
//
// Note that an empty string resolved to PolicyDefault.
func LookupPolicy(s string) (Policy, error) {
	policy, exists := supportedPolicies[s]
	if exists {
		return policy, nil
	}

	// Sort the keys first as maps are non-deterministic.
	keys := []string{}
	for k := range supportedPolicies {
		if k != "" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	return "", fmt.Errorf("invalid auto-update policy %q: valid policies are %+q", s, keys)
}

// / AutoUpdate looks up containers with a specified auto-update policy and acts
// accordingly.
//
// If the policy is set to PolicyRegistryImage, it checks if the image
// on the remote registry is different than the local one. If the image digests
// differ, it pulls the remote image and restarts the systemd unit running the
// container.
//
// If the policy is set to PolicyLocalImage, it checks if the image
// of a running container is different than the local one. If the image digests
// differ, it restarts the systemd unit with the new image.
//
// It returns a slice of successfully restarted systemd units and a slice of
// errors encountered during auto update.
func AutoUpdate(ctx context.Context, runtime *libpod.Runtime, options entities.AutoUpdateOptions) ([]*entities.AutoUpdateReport, []error) {
	// Note that (most) errors are non-fatal such that a single
	// misconfigured container does not prevent others from being updated
	// (which could be a security threat).

	auto := updater{
		options:          &options,
		runtime:          runtime,
		updatedRawImages: make(map[string]bool),
	}

	// Find auto-update tasks and assemble them by unit.
	allErrors := auto.assembleTasks(ctx)

	// Nothing to do.
	if len(auto.unitToTasks) == 0 {
		return nil, allErrors
	}

	// Connect to DBUS.
	conn, err := systemd.ConnectToDBUS()
	if err != nil {
		logrus.Error(err.Error())
		allErrors = append(allErrors, err)
		return nil, allErrors
	}
	defer conn.Close()
	auto.conn = conn

	runtime.NewSystemEvent(events.AutoUpdate)

	// Update all images/container according to their auto-update policy.
	var allReports []*entities.AutoUpdateReport
	for unit, tasks := range auto.unitToTasks {
		unitErrors := auto.updateUnit(ctx, unit, tasks)
		allErrors = append(allErrors, unitErrors...)
		for _, task := range tasks {
			allReports = append(allReports, task.report())
		}
	}

	return allReports, allErrors
}

// updateUnit auto updates the tasks in the specified systemd unit.
func (u *updater) updateUnit(ctx context.Context, unit string, tasks []*task) []error {
	var errors []error
	tasksUpdated := false

	for _, task := range tasks {
		err := func() error { // Use an anonymous function to avoid spaghetti continue's
			updateAvailable, err := task.updateAvailable(ctx)
			if err != nil {
				task.status = statusFailed
				return fmt.Errorf("checking image updates for container %s: %w", task.container.ID(), err)
			}

			if !updateAvailable {
				task.status = statusNotUpdated
				return nil
			}

			if u.options.DryRun {
				task.status = statusPending
				return nil
			}

			if err := task.update(ctx); err != nil {
				task.status = statusFailed
				return fmt.Errorf("updating image for container %s: %w", task.container.ID(), err)
			}

			tasksUpdated = true
			return nil
		}()
		if err != nil {
			errors = append(errors, err)
		}
	}

	// If no task has been updated, we can jump directly to the next unit.
	if !tasksUpdated {
		return errors
	}

	updateError := u.restartSystemdUnit(ctx, unit)
	for _, task := range tasks {
		if updateError == nil {
			task.status = statusUpdated
		} else {
			task.status = statusFailed
		}
	}

	// Jump to the next unit on successful update or if rollbacks are disabled.
	if updateError == nil || !u.options.Rollback {
		if updateError != nil {
			errors = append(errors, fmt.Errorf("restarting unit %s during update: %w", unit, updateError))
		}
		return errors
	}

	// The update has failed and rollbacks are enabled.
	for _, task := range tasks {
		if err := task.rollbackImage(); err != nil {
			err = fmt.Errorf("rolling back image for container %s in unit %s: %w", task.container.ID(), unit, err)
			errors = append(errors, err)
		}
	}

	if err := u.restartSystemdUnit(ctx, unit); err != nil {
		for _, task := range tasks {
			task.status = statusFailed
		}
		err = fmt.Errorf("restarting unit %s during rollback: %w", unit, err)
		errors = append(errors, err)
		return errors
	}

	for _, task := range tasks {
		task.status = statusRolledBack
	}

	return errors
}

// report creates an auto-update report for the task.
func (t *task) report() *entities.AutoUpdateReport {
	return &entities.AutoUpdateReport{
		ContainerID:   t.container.ID(),
		ContainerName: t.container.Name(),
		ImageName:     t.container.RawImageName(),
		Policy:        string(t.policy),
		SystemdUnit:   t.unit,
		Updated:       t.status,
	}
}

// updateAvailable returns whether an update for the task is available.
func (t *task) updateAvailable(ctx context.Context) (bool, error) {
	switch t.policy {
	case PolicyRegistryImage:
		return t.registryUpdateAvailable(ctx)
	case PolicyLocalImage:
		return t.localUpdateAvailable()
	default:
		return false, fmt.Errorf("unexpected auto-update policy %s for container %s", t.policy, t.container.ID())
	}
}

// update the task according to its auto-update policy.
func (t *task) update(ctx context.Context) error {
	switch t.policy {
	case PolicyRegistryImage:
		return t.registryUpdate(ctx)
	case PolicyLocalImage:
		// Nothing to do as the image is already available in the local storage.
		return nil
	default:
		return fmt.Errorf("unexpected auto-update policy %s for container %s", t.policy, t.container.ID())
	}
}

// registryUpdateAvailable returns whether a new image on the registry is available.
func (t *task) registryUpdateAvailable(ctx context.Context) (bool, error) {
	// The newer image has already been pulled for another task, so we know
	// there's a newer one available.
	if _, exists := t.auto.updatedRawImages[t.rawImageName]; exists {
		return true, nil
	}

	remoteRef, err := docker.ParseReference("//" + t.rawImageName)
	if err != nil {
		return false, err
	}
	options := &libimage.HasDifferentDigestOptions{
		AuthFilePath:          t.authfile,
		InsecureSkipTLSVerify: t.auto.options.InsecureSkipTLSVerify,
	}
	return t.image.HasDifferentDigest(ctx, remoteRef, options)
}

// registryUpdate pulls down the image from the registry.
func (t *task) registryUpdate(ctx context.Context) error {
	// The newer image has already been pulled for another task.
	if _, exists := t.auto.updatedRawImages[t.rawImageName]; exists {
		return nil
	}

	pullOptions := &libimage.PullOptions{}
	pullOptions.AuthFilePath = t.authfile
	pullOptions.Writer = os.Stderr
	pullOptions.InsecureSkipTLSVerify = t.auto.options.InsecureSkipTLSVerify
	if _, err := t.auto.runtime.LibimageRuntime().Pull(ctx, t.rawImageName, config.PullPolicyAlways, pullOptions); err != nil {
		return err
	}

	t.auto.updatedRawImages[t.rawImageName] = true
	return nil
}

// localUpdateAvailable returns whether a new image in the local storage is available.
func (t *task) localUpdateAvailable() (bool, error) {
	localImg, _, err := t.auto.runtime.LibimageRuntime().LookupImage(t.rawImageName, nil)
	if err != nil {
		return false, err
	}
	return localImg.ID() != t.image.ID(), nil
}

// rollbackImage rolls back the task's image to the previous version before the update.
func (t *task) rollbackImage() error {
	// To fallback, simply retag the old image and restart the service.
	if err := t.image.Tag(t.rawImageName); err != nil {
		return err
	}
	t.auto.updatedRawImages[t.rawImageName] = false
	return nil
}

// restartSystemdUnit restarts the systemd unit the container is running in.
func (u *updater) restartSystemdUnit(ctx context.Context, unit string) error {
	restartChan := make(chan string)
	if _, err := u.conn.RestartUnitContext(ctx, unit, "replace", restartChan); err != nil {
		return err
	}

	// Wait for the restart to finish and actually check if it was
	// successful or not.
	result := <-restartChan

	switch result {
	case "done":
		logrus.Infof("Successfully restarted systemd unit %q", unit)
		return nil

	default:
		return fmt.Errorf("error restarting systemd unit %q expected %q but received %q", unit, "done", result)
	}
}

// assembleTasks assembles update tasks per unit and populates a mapping from
// `unit -> []*task` such that multiple containers _can_ run in a single unit.
func (u *updater) assembleTasks(ctx context.Context) []error {
	// Assemble a map `image ID -> *libimage.Image` that we can consult
	// later on for lookups.
	imageMap, err := u.assembleImageMap(ctx)
	if err != nil {
		return []error{err}
	}

	allContainers, err := u.runtime.GetAllContainers()
	if err != nil {
		return []error{err}
	}

	u.unitToTasks = make(map[string][]*task)

	errs := []error{}
	for _, c := range allContainers {
		ctr := c
		state, err := ctr.State()
		if err != nil {
			// container may have been removed in the meantime ignore it and not print errors
			if !errors.Is(err, define.ErrNoSuchCtr) {
				errs = append(errs, err)
			}
			continue
		}
		// Only update running containers.
		if state != define.ContainerStateRunning {
			continue
		}

		// Check the container's auto-update policy which is configured
		// as a label.
		labels := ctr.Labels()
		value, exists := labels[define.AutoUpdateLabel]
		if !exists {
			continue
		}
		policy, err := LookupPolicy(value)
		if err != nil {
			errs = append(errs, fmt.Errorf("auto-updating container %q: %w", ctr.ID(), err))
			continue
		}
		if policy == PolicyDefault {
			continue
		}

		// Make sure the container runs in a systemd unit which is
		// stored as a label at container creation.
		unit, exists, err := u.systemdUnitForContainer(ctr, labels)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if !exists {
			errs = append(errs, fmt.Errorf("auto-updating container %q: no %s label found", ctr.ID(), systemdDefine.EnvVariable))
			continue
		}

		id, _ := ctr.Image()
		image, exists := imageMap[id]
		if !exists {
			err := fmt.Errorf("internal error: no image found for ID %s", id)
			errs = append(errs, err)
			continue
		}

		rawImageName := ctr.RawImageName()
		if rawImageName == "" {
			errs = append(errs, fmt.Errorf("locally auto-updating container %q: raw-image name is empty", ctr.ID()))
			continue
		}

		// Use user-specified auth file (CLI or env variable) unless
		// the container was created with the auth-file label.
		authfile := u.options.Authfile
		if fromContainer, ok := labels[define.AutoUpdateAuthfileLabel]; ok {
			authfile = fromContainer
		}
		t := task{
			authfile:     authfile,
			auto:         u,
			container:    ctr,
			policy:       policy,
			image:        image,
			unit:         unit,
			rawImageName: rawImageName,
			status:       statusFailed, // must be updated later on
		}

		// Add the task to the unit.
		u.unitToTasks[unit] = append(u.unitToTasks[unit], &t)
	}

	return errs
}

// systemdUnitForContainer returns the name of the container's systemd unit.
// If the container is part of a pod, the pod's infra container's systemd unit
// is returned.  This allows for auto update to restart the pod's systemd unit.
func (u *updater) systemdUnitForContainer(c *libpod.Container, labels map[string]string) (string, bool, error) {
	podID := c.ConfigNoCopy().Pod
	if podID == "" {
		unit, exists := labels[systemdDefine.EnvVariable]
		return unit, exists, nil
	}

	pod, err := u.runtime.LookupPod(podID)
	if err != nil {
		return "", false, fmt.Errorf("looking up pod's systemd unit: %w", err)
	}

	infra, err := pod.InfraContainer()
	if err != nil {
		return "", false, fmt.Errorf("looking up pod's systemd unit: %w", err)
	}

	infraLabels := infra.Labels()
	unit, exists := infraLabels[systemdDefine.EnvVariable]
	return unit, exists, nil
}

// assembleImageMap creates a map from `image ID -> *libimage.Image` for image lookups.
func (u *updater) assembleImageMap(ctx context.Context) (map[string]*libimage.Image, error) {
	listOptions := &libimage.ListImagesOptions{
		Filters: []string{"readonly=false"},
	}
	imagesSlice, err := u.runtime.LibimageRuntime().ListImages(ctx, listOptions)
	if err != nil {
		return nil, err
	}
	imageMap := make(map[string]*libimage.Image)
	for i := range imagesSlice {
		imageMap[imagesSlice[i].ID()] = imagesSlice[i]
	}

	return imageMap, nil
}
