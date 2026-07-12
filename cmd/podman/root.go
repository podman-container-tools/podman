package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"slices"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/common/pkg/ssh"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/system"
	"go.podman.io/podman/v6/cmd/podman/validate"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/libpod/shutdown"
	"go.podman.io/podman/v6/pkg/bindings"
	"go.podman.io/podman/v6/pkg/checkpoint/crutils"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/parallel"
	"go.podman.io/podman/v6/version"
	"go.podman.io/storage"
	"sigs.k8s.io/yaml"
)

// HelpTemplate is the help template for podman commands
// This uses the short and long options.
// command should not use this.
const helpTemplate = `{{.Short}}

Description:
  {{.Long}}

{{if or .Runnable .HasSubCommands}}{{.UsageString}}{{end}}`

// indentExamples is a Cobra template function registered via cobra.AddTemplateFunc.
// It prepends two spaces to every non-empty line in a command's Example string
// so that examples are consistently indented in --help output.
// Example strings in source must be flush-left (no leading whitespace);
// this is enforced by TestExampleFormat.
func indentExamples(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = "  " + line
		}
	}
	return strings.Join(lines, "\n")
}

// UsageTemplate is the usage template for podman commands
// This blocks the displaying of the global options. The main podman
// command should not use this.
const usageTemplate = `Usage:{{if (and .Runnable (not .HasAvailableSubCommands))}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.UseLine}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
{{.Example | indentExamples}}{{end}}{{if .HasAvailableSubCommands}}

Available Commands:{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Options:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}
{{end}}
`

var (
	rootCmd = &cobra.Command{
		// In shell completion, there is `.exe` suffix on Windows.
		// This does not provide the same experience across platforms
		// and was mentioned in [#16499](https://github.com/containers/podman/issues/16499).
		Use:                   strings.TrimSuffix(filepath.Base(os.Args[0]), ".exe") + " [options]",
		Long:                  "Manage pods, containers and images",
		SilenceUsage:          true,
		SilenceErrors:         true,
		TraverseChildren:      true,
		PersistentPreRunE:     persistentPreRunE,
		RunE:                  validate.SubCommandExists,
		PersistentPostRunE:    persistentPostRunE,
		Version:               version.Version.String(),
		DisableFlagsInUseLine: true,
	}

	defaultLogLevel = "warn"
	logLevel        = defaultLogLevel
	dockerConfig    = ""
	debug           bool

	requireCleanup = true

	// Defaults for capturing/redirecting the command output since (the) cobra is
	// global-hungry and doesn't allow you to attach anything that allows us to
	// transform the noStdout BoolVar to a string that we can assign to useStdout.
	noStdout  = false
	useStdout = ""
)

func init() {
	cobra.AddTemplateFunc("indentExamples", indentExamples)

	// Hooks are called before PersistentPreRunE(). These hooks affect global
	// state and are executed after processing the command-line, but before
	// actually running the command.
	cobra.OnInitialize(
		stdOutHook, // Caution, this hook redirects stdout and output from any following hooks may be affected.
		loggingHook,
		syslogHook,
		earlyInitHook,
		configHook,
	)

	rootFlags(rootCmd, registry.PodmanConfig())

	// backwards compat still allow --cni-config-dir
	rootCmd.Flags().SetNormalizeFunc(func(_ *pflag.FlagSet, name string) pflag.NormalizedName {
		if name == "cni-config-dir" {
			name = "network-config-dir"
		}
		return pflag.NormalizedName(name)
	})
	rootCmd.SetUsageTemplate(usageTemplate)
}

func Execute() {
	if cmd, err := rootCmd.ExecuteContextC(registry.Context()); err != nil {
		if registry.GetExitCode() == 0 {
			registry.SetExitCode(define.ExecErrorCodeGeneric)
		}
		if registry.IsRemote() {
			if errors.As(err, &bindings.ConnectError{}) {
				fmt.Fprintln(os.Stderr, "Cannot connect to Podman. Please verify your connection to the Linux system using `podman system connection list`, or try `podman machine init` and `podman machine start` to manage a new Linux VM")
			}
		}
		fmt.Fprintln(os.Stderr, formatError(err, cmd))
	}

	_ = shutdown.Stop()

	if requireCleanup {
		// The cobra post-run is not being executed in case of
		// a previous error, so make sure that the engine(s)
		// are correctly shutdown.
		//
		// See https://github.com/spf13/cobra/issues/914
		logrus.Debugf("Shutting down engines")
		if engine := registry.ImageEngine(); engine != nil {
			engine.Shutdown(registry.Context())
		}
		if engine := registry.ContainerEngine(); engine != nil {
			engine.Shutdown(registry.Context())
		}
	}

	os.Exit(registry.GetExitCode())
}

