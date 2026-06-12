//go:build !remote && (linux || freebsd)

package abi

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/coreos/go-systemd/v22/daemon"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/selinux/go-selinux"
	"github.com/sirupsen/logrus"
	buildahDefine "go.podman.io/buildah/define"
	bparse "go.podman.io/buildah/pkg/parse"
	"go.podman.io/common/libimage"
	nettypes "go.podman.io/common/libnetwork/types"
	"go.podman.io/common/pkg/config"
	"go.podman.io/common/pkg/secrets"
	"go.podman.io/image/v5/docker/reference"
	"go.podman.io/image/v5/types"
	"go.podman.io/podman/v6/cmd/podman/parse"
	"go.podman.io/podman/v6/libpod"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/annotations"
	"go.podman.io/podman/v6/pkg/domain/entities"
	entitiesTypes "go.podman.io/podman/v6/pkg/domain/entities/types"
	"go.podman.io/podman/v6/pkg/domain/infra/abi/internal/expansion"
	v1apps "go.podman.io/podman/v6/pkg/k8s.io/api/apps/v1"
	v1 "go.podman.io/podman/v6/pkg/k8s.io/api/core/v1"
	metav1 "go.podman.io/podman/v6/pkg/k8s.io/apimachinery/pkg/apis/meta/v1"
	"go.podman.io/podman/v6/pkg/specgen"
	"go.podman.io/podman/v6/pkg/specgen/generate"
	"go.podman.io/podman/v6/pkg/specgen/generate/kube"
	"go.podman.io/podman/v6/pkg/specgenutil"
	"go.podman.io/podman/v6/pkg/systemd/notifyproxy"
	"go.podman.io/podman/v6/pkg/util"
	"go.podman.io/storage/pkg/chrootarchive"
	"go.podman.io/storage/pkg/fileutils"
	yamlv3 "gopkg.in/yaml.v3"
	"sigs.k8s.io/yaml"
)

// sdNotifyAnnotation allows for configuring service-global and
// container-specific sd-notify modes.
const sdNotifyAnnotation = "io.containers.sdnotify"

// default network created/used by kube
const kubeDefaultNetwork = "podman-default-kube-network"

// createServiceContainer creates a container that can later on
// be associated with the pods of a K8s yaml.  It will be started along with
// the first pod.
func (ic *ContainerEngine) createServiceContainer(ctx context.Context, name string, options entities.PlayKubeOptions) (*libpod.Container, error) {
	// Make sure to replace the service container as well if requested by
	// the user.
	if options.Replace {
		if _, err := ic.ContainerRm(ctx, []string{name}, entities.RmOptions{Force: true, Ignore: true}); err != nil {
			return nil, fmt.Errorf("replacing service container: %w", err)
		}
	}

	rtc, err := ic.Libpod.GetConfigNoCopy()
	if err != nil {
		return nil, err
	}
	ctrOpts := entities.ContainerCreateOptions{
		// Inherited from infra containers
		IsInfra:          false,
		MemorySwappiness: -1,
		ReadOnly:         true,
		ReadWriteTmpFS:   false,
		// No need to set up networking.
		Net:                  &entities.NetOptions{Network: specgen.Namespace{NSMode: specgen.NoNetwork}},
		StopTimeout:          rtc.Engine.StopTimeout,
		HealthLogDestination: define.DefaultHealthCheckLocalDestination,
		HealthMaxLogCount:    define.DefaultHealthMaxLogCount,
		HealthMaxLogSize:     define.DefaultHealthMaxLogSize,
	}

	// Create and fill out the runtime spec.
	s := specgen.NewSpecGenerator("", true)
	if err := specgenutil.FillOutSpecGen(s, &ctrOpts, []string{}); err != nil {
		return nil, fmt.Errorf("completing spec for service container: %w", err)
	}
	s.Name = name

	expandForKube(s)
	runtimeSpec, spec, opts, err := generate.MakeContainer(ctx, ic.Libpod, s, false, nil)
	if err != nil {
		return nil, fmt.Errorf("creating runtime spec for service container: %w", err)
	}

	ecp, err := define.ParseKubeExitCodePropagation(options.ExitCodePropagation)
	if err != nil {
		return nil, err
	}
	opts = append(opts, libpod.WithIsService(ecp))

	// Set the sd-notify mode to "ignore".  Podman is responsible for
	// sending the notify messages when all containers are ready.
	// The mode for individual containers or entire pods can be configured
	// via the `sdNotifyAnnotation` annotation in the K8s YAML.
	opts = append(opts, libpod.WithSdNotifyMode(define.SdNotifyModeIgnore))

	if options.Replace {
		if _, err := ic.ContainerRm(ctx, []string{spec.Name}, entities.RmOptions{Force: true, Ignore: true}); err != nil {
			return nil, err
		}
	}

	// Create a new libpod container based on the spec.
	ctr, err := ic.Libpod.NewContainer(ctx, runtimeSpec, spec, false, opts...)
	if err != nil {
		return nil, fmt.Errorf("creating service container: %w", err)
	}

	return ctr, nil
}

func (ic *ContainerEngine) prepareAutomountImages(ctx context.Context, forContainer string, annotations map[string]string) ([]*specgen.ImageVolume, error) {
	volMap := make(map[string]*specgen.ImageVolume)

	ctrAnnotation := define.KubeImageAutomountAnnotation + "/" + forContainer

	automount, ok := annotations[ctrAnnotation]
	if !ok || automount == "" {
		return nil, nil
	}

	for imageName := range strings.SplitSeq(automount, ";") {
		img, fullName, err := ic.Libpod.LibimageRuntime().LookupImage(imageName, nil)
		if err != nil {
			return nil, fmt.Errorf("image %s from container %s does not exist in local storage, cannot automount: %w", imageName, forContainer, err)
		}

		logrus.Infof("Resolved image name %s to %s for automount into container %s", imageName, fullName, forContainer)

		inspect, err := img.Inspect(ctx, nil)
		if err != nil {
			return nil, fmt.Errorf("cannot inspect image %s to automount into container %s: %w", fullName, forContainer, err)
		}

		volumes := inspect.Config.Volumes

		for path := range volumes {
			if oldPath, ok := volMap[path]; ok && oldPath != nil {
				logrus.Warnf("Multiple volume mounts to %q requested, overriding image %q with image %s", path, oldPath.Source, fullName)
			}

			imgVol := new(specgen.ImageVolume)
			imgVol.Source = fullName
			imgVol.Destination = path
			imgVol.ReadWrite = false
			imgVol.SubPath = path

			volMap[path] = imgVol
		}
	}

	toReturn := make([]*specgen.ImageVolume, 0, len(volMap))
	for _, vol := range volMap {
		toReturn = append(toReturn, vol)
	}

	return toReturn, nil
}

func prepareVolumesFrom(forContainer, podName string, ctrNames, annotations map[string]string) ([]string, error) {
	annotationVolsFrom := define.VolumesFromAnnotation + "/" + forContainer

	volsFromCtrs, ok := annotations[annotationVolsFrom]

	// No volumes-from specified
	if !ok || volsFromCtrs == "" {
		return nil, nil
	}

	// The volumes-from string is a semicolon-separated container names
	// optionally with respective mount options.
	volumesFrom := strings.Split(volsFromCtrs, ";")
	for idx, volsFromCtr := range volumesFrom {
		// Each entry is of format "container[:mount-options]"
		fields := strings.Split(volsFromCtr, ":")
		if len(fields) != 1 && len(fields) != 2 {
			return nil, fmt.Errorf("invalid annotation %s value", annotationVolsFrom)
		}

		if fields[0] == "" {
			return nil, fmt.Errorf("from container name cannot be empty in annotation %s", annotationVolsFrom)
		}

		// Source and target containers cannot be same
		if fields[0] == forContainer {
			return nil, fmt.Errorf("to and from container names cannot be same in annotation %s", annotationVolsFrom)
		}

		// Update the source container name if it belongs to the pod
		// the source container must exist before the target container
		// in the kube yaml. Otherwise, the source container will be
		// treated as an external container. This also helps in avoiding
		// cyclic dependencies between containers within the pod.
		if _, ok := ctrNames[fields[0]]; ok {
			volumesFrom[idx] = podName + "-" + fields[0]
			if len(fields) == 2 {
				volumesFrom[idx] = volumesFrom[idx] + ":" + fields[1]
			}
		}
	}

	return volumesFrom, nil
}

// Creates the name for a k8s entity based on the provided content of a
// K8s yaml file and a given suffix.
func k8sName(content []byte, suffix string) string {
	// The name of the service container is the first 12
	// characters of the yaml file's hash followed by the
	// '-service' suffix to guarantee a predictable and
	// discoverable name.
	hash := digest.FromBytes(content).Encoded()
	return hash[0:12] + "-" + suffix
}

