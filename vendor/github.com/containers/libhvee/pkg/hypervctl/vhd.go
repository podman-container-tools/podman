package hypervctl

import (
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"

	"github.com/containers/libhvee/pkg/wmiext"
	"go.podman.io/common/pkg/strongunits"
)

// ResizeDisk takes a diskPath and strongly typed new size and uses powershell
// to change its size.  There is no error protection for trying to size a disk
// smaller than the current size.
func ResizeDisk(diskPath string, newSize strongunits.GiB) error {
	var (
		service *wmiext.Service
		err     error
		job     *wmiext.Instance
		ret     int32
	)

	if service, err = NewLocalHyperVService(); err != nil {
		return err
	}
	defer service.Close()

	imms, err := service.GetSingletonInstance("Msvm_ImageManagementService")
	if err != nil {
		return err
	}
	defer imms.Close()

	err = imms.BeginInvoke("ResizeVirtualHardDisk").
		In("Path", diskPath).
		In("MaxInternalSize", int64(newSize.ToBytes())).
		Execute().
		Out("Job", &job).
		Out("ReturnValue", &ret).
		End()

	if err != nil {
		return fmt.Errorf("failed to resize disk: %w", err)
	}
	return waitVMResult(ret, service, job, "failed to resize disk", nil)
}

func GetDiskSize(diskPath string) (strongunits.B, error) {
	var (
		service *wmiext.Service
		err     error
		job     *wmiext.Instance
		ret     int32
		results string
	)

	if service, err = NewLocalHyperVService(); err != nil {
		return 0, err
	}
	defer service.Close()
	imms, err := service.GetSingletonInstance("Msvm_ImageManagementService")
	if err != nil {
		return 0, err
	}
	defer imms.Close()

	inv := imms.BeginInvoke("GetVirtualHardDiskSettingData").
		In("Path", diskPath).
		Execute().
		Out("Job", &job).
		Out("ReturnValue", &ret)

	if err := inv.Error(); err != nil {
		return 0, fmt.Errorf("failed to get setting data for disk %s: %q", diskPath, err)
	}

	if err := waitVMResult(ret, service, job, "failure waiting on result from disk settings", nil); err != nil {
		return 0, err
	}

	err = inv.Out("SettingData", &results).End()
	if err != nil {
		return 0, fmt.Errorf("failed to retrieve setting object payload for disk: %q", err)
	}

	type CimSingleInstance struct {
		XMLName    xml.Name             `xml:"INSTANCE"`
		Properties []CimKvpItemProperty `xml:"PROPERTY"`
	}

	diskSettings := CimSingleInstance{}
	if err := xml.Unmarshal([]byte(results), &diskSettings); err != nil {
		return 0, fmt.Errorf("unable to parse disk settings xml: %q", err)
	}

	for _, prop := range diskSettings.Properties {
		if strings.EqualFold(prop.Name, "MaxInternalSize") {
			size, err := strconv.ParseUint(prop.Value, 10, 64)
			if err != nil {
				return 0, fmt.Errorf("unable to parse size in disk settings")
			}
			return strongunits.B(size), nil
		}
	}

	return 0, fmt.Errorf("disk settings was missing a size value")
}
