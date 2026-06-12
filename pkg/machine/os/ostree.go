//go:build amd64 || arm64

package os

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
	"go.podman.io/image/v5/docker"
	"go.podman.io/image/v5/docker/reference"
	"go.podman.io/image/v5/image"
	"go.podman.io/image/v5/manifest"
	"go.podman.io/image/v5/transports/alltransports"
	"go.podman.io/image/v5/types"
)

// OSTree deals with operations on ostree based os's
type OSTree struct{}

// Apply takes an input that describes an image based on transports
// defined by bootc.  Omission of a transport assumes the image
// is pulled from an OCI registry.  We simply pass the user
// input to bootc without any manipulation.
func (dist *OSTree) Apply(image string, _ ApplyOptions) error {
	t, pathOrImageRef, err := parseApplyInput(image)
	if err != nil {
		return err
	}
	cli := []string{"bootc", "switch"}
	// registry is the default transport and therefore not
	// necessary
	if t != "registry" {
		cli = append(cli, "--transport", t)
	}
	cli = append(cli, pathOrImageRef)
	cmd := exec.Command("sudo", cli...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (dist *OSTree) Upgrade(ctx context.Context, opts UpgradeOptions) error {
	var (
		updateMessage string
		args          []string
	)
	clientInfo := Host{
		Version: opts.ClientVersion,
	}

	machineInfo := Machine{
		Version: opts.MachineVersion,
	}

	bootcStatus, err := newBootcHost()
	if err != nil {
		return err
	}
	originNamed, originVersion, err := bootcStatus.getLocalOSOCIInfo()
	if err != nil {
		return err
	}

	// compareMajorMinor returns 0 if A == B, -1 if A < B, and 1 if A > B
	switch compareMajorMinor(opts.ClientVersion, opts.MachineVersion) {
	case 1:
		// if caller version < machine version, return an error because this is bad
		// this error message looks odd bc the client version %s is a machineversion, but that
		// is because of how this command is called through ssh.
		return fmt.Errorf("client version %s is older than your machine version %s: upgrade your client", opts.MachineVersion, opts.ClientVersion)
	case 0:
		// If caller version == machine version AND there is actually an update
		// on the registry, then update "in band"
		if compareMajorMinor(*originVersion, opts.ClientVersion) != 0 {
			return fmt.Errorf("version mismatch between podman version (%s) and host os (%s)", originVersion.String(), opts.ClientVersion.String())
		}

		localDigest, err := bootcStatus.getLocalOsImageDigest()
		if err != nil {
			return err
		}

		// check if an in band update exists. this covers two scenarios
		// 5.7.1 -> 5.7.2
		// 5.7.1 -> newer OS image with 5.7.1
		registryDigest, updateExists, err := checkInBandUpgrade(ctx, originNamed, localDigest)
		if err != nil {
			return err
		}
		machineInfo.CurrentHash = localDigest
		machineInfo.NewHash = registryDigest
		if !updateExists {
			// If more formats are ever added, we maybe break this out to a
			// func called print() which does a switch on type and marshalls
			if opts.Format == "json" {
				return printJSON(UpgradeOutput{
					Host:    clientInfo,
					Machine: machineInfo,
				})
			}
			return nil
		}
		machineInfo.InBandUpgradeAvailable = true
		updateMessage = fmt.Sprintf("Updating OS from %s to %s\n", localDigest.String(), registryDigest.String())
		args = []string{"bootc", "upgrade"}
	default:
		// if caller version > machine version, then update to the caller version
		newVersion := fmt.Sprintf("%d.%d", opts.MachineVersion.Major, opts.MachineVersion.Minor)
		updateReference := fmt.Sprintf("%s:%s", originNamed.Name(), newVersion)
		updateMessage = fmt.Sprintf("Updating OS from version %d.%d to %s\n", opts.ClientVersion.Major, opts.ClientVersion.Minor, newVersion)
		args = []string{"bootc", "switch", updateReference}
	}

	if len(opts.Format) > 0 {
		return printJSON(UpgradeOutput{
			Host:    clientInfo,
			Machine: machineInfo,
		})
	}
	// if you change conditions above this, pay attention to this
	// next line because you might need to wrap it in a conditional
	fmt.Print(updateMessage)

	if opts.DryRun {
		return nil
	}

	cmd := exec.Command("sudo", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// compareMajorMinor returns 0 if A == B, -1 if A < B, and 1 if A > B
func compareMajorMinor(versionA, versionB semver.Version) int {
	// ignore patch versions for comparison
	versionA.Patch = 0
	versionB.Patch = 0
	versionA.Pre = nil
	versionB.Pre = nil
	// https://pkg.go.dev/github.com/blang/semver/v4#Version.Compare
	return versionA.Compare(versionB)
}

// checkInBandUpgrade takes a named and the digest of the image that made the operating system.  we then check if the image
// on the registry is different.
func checkInBandUpgrade(ctx context.Context, named reference.Named, localHostDigest digest.Digest) (digest.Digest, bool, error) {
	sysCtx := types.SystemContext{}
	logrus.Debugf("Checking if %s has upgrade available", named.Name())
	// Lookup the image from the os
	ir, err := docker.NewReference(named)
	if err != nil {
		return "", false, err
	}
	src, err := ir.NewImageSource(ctx, &sysCtx)
	if err != nil {
		return "", false, err
	}
	defer src.Close()
	rawManifest, manType, err := image.UnparsedInstance(src, nil).Manifest(ctx)
	if err != nil {
		return "", false, err
	}
	list, err := manifest.ListFromBlob(rawManifest, manType)
	if err != nil {
		return "", false, err
	}
	// Now get the blob for the image
	d, err := list.ChooseInstance(&sysCtx)
	if err != nil {
		return "", false, err
	}
	imageManifest, imageManType, err := image.UnparsedInstance(src, &d).Manifest(ctx)
	if err != nil {
		return "", false, err
	}
	registryImageManifest, err := manifest.FromBlob(imageManifest, imageManType)
	if err != nil {
		return "", false, err
	}

	return registryImageManifest.ConfigInfo().Digest, registryImageManifest.ConfigInfo().Digest != localHostDigest, nil
}

func printJSON(out UpgradeOutput) error {
	b, err := json.Marshal(out)
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}

// parseApplyInput takes well known OCI references and splits them up. this
// function should only be used to deal with bootc transports.  podman takes
// the entire reference as an oci reference but bootc requires we use
// --transport if the default transport is not used.  also, bootc's default
// transport is "registry" which mimics docker in the way it is handled.
func parseApplyInput(arg string) (string, string, error) {
	var (
		containersStorageTransport = "containers-storage"
		dockerTransport            = "docker"
		ociArchiveTransport        = "oci-archive"
		ociTransport               = "oci"
		registryTransport          = "registry"
	)

	// The order of this parsing matters.  Be careful when you edit.

	// containers-storage:/home/user:localhost/fedora-bootc:latest
	afterCS, hasCS := strings.CutPrefix(arg, containersStorageTransport+":")
	if hasCS {
		return containersStorageTransport, afterCS, nil
	}

	imgRef, err := alltransports.ParseImageName(arg)
	if err == nil {
		transportName := imgRef.Transport().Name()
		if imgRef.DockerReference() != nil {
			// bootc  docs do not show the docker transport, but the
			// code apparently does handle it and because docker is
			// a well-known transport, we do as well, but we change
			// it to registry for convenience.
			if transportName == dockerTransport {
				transportName = registryTransport
			}
			// docker://quay.io/fedora/fedora-bootc:40
			return transportName, imgRef.DockerReference().String(), nil
		}

		imagePath := imgRef.StringWithinTransport()
		switch transportName {
		case ociTransport:
			// oci:/tmp/oci-image
			imagePath, _, _ = strings.Cut(imagePath, ":")
		case ociArchiveTransport:
			// oci-archive:/tmp/myimage.tar
			imagePath = strings.TrimSuffix(imagePath, ":")
		}
		return transportName, imagePath, nil
	}

	// quay.io/fedora/fedora-bootc:40
	if _, err := reference.ParseNamed(arg); err == nil {
		return registryTransport, arg, nil
	}

	// registry://quay.io/fedora/fedora-bootc:40
	afterReg, hasRegistry := strings.CutPrefix(arg, registryTransport+"://")
	if hasRegistry {
		return registryTransport, afterReg, nil
	}

	// oci-archive:/var/tmp/fedora-bootc.tar
	// oci-archive:/mnt/usb/images/myapp.tar
	afterOCIArchive, hasOCIArchive := strings.CutPrefix(arg, ociArchiveTransport+":")
	if hasOCIArchive {
		return ociArchiveTransport, afterOCIArchive, nil
	}

	// oci:/tmp/oci-image
	// oci:/var/mnt/usb/bootc-image
	afterOCI, hasOCI := strings.CutPrefix(arg, ociTransport+":")
	if hasOCI {
		return ociTransport, afterOCI, nil
	}

	return "", "", fmt.Errorf("unknown transport %q given", arg)
}
