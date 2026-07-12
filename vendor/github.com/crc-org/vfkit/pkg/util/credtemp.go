package util

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/shirou/gopsutil/v4/process"
	"github.com/sirupsen/logrus"
)

const cloudInitPrefix = "vfkit-cloudinit"

var cloudInitTempDir = filepath.Join(os.TempDir(), cloudInitPrefix)

func CreateCloudInitISOFile() (*os.File, error) {
	if err := os.MkdirAll(cloudInitTempDir, 0700); err != nil {
		return nil, fmt.Errorf("unable to create directory %s: %w", cloudInitTempDir, err)
	}
	pid := os.Getpid()
	file, err := os.CreateTemp(cloudInitTempDir, fmt.Sprintf("%s-%d-*.iso", cloudInitPrefix, pid))
	if err != nil {
		return nil, fmt.Errorf("unable to create cloud-init ISO temporary file: %w", err)
	}
	return file, nil
}

func CleanupStaleCloudInitISO() error {
	entries, err := os.ReadDir(cloudInitTempDir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("unable to read directory %s: %w", cloudInitTempDir, err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		filename := entry.Name()
		var pid int32
		if _, err := fmt.Sscanf(filename, cloudInitPrefix+"-%d-", &pid); err != nil {
			continue
		}
		exists, err := process.PidExists(pid)
		if err != nil {
			continue
		}
		if exists {
			continue
		}
		if err := os.Remove(filepath.Join(cloudInitTempDir, filename)); err != nil {
			return fmt.Errorf("unable to remove file %s: %w", filepath.Join(cloudInitTempDir, filename), err)
		}
		logrus.Debugf("removed stale cloud-init ISO %s", filepath.Join(cloudInitTempDir, filename))
	}
	return nil
}
