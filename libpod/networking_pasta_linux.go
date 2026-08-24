//go:build !remote

// SPDX-License-Identifier: Apache-2.0
//
// networking_pasta_linux.go - Start pasta(1) for user-mode connectivity
//
// Copyright (c) 2022 Red Hat GmbH
// Author: Stefano Brivio <sbrivio@redhat.com>

package libpod

import (
	"crypto/rand"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
	"go.podman.io/common/libnetwork/pasta"
	"go.podman.io/common/pkg/systemd"
)

func (r *Runtime) setupPasta(ctr *Container, netns string) error {
	pidPath := fmt.Sprintf("/tmp/pasta-%s.pid", ctr.ID())

	extraOpts := append([]string{"--pid", pidPath}, ctr.config.NetworkOptions[pasta.BinaryName]...)

	res, err := pasta.Setup(&pasta.SetupOptions{
		Config:       r.config,
		Netns:        netns,
		Ports:        ctr.convertPortMappings(),
		ExtraOptions: extraOpts,
	})
	if err != nil {
		return err
	}

	if systemd.RunsOnSystemd() {
		pid, err := readPidFile(pidPath)
		if err != nil {
			logrus.Debugf("Failed to read pasta PID file: %v", err)
		} else if err := movePastaToScope(pid); err != nil {
			logrus.Debugf("Failed to move pasta process to systemd scope: %v", err)
		}
	}

	// Best-effort cleanup of the PID file.
	os.Remove(pidPath)

	ctr.pastaResult = res
	return nil
}

func readPidFile(path string) (int, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(b)))
}

func movePastaToScope(pid int) error {
	randBytes := make([]byte, 4)
	if _, err := rand.Read(randBytes); err != nil {
		return fmt.Errorf("failed to read random bytes: %w", err)
	}
	return systemd.RunUnderSystemdScope(pid, "user.slice", fmt.Sprintf("libpod-pasta-%x.scope", randBytes))
}
