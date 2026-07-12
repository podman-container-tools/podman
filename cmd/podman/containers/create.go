package containers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"go.podman.io/buildah/pkg/cli"
	"go.podman.io/common/pkg/auth"
	"go.podman.io/common/pkg/config"
	"go.podman.io/image/v5/transports/alltransports"
	"go.podman.io/image/v5/types"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/utils"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/specgen"
	"go.podman.io/podman/v6/pkg/specgenutil"
	"go.podman.io/podman/v6/pkg/util"
	"golang.org/x/term"
)

var (
	createDescription = `Creates a new container from the given image or storage and prepares it for running the specified command.

  The container ID is then printed to stdout. You can then start it at any time with the podman start <container_id> command. The container will be created with the initial state 'created'.`
	createCommand = &cobra.Command{
		Use:               "create [options] IMAGE [COMMAND [ARG...]]",
		Short:             "Create but do not start a container",
		Long:              createDescription,
		RunE:              create,
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: common.AutocompleteCreateRun,
		Example: `podman create alpine ls
podman create --annotation HELLO=WORLD alpine ls
podman create -t -i --name myctr alpine ls`,
	}

	containerCreateCommand = &cobra.Command{
		Args:              createCommand.Args,
		Use:               createCommand.Use,
		Short:             createCommand.Short,
		Long:              createCommand.Long,
		RunE:              createCommand.RunE,
		ValidArgsFunction: createCommand.ValidArgsFunction,
		Example: `podman container create alpine ls
podman container create --annotation HELLO=WORLD alpine ls
podman container create -t -i --name myctr alpine ls`,
	}
)

var (
	InitContainerType string
	cliVals           entities.ContainerCreateOptions
)

func createFlags(cmd *cobra.Command) {
	flags := cmd.Flags()

	initContainerFlagName := "init-ctr"
	flags.StringVar(
		&InitContainerType,
		initContainerFlagName, "",
		"Make this a pod init container.",
	)
	_ = cmd.RegisterFlagCompletionFunc(initContainerFlagName, common.AutocompleteInitCtr)

	flags.SetInterspersed(false)
	common.DefineCreateDefaults(&cliVals)
	common.DefineCreateFlags(cmd, &cliVals, entities.CreateMode)
	common.DefineNetFlags(cmd)

	flags.SetNormalizeFunc(utils.AliasFlags)

	if registry.IsRemote() {
		if cliVals.IsInfra {
			_ = flags.MarkHidden("infra-conmon-pidfile")
		} else {
			_ = flags.MarkHidden("conmon-pidfile")
		}

		_ = flags.MarkHidden("pidfile")
	}
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: createCommand,
	})
	createFlags(createCommand)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: containerCreateCommand,
		Parent:  containerCmd,
	})
	createFlags(containerCreateCommand)
}

func commonFlags(cmd *cobra.Command) error {
	var err error
	flags := cmd.Flags()
	cliVals.Net, err = common.NetFlagsToNetOptions(nil, *flags)
	if err != nil {
		return err
	}

	if cmd.Flags().Changed("image-volume") {
		cliVals.ImageVolume = cmd.Flag("image-volume").Value.String()
	}
	return nil
}

