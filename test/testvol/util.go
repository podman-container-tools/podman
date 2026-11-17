package main

import (
	"path/filepath"
	"strings"

	"go.podman.io/podman/v6/libpod/plugin"
)

const pluginSockDir = "/run/docker/plugins"

func getSocketPath(pathOrName string) string {
	if filepath.IsAbs(pathOrName) {
		return pathOrName
	}

	// only a name join it with the default path
	return filepath.Join(pluginSockDir, pathOrName+".sock")
}

func getPluginName(pathOrName string) string {
	return strings.TrimSuffix(filepath.Base(pathOrName), ".sock")
}

func getPlugin(sockNameOrPath string) (*plugin.VolumePlugin, error) {
	path := getSocketPath(sockNameOrPath)
	name := getPluginName(sockNameOrPath)
	return plugin.GetVolumePlugin(name, path, nil, nil)
}