// readRemoteCliFlags reads cli flags related to operating podman remotely
func readRemoteCliFlags(cmd *cobra.Command, podmanConfig *entities.PodmanConfig) error {
	conf := podmanConfig.ContainersConfDefaultsRO
	contextConn, host := cmd.Root().LocalFlags().Lookup("context"), cmd.Root().LocalFlags().Lookup("host")
	conn, url := cmd.Root().LocalFlags().Lookup("connection"), cmd.Root().LocalFlags().Lookup("url")

	switch {
	case conn != nil && conn.Changed:
		if contextConn != nil && contextConn.Changed {
			return fmt.Errorf("use of --connection and --context at the same time is not allowed")
		}
		con, err := conf.GetConnection(conn.Value.String(), false)
		if err != nil {
			return err
		}
		podmanConfig.URI = con.URI
		podmanConfig.Identity = con.Identity
		podmanConfig.TLSCertFile = con.TLSCert
		podmanConfig.TLSKeyFile = con.TLSKey
		podmanConfig.TLSCAFile = con.TLSCA
		podmanConfig.MachineMode = con.IsMachine
	case url.Changed:
		podmanConfig.URI = url.Value.String()
	case contextConn != nil && contextConn.Changed:
		service := contextConn.Value.String()
		if service != "default" {
			con, err := conf.GetConnection(service, false)
			if err != nil {
				return err
			}
			podmanConfig.URI = con.URI
			podmanConfig.Identity = con.Identity
			podmanConfig.TLSCertFile = con.TLSCert
			podmanConfig.TLSKeyFile = con.TLSKey
			podmanConfig.TLSCAFile = con.TLSCA
			podmanConfig.MachineMode = con.IsMachine
		}
	case host.Changed:
		podmanConfig.URI = host.Value.String()
	default:
		// No cli options set, in case CONTAINER_CONNECTION was set to something
		// invalid this contains the error, see setupRemoteConnection().
		// Important so that we can show a proper useful error message but still
		// allow the cli overwrites (https://github.com/containers/podman/pull/22997).
		return podmanConfig.ConnectionError
	}
	return nil
}

// setupRemoteConnection returns information about the active service destination
// The order of priority is:
// 1. cli flags (--connection ,--url ,--context ,--host);
// 2. Env variables (CONTAINER_HOST and CONTAINER_CONNECTION);
// 3. ActiveService from containers.conf;
// 4. RemoteURI;
// Returns the name of the default connection if any.
func setupRemoteConnection(podmanConfig *entities.PodmanConfig) string {
	conf := podmanConfig.ContainersConfDefaultsRO
	connEnv, hostEnv, sshkeyEnv := os.Getenv("CONTAINER_CONNECTION"), os.Getenv("CONTAINER_HOST"), os.Getenv("CONTAINER_SSHKEY")

	switch {
	case connEnv != "":
		con, err := conf.GetConnection(connEnv, false)
		if err != nil {
			podmanConfig.ConnectionError = err
			return connEnv
		}
		podmanConfig.URI = con.URI
		podmanConfig.Identity = con.Identity
		podmanConfig.TLSCertFile = con.TLSCert
		podmanConfig.TLSKeyFile = con.TLSKey
		podmanConfig.TLSCAFile = con.TLSCA
		podmanConfig.MachineMode = con.IsMachine
		return con.Name
	case hostEnv != "":
		if sshkeyEnv != "" {
			podmanConfig.Identity = sshkeyEnv
		}
		podmanConfig.URI = hostEnv
	default:
		con, err := conf.GetConnection("", true)
		if err == nil {
			podmanConfig.URI = con.URI
			podmanConfig.Identity = con.Identity
			podmanConfig.TLSCertFile = con.TLSCert
			podmanConfig.TLSKeyFile = con.TLSKey
			podmanConfig.TLSCAFile = con.TLSCA
			podmanConfig.MachineMode = con.IsMachine
			return con.Name
		}
		podmanConfig.URI = registry.DefaultAPIAddress()
	}
	return ""
}

