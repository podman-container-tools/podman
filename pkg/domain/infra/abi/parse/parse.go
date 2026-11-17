//go:build !remote && (linux || freebsd)

package parse

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/docker/go-units"
	"github.com/sirupsen/logrus"
	"go.podman.io/podman/v6/libpod"
	"go.podman.io/podman/v6/libpod/define"
)

// Handle volume options from CLI.
// Parse "o" option to find UID, GID, Size.
func VolumeOptions(opts map[string]string) ([]libpod.VolumeCreateOption, error) {
	libpodOptions := []libpod.VolumeCreateOption{}
	volumeOptions := make(map[string]string)

	for key, value := range opts {
		switch key {
		case "o":
			// o has special handling to parse out UID, GID.
			// These are separate Libpod options.
			finalVal := []string{}
			for o := range strings.SplitSeq(value, ",") {
				// Options will be formatted as either "opt" or
				// "opt=value"
				opt, val, hasVal := strings.Cut(o, "=")
				switch strings.ToLower(opt) {
				case "size":
					size, err := units.FromHumanSize(val)
					if err != nil {
						return nil, fmt.Errorf("cannot convert size %s to integer: %w", val, err)
					}
					libpodOptions = append(libpodOptions, libpod.WithVolumeSize(uint64(size)))
					finalVal = append(finalVal, o)
					// set option "SIZE": "$size"
					volumeOptions["SIZE"] = val
				case "inodes":
					inodes, err := strconv.ParseUint(val, 10, 64)
					if err != nil {
						return nil, fmt.Errorf("cannot convert inodes %s to integer: %w", val, err)
					}
					libpodOptions = append(libpodOptions, libpod.WithVolumeInodes(inodes))
					finalVal = append(finalVal, o)
					// set option "INODES": "$size"
					volumeOptions["INODES"] = val
				case "uid":
					if !hasVal {
						return nil, fmt.Errorf("uid option must provide a UID: %w", define.ErrInvalidArg)
					}
					intUID, err := strconv.Atoi(val)
					if err != nil {
						return nil, fmt.Errorf("cannot convert UID %s to integer: %w", val, err)
					}
					logrus.Debugf("Removing uid= from options and adding WithVolumeUID for UID %d", intUID)
					libpodOptions = append(libpodOptions, libpod.WithVolumeUID(intUID), libpod.WithVolumeNoChown())
					finalVal = append(finalVal, o)
					// set option "UID": "$uid"
					volumeOptions["UID"] = val
				case "gid":
					if !hasVal {
						return nil, fmt.Errorf("gid option must provide a GID: %w", define.ErrInvalidArg)
					}
					intGID, err := strconv.Atoi(val)
					if err != nil {
						return nil, fmt.Errorf("cannot convert GID %s to integer: %w", val, err)
					}
					logrus.Debugf("Removing gid= from options and adding WithVolumeGID for GID %d", intGID)
					libpodOptions = append(libpodOptions, libpod.WithVolumeGID(intGID), libpod.WithVolumeNoChown())
					finalVal = append(finalVal, o)
					// set option "GID": "$gid"
					volumeOptions["GID"] = val
				case "noquota":
					logrus.Debugf("Removing noquota from options and adding WithVolumeDisableQuota")
					libpodOptions = append(libpodOptions, libpod.WithVolumeDisableQuota())
					// set option "NOQUOTA": "true"
					volumeOptions["NOQUOTA"] = "true"
				case "timeout":
					if !hasVal {
						return nil, fmt.Errorf("timeout option must provide a valid timeout in seconds: %w", define.ErrInvalidArg)
					}
					intTimeout, err := strconv.Atoi(val)
					if err != nil {
						return nil, fmt.Errorf("cannot convert Timeout %s to an integer: %w", val, err)
					}
					if intTimeout < 0 {
						return nil, fmt.Errorf("volume timeout cannot be negative (got %d)", intTimeout)
					}
					logrus.Debugf("Removing timeout from options and adding WithTimeout for Timeout %d", intTimeout)
					libpodOptions = append(libpodOptions, libpod.WithVolumeDriverTimeout(uint(intTimeout)))
				default:
					finalVal = append(finalVal, o)
				}
			}
			if len(finalVal) > 0 {
				volumeOptions[key] = strings.Join(finalVal, ",")
			}
		default:
			volumeOptions[key] = value
		}
	}

	if len(volumeOptions) > 0 {
		libpodOptions = append(libpodOptions, libpod.WithVolumeOptions(volumeOptions))
	}

	return libpodOptions, nil
}
