//go:build !remote && (linux || freebsd)

package libpod

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/namespaces"
	"go.podman.io/storage/pkg/fileutils"
)

// Migrate stops the rootless pause process and performs any necessary database
// migrations that are required. It can also migrate all containers to a new OCI
// runtime, if requested.
func (r *Runtime) Migrate(newRuntime string, migrateDB bool) error {
	// Acquire the alive lock and hold it.
	// Ensures that we don't let other Podman commands run while we are
	// rewriting things in the DB.
	aliveLock, err := r.getRuntimeAliveLock()
	if err != nil {
		return fmt.Errorf("retrieving alive lock: %w", err)
	}
	aliveLock.Lock()
	defer aliveLock.Unlock()

	if !r.valid {
		return define.ErrRuntimeStopped
	}

	runningContainers, err := r.GetRunningContainers()
	if err != nil {
		return err
	}

	allCtrs, err := r.state.AllContainers(false)
	if err != nil {
		return err
	}

	logrus.Infof("Stopping all containers")
	for _, ctr := range runningContainers {
		fmt.Printf("stopped %s\n", ctr.ID())
		if err := ctr.Stop(); err != nil {
			return fmt.Errorf("cannot stop container %s: %w", ctr.ID(), err)
		}
	}

	// Did the user request a new runtime?
	runtimeChangeRequested := newRuntime != ""
	var requestedRuntime OCIRuntime
	if runtimeChangeRequested {
		runtime, exists := r.ociRuntimes[newRuntime]
		if !exists {
			return fmt.Errorf("change to runtime %q requested but no such runtime is defined: %w", newRuntime, define.ErrInvalidArg)
		}
		requestedRuntime = runtime
	}

	for _, ctr := range allCtrs {
		needsWrite := false

		// Reset pause process location
		oldLocation := filepath.Join(ctr.state.RunDir, "conmon.pid")
		if ctr.config.ConmonPidFile == oldLocation {
			logrus.Infof("Changing conmon PID file for %s", ctr.ID())
			ctr.config.ConmonPidFile = filepath.Join(ctr.config.StaticDir, "conmon.pid")
			needsWrite = true
		}

		// Migrate slirp4netns containers to pasta
		if ctr.config.NetMode == "slirp4netns" || strings.HasPrefix(string(ctr.config.NetMode), "slirp4netns:") {
			logrus.Infof("Migrating container %s from slirp4netns to pasta", ctr.ID())
			if opts, ok := ctr.config.NetworkOptions["slirp4netns"]; ok && len(opts) > 0 {
				logrus.Warnf("Container %s: dropping slirp4netns options %v; see podman-run(1) pasta section for equivalent options", ctr.ID(), opts)
			}
			ctr.config.NetMode = namespaces.NetworkMode("pasta")
			delete(ctr.config.NetworkOptions, "slirp4netns")
			needsWrite = true
		}

		// Reset runtime
		if runtimeChangeRequested && ctr.config.OCIRuntime != newRuntime {
			logrus.Infof("Resetting container %s runtime to runtime %s", ctr.ID(), newRuntime)
			ctr.config.OCIRuntime = newRuntime
			ctr.ociRuntime = requestedRuntime

			needsWrite = true
		}

		if needsWrite {
			if err := r.state.RewriteContainerConfig(ctr, ctr.config); err != nil {
				return fmt.Errorf("rewriting config for container %s: %w", ctr.ID(), err)
			}
		}
	}

	if migrateDB {
		if err := r.checkCanMigrate(); err != nil {
			switch {
			case errors.Is(err, errCannotMigrateNoBolt):
				fmt.Printf("No migration is necessary: %v\n", err)
				return r.stopPauseProcess()
			default:
				return err
			}
		}

		if err := r.migrateDB(); err != nil {
			return fmt.Errorf("migrating database from BoltDB to SQLite: %w", err)
		}
	}

	return r.stopPauseProcess()
}

