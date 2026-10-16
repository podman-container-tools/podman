package quadlet

import (
	"fmt"
	"strings"

	v1 "go.podman.io/podman/v6/pkg/k8s.io/api/core/v1"
	"go.podman.io/podman/v6/pkg/k8s.io/apimachinery/pkg/api/resource"
	"go.podman.io/podman/v6/pkg/systemd/parser"
	"go.podman.io/podman/v6/pkg/systemd/quadlet"
)

// buildContainerUnit assembles a Quadlet .container unit for c.
// It returns the primary unit plus any companion files (a .sh script or .env file).
// pod-level directives (Network=, Sysctl=, After=, etc.) are applied by Convert().
func buildContainerUnit(c v1.Container, pod *v1.Pod, opts Options) ([]*GeneratedFile, error) {
	unit := parser.NewUnitFile()
	unit.Filename = fmt.Sprintf("%s-%s.container", opts.NamePrefix, c.Name)

	unit.Set(quadlet.UnitGroup, "Description",
		fmt.Sprintf("Container %s for pod %s", c.Name, pod.Name))

	// ?6.1 Identity
	unit.Set(quadlet.ContainerGroup, quadlet.KeyImage, c.Image)

	if c.ImagePullPolicy != "" {
		if pull := pullValue(c.ImagePullPolicy); pull != "" {
			unit.Set(quadlet.ContainerGroup, quadlet.KeyPull, pull)
		}
	}
	if c.WorkingDir != "" {
		unit.Set(quadlet.ContainerGroup, quadlet.KeyWorkingDir, c.WorkingDir)
	}

	// ?6.2 Command & args
	scriptFile := applyExec(unit, c.Command, c.Args, opts.NamePrefix, c.Name)

	// ?6.3 Environment (large values collected for env file)
	envLines, err := applyEnv(unit, c.Env, c.EnvFrom, pod, &c, opts)
	if err != nil {
		return nil, err
	}

	// ?6.4 Security context
	applySecurityContext(unit, c.SecurityContext, pod.Spec.SecurityContext)
	// Pod-level supplemental groups are applied to every container.
	applySupplementalGroups(unit, pod.Spec.SecurityContext)

	// ?6.5 Resources
	applyResources(unit, c.Resources)

	// ?6.6 Volumes (ConfigMap/Secret handled via applyConfigMapVolume/applySecretVolume below)
	if err := applyVolumeMountsWithConfigSecrets(unit, c.VolumeMounts, pod.Spec.Volumes, opts); err != nil {
		return nil, err
	}

	// ?6.7 Health probes
	applyProbes(unit, c.LivenessProbe, c.ReadinessProbe, c.StartupProbe, pod.Spec.RestartPolicy)

	// ?6.9 Ports ? not published, just a comment
	if len(c.Ports) > 0 {
		unit.AddComment(quadlet.ContainerGroup, "Ports declared in pod spec (not published by default):")
		for _, p := range c.Ports {
			proto := p.Protocol
			if proto == "" {
				proto = v1.ProtocolTCP
			}
			unit.AddComment(quadlet.ContainerGroup,
				fmt.Sprintf("  - %d/%s", p.ContainerPort, proto))
		}
		unit.AddComment(quadlet.ContainerGroup, "To publish, add a drop-in: [Container]")
		unit.AddComment(quadlet.ContainerGroup, "PublishPort=<hostPort>:<containerPort>")
	}

	// ?6.1 tty / stdin
	if c.TTY {
		unit.Add(quadlet.ContainerGroup, quadlet.KeyPodmanArgs, "--tty")
	}
	if c.Stdin {
		unit.Add(quadlet.ContainerGroup, quadlet.KeyPodmanArgs, "--interactive")
	}

	// ?6.8 Lifecycle
	if c.Lifecycle != nil && c.Lifecycle.StopSignal != nil {
		unit.Set(quadlet.ContainerGroup, quadlet.KeyStopSignal, *c.Lifecycle.StopSignal)
	}

	result := []*GeneratedFile{{Name: unit.Filename, Unit: unit}}
	if scriptFile != nil {
		result = append(result, scriptFile)
	}

	// ?6.3 Env file
	if len(envLines) > 0 {
		envFileName := fmt.Sprintf("%s-%s.env", opts.NamePrefix, c.Name)
		unit.Add(quadlet.ContainerGroup, quadlet.KeyEnvironmentFile, envFileName)
		result = append(result, &GeneratedFile{
			Name:    envFileName,
			Content: strings.Join(envLines, "\n") + "\n",
		})
	}

	return result, nil
}

