//go:build darwin

package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
	"go.podman.io/storage/pkg/fileutils"
)

var uninstallCmd = &cobra.Command{
	Use:    "uninstall",
	Short:  "Uninstall the podman helper agent",
	Long:   "Uninstall the podman helper agent, which manages the /var/run/docker.sock link",
	PreRun: silentUsage,
	RunE:   uninstall,
}

func init() {
	addPrefixFlag(uninstallCmd)
	rootCmd.AddCommand(uninstallCmd)
}

func uninstall(_ *cobra.Command, _ []string) error {
	userName, _, homeDir, err := getUser()
	if err != nil {
		return err
	}

	labelName := fmt.Sprintf("com.github.containers.podman.helper-%s", userName)
	fileName := filepath.Join("/Library", "LaunchDaemons", labelName+".plist")

	if err = runDetectErr("launchctl", "unload", fileName); err != nil {
		// Try removing the service by label in case the service is half uninstalled
		if rerr := runDetectErr("launchctl", "remove", labelName); rerr != nil {
			// Exit code 3 = no service to remove
			if exitErr, ok := rerr.(*exec.ExitError); !ok || exitErr.ExitCode() != 3 {
				fmt.Fprintf(os.Stderr, "Warning: service unloading failed: %s\n", err.Error())
				fmt.Fprintf(os.Stderr, "Warning: remove also failed: %s\n", rerr.Error())
			}
		}
	}

	if err := os.Remove(fileName); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("could not remove plist file: %s", fileName)
		}
	}

	helperPath := filepath.Join(installPrefix, "podman", "helper", userName)
	if err := os.RemoveAll(helperPath); err != nil {
		return fmt.Errorf("could not remove helper binary path: %s", helperPath)
	}

	// Get the file information of dockerSock
	if err := fileutils.Lexists(dockerSock); err != nil {
		// If the error is due to the file not existing, return nil
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		// Return an error if unable to get the file information
		return fmt.Errorf("could not stat dockerSock: %v", err)
	}
	if target, err := os.Readlink(dockerSock); err != nil {
		// Return an error if unable to read the symlink
		return fmt.Errorf("could not read dockerSock symlink: %v", err)
	} else {
		// Check if the target of the symlink matches the expected target
		expectedTarget := filepath.Join(homeDir, ".local", "share", "containers", "podman", "machine", "podman.sock")
		if target != expectedTarget {
			// If the targets do not match, print the information and return with nothing left to do
			fmt.Printf("dockerSock does not point to the expected target: %v\n", target)
			return nil
		}

		// Attempt to remove dockerSock
		if err := os.Remove(dockerSock); err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf("could not remove dockerSock file: %s", err)
			}
		}
	}

	return nil
}