func (ic *ContainerEngine) PlayKube(ctx context.Context, body io.Reader, options entities.PlayKubeOptions) (_ *entities.PlayKubeReport, finalErr error) {
	if options.ServiceContainer && options.Start == types.OptionalBoolFalse { // Sanity check to be future proof
		return nil, fmt.Errorf("running a service container requires starting the pod(s)")
	}

	report := &entities.PlayKubeReport{}
	validKinds := 0

	// when no network options are specified, create a common network for all the pods
	if len(options.Networks) == 0 {
		_, err := ic.NetworkCreate(
			ctx,
			nettypes.Network{
				Name:       kubeDefaultNetwork,
				DNSEnabled: true,
			},
			&nettypes.NetworkCreateOptions{
				IgnoreIfExists: true,
			},
		)
		if err != nil {
			return nil, err
		}
	}

	// read yaml document
	content, err := io.ReadAll(body)
	if err != nil {
		return nil, err
	}

	// split yaml document
	documentList, err := splitMultiDocYAML(content)
	if err != nil {
		return nil, err
	}

	// sort kube kinds
	documentList, err = sortKubeKinds(documentList)
	if err != nil {
		return nil, fmt.Errorf("unable to sort kube kinds: %w", err)
	}

	ipIndex := 0

	var configMaps []v1.ConfigMap
	var numReplicas int32

	ranContainers := false
	// set the ranContainers bool to true if at least one container was successfully started.
	setRanContainers := func(r *entities.PlayKubeReport) {
		if !ranContainers {
			for _, p := range r.Pods {
				numCons := len(p.Containers) + len(p.InitContainers)
				if numCons == 0 {
					// special case, the pod has no containers (besides infra)
					// That seems to be valid per https://github.com/containers/podman/issues/25786
					// and users could depend on it so mark it as running in that case.
					ranContainers = true
					break
				}
				// If the list of container errors is less then the total number of pod containers then we know it did start.
				if len(p.ContainerErrors) < numCons {
					ranContainers = true
					break
				}
			}
		}
	}

	// FIXME: both, the service container and the proxies, should ideally
	// be _state_ of an object. The Kube code below is quite Spaghetti-code
	// which we should refactor at some point to make it easier to extend
	// (via shared state instead of passing data around) and make it more
	// maintainable long term.
	var serviceContainer *libpod.Container
	var notifyProxies []*notifyproxy.NotifyProxy
	defer func() {
		// Close the notify proxy on return.  At that point we know
		// that a) all containers have send their READY message and
		// that b) the service container has exited (and hence all
		// containers).
		for _, proxy := range notifyProxies {
			if err := proxy.Close(); err != nil {
				logrus.Errorf("Closing notify proxy %q: %v", proxy.SocketPath(), err)
			}
		}
	}()

	// create pod on each document if it is a pod or deployment
	// any other kube kind will be skipped
	for _, document := range documentList {
		kind, err := getKubeKind(document)
		if err != nil {
			return nil, fmt.Errorf("unable to read kube YAML: %w", err)
		}

		// TODO: create constants for the various "kinds" of yaml files.
		if options.ServiceContainer && serviceContainer == nil && (kind == "Pod" || kind == "Deployment") {
			ctr, err := ic.createServiceContainer(ctx, k8sName(content, "service"), options)
			if err != nil {
				return nil, err
			}
			serviceContainer = ctr
			// Make sure to remove the container in case something goes wrong below.
			defer func() {
				if finalErr == nil {
					return
				}
				if err := ic.Libpod.RemoveContainer(ctx, ctr, true, true, nil); err != nil {
					// Log this in debug mode so that we don't print out an error and confuse the user
					// when the service container can't be removed because the pod still exists
					// This can happen when an error happens during kube play and we are trying to
					// clean up after the error. The service container will be removed as part of the
					// teardown function.
					logrus.Debugf("Error cleaning up service container after failure: %v", err)
				}
			}()
		}

		switch kind {
		case "Pod":
			var podYAML v1.Pod
			var podTemplateSpec v1.PodTemplateSpec

			if err := yaml.Unmarshal(document, &podYAML); err != nil {
				return nil, fmt.Errorf("unable to read YAML as Kube Pod: %w", err)
			}

			podTemplateSpec.ObjectMeta = podYAML.ObjectMeta
			podTemplateSpec.Spec = podYAML.Spec

			for name, val := range options.Annotations {
				if podYAML.Annotations == nil {
					podYAML.Annotations = make(map[string]string)
				}
				podYAML.Annotations[name] = val
			}

			if err := annotations.ValidateAnnotations(podYAML.Annotations); err != nil {
				return nil, err
			}

			r, proxies, err := ic.playKubePod(ctx, podTemplateSpec.ObjectMeta.Name, &podTemplateSpec, options, &ipIndex, podYAML.Annotations, configMaps, serviceContainer)
			if err != nil {
				return nil, err
			}
			notifyProxies = append(notifyProxies, proxies...)

			report.Pods = append(report.Pods, r.Pods...)
			validKinds++
			setRanContainers(r)
		case "DaemonSet":
			var daemonSetYAML v1apps.DaemonSet

			if err := yaml.Unmarshal(document, &daemonSetYAML); err != nil {
				return nil, fmt.Errorf("unable to read YAML as Kube DaemonSet: %w", err)
			}

			r, proxies, err := ic.playKubeDaemonSet(ctx, &daemonSetYAML, options, &ipIndex, configMaps, serviceContainer)
			if err != nil {
				return nil, err
			}
			notifyProxies = append(notifyProxies, proxies...)

			report.Pods = append(report.Pods, r.Pods...)
			validKinds++
			setRanContainers(r)
		case "Deployment":
			var deploymentYAML v1apps.Deployment

			if err := yaml.Unmarshal(document, &deploymentYAML); err != nil {
				return nil, fmt.Errorf("unable to read YAML as Kube Deployment: %w", err)
			}

			numReplicas = 1
			if deploymentYAML.Spec.Replicas != nil {
				numReplicas = *deploymentYAML.Spec.Replicas
			}

			if numReplicas > 1 && options.Replicas {
				if len(options.PublishPorts) > 0 && len(options.PublishPorts) != int(numReplicas) {
					return nil, fmt.Errorf("number of Pod replics aren't equal to the number of published ports: %d replicas, %d published ports", numReplicas, len(options.PublishPorts))
				}

				portToPublish := make([]string, numReplicas)
				copy(portToPublish, options.PublishPorts)
				options.PublishPorts = make([]string, 1)
				for i := range numReplicas {
					options.PublishPorts[0] = portToPublish[i]
					podName := fmt.Sprintf("%s-pod-%d", deploymentYAML.ObjectMeta.Name, i)
					r, proxies, err := ic.playKubeDeployment(ctx, &deploymentYAML, options, &ipIndex, configMaps, serviceContainer, podName)
					if err != nil {
						return nil, err
					}
					notifyProxies = append(notifyProxies, proxies...)

					report.Pods = append(report.Pods, r.Pods...)
					setRanContainers(r)
				}
			} else {
				if numReplicas > 1 {
					logrus.Warnf("Limiting replica count to 1, use `--replicas` to enable more than one replica")
				}
				podName := fmt.Sprintf("%s-pod", deploymentYAML.ObjectMeta.Name)
				r, proxies, err := ic.playKubeDeployment(ctx, &deploymentYAML, options, &ipIndex, configMaps, serviceContainer, podName)
				if err != nil {
					return nil, err
				}
				notifyProxies = append(notifyProxies, proxies...)

				report.Pods = append(report.Pods, r.Pods...)
				setRanContainers(r)
			}
			validKinds++
		case "Job":
			var jobYAML v1.Job

			if err := yaml.Unmarshal(document, &jobYAML); err != nil {
				return nil, fmt.Errorf("unable to read YAML as Kube Job: %w", err)
			}

			r, proxies, err := ic.playKubeJob(ctx, &jobYAML, options, &ipIndex, configMaps, serviceContainer)
			if err != nil {
				return nil, err
			}
			notifyProxies = append(notifyProxies, proxies...)

			report.Pods = append(report.Pods, r.Pods...)
			validKinds++
			setRanContainers(r)
		case "PersistentVolumeClaim":
			var pvcYAML v1.PersistentVolumeClaim

			if err := yaml.Unmarshal(document, &pvcYAML); err != nil {
				return nil, fmt.Errorf("unable to read YAML as Kube PersistentVolumeClaim: %w", err)
			}

			for name, val := range options.Annotations {
				if pvcYAML.Annotations == nil {
					pvcYAML.Annotations = make(map[string]string)
				}
				pvcYAML.Annotations[name] = val
			}

			if options.IsRemote {
				if _, ok := pvcYAML.Annotations[util.VolumeImportSourceAnnotation]; ok {
					return nil, fmt.Errorf("importing volumes is not supported for remote requests")
				}
			}

			r, err := ic.playKubePVC(ctx, "", &pvcYAML)
			if err != nil {
				return nil, err
			}

			report.Volumes = append(report.Volumes, r.Volumes...)
			validKinds++
		case "ConfigMap":
			var configMap v1.ConfigMap

			if err := yaml.Unmarshal(document, &configMap); err != nil {
				return nil, fmt.Errorf("unable to read YAML as Kube ConfigMap: %w", err)
			}
			configMaps = append(configMaps, configMap)
		case "Secret":
			var secret v1.Secret

			if err := yaml.Unmarshal(document, &secret); err != nil {
				return nil, fmt.Errorf("unable to read YAML as kube secret: %w", err)
			}

			r, err := ic.playKubeSecret(&secret)
			if err != nil {
				return nil, err
			}
			report.Secrets = append(report.Secrets, entities.PlaySecret{CreateReport: r})
			validKinds++
		default:
			logrus.Infof("Kube kind %s not supported", kind)
			continue
		}
	}

	if validKinds == 0 {
		if len(configMaps) > 0 {
			return nil, fmt.Errorf("ConfigMaps in podman are not a standalone object and must be used in a container")
		}
		return nil, fmt.Errorf("YAML document does not contain any supported kube kind")
	}

	if !options.ServiceContainer {
		return report, nil
	}

	// If we started containers along with a service container, we are
	// running inside a systemd unit and need to set the main PID.

	if ranContainers {
		switch len(notifyProxies) {
		case 0: // Optimization for containers/podman/issues/17345
			// No container needs sdnotify, so we can mark the
			// service container's conmon as the main PID and
			// return early.
			data, err := serviceContainer.Inspect(false)
			if err != nil {
				return nil, err
			}
			message := fmt.Sprintf("MAINPID=%d\n%s", data.State.ConmonPid, daemon.SdNotifyReady)
			if err := notifyproxy.SendMessage("", message); err != nil {
				return nil, err
			}
		default:
			// At least one container has a custom sdnotify policy,
			// so we need to let the sdnotify proxies run for the
			// lifetime of the service container.  That means, we
			// need to wait for the service container to stop.
			// Podman will hence be marked as the main PID.  That
			// comes at the cost of keeping Podman running.
			message := fmt.Sprintf("MAINPID=%d\n%s", os.Getpid(), daemon.SdNotifyReady)
			if err := notifyproxy.SendMessage("", message); err != nil {
				return nil, err
			}
			exitCode, err := serviceContainer.Wait(ctx)
			if err != nil {
				return nil, fmt.Errorf("waiting for service container: %w", err)
			}
			report.ExitCode = &exitCode
		}

		report.ServiceContainerID = serviceContainer.ID()
	} else if serviceContainer != nil {
		// No containers started, make sure to stop the service container.
		// Note because the pods still do exists and are not removed by default we cannot remove it.
		if err := serviceContainer.StopWithTimeout(0); err != nil {
			logrus.Errorf("Failed to stop service container: %v", err)
		}
	}

	return report, nil
}

