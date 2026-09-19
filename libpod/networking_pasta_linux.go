//go:build !remote

// SPDX-License-Identifier: Apache-2.0
//
// networking_pasta_linux.go - Start pasta(1) for user-mode connectivity
//
// Copyright (c) 2022 Red Hat GmbH
// Author: Stefano Brivio <sbrivio@redhat.com>

package libpod

import (
	"path/filepath"

	"go.podman.io/common/libnetwork/pasta"
)

func (r *Runtime) setupPasta(ctr *Container, netns string) error {
	extraOpts := ctr.config.NetworkOptions[pasta.BinaryName]
	hasPidFlag := false
	for _, opt := range extraOpts {
		if opt == "--pid" || opt == "-P" {
			hasPidFlag = true
			break
		}
	}
	if !hasPidFlag {
		pidFile := filepath.Join(ctr.state.RunDir, "pasta.pid")
		extraOpts = append([]string{"--pid", pidFile}, extraOpts...)
	}

	res, err := pasta.Setup(&pasta.SetupOptions{
		Config:       r.config,
		Netns:        netns,
		Ports:        ctr.convertPortMappings(),
		ExtraOptions: extraOpts,
	})
	if err != nil {
		return err
	}
	ctr.pastaResult = res
	return nil
}
