//go:build darwin

package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

const (
	defaultPrefix = "/usr/local"
	dockerSock    = "/var/run/docker.sock"
)

var installPrefix string

var rootCmd = &cobra.Command{
	Use:               "podman-mac-helper",
	Short:             "A system helper to manage docker.sock",
	Long:              `podman-mac-helper is a system helper service and tool for managing docker.sock `,
	CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true},
	SilenceErrors:     true,
}

// Note, this code is security sensitive since it runs under privilege.
// Limit actions to what is strictly necessary, and take appropriate
// safeguards
//
// After installation the service call is ran under launchd in a nowait
// inetd style fashion, so stdin, stdout, and stderr are all pointing to
// an accepted connection
//
// This service is installed once per user and will redirect
// /var/run/docker to the fixed user-assigned unix socket location.
//
// Control communication is restricted to each user specific service via
// unix file permissions

func main() {
	if os.Geteuid() != 0 {
		fmt.Printf("This command must be run as root via sudo or as a script\n")
		os.Exit(1)
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
		os.Exit(1)
	}
}

func getUserInfo(name string) (string, string, string, error) {
	// We exec id instead of using user.Lookup to remain compat
	// with CGO disabled.
	cmd := exec.Command("/usr/bin/id", "-P", name)
	output, err := cmd.StdoutPipe()
	if err != nil {
		return "", "", "", err
	}

	if err := cmd.Start(); err != nil {
		return "", "", "", err
	}

	entry := readCapped(output)
	elements := strings.Split(entry, ":")
	if len(elements) < 9 || elements[0] != name {
		return "", "", "", errors.New("could not look up user")
	}

	return elements[0], elements[2], elements[8], nil
}

func getUser() (string, string, string, error) {
	name, found := os.LookupEnv("SUDO_USER")
	if !found {
		name, found = os.LookupEnv("USER")
		if !found {
			return "", "", "", errors.New("could not determine user")
		}
	}

	_, uid, home, err := getUserInfo(name)
	if err != nil {
		return "", "", "", fmt.Errorf("could not look up user: %s", name)
	}
	id, err := strconv.Atoi(uid)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid uid for user: %s", name)
	}
	if id == 0 {
		return "", "", "", errors.New("unexpected root user")
	}

	return name, uid, home, nil
}

// Used for commands that don't return a proper exit code
func runDetectErr(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	errReader, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	err = cmd.Start()
	if err == nil {
		errString := readCapped(errReader)
		if len(errString) > 0 {
			re := regexp.MustCompile(`\r?\n`)
			err = errors.New(re.ReplaceAllString(errString, ": "))
		}
	}

	if werr := cmd.Wait(); werr != nil {
		err = werr
	}

	return err
}

func readCapped(reader io.Reader) string {
	// Cap output
	buffer := make([]byte, 2048)
	n, _ := io.ReadFull(reader, buffer)
	_, _ = io.Copy(io.Discard, reader)
	if n > 0 {
		return string(buffer[:n])
	}

	return ""
}

func addPrefixFlag(cmd *cobra.Command) {
	cmd.Flags().StringVar(&installPrefix, "prefix", defaultPrefix, "Sets the install location prefix")
}

func silentUsage(cmd *cobra.Command, _ []string) {
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
}