func create(cmd *cobra.Command, args []string) error {
	if err := commonFlags(cmd); err != nil {
		return err
	}

	// Check if initctr is used with --pod and the value is correct
	if initctr := InitContainerType; cmd.Flags().Changed("init-ctr") {
		if !cmd.Flags().Changed("pod") {
			return errors.New("must specify pod value with init-ctr")
		}
		if !slices.Contains([]string{define.AlwaysInitContainer, define.OneShotInitContainer}, initctr) {
			return fmt.Errorf("init-ctr value must be '%s' or '%s'", define.AlwaysInitContainer, define.OneShotInitContainer)
		}
		cliVals.InitContainerType = initctr
	}
	// TODO: v5.0 block users from setting restart policy for a container if the container is in a pod

	cliVals, err := CreateInit(cmd, cliVals, false)
	if err != nil {
		return err
	}
	imageName := args[0]
	rawImageName := ""
	if !cliVals.RootFS {
		rawImageName = args[0]
		name, err := pullImage(cmd, args[0], &cliVals)
		if err != nil {
			return err
		}
		imageName = name
	}

	if cmd.Flags().Changed("authfile") {
		if err := auth.CheckAuthFile(cliVals.Authfile); err != nil {
			return err
		}
	}

	s := specgen.NewSpecGenerator(imageName, cliVals.RootFS)
	if err := specgenutil.FillOutSpecGen(s, &cliVals, args); err != nil {
		return err
	}
	s.RawImageName = rawImageName

	// Include the command used to create the container.
	s.ContainerCreateCommand = os.Args

	if err := createPodIfNecessary(cmd, s, cliVals.Net); err != nil {
		return err
	}

	if cliVals.Replace {
		if err := replaceContainer(cliVals.Name); err != nil {
			return err
		}
	}

	if s.HealthConfig == nil {
		s.HealthConfig, err = common.GetHealthCheckOverrideConfig(cmd, &cliVals)
		if err != nil {
			return err
		}
	}

	report, err := registry.ContainerEngine().ContainerCreate(registry.Context(), s)
	if err != nil {
		// if pod was created as part of run
		// remove it in case ctr creation fails
		if err := rmPodIfNecessary(cmd, s); err != nil {
			if !errors.Is(err, define.ErrNoSuchPod) {
				logrus.Error(err.Error())
			}
		}
		return err
	}

	if cliVals.CIDFile != "" {
		if err := util.CreateIDFile(cliVals.CIDFile, report.Id); err != nil {
			return err
		}
	}

	if cliVals.LogDriver != define.PassthroughLogging && cliVals.LogDriver != define.PassthroughTTYLogging {
		fmt.Println(report.Id)
	}
	return nil
}

func replaceContainer(name string) error {
	if len(name) == 0 {
		return errors.New("cannot replace container without --name being set")
	}
	rmOptions := entities.RmOptions{
		Force:  true, // force stop & removal
		Ignore: true, // ignore errors when a container doesn't exit
	}
	return removeContainers([]string{name}, rmOptions, false, true)
}

func createOrUpdateFlags(cmd *cobra.Command, vals *entities.ContainerCreateOptions) error {
	if cmd.Flags().Changed("pids-limit") {
		val := cmd.Flag("pids-limit").Value.String()
		pidsLimit, err := strconv.ParseInt(val, 10, 32)
		if err != nil {
			return err
		}
		vals.PIDsLimit = &pidsLimit
	}

	return nil
}

