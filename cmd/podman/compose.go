package main

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"text/template"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/pkg/errorhandling"
)

var composeCommand = &cobra.Command{
	Use:   "compose [options]",
	Short: "Run compose workloads via an external provider such as docker-compose or podman-compose",
	Long: `This command is a thin wrapper around an external compose provider such as docker-compose or podman-compose.  This means that podman compose is executing another tool that implements the compose functionality but sets up the environment in a way to let the compose provider communicate transparently with the local Podman socket.  The specified options as well the command and argument are passed directly to the compose provider.

The default compose providers are docker-compose and podman-compose.  If installed, docker-compose takes precedence since it is the original implementation of the Compose specification and is widely used on the supported platforms (i.e., Linux, Mac OS, Windows).

If you want to change the default behavior or have a custom installation path for your provider of choice, please change the compose_providers field in containers.conf(5) to compose_providers = ["/path/to/provider"]. You may also set the PODMAN_COMPOSE_PROVIDER environment variable.`,
	RunE:              composeMain,
	ValidArgsFunction: composeCompletion,
	Example: `podman compose -f nginx.yaml up --detach
podman --log-level=debug compose -f many-images.yaml pull`,
	DisableFlagParsing: true,
	Annotations:        map[string]string{registry.ParentNSRequired: ""}, // don't join user NS for SSH to work correctly
}

func init() {
	// NOTE: we need to fully disable flag parsing and manually parse the
	// flags in composeMain. cobra's FParseErrWhitelist will strip off
	// unknown flags _before_ the first argument.  So `--unknown argument`
	// will show as `argument`.

	registry.Commands = append(registry.Commands, registry.CliCommand{Command: composeCommand})
}

func composeCompletion(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	var stdout strings.Builder

	args = append(args, toComplete)
	args = append([]string{"__complete"}, args...)
	if err := composeProviderExec(args, &stdout, io.Discard, false); err != nil {
		// Ignore errors since some providers may not expose a __complete command.
		return nil, cobra.ShellCompDirectiveError
	}

	var num int
	output := strings.Split(strings.TrimRight(stdout.String(), "\n"), "\n")
	if len(output) >= 1 {
		if lastLine := output[len(output)-1]; strings.HasPrefix(lastLine, ":") {
			var err error
			if num, err = strconv.Atoi(lastLine[1:]); err != nil {
				return nil, cobra.ShellCompDirectiveError
			}
			output = output[:len(output)-1]
		}
	}
	return output, cobra.ShellCompDirective(num)
}

// composeProvider provides the name of or absolute path to the compose
// provider (i.e., the external binary such as docker-compose).
func composeProvider() (string, error) {
	if value, ok := os.LookupEnv("PODMAN_COMPOSE_PROVIDER"); ok {
		return value, nil
	}

	candidates := registry.PodmanConfig().ContainersConfDefaultsRO.Engine.ComposeProviders.Get()
	if len(candidates) == 0 {
		return "", errors.New("no compose provider specified, please refer to `man podman-compose` for details")
	}

	lookupErrors := make([]error, 0, len(candidates))
	for _, candidate := range candidates {
		path, err := exec.LookPath(os.ExpandEnv(candidate))
		if err == nil {
			// First specified provider "candidate" wins.
			logrus.Debugf("Found compose provider %q", path)
			return path, nil
		}
		logrus.Debugf("Error looking up compose provider %q: %v", candidate, err)
		lookupErrors = append(lookupErrors, err)
	}

	return "", fmt.Errorf("looking up compose provider failed\n%v", errorhandling.JoinErrors(lookupErrors))
}

// composeDockerHost returns the value to be set in the DOCKER_HOST environment
// variable.
func composeDockerHost() (string, error) {
	if value, ok := os.LookupEnv("DOCKER_HOST"); ok {
		return value, nil
	}

	// For local clients (Linux/FreeBSD), use the default API
	// address.
	if !registry.IsRemote() {
		return registry.DefaultAPIAddress(), nil
	}

	conf := registry.PodmanConfig()
	if conf.URI == "" {
		switch runtime.GOOS {
		// If no default connection is set on Linux or FreeBSD,
		// we just use the local socket by default - just as
		// the remote client does.
		case "linux", "freebsd":
			return registry.DefaultAPIAddress(), nil
		// If there is no default connection on Windows or Mac
		// OS, we can safely assume that something went wrong.
		// A `podman machine init` will set the connection.
		default:
			return "", fmt.Errorf("cannot connect to a socket or via SSH: no default connection found: consider running `podman machine init`")
		}
	}

	parsedConnection, err := url.Parse(conf.URI)
	if err != nil {
		return "", fmt.Errorf("preparing connection to remote machine: %w", err)
	}

	// If the default connection does not point to a `podman
	// machine`, we cannot use a local path and need to use SSH.
	if !conf.MachineMode {
		// Docker Compose v1 doesn't like paths for ssh, so we optimistically
		// assume the presence of a Docker socket on the remote
		// machine which is the case for podman machines.
		if parsedConnection.Scheme == "ssh" {
			return strings.TrimSuffix(conf.URI, parsedConnection.Path), nil
		}
		return conf.URI, nil
	}
	uri, err := getMachineConn(conf.URI, parsedConnection)
	if err != nil {
		return "", fmt.Errorf("get machine connection URI: %w", err)
	}
	return uri, nil
}