var errCannotMigrateNoBolt = errors.New("no BoltDB database to migrate")

func (r *Runtime) checkCanMigrate() error {
	boltPath := getBoltDBPath(r)
	if err := fileutils.Exists(boltPath); err != nil {
		return errCannotMigrateNoBolt
	}

	return nil
}

func (r *Runtime) migrateDB() error {
	boltPath := getBoltDBPath(r)
	// Get us a Bolt database
	oldState, err := NewBoltState(boltPath, r)
	if err != nil {
		return fmt.Errorf("opening legacy Bolt database at %s: %w", boltPath, err)
	}

	// Migrate volumes, then pods, then containers.
	// Containers must be last as the pods they are part of and volumes they use must already exist.
	allVolumes, err := oldState.AllVolumes()
	if err != nil {
		return fmt.Errorf("retrieving volumes from boltdb: %w", err)
	}
	for _, vol := range allVolumes {
		if err := r.state.AddVolume(vol); err != nil {
			if errors.Is(err, define.ErrVolumeExists) {
				logrus.Warnf("Volume with name %s already exists in the SQLite database; refusing to migrate from BoltDB", vol.Name())
				continue
			}
			return err
		}
		if err := oldState.UpdateVolume(vol); err != nil {
			return err
		}
		if err := r.state.SaveVolume(vol); err != nil {
			return err
		}
	}

	allPods, err := oldState.AllPods()
	if err != nil {
		return fmt.Errorf("retrieving pods from boltdb: %w", err)
	}
	for _, pod := range allPods {
		if err := r.state.AddPod(pod); err != nil {
			if errors.Is(err, define.ErrPodExists) {
				logrus.Warnf("Pod with name %s already exists in the SQLite database; refusing to migrate from BoltDB", pod.Name())
				continue
			}
			return err
		}
		if err := oldState.UpdatePod(pod); err != nil {
			return err
		}
		if err := r.state.SavePod(pod); err != nil {
			return err
		}
	}

	// Containers must be done as a graph due to dependencies.
	// The state will error if we add a container before its dependencies.
	allCtrs, err := oldState.AllContainers(true)
	if err != nil {
		return fmt.Errorf("retrieving containers from boltdb: %w", err)
	}

	// BoltDB doesn't actually populate container networks on initial pull
	// from the database, that needs to be done separately.
	for _, ctr := range allCtrs {
		ctrNetworks, err := oldState.GetNetworks(ctr)
		if err != nil {
			return err
		}
		ctr.config.Networks = convertLegacyNetworks(ctrNetworks)
	}

	graph, err := BuildContainerGraph(allCtrs)
	if err != nil {
		return err
	}

	ctrErrors := make(map[string]error)
	ctrsVisited := make(map[string]bool)

	for _, node := range graph.noDepNodes {
		migrateNodeDatabase(node, false, ctrErrors, ctrsVisited, r.state)
	}
	if len(ctrErrors) > 0 {
		newErrors := make([]error, 0, len(ctrErrors))
		for id, err := range ctrErrors {
			newErrors = append(newErrors, fmt.Errorf("migrating container %s: %w", id, err))
		}
		return errors.Join(newErrors...)
	}

	oldState.Close()

	// Move the Bolt database so it is not reused, but preserve so data is not lost.
	newBoltDBPath := fmt.Sprintf("%s-old", boltPath)
	if err := os.Rename(boltPath, newBoltDBPath); err != nil {
		return fmt.Errorf("renaming old database %s to %s: %w", boltPath, newBoltDBPath, err)
	}
	fmt.Printf("Old database has been renamed to %s and will no longer be used\n", newBoltDBPath)

	return nil
}

func getBoltDBPath(runtime *Runtime) string {
	baseDir := runtime.config.Engine.StaticDir
	if runtime.storageConfig.TransientStore {
		baseDir = runtime.config.Engine.TmpDir
	}
	return filepath.Join(baseDir, "bolt_state.db")
}