// applyVolumeMountsWithConfigSecrets extends applyVolumeMounts with ConfigMap/Secret volumes.
func applyVolumeMountsWithConfigSecrets(unit *parser.UnitFile, mounts []v1.VolumeMount, volumes []v1.Volume, opts Options) error {
	volByName := make(map[string]v1.VolumeSource, len(volumes))
	for _, v := range volumes {
		volByName[v.Name] = v.VolumeSource
	}

	for _, m := range mounts {
		src, ok := volByName[m.Name]
		if !ok {
			return fmt.Errorf("volumeMount %q references unknown volume %q", m.Name, m.Name)
		}
		switch {
		case src.ConfigMap != nil:
			if err := applyConfigMapVolume(unit, m, src.ConfigMap, opts); err != nil {
				return err
			}
		case src.Secret != nil:
			if err := applySecretVolume(unit, m, src.Secret, opts); err != nil {
				return err
			}
		default:
			if err := applyMount(unit, m, src, opts.NamePrefix); err != nil {
				return fmt.Errorf("volume %q: %w", m.Name, err)
			}
		}
	}
	return nil
}

// pullValue maps an ImagePullPolicy to the Quadlet Pull= directive value.
func pullValue(p v1.PullPolicy) string {
	switch p {
	case v1.PullAlways:
		return "always"
	case v1.PullNever:
		return "never"
	case v1.PullIfNotPresent:
		return "missing"
	}
	return ""
}

// largeEnvThreshold is the byte size above which an env value is moved to an env file.
const largeEnvThreshold = 1024

// applyEnv maps env[] and envFrom[] sources onto unit.
// Large or complex values are collected into envFileLines and written by the caller
// as a companion .env GeneratedFile.
// Returns companion env file lines (nil when none).
func applyEnv(
	unit *parser.UnitFile,
	envVars []v1.EnvVar,
	envFrom []v1.EnvFromSource,
	pod *v1.Pod,
	container *v1.Container,
	opts Options,
) (envFileLines []string, err error) {
	for _, e := range envVars {
		val, skip, envErr := resolveEnvVar(e, pod, container, opts)
		if envErr != nil {
			return nil, envErr
		}
		if skip {
			continue
		}
		kv := e.Name + "=" + val
		if isLargeEnvValue(val) {
			envFileLines = append(envFileLines, kv)
		} else {
			unit.AddEscaped(quadlet.ContainerGroup, quadlet.KeyEnvironment, kv)
		}
	}

	for _, ef := range envFrom {
		lines, envErr := resolveEnvFrom(ef, opts)
		if envErr != nil {
			return nil, envErr
		}
		for _, kv := range lines {
			unit.AddEscaped(quadlet.ContainerGroup, quadlet.KeyEnvironment, kv)
		}
	}

	return envFileLines, nil
}

// resolveEnvVar resolves a single EnvVar to its string value.
// Returns (value, skip, error). skip=true means the var is optional and missing.
func resolveEnvVar(e v1.EnvVar, pod *v1.Pod, container *v1.Container, opts Options) (string, bool, error) {
	if e.ValueFrom == nil {
		return e.Value, false, nil
	}
	vf := e.ValueFrom

	switch {
	case vf.ConfigMapKeyRef != nil:
		return resolveConfigMapKeyRef(vf.ConfigMapKeyRef, opts)
	case vf.SecretKeyRef != nil:
		return resolveSecretKeyRef(vf.SecretKeyRef, opts)
	case vf.FieldRef != nil:
		return resolveFieldRef(vf.FieldRef, pod, e.Name)
	case vf.ResourceFieldRef != nil:
		return resolveResourceFieldRef(vf.ResourceFieldRef, container)
	}
	return "", false, nil
}

func resolveConfigMapKeyRef(ref *v1.ConfigMapKeySelector, opts Options) (string, bool, error) {
	optional := ref.Optional != nil && *ref.Optional
	cm, ok := opts.ConfigMaps[ref.Name]
	if !ok {
		if optional {
			return "", true, nil
		}
		return "", false, fmt.Errorf("configMap %q not found in opts.ConfigMaps", ref.Name)
	}
	val, ok := cm.Data[ref.Key]
	if !ok {
		if optional {
			return "", true, nil
		}
		return "", false, fmt.Errorf("configMap %q has no key %q", ref.Name, ref.Key)
	}
	return val, false, nil
}

