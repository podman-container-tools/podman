package mkcw

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/sirupsen/logrus"
)

// MakeFS formats the imageFile as a filesystem of the specified type,
// populating it with the contents of the directory at sourcePath.
// Recognized filesystem types are "btrfs", "ext2", "ext3", "ext4", and "xfs".
// Note that krun's init is currently hard-wired to assume "ext4".
// Returns the stdout, stderr, and any error returned by the mkfs command.
func MakeFS(ctx context.Context, sourcePath, imageFile, filesystem string) (string, string, error) {
	select {
	case <-ctx.Done():
		return "", "", ctx.Err()
	default:
	}

	var stdout, stderr strings.Builder
	switch filesystem {
	case "ext2", "ext3", "ext4":
		logrus.Debugf("mkfs -t %s --rootdir %q %q", filesystem, sourcePath, imageFile)
		cmd := exec.CommandContext(ctx, "mkfs", "-t", filesystem, "-d", sourcePath, imageFile)
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Run()
		return stdout.String(), stderr.String(), err
	case "btrfs":
		logrus.Debugf("mkfs -t %s --rootdir %q %q", filesystem, sourcePath, imageFile)
		cmd := exec.CommandContext(ctx, "mkfs", "-t", filesystem, "--rootdir", sourcePath, imageFile)
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Run()
		return stdout.String(), stderr.String(), err
	case "xfs":
		// N.B. -p treating directories as source for the filesystem contents only
		// available in xfsprogs-6.17.0 or later; before that, it only accepts prototype
		// files
		logrus.Debugf("mkfs -t %s -p %q %q", filesystem, sourcePath, imageFile)
		cmd := exec.CommandContext(ctx, "mkfs", "-t", filesystem, "-p", sourcePath, imageFile)
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Run()
		return stdout.String(), stderr.String(), err
	}
	return "", "", fmt.Errorf("don't know how to make a %q filesystem with contents", filesystem)
}