func (ic *ContainerEngine) playKubeDaemonSet(ctx context.Context, daemonSetYAML *v1apps.DaemonSet, options entities.PlayKubeOptions, ipIndex *int, configMaps []v1.ConfigMap, serviceContainer *libpod.Container) (*entities.PlayKubeReport, []*notifyproxy.NotifyProxy, error) {
	var (
		daemonSetName string
		podSpec       v1.PodTemplateSpec
		report        entities.PlayKubeReport
	)

	daemonSetName = daemonSetYAML.ObjectMeta.Name
	if daemonSetName == "" {
		return nil, nil, errors.New("daemonSet does not have a name")
	}
	podSpec = daemonSetYAML.Spec.Template

	podName := fmt.Sprintf("%s-pod", daemonSetName)
	podReport, proxies, err := ic.playKubePod(ctx, podName, &podSpec, options, ipIndex, daemonSetYAML.Annotations, configMaps, serviceContainer)
	if err != nil {
		return nil, nil, fmt.Errorf("encountered while bringing up pod %s: %w", podName, err)
	}
	report.Pods = podReport.Pods

	return &report, proxies, nil
}

func (ic *ContainerEngine) playKubeDeployment(ctx context.Context, deploymentYAML *v1apps.Deployment, options entities.PlayKubeOptions, ipIndex *int, configMaps []v1.ConfigMap, serviceContainer *libpod.Container, podName string) (*entities.PlayKubeReport, []*notifyproxy.NotifyProxy, error) {
	var (
		deploymentName string
		podSpec        v1.PodTemplateSpec
		report         entities.PlayKubeReport
	)

	deploymentName = deploymentYAML.ObjectMeta.Name
	if deploymentName == "" {
		return nil, nil, errors.New("deployment does not have a name")
	}

	podSpec = deploymentYAML.Spec.Template

	podReport, proxies, err := ic.playKubePod(ctx, podName, &podSpec, options, ipIndex, deploymentYAML.Annotations, configMaps, serviceContainer)
	if err != nil {
		return nil, nil, fmt.Errorf("encountered while bringing up pod %s: %w", podName, err)
	}
	report.Pods = podReport.Pods

	return &report, proxies, nil
}

func (ic *ContainerEngine) playKubeJob(ctx context.Context, jobYAML *v1.Job, options entities.PlayKubeOptions, ipIndex *int, configMaps []v1.ConfigMap, serviceContainer *libpod.Container) (*entities.PlayKubeReport, []*notifyproxy.NotifyProxy, error) {
	var (
		jobName string
		podSpec v1.PodTemplateSpec
		report  entities.PlayKubeReport
	)

	jobName = jobYAML.ObjectMeta.Name
	if jobName == "" {
		return nil, nil, errors.New("job does not have a name")
	}
	podSpec = jobYAML.Spec.Template

	podName := fmt.Sprintf("%s-pod", jobName)
	podReport, proxies, err := ic.playKubePod(ctx, podName, &podSpec, options, ipIndex, jobYAML.Annotations, configMaps, serviceContainer)
	if err != nil {
		return nil, nil, fmt.Errorf("encountered while bringing up pod %s: %w", podName, err)
	}
	report.Pods = podReport.Pods

	return &report, proxies, nil
}