// composeEnv returns the compose-specific environment variables.
func composeEnv() ([]string, error) {
	hostValue, err := composeDockerHost()
	if err != nil {
		return nil, err
	}

	return []string{
		"DOCKER_HOST=" + hostValue,
		// Podman doesn't support all buildkit features and since it's
		// a continuous catch-up game, disable buildkit on the client
		// side.
		//
		// See https://github.com/containers/podman/issues/18617#issuecomment-1600495841
		"DOCKER_BUILDKIT=0",
		// FIXME: DOCKER_CONFIG is limited by containers/podman/issues/18617
		//        and it remains unclear which default path should be set
		//        w.r.t. Docker compatibility and a smooth experience of podman-login
		//        working with podman-compose _by default_.
		"DOCKER_CONFIG=" + os.Getenv("DOCKER_CONFIG"),
	}, nil
}

// composeShouldLogWarning returns whether a notice on engine redirection should be piped to stderr regardless of logging configuration
func composeShouldLogWarning() (bool, error) {
	if shouldWarnLogsEnv, ok := os.LookupEnv("PODMAN_COMPOSE_WARNING_LOGS"); ok {
		if shouldWarnLogsEnvVal, err := strconv.ParseBool(shouldWarnLogsEnv); err == nil {
			return shouldWarnLogsEnvVal, nil
		} else if shouldWarnLogsEnv != "" {
			return true, fmt.Errorf("PODMAN_COMPOSE_WARNING_LOGS should be a boolean: %w", err)
		}
	}
	return registry.PodmanConfig().ContainersConfDefaultsRO.Engine.ComposeWarningLogs, nil
}

// underline uses ANSI codes to underline the specified string.
func underline(str string) string {
	return "\033[4m" + str + "\033[0m"
}

// composeProviderExec executes the compose provider with the specified arguments.
func composeProviderExec(args []string, stdout io.Writer, stderr io.Writer, warn bool) error {
	provider, err := composeProvider()
	if err != nil {
		return err
	}

	env, err := composeEnv()
	if err != nil {
		return err
	}

	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}

	cmd := exec.Command(provider, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Env = append(os.Environ(), env...)
	logrus.Debugf("Executing compose provider (%s %s) with additional env %s", provider, strings.Join(args, " "), strings.Join(env, " "))

	if warn {
		fmt.Fprint(os.Stderr, underline(fmt.Sprintf(">>>> Executing external compose provider %q. Please see podman-compose(1) for how to disable this message. <<<<\n\n", provider)))
	}

	if err := cmd.Run(); err != nil {
		// Make sure podman returns with the same exit code as the compose provider.
		if exitErr, isExit := err.(*exec.ExitError); isExit {
			registry.SetExitCode(exitErr.ExitCode())
		}
		// Format the error to make it explicit that error did not come
		// from podman but from the executed compose provider.
		return fmt.Errorf("executing %s %s: %w", provider, strings.Join(args, " "), err)
	}

	return nil
}

// composeHelp is a custom help function to display the help message of the
// configured compose-provider.
func composeHelp(cmd *cobra.Command) error {
	tmpl, err := template.New("help_template").Parse(helpTemplate)
	if err != nil {
		return err
	}
	if err := tmpl.Execute(os.Stdout, cmd); err != nil {
		return err
	}

	shouldLog, err := composeShouldLogWarning()
	if err != nil {
		return err
	}
	return composeProviderExec([]string{"--help"}, nil, nil, shouldLog)
}

// composeMain is the main function of the compose command.
func composeMain(cmd *cobra.Command, args []string) error {
	// We have to manually parse the flags here to make sure all arguments
	// after `podman compose [ARGS]` are passed to the compose provider.
	// For now, we only look for the --help flag.
	fs := pflag.NewFlagSet("args", pflag.ContinueOnError)
	fs.ParseErrorsAllowlist.UnknownFlags = true
	fs.SetInterspersed(false)
	fs.BoolP("help", "h", false, "")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parsing arguments: %w", err)
	}

	if len(args) == 0 || fs.Lookup("help").Changed {
		return composeHelp(cmd)
	}

	shouldLog, err := composeShouldLogWarning()
	if err != nil {
		return err
	}
	return composeProviderExec(args, nil, nil, shouldLog)
}