func persistentPreRunE(cmd *cobra.Command, args []string) error {
	logrus.Debugf("Called %s.PersistentPreRunE(%s)", cmd.Name(), strings.Join(os.Args, " "))

	checkSupportedCgroups()

	// Help, completion and commands with subcommands are special cases, no need for more setup
	// Completion cmd is used to generate the shell scripts
	if cmd.Name() == "help" || cmd.Name() == "completion" || cmd.HasSubCommands() {
		requireCleanup = false
		return nil
	}

	podmanConfig := registry.PodmanConfig()

	if !registry.IsRemote() {
		if cmd.Flag("hooks-dir").Changed {
			podmanConfig.ContainersConf.Engine.HooksDir.Set(podmanConfig.HooksDir)
		}
		if cmd.Flag("cdi-spec-dir").Changed {
			podmanConfig.ContainersConf.Engine.CdiSpecDirs.Set(podmanConfig.CdiSpecDirs)
		}

		// Currently it is only possible to restore a container with the same runtime
		// as used for checkpointing. It should be possible to make crun and runc
		// compatible to restore a container with another runtime then checkpointed.
		// Currently that does not work.
		// To make it easier for users we will look into the checkpoint archive and
		// set the runtime to the one used during checkpointing.
		if cmd.Name() == "restore" {
			if cmd.Flag("import").Changed {
				runtime, err := crutils.CRGetRuntimeFromArchive(cmd.Flag("import").Value.String())
				if err != nil {
					return fmt.Errorf(
						"failed extracting runtime information from %s: %w",
						cmd.Flag("import").Value.String(), err,
					)
				}

				runtimeFlag := cmd.Root().Flag("runtime")
				if runtimeFlag == nil {
					return errors.New("failed to load --runtime flag")
				}

				if !runtimeFlag.Changed {
					// If the user did not select a runtime, this takes the one from
					// the checkpoint archives and tells Podman to use it for the restore.
					if err := runtimeFlag.Value.Set(*runtime); err != nil {
						return err
					}
					runtimeFlag.Changed = true
					logrus.Debugf("Checkpoint was created using '%s'. Restore will use the same runtime", *runtime)
				} else if podmanConfig.RuntimePath != *runtime {
					// If the user selected a runtime on the command-line this checks if
					// it is the same then during checkpointing and errors out if not.
					return fmt.Errorf(
						"checkpoint archive %s was created with runtime '%s' and cannot be restored with runtime '%s'",
						cmd.Flag("import").Value.String(),
						*runtime,
						podmanConfig.RuntimePath,
					)
				}
			}
		}

		if cmd.Flag("cpu-profile").Changed {
			f, err := os.Create(podmanConfig.CPUProfile)
			if err != nil {
				return err
			}
			if err := pprof.StartCPUProfile(f); err != nil {
				return err
			}
		}
		if cmd.Flag("memory-profile").Changed {
			// Same value as the default in github.com/pkg/profile.
			runtime.MemProfileRate = 4096
			if rate := os.Getenv("MemProfileRate"); rate != "" {
				r, err := strconv.Atoi(rate)
				if err != nil {
					return err
				}
				runtime.MemProfileRate = r
			}
		}

		if podmanConfig.MaxWorks <= 0 {
			return fmt.Errorf("maximum workers must be set to a positive number (got %d)", podmanConfig.MaxWorks)
		}
		if err := parallel.SetMaxThreads(uint(podmanConfig.MaxWorks)); err != nil {
			return err
		}
	}

	if err := readRemoteCliFlags(cmd, podmanConfig); err != nil {
		return fmt.Errorf("read cli flags: %w", err)
	}

	// Special case if command is hidden completion command ("__complete","__completeNoDesc")
	// Since __completeNoDesc is an alias the cm.Name is always __complete
	if cmd.Name() == cobra.ShellCompRequestCmd {
		// Parse the cli arguments after the completion cmd (always called as second argument)
		// This ensures that the --url, --identity and --connection flags are properly set
		compCmd, _, err := cmd.Root().Traverse(os.Args[2:])
		if err != nil {
			return err
		}
		// If we don't complete the root cmd hide all root flags
		// so they won't show up in the completions on subcommands.
		if compCmd != compCmd.Root() {
			compCmd.Root().Flags().VisitAll(func(flag *pflag.Flag) {
				flag.Hidden = true
			})
		}
		// No need for further setup the completion logic setups the engines as needed.
		requireCleanup = false
		return nil
	}

	// Prep the engines
	// Container engine MUST be first.
	// We have special handling in there for passing flags to the runtime, that is ignored by the image engine.
	// Which creates a container engine, because there isn't an entrypoint to Libimage that isn't Libpod.
	if _, err := registry.NewContainerEngine(cmd, args); err != nil {
		// Note: this is gross, but it is the hand we are dealt
		if registry.IsRemote() && errors.As(err, &bindings.ConnectError{}) && cmd.Parent() == cmd.Root() {
			switch cmd.Name() {
			case "info":
				clientDesc, err := getClientInfo()
				// we eat the error here. if this fails, they just don't any client info
				if err == nil {
					b, _ := yaml.Marshal(clientDesc)
					fmt.Println(string(b))
				}
			case "version":
				versions, err := define.GetVersion()
				if err == nil {
					if printErr := system.PrintVersion(cmd, &entities.SystemVersionReport{Client: &versions}); printErr != nil {
						logrus.Debugf("Failed to print client version information: %v", printErr)
					}
				}
			}
		}
		return err
	}
	if _, err := registry.NewImageEngine(cmd, args); err != nil {
		return err
	}

	// Hard code TMPDIR functions to use /var/tmp, if user did not override
	if _, ok := os.LookupEnv("TMPDIR"); !ok {
		if tmpdir, err := podmanConfig.ContainersConfDefaultsRO.ImageCopyTmpDir(); err != nil {
			logrus.Warnf("Failed to retrieve default tmp dir: %s", err.Error())
		} else {
			os.Setenv("TMPDIR", tmpdir)
		}
	}

	// Setup Rootless environment, IFF:
	// 1) in ABI mode
	// 2) running as non-root
	// 3) command doesn't require Parent Namespace
	_, found := cmd.Annotations[registry.ParentNSRequired]
	if !registry.IsRemote() && !found {
		cgroupMode := ""
		_, noMoveProcess := cmd.Annotations[registry.NoMoveProcess]
		if flag := cmd.LocalFlags().Lookup("cgroups"); flag != nil {
			cgroupMode = flag.Value.String()
		}
		err := registry.ContainerEngine().SetupRootless(registry.Context(), noMoveProcess, cgroupMode)
		if err != nil {
			return err
		}
	}
	return nil
}