func resolveSecretKeyRef(ref *v1.SecretKeySelector, opts Options) (string, bool, error) {
	optional := ref.Optional != nil && *ref.Optional
	sec, ok := opts.Secrets[ref.Name]
	if !ok {
		if optional {
			return "", true, nil
		}
		return "", false, fmt.Errorf("secret %q not found in opts.Secrets", ref.Name)
	}
	val, ok := sec.Data[ref.Key]
	if !ok {
		if optional {
			return "", true, nil
		}
		return "", false, fmt.Errorf("secret %q has no key %q", ref.Name, ref.Key)
	}
	return string(val), false, nil
}

func resolveFieldRef(ref *v1.ObjectFieldSelector, pod *v1.Pod, envName string) (string, bool, error) {
	switch ref.FieldPath {
	case "metadata.name":
		return pod.Name, false, nil
	case "metadata.namespace":
		return pod.Namespace, false, nil
	case "metadata.uid":
		return string(pod.UID), false, nil
	}

	// metadata.labels["key"] and metadata.annotations["key"]
	if after, ok := strings.CutPrefix(ref.FieldPath, "metadata.labels[\""); ok {
		key := strings.TrimSuffix(after, "\"]")
		return pod.Labels[key], false, nil
	}
	if after, ok := strings.CutPrefix(ref.FieldPath, "metadata.annotations[\""); ok {
		key := strings.TrimSuffix(after, "\"]")
		return pod.Annotations[key], false, nil
	}

	// status.* fields are not knowable at generation time.
	return "", false, fmt.Errorf("fieldRef path %q for env %q is not supported at generation time", ref.FieldPath, envName)
}

func resolveResourceFieldRef(ref *v1.ResourceFieldSelector, container *v1.Container) (string, bool, error) {
	if container == nil {
		return "0", false, nil
	}
	divisor := ref.Divisor
	if divisor.IsZero() {
		// Default divisor is 1.
		divisor = *resource.NewMilliQuantity(1000, resource.DecimalSI)
	}

	switch ref.Resource {
	case "limits.cpu":
		if container.Resources.Limits != nil {
			if q, ok := container.Resources.Limits[v1.ResourceCPU]; ok {
				return fmt.Sprintf("%d", q.MilliValue()/divisor.MilliValue()), false, nil
			}
		}
	case "limits.memory":
		if container.Resources.Limits != nil {
			if q, ok := container.Resources.Limits[v1.ResourceMemory]; ok {
				return fmt.Sprintf("%d", q.Value()/divisor.Value()), false, nil
			}
		}
	case "requests.cpu":
		if container.Resources.Requests != nil {
			if q, ok := container.Resources.Requests[v1.ResourceCPU]; ok {
				return fmt.Sprintf("%d", q.MilliValue()/divisor.MilliValue()), false, nil
			}
		}
	case "requests.memory":
		if container.Resources.Requests != nil {
			if q, ok := container.Resources.Requests[v1.ResourceMemory]; ok {
				return fmt.Sprintf("%d", q.Value()/divisor.Value()), false, nil
			}
		}
	}
	return "0", false, nil
}

func resolveEnvFrom(ef v1.EnvFromSource, opts Options) ([]string, error) {
	switch {
	case ef.ConfigMapRef != nil:
		return resolveEnvFromConfigMap(ef.ConfigMapRef, opts)
	case ef.SecretRef != nil:
		return resolveEnvFromSecret(ef.SecretRef, opts)
	}
	return nil, nil
}

func resolveEnvFromConfigMap(ref *v1.ConfigMapEnvSource, opts Options) ([]string, error) {
	optional := ref.Optional != nil && *ref.Optional
	cm, ok := opts.ConfigMaps[ref.Name]
	if !ok {
		if optional {
			return nil, nil
		}
		return nil, fmt.Errorf("configMap %q not found in opts.ConfigMaps", ref.Name)
	}
	lines := make([]string, 0, len(cm.Data))
	for k, v := range cm.Data {
		lines = append(lines, k+"="+v)
	}
	return lines, nil
}

func resolveEnvFromSecret(ref *v1.SecretEnvSource, opts Options) ([]string, error) {
	optional := ref.Optional != nil && *ref.Optional
	sec, ok := opts.Secrets[ref.Name]
	if !ok {
		if optional {
			return nil, nil
		}
		return nil, fmt.Errorf("secret %q not found in opts.Secrets", ref.Name)
	}
	lines := make([]string, 0, len(sec.Data))
	for k, v := range sec.Data {
		lines = append(lines, k+"="+string(v))
	}
	return lines, nil
}

// isLargeEnvValue reports whether a value should be moved to an env file.
func isLargeEnvValue(val string) bool {
	return len(val) > largeEnvThreshold ||
		strings.ContainsAny(val, "\n=\"")
}
