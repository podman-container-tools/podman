//go:build !remote

// SPDX-License-Identifier: Apache-2.0
//
// networking_pasta_linux.go - Start pasta(1) for user-mode connectivity
//
// Copyright (c) 2022 Red Hat GmbH
// Author: Stefano Brivio <sbrivio@redhat.com>

package libpod

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
	"go.podman.io/common/libnetwork/pasta"
)

func (r *Runtime) setupPasta(ctr *Container, netns string) error {
	pidPath := filepath.Join(ctr.state.RunDir, "pasta.pid")

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

	// Read and save the pasta PID so it can be moved to the conmon
	// cgroup scope later in moveConmonToCgroupAndSignal.
	pid, err := readPidFile(pidPath)
	if err != nil {
		logrus.Debugf("Failed to read pasta PID file: %v", err)
	} else {
		ctr.pastaPID = pid
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
