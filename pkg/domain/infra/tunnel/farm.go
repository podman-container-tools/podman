package tunnel

import (
	"context"
	"fmt"

	"go.podman.io/podman/v6/pkg/bindings/system"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

const (
	remoteFarmImageBuilderDriver = "podman-remote"
)

// FarmNodeName returns the remote engine's name.
func (ir *ImageEngine) FarmNodeName(_ context.Context) string {
	return ir.NodeName
}

// FarmNodeDriver returns a description of the image builder driver
func (ir *ImageEngine) FarmNodeDriver(_ context.Context) string {
	return remoteFarmImageBuilderDriver
}

func (ir *ImageEngine) fetchInfo(_ context.Context) (os, arch, variant string, nativePlatforms []string, err error) {
	engineInfo, err := system.Info(ir.ClientCtx, &system.InfoOptions{})
	if err != nil {
		return "", "", "", nil, fmt.Errorf("retrieving host info from %q: %w", ir.NodeName, err)
	}
	nativePlatform := engineInfo.Host.OS + "/" + engineInfo.Host.Arch
	if engineInfo.Host.Variant != "" {
		nativePlatform = nativePlatform + "/" + engineInfo.Host.Variant
	}
	return engineInfo.Host.OS, engineInfo.Host.Arch, engineInfo.Host.Variant, []string{nativePlatform}, nil
}

// FarmNodeInspect returns information about the remote engines in the farm
func (ir *ImageEngine) FarmNodeInspect(ctx context.Context) (*entities.FarmInspectReport, error) {
	ir.platforms.Do(func() {
		ir.os, ir.arch, ir.variant, ir.nativePlatforms, ir.platformsErr = ir.fetchInfo(ctx)
	})
	return &entities.FarmInspectReport{
		NativePlatforms: ir.nativePlatforms,
		OS:              ir.os,
		Arch:            ir.arch,
		Variant:         ir.variant,
	}, ir.platformsErr
}
