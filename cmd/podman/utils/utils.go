package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/domain/entities/reports"
)

// IsDir returns true if the specified path refers to a directory.
func IsDir(path string) bool {
	file, err := os.Stat(path)
	if err != nil {
		return false
	}
	return file.IsDir()
}

// FileExists returns true if path refers to an existing file.
func FileExists(path string) bool {
	file, err := os.Stat(path)
	// All errors return file == nil
	if err != nil {
		return false
	}
	return !file.IsDir()
}

func PrintPodPruneResults(podPruneReports []*entities.PodPruneReport, heading bool) error {
	var errs OutputErrors
	if heading && len(podPruneReports) > 0 {
		fmt.Println("Deleted Pods")
	}
	for _, r := range podPruneReports {
		if r.Err == nil {
			fmt.Println(r.Id)
		} else {
			errs = append(errs, r.Err)
		}
	}
	return errs.PrintErrors()
}

func PrintContainerPruneResults(containerPruneReports []*reports.PruneReport, heading bool) error {
	var errs OutputErrors
	if heading && len(containerPruneReports) > 0 {
		fmt.Println("Deleted Containers")
	}
	for _, v := range containerPruneReports {
		fmt.Println(v.Id)
		if v.Err != nil {
			errs = append(errs, v.Err)
		}
	}
	return errs.PrintErrors()
}

func PrintVolumePruneResults(volumePruneReport []*reports.PruneReport, heading bool) error {
	var errs OutputErrors
	if heading && len(volumePruneReport) > 0 {
		fmt.Println("Deleted Volumes")
	}
	for _, r := range volumePruneReport {
		if r.Err == nil {
			fmt.Println(r.Id)
		} else {
			errs = append(errs, r.Err)
		}
	}
	return errs.PrintErrors()
}

func PrintImagePruneResults(imagePruneReports []*reports.PruneReport, heading bool) error {
	if heading && len(imagePruneReports) > 0 {
		fmt.Println("Deleted Images")
	}
	for _, r := range imagePruneReports {
		fmt.Println(r.Id)
		if r.Err != nil {
			fmt.Fprint(os.Stderr, r.Err.Error()+"\n")
		}
	}

	return nil
}

func PrintNetworkPruneResults(networkPruneReport []*entities.NetworkPruneReport, heading bool) error {
	var errs OutputErrors
	if heading && len(networkPruneReport) > 0 {
		fmt.Println("Deleted Networks")
	}
	for _, r := range networkPruneReport {
		if r.Error == nil {
			fmt.Println(r.Name)
		} else {
			errs = append(errs, r.Error)
		}
	}
	return errs.PrintErrors()
}

// IsCheckpointImage returns true with no error only if all values in
// namesOrIDs correspond to checkpoint images AND these images are
// compatible with the container runtime that is currently in use,
// e.g., crun or runc.
//
// IsCheckpointImage returns false with no error when none of the values
// in namesOrIDs corresponds to an ID or name of an image.
//
// Otherwise, IsCheckpointImage returns false with appropriate error.
func IsCheckpointImage(ctx context.Context, namesOrIDs []string) (bool, error) {
	inspectOpts := entities.InspectOptions{}
	imgData, _, err := registry.ImageEngine().Inspect(ctx, namesOrIDs, inspectOpts)
	if err != nil {
		return false, err
	}
	if len(imgData) == 0 {
		return false, nil
	}
	imgID := imgData[0].ID

	hostInfo, err := registry.ContainerEngine().Info(ctx)
	if err != nil {
		return false, err
	}

	for i := range imgData {
		checkpointRuntimeName, found := imgData[i].Annotations[define.CheckpointAnnotationRuntimeName]
		if !found {
			return false, fmt.Errorf("image is not a checkpoint: %s", imgID)
		}
		if hostInfo.Host.OCIRuntime.Name != checkpointRuntimeName {
			return false, fmt.Errorf("container image \"%s\" requires runtime: \"%s\"", imgID, checkpointRuntimeName)
		}
	}
	return true, nil
}

func RemoveSlash(input []string) []string {
	output := make([]string, 0, len(input))
	for _, in := range input {
		output = append(output, strings.TrimPrefix(in, "/"))
	}
	return output
}

func PrintGenericJSON(data any) error {
	enc := json.NewEncoder(os.Stdout)
	// by default, json marshallers will force utf=8 from
	// a string.
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "     ")
	return enc.Encode(data)
}
