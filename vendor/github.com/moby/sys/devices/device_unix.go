//go:build !windows

// SPDX-License-Identifier: Apache-2.0
/*
 * Copyright (C) 2015-2026 Open Containers Initiative Contributors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// This code originally comes from runc and was taken from this tree:
// <https://github.com/opencontainers/runc/tree/v1.4.0/libcontainer/devices>.

package devices

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/opencontainers/cgroups/devices/config"
	"golang.org/x/sys/unix"
)

// ErrNotADevice denotes that a file is not a valid linux device.
var ErrNotADevice = errors.New("not a device node")

// Testing dependencies
var (
	unixLstat = unix.Lstat
	osReadDir = os.ReadDir
)

// DeviceFromPath takes the path to a device and its cgroup_permissions (which
// cannot be easily queried) to look up the information about a linux device
// and returns that information as a Device struct.
func DeviceFromPath(path, permissions string) (*config.Device, error) {
	var stat unix.Stat_t
	err := unixLstat(path, &stat)
	if err != nil {
		return nil, err
	}

	var (
		devType   config.Type
		mode      = stat.Mode
		devNumber = uint64(stat.Rdev) //nolint:unconvert // Rdev is uint32 on e.g. MIPS.
		major     = unix.Major(devNumber)
		minor     = unix.Minor(devNumber)
	)
	switch mode & unix.S_IFMT {
	case unix.S_IFBLK:
		devType = config.BlockDevice
	case unix.S_IFCHR:
		devType = config.CharDevice
	case unix.S_IFIFO:
		devType = config.FifoDevice
	default:
		return nil, ErrNotADevice
	}
	return &config.Device{
		Rule: config.Rule{
			Type:        devType,
			Major:       int64(major),
			Minor:       int64(minor),
			Permissions: config.Permissions(permissions),
		},
		Path:     path,
		FileMode: os.FileMode(mode &^ unix.S_IFMT),
		Uid:      stat.Uid,
		Gid:      stat.Gid,
	}, nil
}

// HostDevices returns all devices that can be found under /dev directory.
func HostDevices() ([]*config.Device, error) {
	return GetDevices("/dev")
}

// GetDevices recursively traverses a directory specified by path
// and returns all devices found there.
func GetDevices(path string) ([]*config.Device, error) {
	files, err := osReadDir(path)
	if err != nil {
		return nil, err
	}
	var out []*config.Device
	for _, f := range files {
		switch {
		case f.IsDir():
			switch f.Name() {
			// ".lxc" & ".lxd-mounts" added to address https://github.com/lxc/lxd/issues/2825
			// ".udev" added to address https://github.com/opencontainers/runc/issues/2093
			case "pts", "shm", "fd", "mqueue", ".lxc", ".lxd-mounts", ".udev":
				continue
			default:
				sub, err := GetDevices(filepath.Join(path, f.Name()))
				if err != nil {
					return nil, err
				}

				out = append(out, sub...)
				continue
			}
		case f.Name() == "console":
			continue
		}
		device, err := DeviceFromPath(filepath.Join(path, f.Name()), "rwm")
		if err != nil {
			if errors.Is(err, ErrNotADevice) {
				continue
			}
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, err
		}
		if device.Type == config.FifoDevice {
			continue
		}
		out = append(out, device)
	}
	return out, nil
}