func (ic *ContainerEngine) playKubePod(ctx context.Context, podName string, podYAML *v1.PodTemplateSpec, options entities.PlayKubeOptions, ipIndex *int, annotations map[string]string, configMaps []v1.ConfigMap, serviceContainer *libpod.Container) (*entities.PlayKubeReport, []*notifyproxy.NotifyProxy, error) {
	cfg, err := ic.Libpod.GetConfigNoCopy()
	if err != nil {
		return nil, nil, err
	}

	var (
		writer      io.Writer
		playKubePod entities.PlayKubePod
		report      entities.PlayKubeReport
	)

	mainSdNotifyMode, err := getSdNotifyMode(annotations, "")
	if err != nil {
		return nil, nil, err
	}

	// Create the secret manager before hand
	secretsManager, err := ic.Libpod.SecretsManager()
	if err != nil {
		return nil, nil, err
	}

	// Assert the pod has a name
	if podName == "" {
		return nil, nil, fmt.Errorf("pod does not have a name")
	}

	if _, ok := annotations[define.VolumesFromAnnotation]; ok {
		return nil, nil, fmt.Errorf("annotation %s without target volume is reserved for internal use", define.VolumesFromAnnotation)
	}

	podOpt := entities.PodCreateOptions{
		Infra:      true,
		Net:        &entities.NetOptions{NoHosts: options.NoHosts, NoHostname: options.NoHostname},
		ExitPolicy: string(config.PodExitPolicyStop),
	}
	podOpt, err = kube.ToPodOpt(ctx, podName, podOpt, options.PublishAllPorts, podYAML)
	if err != nil {
		return nil, nil, err
	}

	// add kube default network if no network is explicitly added
	if podOpt.Net.Network.NSMode != "host" && len(options.Networks) == 0 {
		options.Networks = []string{kubeDefaultNetwork}
	}

	if len(options.Networks) > 0 {
		ns, networks, netOrder, netOpts, err := specgen.ParseNetworkFlag(options.Networks)
		if err != nil {
			return nil, nil, err
		}

		podOpt.Net.Network = ns
		podOpt.Net.Networks = networks
		podOpt.Net.NetworkOrder = netOrder
		podOpt.Net.NetworkOptions = netOpts
	}

	if options.Userns == "" {
		if v, ok := annotations[define.UserNsAnnotation]; ok {
			options.Userns = v
		} else if v, ok := annotations[define.UserNsAnnotation+"/"+podName]; ok {
			options.Userns = v
		} else if podYAML.Spec.HostUsers != nil && !*podYAML.Spec.HostUsers {
			options.Userns = "auto"
		} else {
			options.Userns = "host"
		}
		// FIXME: how to deal with explicit mappings?
		if options.Userns == "private" {
			options.Userns = "auto"
		}
	} else if podYAML.Spec.HostUsers != nil {
		logrus.Info("overriding the user namespace mode in the pod spec")
	}

	// Validate the userns modes supported.
	podOpt.Userns, err = specgen.ParseUserNamespace(options.Userns)
	if err != nil {
		return nil, nil, err
	}

	// FIXME This is very hard to support properly with a good ux
	if len(options.StaticIPs) > *ipIndex {
		if !podOpt.Net.Network.IsBridge() {
			return nil, nil, fmt.Errorf("static ip addresses can only be set when the network mode is bridge: %w", define.ErrInvalidArg)
		}
		if len(podOpt.Net.Networks) != 1 {
			return nil, nil, fmt.Errorf("cannot set static ip addresses for more than network, use netname:ip=<ip> syntax to specify ips for more than network: %w", define.ErrInvalidArg)
		}
		for name, netOpts := range podOpt.Net.Networks {
			netOpts.StaticIPs = append(netOpts.StaticIPs, options.StaticIPs[*ipIndex])
			podOpt.Net.Networks[name] = netOpts
		}
	} else if len(options.StaticIPs) > 0 {
		// only warn if the user has set at least one ip
		logrus.Warn("No more static ips left using a random one")
	}
	if len(options.StaticMACs) > *ipIndex {
		if !podOpt.Net.Network.IsBridge() {
			return nil, nil, fmt.Errorf("static mac address can only be set when the network mode is bridge: %w", define.ErrInvalidArg)
		}
		if len(podOpt.Net.Networks) != 1 {
			return nil, nil, fmt.Errorf("cannot set static mac address for more than network, use netname:mac=<mac> syntax to specify mac for more than network: %w", define.ErrInvalidArg)
		}
		for name, netOpts := range podOpt.Net.Networks {
			netOpts.StaticMAC = nettypes.HardwareAddr(options.StaticMACs[*ipIndex])
			podOpt.Net.Networks[name] = netOpts
		}
	} else if len(options.StaticIPs) > 0 {
		// only warn if the user has set at least one mac
		logrus.Warn("No more static macs left using a random one")
	}
	*ipIndex++

	if len(options.PublishPorts) > 0 {
		publishPorts, err := specgenutil.CreatePortBindings(options.PublishPorts)
		if err != nil {
			return nil, nil, err
		}
		mergePublishPorts(&podOpt, publishPorts)
	}

	p := specgen.NewPodSpecGenerator()

	p, err = entities.ToPodSpecGen(*p, &podOpt)
	if err != nil {
		return nil, nil, err
	}
	podSpec := entities.PodSpec{PodSpecGen: *p}

	configMapIndex := make(map[string]struct{})
	for _, configMap := range configMaps {
		configMapIndex[configMap.Name] = struct{}{}
	}
	for _, p := range options.ConfigMaps {
		f, err := os.Open(p)
		if err != nil {
			return nil, nil, err
		}
		defer f.Close()

		cms, err := readConfigMapFromFile(f)
		if err != nil {
			return nil, nil, fmt.Errorf("%q: %w", p, err)
		}

		for _, cm := range cms {
			if _, present := configMapIndex[cm.Name]; present {
				return nil, nil, fmt.Errorf("ambiguous configuration: the same config map %s is present in YAML and in --configmaps %s file", cm.Name, p)
			}

			configMaps = append(configMaps, cm)
		}
	}

	mountLabel, err := getMountLabel(podYAML.Spec.SecurityContext)
	if err != nil {
		return nil, nil, err
	}

	volumes, err := kube.InitializeVolumes(podYAML.Spec.Volumes, configMaps, secretsManager, mountLabel)
	if err != nil {
		return nil, nil, err
	}

	// Go through the volumes and create a podman volume for all volumes that have been
	// defined by a configmap or secret
	for _, v := range volumes {
		if (v.Type == kube.KubeVolumeTypeConfigMap || v.Type == kube.KubeVolumeTypeSecret) && !v.Optional {
			volumeOptions := []libpod.VolumeCreateOption{
				libpod.WithVolumeName(v.Source),
				libpod.WithVolumeMountLabel(mountLabel),
			}
			vol, err := ic.Libpod.NewVolume(ctx, volumeOptions...)
			if err != nil {
				if errors.Is(err, define.ErrVolumeExists) {
					// Volume for this configmap already exists do not
					// error out instead reuse the current volume.
					vol, err = ic.Libpod.GetVolume(v.Source)
					if err != nil {
						return nil, nil, fmt.Errorf("cannot reuse local volume for volume from configmap %q: %w", v.Source, err)
					}
				} else {
					return nil, nil, fmt.Errorf("cannot create a local volume for volume from configmap %q: %w", v.Source, err)
				}
			}
			mountPoint, err := vol.MountPoint()
			if err != nil || mountPoint == "" {
				return nil, nil, fmt.Errorf("unable to get mountpoint of volume %q: %w", vol.Name(), err)
			}
			defaultMode := v.DefaultMode
			// Create files and add data to the volume mountpoint based on the Items in the volume
			for k, v := range v.Items {
				f, err := openPathSafely(mountPoint, k)
				if err != nil {
					return nil, nil, fmt.Errorf("cannot create file %q at volume mountpoint %q: %w", k, mountPoint, err)
				}
				defer f.Close()
				_, err = f.Write(v)
				if err != nil {
					return nil, nil, err
				}
				// Set file permissions
				if err := f.Chmod(os.FileMode(defaultMode)); err != nil {
					return nil, nil, err
				}
			}
		} else if v.Type == kube.KubeVolumeTypeImage {
			var cwd string
			if options.ContextDir != "" {
				cwd = options.ContextDir
			} else {
				cwd, err = os.Getwd()
				if err != nil {
					return nil, nil, err
				}
			}

			_, err := ic.buildOrPullImage(ctx, cwd, writer, v.Source, v.ImagePullPolicy, options)
			if err != nil {
				return nil, nil, err
			}
		}
	}

	seccompPaths, err := kube.InitializeSeccompPaths(podYAML.ObjectMeta.Annotations, options.SeccompProfileRoot)
	if err != nil {
		return nil, nil, err
	}

	// Set the restart policy from the kube yaml at the pod level in podman
	switch podYAML.Spec.RestartPolicy {
	case v1.RestartPolicyAlways:
		podSpec.PodSpecGen.RestartPolicy = define.RestartPolicyAlways
	case v1.RestartPolicyOnFailure:
		podSpec.PodSpecGen.RestartPolicy = define.RestartPolicyOnFailure
	case v1.RestartPolicyNever:
		podSpec.PodSpecGen.RestartPolicy = define.RestartPolicyNo
	default: // Default to Always
		podSpec.PodSpecGen.RestartPolicy = define.RestartPolicyAlways
	}

	if podOpt.Infra {
		infraImage := cfg.Engine.InfraImage
		infraOptions := entities.NewInfraContainerCreateOptions()
		infraOptions.Hostname = podSpec.PodSpecGen.PodBasicConfig.Hostname
		infraOptions.ReadOnly = true
		infraOptions.ReadWriteTmpFS = false
		infraOptions.UserNS = options.Userns
		podSpec.PodSpecGen.InfraImage = infraImage
		podSpec.PodSpecGen.NoInfra = false
		podSpec.PodSpecGen.InfraContainerSpec = specgen.NewSpecGenerator(infraImage, false)
		podSpec.PodSpecGen.InfraContainerSpec.NetworkOptions = p.NetworkOptions
		podSpec.PodSpecGen.InfraContainerSpec.SdNotifyMode = define.SdNotifyModeIgnore
		// If the infraNameAnnotation is set in the yaml, use that as the infra container name
		// If not, fall back to the default infra container name
		if v, ok := podYAML.Annotations[define.InfraNameAnnotation]; ok {
			podSpec.PodSpecGen.InfraContainerSpec.Name = v
		}

		err = specgenutil.FillOutSpecGen(podSpec.PodSpecGen.InfraContainerSpec, &infraOptions, []string{})
		if err != nil {
			return nil, nil, err
		}
	}

	// Add the original container names from the kube yaml as aliases for it. This will allow network to work with
	// both just containerName as well as containerName-podName.
	// In the future, we want to extend this to the CLI as well, where the name of the container created will not have
	// the podName appended to it, but this is a breaking change and will be done in podman 5.0
	ctrNameAliases := make([]string, 0, len(podYAML.Spec.Containers))
	for _, container := range podYAML.Spec.Containers {
		if container.Name != "" {
			ctrNameAliases = append(ctrNameAliases, container.Name)
		}
	}
	for k, v := range podSpec.PodSpecGen.Networks {
		v.Aliases = append(v.Aliases, ctrNameAliases...)
		podSpec.PodSpecGen.Networks[k] = v
	}

	if serviceContainer != nil {
		podSpec.PodSpecGen.ServiceContainerID = serviceContainer.ID()
	}

	if options.Replace {
		if _, err := ic.PodRm(ctx, []string{podName}, entities.PodRmOptions{Force: true, Ignore: true}); err != nil {
			return nil, nil, fmt.Errorf("replacing pod %v: %w", podName, err)
		}
	}
	// Create the Pod
	pod, err := generate.MakePod(&podSpec, ic.Libpod)
	if err != nil {
		return nil, nil, err
	}

	podInfraID, err := pod.InfraContainerID()
	if err != nil {
		return nil, nil, err
	}

	if !options.Quiet {
		writer = os.Stderr
	}

	containers := make([]*libpod.Container, 0, len(podYAML.Spec.Containers))
	initContainers := make([]*libpod.Container, 0, len(podYAML.Spec.InitContainers))

	var cwd string
	if options.ContextDir != "" {
		cwd = options.ContextDir
	} else {
		cwd, err = os.Getwd()
		if err != nil {
			return nil, nil, err
		}
	}

	var readOnly types.OptionalBool
	if cfg.Containers.ReadOnly {
		readOnly = types.NewOptionalBool(cfg.Containers.ReadOnly)
	}

	ctrNames := make(map[string]string)
	for _, initCtr := range podYAML.Spec.InitContainers {
		// Error out if same name is used for more than one container
		if _, ok := ctrNames[initCtr.Name]; ok {
			return nil, nil, fmt.Errorf("the pod %q is invalid; duplicate container name %q detected", podName, initCtr.Name)
		}
		ctrNames[initCtr.Name] = ""
		// Init containers cannot have either of lifecycle, livenessProbe, readinessProbe, or startupProbe set
		if initCtr.Lifecycle != nil || initCtr.LivenessProbe != nil || initCtr.ReadinessProbe != nil || initCtr.StartupProbe != nil {
			return nil, nil, fmt.Errorf("cannot create an init container that has either of lifecycle, livenessProbe, readinessProbe, or startupProbe set")
		}
		pulledImage, labels, err := ic.getImageAndLabelInfo(ctx, cwd, annotations, writer, initCtr, options)
		if err != nil {
			return nil, nil, err
		}

		// add podYAML labels
		maps.Copy(labels, podSpec.PodSpecGen.Labels)
		initCtrType := annotations[define.InitContainerType]
		if initCtrType == "" {
			initCtrType = define.OneShotInitContainer
		}

		automountImages, err := ic.prepareAutomountImages(ctx, initCtr.Name, annotations)
		if err != nil {
			return nil, nil, err
		}

		var volumesFrom []string
		if list, err := prepareVolumesFrom(initCtr.Name, podName, ctrNames, annotations); err != nil {
			return nil, nil, err
		} else if list != nil {
			volumesFrom = list
		}

		specgenOpts := kube.CtrSpecGenOptions{
			Annotations:        annotations,
			ConfigMaps:         configMaps,
			Container:          initCtr,
			Image:              pulledImage,
			InitContainerType:  initCtrType,
			Labels:             labels,
			LogDriver:          options.LogDriver,
			LogOptions:         options.LogOptions,
			NetNSIsHost:        p.NetNS.IsHost(),
			PodID:              pod.ID(),
			PodInfraID:         podInfraID,
			PodName:            podName,
			PodSecurityContext: podYAML.Spec.SecurityContext,
			ReadOnly:           readOnly,
			RestartPolicy:      define.RestartPolicyNo,
			SeccompPaths:       seccompPaths,
			SecretsManager:     secretsManager,
			UserNSIsHost:       p.Userns.IsHost(),
			Volumes:            volumes,
			VolumesFrom:        volumesFrom,
			ImageVolumes:       automountImages,
			UtsNSIsHost:        p.UtsNs.IsHost(),
		}
		specGen, err := kube.ToSpecGen(ctx, &specgenOpts)
		if err != nil {
			return nil, nil, err
		}

		// ensure the environment is setup for initContainers as well: https://github.com/containers/podman/issues/18384
		warn, err := generate.CompleteSpec(ctx, ic.Libpod, specGen)
		if err != nil {
			return nil, nil, err
		}
		for _, w := range warn {
			logrus.Warn(w)
		}

		specGen.SdNotifyMode = define.SdNotifyModeIgnore
		expandForKube(specGen)
		rtSpec, spec, opts, err := generate.MakeContainer(ctx, ic.Libpod, specGen, false, nil)
		if err != nil {
			return nil, nil, err
		}
		opts = append(opts, libpod.WithSdNotifyMode(define.SdNotifyModeIgnore))
		if options.Replace {
			if _, err := ic.ContainerRm(ctx, []string{spec.Name}, entities.RmOptions{Force: true, Ignore: true}); err != nil {
				return nil, nil, err
			}
		}

		ctr, err := generate.ExecuteCreate(ctx, ic.Libpod, rtSpec, spec, false, opts...)
		if err != nil {
			return nil, nil, err
		}

		initContainers = append(initContainers, ctr)
	}

	// Callers are expected to close the proxies
	var sdNotifyProxies []*notifyproxy.NotifyProxy

	for _, container := range podYAML.Spec.Containers {
		// Error out if the same name is used for more than one container
		if _, ok := ctrNames[container.Name]; ok {
			return nil, nil, fmt.Errorf("the pod %q is invalid; duplicate container name %q detected", podName, container.Name)
		}

		ctrNames[container.Name] = ""
		pulledImage, labels, err := ic.getImageAndLabelInfo(ctx, cwd, annotations, writer, container, options)
		if err != nil {
			return nil, nil, err
		}

		// add podYAML labels
		maps.Copy(labels, podSpec.PodSpecGen.Labels)

		automountImages, err := ic.prepareAutomountImages(ctx, container.Name, annotations)
		if err != nil {
			return nil, nil, err
		}

		var volumesFrom []string
		if list, err := prepareVolumesFrom(container.Name, podName, ctrNames, annotations); err != nil {
			return nil, nil, err
		} else if list != nil {
			volumesFrom = list
		}

		specgenOpts := kube.CtrSpecGenOptions{
			Annotations:        annotations,
			ConfigMaps:         configMaps,
			Container:          container,
			Image:              pulledImage,
			IpcNSIsHost:        p.Ipc.IsHost(),
			Labels:             labels,
			LogDriver:          options.LogDriver,
			LogOptions:         options.LogOptions,
			NetNSIsHost:        p.NetNS.IsHost(),
			PidNSIsHost:        p.Pid.IsHost(),
			PodID:              pod.ID(),
			PodInfraID:         podInfraID,
			PodName:            podName,
			PodSecurityContext: podYAML.Spec.SecurityContext,
			RestartPolicy:      podSpec.PodSpecGen.RestartPolicy, // pass the restart policy to the container (https://github.com/containers/podman/issues/20903)
			ReadOnly:           readOnly,
			SeccompPaths:       seccompPaths,
			SecretsManager:     secretsManager,
			UserNSIsHost:       p.Userns.IsHost(),
			Volumes:            volumes,
			VolumesFrom:        volumesFrom,
			ImageVolumes:       automountImages,
			UtsNSIsHost:        p.UtsNs.IsHost(),
			NoPodPrefix:        options.NoPodPrefix,
		}

		if podYAML.Spec.TerminationGracePeriodSeconds != nil {
			specgenOpts.TerminationGracePeriodSeconds = podYAML.Spec.TerminationGracePeriodSeconds
		}

		specGen, err := kube.ToSpecGen(ctx, &specgenOpts)
		if err != nil {
			return nil, nil, err
		}

		// Make sure to complete the spec (#17016)
		warn, err := generate.CompleteSpec(ctx, ic.Libpod, specGen)
		if err != nil {
			return nil, nil, err
		}
		for _, w := range warn {
			logrus.Warn(w)
		}

		specGen.RawImageName = container.Image
		expandForKube(specGen)
		rtSpec, spec, opts, err := generate.MakeContainer(ctx, ic.Libpod, specGen, false, nil)
		if err != nil {
			return nil, nil, err
		}

		sdNotifyMode := mainSdNotifyMode
		ctrNotifyMode, err := getSdNotifyMode(annotations, container.Name)
		if err != nil {
			return nil, nil, err
		}
		if ctrNotifyMode != "" {
			sdNotifyMode = ctrNotifyMode
		}
		if sdNotifyMode == "" { // Default to "ignore"
			sdNotifyMode = define.SdNotifyModeIgnore
		}

		opts = append(opts, libpod.WithSdNotifyMode(sdNotifyMode))

		var proxy *notifyproxy.NotifyProxy
		// Create a notify proxy for the container.
		if sdNotifyMode != "" && sdNotifyMode != define.SdNotifyModeIgnore {
			proxy, err = notifyproxy.New("")
			if err != nil {
				return nil, nil, err
			}
			sdNotifyProxies = append(sdNotifyProxies, proxy)
			opts = append(opts, libpod.WithSdNotifySocket(proxy.SocketPath()))
		}

		if options.Replace {
			if _, err := ic.ContainerRm(ctx, []string{spec.Name}, entities.RmOptions{Force: true, Ignore: true}); err != nil {
				return nil, nil, err
			}
		}

		ctr, err := generate.ExecuteCreate(ctx, ic.Libpod, rtSpec, spec, false, opts...)
		if err != nil {
			return nil, nil, err
		}
		if proxy != nil {
			proxy.AddContainer(ctr)
		}
		containers = append(containers, ctr)
	}

	if options.Start != types.OptionalBoolFalse {
		// Start the containers
		podStartErrors, err := pod.Start(ctx)
		if err != nil && !errors.Is(err, define.ErrPodPartialFail) {
			return nil, nil, err
		}
		for id, err := range podStartErrors {
			playKubePod.ContainerErrors = append(playKubePod.ContainerErrors, fmt.Errorf("starting container %s: %w", id, err).Error())
		}

		// Wait for each proxy to receive a READY message. Use a wait
		// group to prevent the potential for ABBA kinds of deadlocks.
		var wg sync.WaitGroup
		errors := make([]error, len(sdNotifyProxies))
		for i := range sdNotifyProxies {
			wg.Add(1)
			go func(i int) {
				err := sdNotifyProxies[i].Wait()
				if err != nil {
					err = fmt.Errorf("waiting for sd-notify proxy: %w", err)
				}
				errors[i] = err
				wg.Done()
			}(i)
		}
		wg.Wait()
		for _, err := range errors {
			if err != nil {
				// Close all proxies on error.
				for _, proxy := range sdNotifyProxies {
					_ = proxy.Close()
				}
				return nil, nil, err
			}
		}
	}

	playKubePod.ID = pod.ID()
	for _, ctr := range containers {
		playKubePod.Containers = append(playKubePod.Containers, ctr.ID())
	}
	for _, initCtr := range initContainers {
		playKubePod.InitContainers = append(playKubePod.InitContainers, initCtr.ID())
	}

	report.Pods = append(report.Pods, playKubePod)

	return &report, sdNotifyProxies, nil
}

