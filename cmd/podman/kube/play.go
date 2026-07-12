package kube

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	buildahParse "go.podman.io/buildah/pkg/parse"
	"go.podman.io/common/pkg/auth"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/image/v5/types"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/parse"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/utils"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/libpod/shutdown"
	"go.podman.io/podman/v6/pkg/annotations"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/errorhandling"
	"go.podman.io/podman/v6/pkg/util"
)

// playKubeOptionsWrapper allows for separating CLI-only fields from API-only
// fields.
type playKubeOptionsWrapper struct {
	entities.PlayKubeOptions

	TLSVerifyCLI   bool
	CredentialsCLI string
	StartCLI       bool
	BuildCLI       bool
	annotations    []string
	macs           []string
}

const yamlFileSeparator = "\n---\n"

var (
	// https://kubernetes.io/docs/reference/command-line-tools-reference/kubelet/
	defaultSeccompRoot = "/var/lib/kubelet/seccomp"
	playOptions        = playKubeOptionsWrapper{}
	playDescription    = `Reads in a structured file of Kubernetes YAML.

  Creates pods or volumes based on the Kubernetes kind described in the YAML. Supported kinds are Pods, Deployments, DaemonSets, Jobs, and PersistentVolumeClaims.`

	playCmd = &cobra.Command{
		Use:               "play [options] [KUBEFILE [KUBEFILE...]]|-",
		Short:             "Play a pod or volume based on Kubernetes YAML",
		Long:              playDescription,
		RunE:              play,
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: completion.AutocompleteDefault,
		Example: `podman kube play nginx.yml
cat nginx.yml | podman kube play -
podman kube play --creds user:password --seccomp-profile-root /custom/path apache.yml
podman kube play https://example.com/nginx.yml`,
	}
)

var (
	playKubeCmd = &cobra.Command{
		Use:               "kube [options] [KUBEFILE [KUBEFILE...]]|-",
		Short:             "Play a pod or volume based on Kubernetes YAML",
		Long:              playDescription,
		Hidden:            true,
		RunE:              playKube,
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: completion.AutocompleteDefault,
		Example: `podman play kube nginx.yml
cat nginx.yml | podman play kube -
podman play kube --creds user:password --seccomp-profile-root /custom/path apache.yml
podman play kube https://example.com/nginx.yml`,
	}
	logDriverFlagName = "log-driver"
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: playCmd,
		Parent:  kubeCmd,
	})
	playFlags(playCmd)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: playKubeCmd,
		Parent:  playKubeParentCmd,
	})
	playFlags(playKubeCmd)
}

