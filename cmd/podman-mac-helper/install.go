//go:build darwin

package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"text/template"

	"github.com/spf13/cobra"
	"go.podman.io/storage/pkg/fileutils"
)

const (
	mode755 = 0o755
	mode644 = 0o644
)

const launchConfig = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>com.github.containers.podman.helper-{{.User}}</string>
	<key>ProgramArguments</key>
	<array>
		<string>{{.Program}}</string>
		<string>service</string>
		<string>{{.Target}}</string>
	</array>
	<key>inetdCompatibility</key>
	<dict>
		<key>Wait</key>
		<false/>
	</dict>
	<key>UserName</key>
	<string>root</string>
	<key>Sockets</key>
	<dict>
		<key>Listeners</key>
		<dict>
			<key>SockFamily</key>
			<string>Unix</string>
			<key>SockPathName</key>
			<string>/private/var/run/podman-helper-{{.User}}.socket</string>
			<key>SockPathOwner</key>
			<integer>{{.UID}}</integer>
			<key>SockPathMode</key>
			<!-- SockPathMode takes base 10 (384 = 0600) -->
			<integer>384</integer>
			<key>SockType</key>
			<string>stream</string>
		</dict>
	</dict>
</dict>
</plist>
`

type launchParams struct {
	Program string
	User    string
	UID     string
	Target  string
}

var installCmd = &cobra.Command{
	Use:    "install",
	Short:  "Install the podman helper agent",
	Long:   "Install the podman helper agent, which manages the /var/run/docker.sock link",
	PreRun: silentUsage,
	RunE:   install,
}

func init() {
	addPrefixFlag(installCmd)
	rootCmd.AddCommand(installCmd)
}

func install(_ *cobra.Command, _ []string) error {
	userName, uid, homeDir, err := getUser()
	if err != nil {
		return err
	}

	labelName := fmt.Sprintf("com.github.containers.podman.helper-%s.plist", userName)
	fileName := filepath.Join("/Library", "LaunchDaemons", labelName)

	if err := fileutils.Exists(fileName); err == nil || !errors.Is(err, fs.ErrNotExist) {
		fmt.Fprintln(os.Stderr, "helper is already installed, skipping the install, uninstall first if you want to reinstall")
		return nil
	}

	prog, err := installExecutable(userName)
	if err != nil {
		return err
	}

	target := filepath.Join(homeDir, ".local", "share", "containers", "podman", "machine", "podman.sock")
	var buf bytes.Buffer
	t := template.Must(template.New("launchdConfig").Parse(launchConfig))
	err = t.Execute(&buf, launchParams{prog, userName, uid, target})
	if err != nil {
		return err
	}

	file, err := os.OpenFile(fileName, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode644)
	if err != nil {
		return fmt.Errorf("creating helper plist file: %w", err)
	}
	defer file.Close()
	_, err = buf.WriteTo(file)
	if err != nil {
		return err
	}

	if err = runDetectErr("launchctl", "load", fileName); err != nil {
		return fmt.Errorf("launchctl failed loading service: %w", err)
	}

	return nil
}

func restrictRecursive(targetDir string, until string) error {
	for targetDir != until && len(targetDir) > 1 {
		info, err := os.Lstat(targetDir)
		if err != nil {
			return err
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			return fmt.Errorf("symlinks not allowed in helper paths (remove them and rerun): %s", targetDir)
		}
		if err = os.Chown(targetDir, 0, 0); err != nil {
			return fmt.Errorf("could not update ownership of helper path: %w", err)
		}
		if err = os.Chmod(targetDir, mode755|fs.ModeSticky); err != nil {
			return fmt.Errorf("could not update permissions of helper path: %w", err)
		}
		targetDir = filepath.Dir(targetDir)
	}

	return nil
}

func verifyRootDeep(path string) error {
	path = filepath.Clean(path)
	current := "/"
	segs := strings.Split(path, "/")
	depth := 0
	for i := 1; i < len(segs); i++ {
		seg := segs[i]
		current = filepath.Join(current, seg)
		info, err := os.Lstat(current)
		if err != nil {
			return err
		}

		stat := info.Sys().(*syscall.Stat_t)
		if stat.Uid != 0 {
			return fmt.Errorf("installation target path must be solely owned by root: %s is not", current)
		}

		if info.Mode()&fs.ModeSymlink != 0 {
			target, err := os.Readlink(current)
			if err != nil {
				return err
			}

			targetParts := strings.Split(target, "/")
			segs = append(targetParts, segs[i+1:]...)

			if depth++; depth > 1000 {
				return errors.New("reached max recursion depth, link structure is cyclical or too complex")
			}

			if !filepath.IsAbs(target) {
				current = filepath.Dir(current)
				i = -1 // Start at 0
			} else {
				current = "/"
				i = 0 // Skip empty first segment
			}
		}
	}

	return nil
}

func installExecutable(user string) (string, error) {
	// Since the installed executable runs as root, as a precaution verify root ownership of
	// the entire installation path, and utilize sticky + read-only perms for the helper path
	// suffix. The goal is to help users harden against privilege escalation from loose
	// filesystem permissions.
	//
	// Since userspace package management tools, such as brew, delegate management of system
	// paths to standard unix users, the daemon executable is copied into a separate more
	// restricted area of the filesystem.
	if err := verifyRootDeep(installPrefix); err != nil {
		return "", err
	}

	targetDir := filepath.Join(installPrefix, "podman", "helper", user)
	if err := os.MkdirAll(targetDir, mode755); err != nil {
		return "", fmt.Errorf("could not create helper directory structure: %w", err)
	}

	// Correct any incorrect perms on previously existing directories and verify no symlinks
	if err := restrictRecursive(targetDir, installPrefix); err != nil {
		return "", err
	}

	exec, err := os.Executable()
	if err != nil {
		return "", err
	}
	install := filepath.Join(targetDir, filepath.Base(exec))

	return install, copyFile(install, exec, mode755)
}

func copyFile(dest string, source string, perms fs.FileMode) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}

	defer in.Close()
	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, perms)
	if err != nil {
		return err
	}

	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	return nil
}