// buildImageFromContainerfile builds the container image and returns its details if these conditions are met:
//   - A folder with the name of the image exists in current directory
//   - A Dockerfile or Containerfile exists in that folder
//   - The image doesn't exist locally OR the user explicitly provided the option `--build`
func (ic *ContainerEngine) buildImageFromContainerfile(ctx context.Context, cwd string, writer io.Writer, image string, options entities.PlayKubeOptions) (*libimage.Image, error) {
	buildFile, err := getBuildFile(image, cwd)
	if err != nil {
		return nil, err
	}
	existsLocally, err := ic.Libpod.LibimageRuntime().Exists(image)
	if err != nil {
		return nil, err
	}
	if (len(buildFile) > 0) && ((!existsLocally && options.Build != types.OptionalBoolFalse) || (options.Build == types.OptionalBoolTrue)) {
		buildOpts := new(buildahDefine.BuildOptions)
		commonOpts := new(buildahDefine.CommonBuildOptions)
		buildOpts.ConfigureNetwork = buildahDefine.NetworkDefault
		isolation, err := bparse.IsolationOption("")
		if err != nil {
			return nil, err
		}
		buildOpts.Isolation = isolation
		buildOpts.CommonBuildOpts = commonOpts
		buildOpts.SystemContext = options.SystemContext
		buildOpts.Output = image
		buildOpts.ContextDirectory = filepath.Dir(buildFile)
		buildOpts.ReportWriter = writer
		if _, _, err := ic.Libpod.Build(ctx, *buildOpts, []string{buildFile}...); err != nil {
			return nil, err
		}
		builtImage, _, err := ic.Libpod.LibimageRuntime().LookupImage(image, new(libimage.LookupImageOptions))
		if err != nil {
			return nil, err
		}
		return builtImage, nil
	}
	return nil, nil
}