func playFlags(cmd *cobra.Command) {
	flags := cmd.Flags()
	flags.SetNormalizeFunc(utils.AliasFlags)
	podmanConfig := registry.PodmanConfig()

	annotationFlagName := "annotation"
	flags.StringArrayVar(
		&playOptions.annotations,
		annotationFlagName, []string{},
		"Add Podman-specific annotations to containers and pods created by Podman (key=value)",
	)
	_ = cmd.RegisterFlagCompletionFunc(annotationFlagName, completion.AutocompleteNone)
	credsFlagName := "creds"
	flags.StringVar(&playOptions.CredentialsCLI, credsFlagName, "", "`Credentials` (USERNAME:PASSWORD) to use for authenticating to a registry")
	_ = cmd.RegisterFlagCompletionFunc(credsFlagName, completion.AutocompleteNone)

	staticMACFlagName := "mac-address"
	flags.StringSliceVar(&playOptions.macs, staticMACFlagName, nil, "Static MAC addresses to assign to the pods")
	_ = cmd.RegisterFlagCompletionFunc(staticMACFlagName, completion.AutocompleteNone)

	networkFlagName := "network"
	flags.StringArrayVar(&playOptions.Networks, networkFlagName, nil, "Connect pod to network(s) or network mode")
	_ = cmd.RegisterFlagCompletionFunc(networkFlagName, common.AutocompleteNetworkFlag)

	staticIPFlagName := "ip"
	flags.IPSliceVar(&playOptions.StaticIPs, staticIPFlagName, nil, "Static IP addresses to assign to the pods")
	_ = cmd.RegisterFlagCompletionFunc(staticIPFlagName, completion.AutocompleteNone)

	flags.StringVar(&playOptions.LogDriver, logDriverFlagName, common.LogDriver(), "Logging driver for the container")
	_ = cmd.RegisterFlagCompletionFunc(logDriverFlagName, common.AutocompleteLogDriver)

	logOptFlagName := "log-opt"
	flags.StringArrayVar(
		&playOptions.LogOptions,
		logOptFlagName, []string{},
		"Logging driver options",
	)
	_ = cmd.RegisterFlagCompletionFunc(logOptFlagName, common.AutocompleteLogOpt)

	usernsFlagName := "userns"
	flags.StringVar(&playOptions.Userns, usernsFlagName, "",
		"User namespace to use",
	)
	_ = cmd.RegisterFlagCompletionFunc(usernsFlagName, common.AutocompleteUserNamespace)

	flags.BoolVar(&playOptions.NoHostname, "no-hostname", false, "Do not create /etc/hostname within the container, instead use the version from the image")
	flags.BoolVar(&playOptions.NoHosts, "no-hosts", podmanConfig.ContainersConfDefaultsRO.Containers.NoHosts, "Do not create /etc/hosts within the pod's containers, instead use the version from the image")
	flags.BoolVarP(&playOptions.Quiet, "quiet", "q", false, "Suppress output information when pulling images")
	flags.BoolVar(&playOptions.TLSVerifyCLI, "tls-verify", true, "Require HTTPS and verify certificates when contacting registries")
	flags.BoolVar(&playOptions.StartCLI, "start", true, "Start the pod after creating it")
	flags.BoolVar(&playOptions.Force, "force", false, "Remove volumes as part of --down")

	authfileFlagName := "authfile"
	flags.StringVar(&playOptions.Authfile, authfileFlagName, auth.GetDefaultAuthFile(), "Path of the authentication file. Use REGISTRY_AUTH_FILE environment variable to override")
	_ = cmd.RegisterFlagCompletionFunc(authfileFlagName, completion.AutocompleteDefault)

	downFlagName := "down"
	flags.BoolVar(&playOptions.Down, downFlagName, false, "Stop pods defined in the YAML file")
	_ = flags.MarkHidden("down")

	replaceFlagName := "replace"
	flags.BoolVar(&playOptions.Replace, replaceFlagName, false, "Delete and recreate pods defined in the YAML file")

	publishPortsFlagName := "publish"
	flags.StringSliceVar(&playOptions.PublishPorts, publishPortsFlagName, []string{}, "Publish a container's port, or a range of ports, to the host")
	_ = cmd.RegisterFlagCompletionFunc(publishPortsFlagName, completion.AutocompleteNone)

	publishAllPortsFlagName := "publish-all"
	flags.BoolVar(&playOptions.PublishAllPorts, publishAllPortsFlagName, false, "Whether to publish all ports defined in the K8S YAML file (containerPort, hostPort), if false only hostPort will be published")

	waitFlagName := "wait"
	flags.BoolVarP(&playOptions.Wait, waitFlagName, "w", false, "Clean up all objects created when a SIGTERM is received or pods exit")

	configmapFlagName := "configmap"
	flags.StringArrayVar(&playOptions.ConfigMaps, configmapFlagName, []string{}, "`Pathname` of a YAML file containing a kubernetes configmap")
	_ = cmd.RegisterFlagCompletionFunc(configmapFlagName, completion.AutocompleteDefault)

	noTruncFlagName := "no-trunc"
	flags.BoolVar(&playOptions.UseLongAnnotations, noTruncFlagName, false, "Use annotations that are not truncated to the Kubernetes maximum length of 63 characters")
	_ = flags.MarkHidden(noTruncFlagName)

	noPodPrefix := "no-pod-prefix"
	flags.BoolVar(&playOptions.NoPodPrefix, noPodPrefix, false, "Do not prefix container name with pod name")

	if !registry.IsRemote() {
		certDirFlagName := "cert-dir"
		flags.StringVar(&playOptions.CertDir, certDirFlagName, "", "`Pathname` of a directory containing TLS certificates and keys")
		_ = cmd.RegisterFlagCompletionFunc(certDirFlagName, completion.AutocompleteDefault)

		seccompProfileRootFlagName := "seccomp-profile-root"
		flags.StringVar(&playOptions.SeccompProfileRoot, seccompProfileRootFlagName, defaultSeccompRoot, "Directory path for seccomp profiles")
		_ = cmd.RegisterFlagCompletionFunc(seccompProfileRootFlagName, completion.AutocompleteDefault)

		buildFlagName := "build"
		flags.BoolVar(&playOptions.BuildCLI, buildFlagName, false, "Build all images in a YAML (given Containerfiles exist)")

		contextDirFlagName := "context-dir"
		flags.StringVar(&playOptions.ContextDir, contextDirFlagName, "", "Path to top level of context directory")
		_ = cmd.RegisterFlagCompletionFunc(contextDirFlagName, completion.AutocompleteDefault)

		flags.StringVar(&playOptions.SignaturePolicy, "signature-policy", "", "`Pathname` of signature policy file (not usually used)")

		_ = flags.MarkHidden("signature-policy")

		// Below flags are local-only and hidden since they are used in
		// kube-play's systemd integration only and hence hidden from
		// users.
		serviceFlagName := "service-container"
		flags.BoolVar(&playOptions.ServiceContainer, serviceFlagName, false, "Starts a service container before all pods")
		_ = flags.MarkHidden(serviceFlagName)
		exitFlagName := "service-exit-code-propagation"
		flags.StringVar(&playOptions.ExitCodePropagation, exitFlagName, "", "Exit-code propagation of the service container")
		_ = flags.MarkHidden(exitFlagName)
	}
}