func persistentPostRunE(cmd *cobra.Command, _ []string) error {
	logrus.Debugf("Called %s.PersistentPostRunE(%s)", cmd.Name(), strings.Join(os.Args, " "))

	if registry.IsRemote() {
		return nil
	}

	// CPU and memory profiling.
	if cmd.Flag("cpu-profile").Changed {
		pprof.StopCPUProfile()
	}
	if cmd.Flag("memory-profile").Changed {
		f, err := os.Create(registry.PodmanConfig().MemoryProfile)
		if err != nil {
			return fmt.Errorf("creating memory profile: %w", err)
		}
		defer f.Close()
		runtime.GC() // get up-to-date GC statistics
		if err := pprof.WriteHeapProfile(f); err != nil {
			return fmt.Errorf("writing memory profile: %w", err)
		}
	}

	return nil
}

func configHook() {
	if dockerConfig != "" {
		// NOTE: we dont allow pointing --config to a regular file. Code assumed config is a directory
		// We do allow though pointing at a nonexistent path. Some downstream code will create the folder
		// at runtime if it does not yet exist.
		statInfo, err := os.Stat(dockerConfig)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			// Cases where the folder does not exist are allowed, BUT cases where some other Stat() error
			// is returned should fail
			fmt.Fprintf(os.Stderr, "Supplied --config folder (%s) exists but is not accessible: %s\n", dockerConfig, err.Error())
			os.Exit(1)
		}
		if err == nil && !statInfo.IsDir() {
			// Cases where it does exist but is a file should fail
			fmt.Fprintf(os.Stderr, "Supplied --config file (%s) is not a directory\n", dockerConfig)
			os.Exit(1)
		}
		if err := os.Setenv("DOCKER_CONFIG", dockerConfig); err != nil {
			fmt.Fprintf(os.Stderr, "cannot set DOCKER_CONFIG=%s: %s\n", dockerConfig, err.Error())
			os.Exit(1)
		}
	}
}