// pullImageWithPolicy invokes libimage.Pull() to pull an image with the given PullPolicy.
// If the PullPolicy is not set:
// - use PullPolicyNewer if the image tag is set to "latest" or is not set
// - use PullPolicyMissing the policy is set to PullPolicyNewer.
func (ic *ContainerEngine) pullImageWithPolicy(ctx context.Context, writer io.Writer, image string, policy v1.PullPolicy, options entities.PlayKubeOptions) (*libimage.Image, error) {
	pullPolicy := config.PullPolicyMissing
	if len(policy) > 0 {
		// Make sure to lower the strings since K8s pull policy
		// may be capitalized (see bugzilla.redhat.com/show_bug.cgi?id=1985905).
		rawPolicy := string(policy)
		parsedPolicy, err := config.ParsePullPolicy(strings.ToLower(rawPolicy))
		if err != nil {
			return nil, err
		}
		pullPolicy = parsedPolicy
	} else {
		if named, err := reference.ParseNamed(image); err == nil {
			tagged, isTagged := named.(reference.NamedTagged)
			if !isTagged || tagged.Tag() == "latest" {
				// Make sure to always pull the latest image in case it got updated.
				pullPolicy = config.PullPolicyNewer
			}
		}
	}
	// This ensures the image is the image store
	pullOptions := &libimage.PullOptions{}
	pullOptions.AuthFilePath = options.Authfile
	pullOptions.CertDirPath = options.CertDir
	pullOptions.SignaturePolicyPath = options.SignaturePolicy
	pullOptions.Writer = writer
	pullOptions.Username = options.Username
	pullOptions.Password = options.Password
	pullOptions.InsecureSkipTLSVerify = options.SkipTLSVerify

	pulledImages, err := ic.Libpod.LibimageRuntime().Pull(ctx, image, pullPolicy, pullOptions)
	if err != nil {
		return nil, err
	}
	return pulledImages[0], err
}

// buildOrPullImage builds the image if a Containerfile is present in a directory
// with the name of the image. It pulls the image otherwise. It returns the image
// details.
func (ic *ContainerEngine) buildOrPullImage(ctx context.Context, cwd string, writer io.Writer, image string, policy v1.PullPolicy, options entities.PlayKubeOptions) (*libimage.Image, error) {
	buildImage, err := ic.buildImageFromContainerfile(ctx, cwd, writer, image, options)
	if err != nil {
		return nil, err
	}
	if buildImage != nil {
		return buildImage, nil
	} else {
		return ic.pullImageWithPolicy(ctx, writer, image, policy, options)
	}
}

// getImageAndLabelInfo returns the image information and how the image should be pulled plus as well as labels to be used for the container in the pod.
// Moved this to a separate function so that it can be used for both init and regular containers when playing a kube yaml.
func (ic *ContainerEngine) getImageAndLabelInfo(ctx context.Context, cwd string, annotations map[string]string, writer io.Writer, container v1.Container, options entities.PlayKubeOptions) (*libimage.Image, map[string]string, error) {
	// Contains all labels obtained from kube
	labels := make(map[string]string)

	if len(container.Image) == 0 {
		return nil, labels, nil
	}

	pulledImage, err := ic.buildOrPullImage(ctx, cwd, writer, container.Image, container.ImagePullPolicy, options)
	if err != nil {
		return nil, labels, err
	}

	// Handle kube annotations
	setLabel := func(label string) {
		var result string
		ctrSpecific := fmt.Sprintf("%s/%s", label, container.Name)
		for k, v := range annotations {
			switch k {
			case label:
				result = v
			case ctrSpecific:
				labels[label] = v
				return
			}
		}
		if result != "" {
			labels[label] = result
		}
	}

	setLabel(define.AutoUpdateLabel)
	setLabel(define.AutoUpdateAuthfileLabel)

	return pulledImage, labels, nil
}

