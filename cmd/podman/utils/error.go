package utils

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	buildahCLI "go.podman.io/buildah/pkg/cli"
	"go.podman.io/podman/v6/cmd/podman/registry"
)

type OutputErrors []error

func (o OutputErrors) PrintErrors() (lastError error) {
	if len(o) == 0 {
		return lastError
	}
	lastError = o[len(o)-1]
	for e := 0; e < len(o)-1; e++ {
		fmt.Fprintf(os.Stderr, "Error: %s\n", o[e])
	}
	return lastError
}

/*
For remote client, server does not return error with exit code

	instead returns a message and we cast it to a new error.

	Following function performs parsing on build error and returns
	exit status which was expected for this current build
*/
func ExitCodeFromBuildError(errorMsg string) (int, error) {
	if strings.Contains(errorMsg, "exit status") {
		errorSplit := strings.Split(errorMsg, " ")
		if errorSplit[len(errorSplit)-2] == "status" {
			tmpSplit := strings.Split(errorSplit[len(errorSplit)-1], "\n")
			exitCodeRemote, err := strconv.Atoi(tmpSplit[0])
			if err == nil {
				return exitCodeRemote, nil
			}
			return buildahCLI.ExecErrorCodeGeneric, err
		}
	}
	return buildahCLI.ExecErrorCodeGeneric, errors.New("message does not contain a valid exit code")
}

// HandleOSExecError checks the given error for an exec.ExitError error and
// sets the same podman exit code as the error.
// No error will be returned in this case to make sure things like podman
// unshare false work correctly without extra output.
// When the exec file does not exist we set the exit code to 127, for
// permission errors 126 is used as exit code. In this case we still return
// the error so the user gets an error message.
// If the error is nil it returns nil.
func HandleOSExecError(err error) error {
	if err == nil {
		return nil
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		// the user command inside the unshare/ssh env has failed
		// we set the exit code, do not return the error to the user
		// otherwise "exit status X" will be printed
		registry.SetExitCode(exitError.ExitCode())
		return nil
	}
	// cmd.Run() can return fs.ErrNotExist, fs.ErrPermission or exec.ErrNotFound
	// follow podman run/exec standard with the exit codes
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, exec.ErrNotFound) {
		registry.SetExitCode(127)
	} else if errors.Is(err, os.ErrPermission) {
		registry.SetExitCode(126)
	}
	return err
}