func play(cmd *cobra.Command, args []string) error {
	if playOptions.ServiceContainer && !playOptions.StartCLI { // Sanity check to be future proof
		return fmt.Errorf("--service-container does not work with --start=stop")
	}
	// TLS verification in c/image is controlled via a `types.OptionalBool`
	// which allows for distinguishing among set-true, set-false, unspecified
	// which is important to implement a sane way of dealing with defaults of
	// boolean CLI flags.
	if cmd.Flags().Changed("tls-verify") {
		playOptions.SkipTLSVerify = types.NewOptionalBool(!playOptions.TLSVerifyCLI)
	}
	if cmd.Flags().Changed("start") {
		playOptions.Start = types.NewOptionalBool(playOptions.StartCLI)
	}
	if cmd.Flags().Changed("build") {
		playOptions.Build = types.NewOptionalBool(playOptions.BuildCLI)
		if playOptions.Build == types.OptionalBoolTrue {
			systemContext, err := buildahParse.SystemContextFromOptions(cmd)
			if err != nil {
				return err
			}
			playOptions.SystemContext = systemContext
		}
	}
	if cmd.Flags().Changed("authfile") {
		if err := auth.CheckAuthFile(playOptions.Authfile); err != nil {
			return err
		}
	}
	if playOptions.ContextDir != "" && playOptions.Build != types.OptionalBoolTrue {
		return errors.New("--build must be specified when using --context-dir option")
	}
	if playOptions.CredentialsCLI != "" {
		creds, err := util.ParseRegistryCreds(playOptions.CredentialsCLI)
		if err != nil {
			return err
		}
		playOptions.Username = creds.Username
		playOptions.Password = creds.Password
	}

	for _, annotation := range playOptions.annotations {
		key, val, hasVal := strings.Cut(annotation, "=")
		if !hasVal {
			return fmt.Errorf("annotation %q must include an '=' sign", annotation)
		}
		if playOptions.Annotations == nil {
			playOptions.Annotations = make(map[string]string)
		}
		playOptions.Annotations[key] = val
	}

	if err := annotations.ValidateAnnotations(playOptions.Annotations); err != nil {
		return err
	}

	for _, mac := range playOptions.macs {
		m, err := net.ParseMAC(mac)
		if err != nil {
			return err
		}
		playOptions.StaticMACs = append(playOptions.StaticMACs, m)
	}

	if playOptions.Force && !playOptions.Down {
		return errors.New("--force may be specified only with --down")
	}

	reader, err := readerFromArgs(args)
	if err != nil {
		return err
	}

	if playOptions.Down {
		return teardown(reader, entities.PlayKubeDownOptions{Force: playOptions.Force})
	}

	if playOptions.Replace {
		if err := teardown(reader, entities.PlayKubeDownOptions{Force: playOptions.Force}); err != nil && !errorhandling.Contains(err, define.ErrNoSuchPod) {
			return err
		}
		if _, err := reader.Seek(0, 0); err != nil {
			return err
		}
	}

	// Create a channel to catch an interrupt or SIGTERM signal
	ch := make(chan os.Signal, 1)
	var teardownReader *bytes.Reader
	if playOptions.Wait {
		// Stop the shutdown signal handler so we can actually clean up after a SIGTERM or interrupt
		if err := shutdown.Stop(); err != nil && err != shutdown.ErrNotStarted {
			return err
		}
		signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
		playOptions.ServiceContainer = true

		// Read the kube yaml file again so that a reader can be passed down to the teardown function
		teardownReader, err = readerFromArgs(args)
		if err != nil {
			return err
		}
		fmt.Println("Use ctrl+c to clean up or wait for pods to exit")
	}

	var teardownErr error
	cancelled := false
	if playOptions.Wait {
		// use a goroutine to wait for a sigterm or interrupt
		go func() {
			<-ch
			// clean up any volumes that were created as well
			fmt.Println("\nCleaning up containers, pods, and volumes...")
			cancelled = true
			if err := teardown(teardownReader, entities.PlayKubeDownOptions{Force: true}); err != nil && !errorhandling.Contains(err, define.ErrNoSuchPod) {
				teardownErr = fmt.Errorf("error during cleanup: %v", err)
			}
		}()
	}

	if playErr := kubeplay(reader); playErr != nil {
		// FIXME: The cleanup logic below must be fixed to only remove
		// resources that were created before a failure.  Otherwise,
		// rerunning the same YAML file will cause an error and remove
		// the previously created workload.
		//
		// teardown any containers, pods, and volumes that might have been created before we hit the error
		// reader, err := readerFromArg(args[0])
		// if err != nil {
		// 	return err
		// }
		// if err := teardown(reader, entities.PlayKubeDownOptions{Force: true}, true); err != nil && !errorhandling.Contains(err, define.ErrNoSuchPod) {
		// 	return fmt.Errorf("error tearing down workloads %q after kube play error %q", err, playErr)
		// }
		return playErr
	}
	if teardownErr != nil {
		return teardownErr
	}

	// cleanup if --wait=true and the pods have exited
	if playOptions.Wait && !cancelled {
		fmt.Println("Cleaning up containers, pods, and volumes...")
		// clean up any volumes that were created as well
		if err := teardown(teardownReader, entities.PlayKubeDownOptions{Force: true}); err != nil && !errorhandling.Contains(err, define.ErrNoSuchPod) {
			return err
		}
	}

	return nil
}