// playKubePVC creates a podman volume from a kube persistent volume claim.
func (ic *ContainerEngine) playKubePVC(ctx context.Context, mountLabel string, pvcYAML *v1.PersistentVolumeClaim) (*entities.PlayKubeReport, error) {
	var report entities.PlayKubeReport
	opts := make(map[string]string)

	// Get pvc name.
	// This is the only required pvc attribute to create a podman volume.
	name := pvcYAML.Name
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("persistent volume claim name can not be empty")
	}

	// Create podman volume options.
	volOptions := []libpod.VolumeCreateOption{
		libpod.WithVolumeName(name),
		libpod.WithVolumeLabels(pvcYAML.Labels),
		libpod.WithVolumeIgnoreIfExist(),
		libpod.WithVolumeMountLabel(mountLabel),
	}

	// Get pvc annotations and create remaining podman volume options if available.
	// These are podman volume options that do not match any of the persistent volume claim
	// attributes, so they can be configured using annotations since they will not affect k8s.
	var importFrom string
	for k, v := range pvcYAML.Annotations {
		switch k {
		case util.VolumeDriverAnnotation:
			volOptions = append(volOptions, libpod.WithVolumeDriver(v))
		case util.VolumeDeviceAnnotation:
			opts["device"] = v
		case util.VolumeTypeAnnotation:
			opts["type"] = v
		case util.VolumeUIDAnnotation:
			uid, err := strconv.Atoi(v)
			if err != nil {
				return nil, fmt.Errorf("cannot convert uid %s to integer: %w", v, err)
			}
			volOptions = append(volOptions, libpod.WithVolumeUID(uid))
			opts["UID"] = v
		case util.VolumeGIDAnnotation:
			gid, err := strconv.Atoi(v)
			if err != nil {
				return nil, fmt.Errorf("cannot convert gid %s to integer: %w", v, err)
			}
			volOptions = append(volOptions, libpod.WithVolumeGID(gid))
			opts["GID"] = v
		case util.VolumeMountOptsAnnotation:
			opts["o"] = v
		case util.VolumeImportSourceAnnotation:
			importFrom = v
		case util.VolumeImageAnnotation:
			opts["image"] = v
		}
	}
	volOptions = append(volOptions, libpod.WithVolumeOptions(opts))

	// Validate the file and open it before creating the volume for fast-fail
	var tarFile *os.File
	if len(importFrom) > 0 {
		err := parse.ValidateFileName(importFrom)
		if err != nil {
			return nil, err
		}

		// open tar file
		tarFile, err = os.Open(importFrom)
		if err != nil {
			return nil, err
		}
		defer tarFile.Close()
	}

	// Create volume.
	vol, err := ic.Libpod.NewVolume(ctx, volOptions...)
	if err != nil {
		return nil, err
	}

	if tarFile != nil {
		err = ic.importVolume(ctx, vol, tarFile)
		if err != nil {
			// Remove the volume to avoid partial success
			if rmErr := ic.Libpod.RemoveVolume(ctx, vol, true, nil); rmErr != nil {
				logrus.Debug(rmErr)
			}
			return nil, err
		}
	}

	report.Volumes = append(report.Volumes, entitiesTypes.PlayKubeVolume{
		Name: vol.Name(),
	})

	return &report, nil
}

func mergePublishPorts(p *entities.PodCreateOptions, publishPortsOption []nettypes.PortMapping) {
	for _, publishPortSpec := range p.Net.PublishPorts {
		if !portAlreadyPublished(publishPortSpec, publishPortsOption) {
			publishPortsOption = append(publishPortsOption, publishPortSpec)
		}
	}
	p.Net.PublishPorts = publishPortsOption
}

func portAlreadyPublished(port nettypes.PortMapping, publishedPorts []nettypes.PortMapping) bool {
	for _, publishedPort := range publishedPorts {
		if port.ContainerPort >= publishedPort.ContainerPort &&
			port.ContainerPort < publishedPort.ContainerPort+publishedPort.Range &&
			isSamePortProtocol(port.Protocol, publishedPort.Protocol) {
			return true
		}
	}
	return false
}

func isSamePortProtocol(a, b string) bool {
	if len(a) == 0 {
		a = string(v1.ProtocolTCP)
	}
	if len(b) == 0 {
		b = string(v1.ProtocolTCP)
	}

	ret := strings.EqualFold(a, b)
	return ret
}

func (ic *ContainerEngine) importVolume(ctx context.Context, vol *libpod.Volume, tarFile *os.File) error {
	volumeConfig, err := vol.Config()
	if err != nil {
		return err
	}

	mountPoint := volumeConfig.MountPoint
	if len(mountPoint) == 0 {
		return errors.New("volume is not mounted anywhere on host")
	}

	driver := volumeConfig.Driver
	volumeOptions := volumeConfig.Options
	volumeMountStatus, err := ic.VolumeMounted(ctx, vol.Name())
	if err != nil {
		return err
	}

	// Check if volume needs a mount and export only if volume is mounted
	if vol.NeedsMount() && !volumeMountStatus.Value {
		return fmt.Errorf("volume needs to be mounted but is not mounted on %s", mountPoint)
	}

	// Check if volume is using `local` driver and has mount options type other than tmpfs
	if len(driver) == 0 || driver == define.VolumeDriverLocal {
		if mountOptionType, ok := volumeOptions["type"]; ok {
			if mountOptionType != define.TypeTmpfs && !volumeMountStatus.Value {
				return fmt.Errorf("volume is using a driver %s and volume is not mounted on %s", driver, mountPoint)
			}
		}
	}

	// dont care if volume is mounted or not we are gonna import everything to mountPoint
	return chrootarchive.Untar(tarFile, mountPoint, nil)
}

// readConfigMapFromFile returns a kubernetes configMap obtained from --configmap flag
func readConfigMapFromFile(r io.Reader) ([]v1.ConfigMap, error) {
	configMaps := make([]v1.ConfigMap, 0)

	content, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("unable to read ConfigMap YAML content: %w", err)
	}

	// split yaml document
	documentList, err := splitMultiDocYAML(content)
	if err != nil {
		return nil, fmt.Errorf("unable to read as kube YAML: %w", err)
	}

	for _, document := range documentList {
		kind, err := getKubeKind(document)
		if err != nil {
			return nil, fmt.Errorf("unable to read as kube YAML: %w", err)
		}

		if kind != "ConfigMap" {
			return nil, fmt.Errorf("invalid YAML kind: %q. [ConfigMap] is the only supported by --configmap", kind)
		}

		var configMap v1.ConfigMap
		if err := yaml.Unmarshal(document, &configMap); err != nil {
			return nil, fmt.Errorf("unable to read YAML as Kube ConfigMap: %w", err)
		}
		configMaps = append(configMaps, configMap)
	}

	return configMaps, nil
}

// splitMultiDocYAML reads multiple documents in a YAML file and
// returns them as a list.
func splitMultiDocYAML(yamlContent []byte) ([][]byte, error) {
	var documentList [][]byte

	d := yamlv3.NewDecoder(bytes.NewReader(yamlContent))
	for {
		var o any
		// read individual document
		err := d.Decode(&o)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("multi doc yaml could not be split: %w", err)
		}

		if o == nil {
			continue
		}

		// back to bytes
		document, err := yamlv3.Marshal(o)
		if err != nil {
			return nil, fmt.Errorf("individual doc yaml could not be marshalled: %w", err)
		}

		kind, err := getKubeKind(document)
		if err != nil {
			return nil, fmt.Errorf("couldn't get object kind: %w", err)
		}

		// The items in a document of kind "List" are fully qualified resources
		// So, they can be treated as separate documents
		if kind == "List" {
			var kubeList metav1.List
			if err := yaml.Unmarshal(document, &kubeList); err != nil {
				return nil, err
			}
			for _, item := range kubeList.Items {
				itemDocument, err := yamlv3.Marshal(item)
				if err != nil {
					return nil, fmt.Errorf("individual doc yaml could not be marshalled: %w", err)
				}

				documentList = append(documentList, itemDocument)
			}
		} else {
			documentList = append(documentList, document)
		}
	}

	return documentList, nil
}

// getKubeKind unmarshals a kube YAML document and returns its kind.
func getKubeKind(obj []byte) (string, error) {
	var kubeObject v1.ObjectReference

	if err := yaml.Unmarshal(obj, &kubeObject); err != nil {
		return "", err
	}

	return kubeObject.Kind, nil
}

// sortKubeKinds adds the correct creation order for the kube kinds.
// Any pod dependency will be created first like volumes, secrets, etc.
func sortKubeKinds(documentList [][]byte) ([][]byte, error) {
	var sortedDocumentList [][]byte

	for _, document := range documentList {
		kind, err := getKubeKind(document)
		if err != nil {
			return nil, err
		}

		switch kind {
		case "Pod", "Deployment", "DaemonSet", "Job":
			sortedDocumentList = append(sortedDocumentList, document)
		default:
			sortedDocumentList = append([][]byte{document}, sortedDocumentList...)
		}
	}

	return sortedDocumentList, nil
}

func imageNamePrefix(imageName string) string {
	prefix := imageName
	s := strings.Split(prefix, ":")
	if len(s) > 0 {
		prefix = s[0]
	}
	s = strings.Split(prefix, "/")
	if len(s) > 0 {
		prefix = s[len(s)-1]
	}
	s = strings.Split(prefix, "@")
	if len(s) > 0 {
		prefix = s[0]
	}
	return prefix
}