func CreateInit(c *cobra.Command, vals entities.ContainerCreateOptions, isInfra bool) (entities.ContainerCreateOptions, error) {
	if len(vals.UIDMap) > 0 || len(vals.GIDMap) > 0 || vals.SubUIDName != "" || vals.SubGIDName != "" {
		if c.Flag("userns").Changed {
			return vals, errors.New("--userns and --uidmap/--gidmap/--subuidname/--subgidname are mutually exclusive")
		}
		// force userns flag to "private"
		vals.UserNS = "private"
	} else {
		vals.UserNS = c.Flag("userns").Value.String()
	}
	if c.Flag("kernel-memory") != nil && c.Flag("kernel-memory").Changed {
		logrus.Warnf("The --kernel-memory flag is no longer supported. This flag is a noop.")
	}

	if cliVals.LogDriver == define.PassthroughLogging {
		if term.IsTerminal(0) || term.IsTerminal(1) || term.IsTerminal(2) {
			return vals, errors.New("the '--log-driver passthrough' option cannot be used on a TTY.  If you really want it, use '--log-driver passthrough-tty'")
		}
		if registry.IsRemote() {
			return vals, errors.New("the '--log-driver passthrough' option is not supported in remote mode")
		}
	}
	if cliVals.LogDriver == define.PassthroughTTYLogging {
		if registry.IsRemote() {
			return vals, errors.New("the '--log-driver passthrough-tty' option is not supported in remote mode")
		}
	}

	if !isInfra {
		if c.Flag("cpu-period").Changed && c.Flag("cpus").Changed {
			return vals, errors.New("--cpu-period and --cpus cannot be set together")
		}
		if c.Flag("cpu-quota").Changed && c.Flag("cpus").Changed {
			return vals, errors.New("--cpu-quota and --cpus cannot be set together")
		}
		vals.IPC = c.Flag("ipc").Value.String()
		vals.PID = c.Flag("pid").Value.String()
		vals.CgroupNS = c.Flag("cgroupns").Value.String()

		if c.Flags().Changed("group-add") {
			groups := []string{}
			for _, g := range cliVals.GroupAdd {
				if g == "keep-groups" {
					if len(cliVals.GroupAdd) > 1 {
						return vals, errors.New("the '--group-add keep-groups' option is not allowed with any other --group-add options")
					}
					if registry.IsRemote() {
						return vals, errors.New("the '--group-add keep-groups' option is not supported in remote mode")
					}
					vals.Annotation = append(vals.Annotation, fmt.Sprintf("%s=1", define.RunOCIKeepOriginalGroups))
				} else {
					groups = append(groups, g)
				}
			}
			vals.GroupAdd = groups
		}

		if c.Flags().Changed("oom-score-adj") {
			val, err := c.Flags().GetInt("oom-score-adj")
			if err != nil {
				return vals, err
			}
			vals.OOMScoreAdj = &val
		}

		if err := createOrUpdateFlags(c, &vals); err != nil {
			return vals, err
		}

		if c.Flags().Changed("env") {
			env, err := c.Flags().GetStringArray("env")
			if err != nil {
				return vals, fmt.Errorf("retrieve env flag: %w", err)
			}
			vals.Env = env
		}
		if c.Flag("cgroups").Changed && vals.CgroupsMode == "split" && registry.IsRemote() {
			return vals, fmt.Errorf("the option --cgroups=%q is not supported in remote mode", vals.CgroupsMode)
		}
	}
	if c.Flag("shm-size").Changed {
		vals.ShmSize = c.Flag("shm-size").Value.String()
	}
	if c.Flag("shm-size-systemd").Changed {
		vals.ShmSizeSystemd = c.Flag("shm-size-systemd").Value.String()
	}
	if (c.Flag("dns").Changed || c.Flag("dns-option").Changed || c.Flag("dns-search").Changed) && vals.Net != nil && (vals.Net.Network.NSMode == specgen.NoNetwork || vals.Net.Network.IsContainer()) {
		return vals, errors.New("conflicting options: dns and the network mode: " + string(vals.Net.Network.NSMode))
	}
	noHosts, err := c.Flags().GetBool("no-hosts")
	if err != nil {
		return vals, err
	}
	if noHosts && c.Flag("add-host").Changed {
		return vals, errors.New("--no-hosts and --add-host cannot be set together")
	}
	if noHosts && c.Flag("hosts-file").Changed {
		return vals, errors.New("--no-hosts and --hosts-file cannot be set together")
	}

	if !isInfra && c.Flag("entrypoint").Changed {
		val := c.Flag("entrypoint").Value.String()
		vals.Entrypoint = &val
	}

	// Docker-compatibility: the "-h" flag for run/create is reserved for
	// the hostname (see https://github.com/containers/podman/issues/1367).

	return vals, nil
}

// Pulls image if any also parses and populates OS, Arch and Variant in specified container create options
func pullImage(cmd *cobra.Command, imageName string, cliVals *entities.ContainerCreateOptions) (string, error) {
	pullPolicy, err := config.ParsePullPolicy(cliVals.Pull)
	if err != nil {
		return "", err
	}

	if cliVals.Platform != "" || cliVals.Arch != "" || cliVals.OS != "" {
		if cliVals.Platform != "" {
			if cliVals.Arch != "" || cliVals.OS != "" {
				return "", errors.New("--platform option can not be specified with --arch or --os")
			}
			OS, Arch, hasArch := strings.Cut(cliVals.Platform, "/")
			cliVals.OS = OS
			if hasArch {
				cliVals.Arch = Arch
			}
		}
	}

	skipTLSVerify := types.OptionalBoolUndefined
	if cliVals.TLSVerify.Present() {
		skipTLSVerify = types.NewOptionalBool(!cliVals.TLSVerify.Value())
	}

	decConfig, err := cli.DecryptConfig(cliVals.DecryptionKeys)
	if err != nil {
		return "unable to obtain decryption config", err
	}

	pullOptions := entities.ImagePullOptions{
		Authfile:         cliVals.Authfile,
		Quiet:            cliVals.Quiet,
		Arch:             cliVals.Arch,
		OS:               cliVals.OS,
		Variant:          cliVals.Variant,
		SignaturePolicy:  cliVals.SignaturePolicy,
		PullPolicy:       pullPolicy,
		SkipTLSVerify:    skipTLSVerify,
		OciDecryptConfig: decConfig,
		CertDir:          cliVals.CertDir,
	}

	if cmd.Flags().Changed("retry") {
		retry, err := cmd.Flags().GetUint("retry")
		if err != nil {
			return "", err
		}

		pullOptions.Retry = &retry
	}

	if cmd.Flags().Changed("retry-delay") {
		val, err := cmd.Flags().GetString("retry-delay")
		if err != nil {
			return "", err
		}

		pullOptions.RetryDelay = val
	}

	if cliVals.Creds != "" {
		creds, err := util.ParseRegistryCreds(cliVals.Creds)
		if err != nil {
			return "", err
		}
		pullOptions.Username = creds.Username
		pullOptions.Password = creds.Password
	}

	pullReport, pullErr := registry.ImageEngine().Pull(registry.Context(), imageName, pullOptions)
	if pullErr != nil {
		return "", pullErr
	}

	// Return the input name such that the image resolves to correct
	// repo/tag in the backend (see #8082).  Unless we're referring to
	// the image via a transport.
	if _, err := alltransports.ParseImageName(imageName); err == nil {
		imageName = pullReport.Images[0]
	}

	return imageName, nil
}