func playKube(cmd *cobra.Command, args []string) error {
	return play(cmd, args)
}

func readerFromArgs(args []string) (*bytes.Reader, error) {
	return readerFromArgsWithStdin(args, os.Stdin)
}

func readerFromArgsWithStdin(args []string, stdin io.Reader) (*bytes.Reader, error) {
	// if user tried to pipe, shortcut the reading
	if len(args) == 1 && args[0] == "-" {
		data, err := io.ReadAll(stdin)
		if err != nil {
			return nil, err
		}
		return bytes.NewReader(data), nil
	}

	var combined bytes.Buffer

	for i, arg := range args {
		reader, err := readerFromArg(arg)
		if err != nil {
			return nil, err
		}

		_, err = io.Copy(&combined, reader)
		reader.Close()
		if err != nil {
			return nil, err
		}

		if i < len(args)-1 {
			// separate multiple files with YAML document separator
			combined.WriteString(yamlFileSeparator)
		}
	}

	return bytes.NewReader(combined.Bytes()), nil
}

func readerFromArg(fileOrURL string) (io.ReadCloser, error) {
	switch {
	case parse.ValidWebURL(fileOrURL) == nil:
		response, err := http.Get(fileOrURL)
		if err != nil {
			return nil, err
		}
		return response.Body, nil
	default:
		return os.Open(fileOrURL)
	}
}