func getBuildFile(imageName string, cwd string) (string, error) {
	buildDirName := imageNamePrefix(imageName)
	containerfilePath := filepath.Join(cwd, buildDirName, "Containerfile")
	dockerfilePath := filepath.Join(cwd, buildDirName, "Dockerfile")

	err := fileutils.Exists(containerfilePath)
	if err == nil {
		logrus.Debugf("Building %s with %s", imageName, containerfilePath)
		return containerfilePath, nil
	}
	// If the error is not because the file does not exist, take
	// a mulligan and try Dockerfile.  If that also fails, return that
	// error
	if !errors.Is(err, os.ErrNotExist) {
		logrus.Error(err.Error())
	}

	err = fileutils.Exists(dockerfilePath)
	if err == nil {
		logrus.Debugf("Building %s with %s", imageName, dockerfilePath)
		return dockerfilePath, nil
	}
	// Strike two
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	return "", err
}

func (ic *ContainerEngine) PlayKubeDown(ctx context.Context, body io.Reader, options entities.PlayKubeDownOptions) (*entities.PlayKubeReport, error) {
	var (
		podNames    []string
		volumeNames []string
		secretNames []string
	)
	reports := new(entities.PlayKubeReport)

	// read yaml document
	content, err := io.ReadAll(body)
	if err != nil {
		return nil, err
	}

	// split yaml document
	documentList, err := splitMultiDocYAML(content)
	if err != nil {
		return nil, err
	}

	// sort kube kinds
	documentList, err = sortKubeKinds(documentList)
	if err != nil {
		return nil, fmt.Errorf("unable to sort kube kinds: %w", err)
	}

	for _, document := range documentList {
		kind, err := getKubeKind(document)
		if err != nil {
			return nil, fmt.Errorf("unable to read as kube YAML: %w", err)
		}

		switch kind {
		case "Pod":
			var podYAML v1.Pod
			if err := yaml.Unmarshal(document, &podYAML); err != nil {
				return nil, fmt.Errorf("unable to read YAML as Kube Pod: %w", err)
			}
			podNames = append(podNames, podYAML.ObjectMeta.Name)

			for _, vol := range podYAML.Spec.Volumes {
				switch vs := vol.VolumeSource; {
				case vs.PersistentVolumeClaim != nil:
					volumeNames = append(volumeNames, vs.PersistentVolumeClaim.ClaimName)
				case vs.ConfigMap != nil:
					volumeNames = append(volumeNames, vs.ConfigMap.Name)
				case vs.Secret != nil:
					volumeNames = append(volumeNames, vs.Secret.SecretName)
				}
			}
		case "DaemonSet":
			var daemonSetYAML v1apps.DaemonSet

			if err := yaml.Unmarshal(document, &daemonSetYAML); err != nil {
				return nil, fmt.Errorf("unable to read YAML as Kube DaemonSet: %w", err)
			}

			podName := fmt.Sprintf("%s-pod", daemonSetYAML.Name)
			podNames = append(podNames, podName)
		case "Deployment":
			var deploymentYAML v1apps.Deployment

			if err := yaml.Unmarshal(document, &deploymentYAML); err != nil {
				return nil, fmt.Errorf("unable to read YAML as Kube Deployment: %w", err)
			}
			var numReplicas int32 = 1
			deploymentName := deploymentYAML.ObjectMeta.Name
			if deploymentYAML.Spec.Replicas != nil {
				numReplicas = *deploymentYAML.Spec.Replicas
			}
			if numReplicas > 1 {
				logrus.Warnf("Limiting replica count to 1, more than one replica is not supported by Podman")
			}
			podName := fmt.Sprintf("%s-pod", deploymentName)
			podNames = append(podNames, podName)
		case "Job":
			var jobYAML v1.Job

			if err := yaml.Unmarshal(document, &jobYAML); err != nil {
				return nil, fmt.Errorf("unable to read YAML as Kube Job: %w", err)
			}
			jobName := jobYAML.ObjectMeta.Name
			podName := fmt.Sprintf("%s-pod", jobName)
			podNames = append(podNames, podName)
		case "PersistentVolumeClaim":
			var pvcYAML v1.PersistentVolumeClaim
			if err := yaml.Unmarshal(document, &pvcYAML); err != nil {
				return nil, fmt.Errorf("unable to read YAML as Kube PersistentVolumeClaim: %w", err)
			}
			volumeNames = append(volumeNames, pvcYAML.Name)
		case "Secret":
			var secret v1.Secret
			if err := yaml.Unmarshal(document, &secret); err != nil {
				return nil, fmt.Errorf("unable to read YAML as Kube Secret: %w", err)
			}
			secretNames = append(secretNames, secret.Name)
		default:
			continue
		}
	}

	// Get the service containers associated with the pods if any
	serviceCtrIDs := []string{}
	for _, name := range podNames {
		pod, err := ic.Libpod.LookupPod(name)
		if err != nil {
			if errors.Is(err, define.ErrNoSuchPod) {
				continue
			}
			return nil, err
		}
		ctr, err := pod.ServiceContainer()
		if errors.Is(err, define.ErrNoSuchCtr) {
			continue
		}
		if err != nil {
			return nil, err
		}
		serviceCtrIDs = append(serviceCtrIDs, ctr.ID())
	}

	// Add the reports
	reports.StopReport, err = ic.PodStop(ctx, podNames, entities.PodStopOptions{
		Ignore:  true,
		Timeout: -1,
	})
	if err != nil {
		return nil, err
	}

	reports.RmReport, err = ic.PodRm(ctx, podNames, entities.PodRmOptions{Ignore: true, Force: true})
	if err != nil {
		return nil, err
	}

	reports.SecretRmReport, err = ic.SecretRm(ctx, secretNames, entities.SecretRmOptions{Ignore: true})
	if err != nil {
		return nil, err
	}

	if options.Force {
		reports.VolumeRmReport, err = ic.VolumeRm(ctx, volumeNames, entities.VolumeRmOptions{Ignore: true})
		if err != nil {
			return nil, err
		}
	}

	// Remove the service container to ensure it is removed before we return for the remote case
	// Needed for the clean up with podman kube play --wait in the remote case
	if len(serviceCtrIDs) > 0 {
		for _, ctrID := range serviceCtrIDs {
			_, err = ic.ContainerRm(ctx, []string{ctrID}, entities.RmOptions{})
			if err != nil && !errors.Is(err, define.ErrNoSuchCtr) {
				return nil, err
			}
		}
	}

	return reports, nil
}

// playKubeSecret allows users to create and store a kubernetes secret as a podman secret
func (ic *ContainerEngine) playKubeSecret(secret *v1.Secret) (*entities.SecretCreateReport, error) {
	r := &entities.SecretCreateReport{}

	// Create the secret manager before hand
	secretsManager, err := ic.Libpod.SecretsManager()
	if err != nil {
		return nil, err
	}

	data, err := yaml.Marshal(secret)
	if err != nil {
		return nil, err
	}

	secretsPath := ic.Libpod.GetSecretsStorageDir()
	opts := make(map[string]string)
	opts["path"] = filepath.Join(secretsPath, "filedriver")
	// maybe k8sName(data)...
	// using this does not allow the user to use the name given to the secret
	// but keeping secret.Name as the ID can lead to a collision.

	s, err := secretsManager.Lookup(secret.Name)
	if err == nil {
		if val, ok := s.Metadata["immutable"]; ok {
			if val == "true" {
				return nil, fmt.Errorf("cannot remove colliding secret as it is set to immutable")
			}
		}
		_, err = secretsManager.Delete(s.Name)
		if err != nil {
			return nil, err
		}
	}

	// now we have either removed the old secret w/ the same name or
	// the name was not taken. Either way, we can now store.

	meta := make(map[string]string)
	if secret.Immutable != nil && *secret.Immutable {
		meta["immutable"] = "true"
	}

	storeOpts := secrets.StoreOptions{
		DriverOpts: opts,
		Metadata:   meta,
	}

	secretID, err := secretsManager.Store(secret.Name, data, "file", storeOpts)
	if err != nil {
		return nil, err
	}

	r.ID = secretID

	return r, nil
}

func getMountLabel(securityContext *v1.PodSecurityContext) (string, error) {
	var mountLabel string
	if securityContext == nil {
		return "", nil
	}
	seopt := securityContext.SELinuxOptions
	if seopt == nil {
		return mountLabel, nil
	}
	if seopt.Level == "" && seopt.FileType == "" {
		return mountLabel, nil
	}
	privLabel := selinux.PrivContainerMountLabel()
	con, err := selinux.NewContext(privLabel)
	if err != nil {
		return mountLabel, err
	}

	con["level"] = "s0"
	if seopt.Level != "" {
		con["level"] = seopt.Level
	}
	if seopt.FileType != "" {
		con["type"] = seopt.FileType
	}
	return con.Get(), nil
}

func expandForKube(s *specgen.SpecGenerator) {
	mapping := expansion.MappingFuncFor(s.Env)
	for i, subCmd := range s.Entrypoint {
		s.Entrypoint[i] = expansion.Expand(subCmd, mapping)
	}
	for i, subCmd := range s.Command {
		s.Command[i] = expansion.Expand(subCmd, mapping)
	}
}