func rmPodIfNecessary(cmd *cobra.Command, s *specgen.SpecGenerator) error {
	if !strings.HasPrefix(cmd.Flag("pod").Value.String(), "new:") {
		return nil
	}

	// errcheck not necessary since
	// pod creation would've failed
	podName := strings.Replace(s.Pod, "new:", "", 1)
	_, err := registry.ContainerEngine().PodRm(context.Background(), []string{podName}, entities.PodRmOptions{})
	return err
}

// createPodIfNecessary automatically creates a pod when requested.  if the pod name
// has the form new:ID, the pod ID is created and the name in the spec generator is replaced
// with ID.
func createPodIfNecessary(cmd *cobra.Command, s *specgen.SpecGenerator, netOpts *entities.NetOptions) error {
	if !strings.HasPrefix(s.Pod, "new:") {
		return nil
	}
	podName := strings.Replace(s.Pod, "new:", "", 1)
	if len(podName) < 1 {
		return errors.New("new pod name must be at least one character")
	}

	var err error
	uns := specgen.Namespace{NSMode: specgen.Default}
	if cliVals.UserNS != "" {
		uns, err = specgen.ParseUserNamespace(cliVals.UserNS)
		if err != nil {
			return err
		}
	}
	createOptions := entities.PodCreateOptions{
		Name:          podName,
		Infra:         true,
		Net:           netOpts,
		CreateCommand: os.Args,
		Hostname:      s.ContainerBasicConfig.Hostname,
		Cpus:          cliVals.CPUS,
		CpusetCpus:    cliVals.CPUSetCPUs,
		Pid:           cliVals.PID,
		Userns:        uns,
		Restart:       cliVals.Restart,
	}
	// Unset config values we passed to the pod to prevent them being used twice for the container and pod.
	s.ContainerBasicConfig.Hostname = ""
	s.ContainerNetworkConfig = specgen.ContainerNetworkConfig{}

	s.Pod = podName
	podSpec := entities.PodSpec{}
	podGen := specgen.NewPodSpecGenerator()
	podSpec.PodSpecGen = *podGen
	podGen, err = entities.ToPodSpecGen(podSpec.PodSpecGen, &createOptions)
	if err != nil {
		return err
	}

	infraOpts := entities.NewInfraContainerCreateOptions()
	infraOpts.Net = netOpts
	infraOpts.Quiet = true
	infraOpts.ReadOnly = true
	infraOpts.ReadWriteTmpFS = false
	infraOpts.Hostname, err = cmd.Flags().GetString("hostname")
	if err != nil {
		return err
	}
	podGen.InfraContainerSpec = specgen.NewSpecGenerator("", false)
	podGen.InfraContainerSpec.NetworkOptions = podGen.NetworkOptions
	err = specgenutil.FillOutSpecGen(podGen.InfraContainerSpec, &infraOpts, []string{})
	if err != nil {
		return err
	}
	podSpec.PodSpecGen = *podGen
	_, err = registry.ContainerEngine().PodCreate(context.Background(), podSpec)
	return err
}