func teardown(body io.Reader, options entities.PlayKubeDownOptions) error {
	var (
		podStopErrors utils.OutputErrors
		podRmErrors   utils.OutputErrors
		volRmErrors   utils.OutputErrors
		secRmErrors   utils.OutputErrors
	)
	reports, err := registry.ContainerEngine().PlayKubeDown(registry.Context(), body, options)
	if err != nil {
		return err
	}

	// Output stopped pods
	fmt.Println("Pods stopped:")
	for _, stopped := range reports.StopReport {
		switch {
		case len(stopped.Errs) > 0:
			podStopErrors = append(podStopErrors, stopped.Errs...)
		default:
			fmt.Println(stopped.Id)
		}
	}
	// Dump any stop errors
	lastStopError := podStopErrors.PrintErrors()
	if lastStopError != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", lastStopError)
	}

	// Output rm'd pods
	fmt.Println("Pods removed:")
	for _, removed := range reports.RmReport {
		switch {
		case removed.Err != nil:
			podRmErrors = append(podRmErrors, removed.Err)
		default:
			fmt.Println(removed.Id)
		}
	}

	lastPodRmError := podRmErrors.PrintErrors()
	if lastPodRmError != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", lastPodRmError)
	}

	// Output rm'd volumes
	fmt.Println("Secrets removed:")
	for _, removed := range reports.SecretRmReport {
		switch {
		case removed.Err != nil:
			secRmErrors = append(secRmErrors, removed.Err)
		default:
			fmt.Println(removed.ID)
		}
	}
	lastSecretRmError := secRmErrors.PrintErrors()
	if lastPodRmError != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", lastSecretRmError)
	}

	// Output rm'd volumes
	fmt.Println("Volumes removed:")
	for _, removed := range reports.VolumeRmReport {
		switch {
		case removed.Err != nil:
			volRmErrors = append(volRmErrors, removed.Err)
		default:
			fmt.Println(removed.Id)
		}
	}

	return volRmErrors.PrintErrors()
}

func kubeplay(body io.Reader) error {
	report, err := registry.ContainerEngine().PlayKube(registry.Context(), body, playOptions.PlayKubeOptions)
	if err != nil {
		return err
	}
	if report.ExitCode != nil {
		registry.SetExitCode(int(*report.ExitCode))
	}
	if err := printPlayReport(report); err != nil {
		return err
	}

	// If --wait=true, we need wait for the service container to exit so that we know that the pod has exited and we can clean up
	if playOptions.Wait {
		_, err := registry.ContainerEngine().ContainerWait(registry.Context(), []string{report.ServiceContainerID}, entities.WaitOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

// printPlayReport goes through the report returned by KubePlay and prints it out in a human
// friendly format.
func printPlayReport(report *entities.PlayKubeReport) error {
	// Print volumes report
	for i, volume := range report.Volumes {
		if i == 0 {
			fmt.Println("Volumes:")
		}
		fmt.Println(volume.Name)
	}

	// Print secrets report
	for i, secret := range report.Secrets {
		if i == 0 {
			fmt.Println("Secrets:")
		}
		fmt.Println(secret.CreateReport.ID)
	}

	// Print pods report
	for _, pod := range report.Pods {
		for _, l := range pod.Logs {
			fmt.Fprint(os.Stderr, l)
		}
	}

	ctrsFailed := 0

	for _, pod := range report.Pods {
		fmt.Println("Pod:")
		fmt.Println(pod.ID)

		switch len(pod.Containers) {
		case 0:
			continue
		case 1:
			fmt.Println("Container:")
		default:
			fmt.Println("Containers:")
		}
		for _, ctr := range pod.Containers {
			fmt.Println(ctr)
		}
		ctrsFailed += len(pod.ContainerErrors)
		// If We have errors, add a newline
		if len(pod.ContainerErrors) > 0 {
			fmt.Println()
		}
		for _, err := range pod.ContainerErrors {
			fmt.Fprintln(os.Stderr, err)
		}
		// Empty line for space for next block
		fmt.Println()
	}

	if ctrsFailed > 0 {
		return fmt.Errorf("failed to start %d containers", ctrsFailed)
	}
	return nil
}