func loggingHook() {
	if debug {
		if logLevel != defaultLogLevel {
			fmt.Fprintf(os.Stderr, "Setting --log-level and --debug is not allowed\n")
			os.Exit(1)
		}
		logLevel = "debug"
	}

	if !slices.Contains(common.LogLevels, strings.ToLower(logLevel)) {
		fmt.Fprintf(os.Stderr, "Log Level %q is not supported, choose from: %s\n", logLevel, strings.Join(common.LogLevels, ", "))
		os.Exit(1)
	}

	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		fmt.Fprint(os.Stderr, err.Error())
		os.Exit(1)
	}
	logrus.SetLevel(level)

	if logrus.IsLevelEnabled(logrus.InfoLevel) {
		logrus.Infof("%s filtering at log level %s", os.Args[0], logrus.GetLevel())
	}
}

// used for capturing podman's formatted output to some file as per the -out and -noout flags.
func stdOutHook() {
	// if noStdOut was specified, then assign /dev/null as the standard file for output.
	if noStdout {
		useStdout = os.DevNull
	}
	// if we were given a filename for output, then open that and use it. we end up leaking
	// the file since it's intended to be in scope as long as our process is running.
	if useStdout != "" {
		if fd, err := os.OpenFile(useStdout, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.ModePerm); err == nil {
			os.Stdout = fd

			// if we couldn't open the file for write, then just bail with an error.
		} else {
			fmt.Fprintf(os.Stderr, "unable to open file for standard output: %s\n", err.Error())
			os.Exit(1)
		}
	}
}

