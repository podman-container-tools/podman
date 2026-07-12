package main

import "go.podman.io/podman/v6/libpod/define"

type clientInfo struct {
	OSArch      string `json:"OS"`
	Provider    string `json:"provider"`
	Version     string `json:"version"`
	BuildOrigin string `json:"buildOrigin,omitempty" yaml:",omitempty"`
}

func getClientInfo() (*clientInfo, error) {
	p, err := getProvider()
	if err != nil {
		return nil, err
	}
	vinfo, err := define.GetVersion()
	if err != nil {
		return nil, err
	}
	return &clientInfo{
		OSArch:      vinfo.OsArch,
		Provider:    p,
		Version:     vinfo.Version,
		BuildOrigin: vinfo.BuildOrigin,
	}, nil
}