func rootFlags(cmd *cobra.Command, podmanConfig *entities.PodmanConfig) {
	connectionName := setupRemoteConnection(podmanConfig)

	lFlags := cmd.Flags()

	sshFlagName := "ssh"
	lFlags.StringVar(&podmanConfig.SSHMode, sshFlagName, string(ssh.GolangMode), "define the ssh mode")
	_ = cmd.RegisterFlagCompletionFunc(sshFlagName, common.AutocompleteSSH)

	connectionFlagName := "connection"
	lFlags.StringP(connectionFlagName, "c", connectionName, "Connection to use for remote Podman service (CONTAINER_CONNECTION)")
	_ = cmd.RegisterFlagCompletionFunc(connectionFlagName, common.AutocompleteSystemConnections)

	urlFlagName := "url"
	lFlags.StringVar(&podmanConfig.URI, urlFlagName, podmanConfig.URI, "URL to access Podman service (CONTAINER_HOST)")
	_ = cmd.RegisterFlagCompletionFunc(urlFlagName, completion.AutocompleteDefault)
	lFlags.StringVarP(&podmanConfig.URI, "host", "H", podmanConfig.URI, "Used for Docker compatibility")
	_ = lFlags.MarkHidden("host")

	configFlagName := "config"
	lFlags.StringVar(&dockerConfig, "config", "", "Path to directory containing authentication config file")
	_ = cmd.RegisterFlagCompletionFunc(configFlagName, completion.AutocompleteDefault)

	// Context option added just for compatibility with DockerCLI.
	lFlags.String("context", "default", "Name of the context to use to connect to the daemon (This flag is a NOOP and provided solely for scripting compatibility.)")
	_ = lFlags.MarkHidden("context")

	identityFlagName := "identity"
	lFlags.StringVar(&podmanConfig.Identity, identityFlagName, podmanConfig.Identity, "path to SSH identity file, (CONTAINER_SSHKEY)")
	_ = cmd.RegisterFlagCompletionFunc(identityFlagName, completion.AutocompleteDefault)

	tlsCertFileFlagName := "tls-cert"
	lFlags.StringVar(&podmanConfig.TLSCertFile, tlsCertFileFlagName, podmanConfig.TLSCertFile, "path to TLS client certificate PEM file for remote.")
	_ = cmd.RegisterFlagCompletionFunc(tlsCertFileFlagName, completion.AutocompleteDefault)

	tlsKeyFileFlagName := "tls-key"
	lFlags.StringVar(&podmanConfig.TLSKeyFile, tlsKeyFileFlagName, podmanConfig.TLSKeyFile, "path to TLS client certificate private key PEM file for remote.")
	_ = cmd.RegisterFlagCompletionFunc(tlsKeyFileFlagName, completion.AutocompleteDefault)

	tlsCAFileFlagName := "tls-ca"
	lFlags.StringVar(&podmanConfig.TLSCAFile, tlsCAFileFlagName, podmanConfig.TLSCAFile, "path to TLS certificate Authority PEM file for remote.")
	_ = cmd.RegisterFlagCompletionFunc(tlsCAFileFlagName, completion.AutocompleteDefault)

	tlsDetailsFlagName := "tls-details"
	lFlags.StringVar(&podmanConfig.TLSDetailsFile, tlsDetailsFlagName, "", "Path to a containers-tls-details.yaml(5) file")
	_ = cmd.RegisterFlagCompletionFunc(tlsDetailsFlagName, completion.AutocompleteDefault)

	// Flags that control or influence any kind of output.
	outFlagName := "out"
	lFlags.StringVar(&useStdout, outFlagName, "", "Send output (stdout) from podman to a file")
	_ = cmd.RegisterFlagCompletionFunc(outFlagName, completion.AutocompleteDefault)

	lFlags.BoolVar(&noStdout, "noout", false, "do not output to stdout")
	_ = lFlags.MarkHidden("noout") // Superseded by --out

	lFlags.BoolVarP(&podmanConfig.Remote, "remote", "r", registry.IsRemote(), "Access remote Podman service")
	pFlags := cmd.PersistentFlags()
	if registry.IsRemote() {
		if err := lFlags.MarkHidden("remote"); err != nil {
			logrus.Warnf("Unable to mark --remote flag as hidden: %s", err.Error())
		}
		podmanConfig.Remote = true
	} else {
		// The --module's are actually used and parsed in
		// `registry.PodmanConfig()`.  But we also need to expose them
		// as a flag here to a) make sure that rootflags are aware of
		// this flag and b) to have shell completions.
		moduleFlagName := "module"
		lFlags.StringArray(moduleFlagName, nil, "Load the containers.conf(5) module")
		_ = cmd.RegisterFlagCompletionFunc(moduleFlagName, common.AutocompleteContainersConfModules)

		cgroupManagerFlagName := "cgroup-manager"
		pFlags.StringVar(&podmanConfig.ContainersConf.Engine.CgroupManager, cgroupManagerFlagName, podmanConfig.ContainersConfDefaultsRO.Engine.CgroupManager, "Cgroup manager to use (\"cgroupfs\"|\"systemd\")")
		_ = cmd.RegisterFlagCompletionFunc(cgroupManagerFlagName, common.AutocompleteCgroupManager)

		pFlags.StringVar(&podmanConfig.CPUProfile, "cpu-profile", "", "Path for the cpu-profiling results")
		pFlags.StringVar(&podmanConfig.MemoryProfile, "memory-profile", "", "Path for the memory-profiling results")

		conmonFlagName := "conmon"
		pFlags.StringVar(&podmanConfig.ConmonPath, conmonFlagName, "", "Path of the conmon binary")
		_ = cmd.RegisterFlagCompletionFunc(conmonFlagName, completion.AutocompleteDefault)

		networkConfigDirFlagName := "network-config-dir"
		pFlags.StringVar(&podmanConfig.ContainersConf.Network.NetworkConfigDir, networkConfigDirFlagName, podmanConfig.ContainersConfDefaultsRO.Network.NetworkConfigDir, "Path of the configuration directory for networks")
		_ = cmd.RegisterFlagCompletionFunc(networkConfigDirFlagName, completion.AutocompleteDefault)

		pFlags.StringVar(&podmanConfig.ContainersConf.Containers.DefaultMountsFile, "default-mounts-file", podmanConfig.ContainersConfDefaultsRO.Containers.DefaultMountsFile, "Path to default mounts file")

		eventsBackendFlagName := "events-backend"
		pFlags.StringVar(&podmanConfig.ContainersConf.Engine.EventsLogger, eventsBackendFlagName, podmanConfig.ContainersConfDefaultsRO.Engine.EventsLogger, `Events backend to use ("file"|"journald"|"none")`)
		_ = cmd.RegisterFlagCompletionFunc(eventsBackendFlagName, common.AutocompleteEventBackend)

		hooksDirFlagName := "hooks-dir"
		pFlags.StringArrayVar(&podmanConfig.HooksDir, hooksDirFlagName, podmanConfig.ContainersConfDefaultsRO.Engine.HooksDir.Get(), "Set the OCI hooks directory path (may be set multiple times)")
		_ = cmd.RegisterFlagCompletionFunc(hooksDirFlagName, completion.AutocompleteDefault)

		cdiSpecDirFlagName := "cdi-spec-dir"
		pFlags.StringArrayVar(&podmanConfig.CdiSpecDirs, cdiSpecDirFlagName, podmanConfig.ContainersConfDefaultsRO.Engine.CdiSpecDirs.Get(), "Set the CDI spec directory path (may be set multiple times)")
		_ = cmd.RegisterFlagCompletionFunc(cdiSpecDirFlagName, completion.AutocompleteDefault)

		pFlags.IntVar(&podmanConfig.MaxWorks, "max-workers", (runtime.NumCPU()*3)+1, "The maximum number of workers for parallel operations")

		namespaceFlagName := "namespace"
		pFlags.StringVar(&podmanConfig.ContainersConf.Engine.Namespace, namespaceFlagName, podmanConfig.ContainersConfDefaultsRO.Engine.Namespace, "Set the libpod namespace, used to create separate views of the containers and pods on the system")
		_ = cmd.RegisterFlagCompletionFunc(namespaceFlagName, completion.AutocompleteNone)
		_ = pFlags.MarkHidden(namespaceFlagName)

		// Keep network-backend flag as hidden for backward compatibility with cleanup commands from 5.x containers
		networkBackendFlagName := "network-backend"
		var networkBackendDeprecated string
		pFlags.StringVar(&networkBackendDeprecated, networkBackendFlagName, "", "Deprecated: Network backend (flag kept for backward compatibility)")
		_ = cmd.RegisterFlagCompletionFunc(networkBackendFlagName, completion.AutocompleteNone)
		_ = pFlags.MarkHidden(networkBackendFlagName)

		rootFlagName := "root"
		pFlags.StringVar(&podmanConfig.GraphRoot, rootFlagName, "", "Path to the graph root directory where images, containers, etc. are stored")
		_ = cmd.RegisterFlagCompletionFunc(rootFlagName, completion.AutocompleteDefault)

		pFlags.StringVar(&podmanConfig.RegistriesConf, "registries-conf", "", "Path to a registries.conf to use for image processing")

		runrootFlagName := "runroot"
		pFlags.StringVar(&podmanConfig.Runroot, runrootFlagName, "", "Path to the 'run directory' where all state information is stored")
		_ = cmd.RegisterFlagCompletionFunc(runrootFlagName, completion.AutocompleteDefault)

		imageStoreFlagName := "imagestore"
		pFlags.StringVar(&podmanConfig.ImageStore, imageStoreFlagName, "", "Path to the 'image store', different from 'graph root', use this to split storing the image into a separate 'image store', see 'man containers-storage.conf' for details")
		_ = cmd.RegisterFlagCompletionFunc(imageStoreFlagName, completion.AutocompleteDefault)

		pFlags.BoolVar(&podmanConfig.TransientStore, "transient-store", false, "Enable transient container storage")

		pFlags.StringArrayVar(&podmanConfig.PullOptions, "pull-option", nil, "Specify an option to change how the image is pulled")

		runtimeFlagName := "runtime"
		pFlags.StringVar(&podmanConfig.RuntimePath, runtimeFlagName, podmanConfig.ContainersConfDefaultsRO.Engine.OCIRuntime, "Path to the OCI-compatible binary used to run containers.")
		_ = cmd.RegisterFlagCompletionFunc(runtimeFlagName, completion.AutocompleteDefault)

		// -s is deprecated due to conflict with -s on subcommands
		storageDriverFlagName := "storage-driver"
		pFlags.StringVar(&podmanConfig.StorageDriver, storageDriverFlagName, "", "Select which storage driver is used to manage storage of images and containers")
		_ = cmd.RegisterFlagCompletionFunc(storageDriverFlagName, completion.AutocompleteNone)

		tmpdirFlagName := "tmpdir"
		pFlags.StringVar(&podmanConfig.ContainersConf.Engine.TmpDir, tmpdirFlagName, podmanConfig.ContainersConfDefaultsRO.Engine.TmpDir, "Path to the tmp directory for libpod state content.\n\nNote: use the environment variable 'TMPDIR' to change the temporary storage location for container images, '/var/tmp'.\n")
		_ = cmd.RegisterFlagCompletionFunc(tmpdirFlagName, completion.AutocompleteDefault)

		pFlags.BoolVar(&podmanConfig.Trace, "trace", false, "Enable opentracing output (default false)")

		volumePathFlagName := "volumepath"
		pFlags.StringVar(&podmanConfig.ContainersConf.Engine.VolumePath, volumePathFlagName, podmanConfig.ContainersConfDefaultsRO.Engine.VolumePath, "Path to the volume directory in which volume data is stored")
		_ = cmd.RegisterFlagCompletionFunc(volumePathFlagName, completion.AutocompleteDefault)

		// Hide these flags for both ABI and Tunneling
		for _, f := range []string{
			"cpu-profile",
			"default-mounts-file",
			"max-workers",
			"memory-profile",
			"pull-option",
			"registries-conf",
			"trace",
		} {
			if err := pFlags.MarkHidden(f); err != nil {
				logrus.Warnf("Unable to mark %s flag as hidden: %s", f, err.Error())
			}
		}
	}
	storageOptFlagName := "storage-opt"
	pFlags.StringArrayVar(&podmanConfig.StorageOpts, storageOptFlagName, []string{}, "Used to pass an option to the storage driver")
	_ = cmd.RegisterFlagCompletionFunc(storageOptFlagName, completion.AutocompleteNone)

	// Override default --help information of `--help` global flag
	var dummyHelp bool
	pFlags.BoolVar(&dummyHelp, "help", false, "Help for podman")

	logLevelFlagName := "log-level"
	pFlags.StringVar(&logLevel, logLevelFlagName, logLevel, fmt.Sprintf("Log messages above specified level (%s)", strings.Join(common.LogLevels, ", ")))
	_ = rootCmd.RegisterFlagCompletionFunc(logLevelFlagName, common.AutocompleteLogLevel)

	lFlags.BoolVarP(&debug, "debug", "D", false, "Docker compatibility, force setting of log-level")
	_ = lFlags.MarkHidden("debug")

	// Only create these flags for ABI connections
	if !registry.IsRemote() {
		runtimeflagFlagName := "runtime-flag"
		pFlags.StringArrayVar(&podmanConfig.RuntimeFlags, runtimeflagFlagName, []string{}, "add global flags for the container runtime")
		_ = rootCmd.RegisterFlagCompletionFunc(runtimeflagFlagName, completion.AutocompleteNone)

		pFlags.BoolVar(&podmanConfig.Syslog, "syslog", false, "Output podman-internal logs to syslog as well as the console (default false)")
	}
}

func formatError(err error, cmd *cobra.Command) string {
	var message string
	switch {
	case errors.Is(err, define.ErrOCIRuntime):
		// OCIRuntimeErrors include the reason for the failure in the
		// second to last message in the error chain.
		message = fmt.Sprintf(
			"Error: %s: %s",
			define.ErrOCIRuntime.Error(),
			strings.TrimSuffix(err.Error(), ": "+define.ErrOCIRuntime.Error()),
		)
	case errors.Is(err, storage.ErrDuplicateName) && cmd != nil && cmd.Flags().Lookup("replace") != nil:
		// Only suggest --replace when the invoked command actually has
		// the flag (e.g. "manifest create" does not).
		message = fmt.Sprintf("Error: %s, or use --replace to instruct Podman to do so.", err.Error())
	default:
		if logrus.IsLevelEnabled(logrus.TraceLevel) {
			message = fmt.Sprintf("Error: %+v", err)
		} else {
			message = fmt.Sprintf("Error: %v", err)
		}
	}
	return message
}
