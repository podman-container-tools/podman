//go:build windows

package wutil

import (
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	WSL1InstalledWithWSLAndVMPEnabled = `Default Version: 1`
	WSL2InstalledWithWSLAndVMPEnabled = `Default Version: 2`
	WSL1NotInstalled                  = `Default Version: 1

The Windows Subsystem for Linux kernel can be manually updated with 'wsl --update', but automatic updates cannot occur due to your system settings.
To receive automatic kernel updates, please enable the Windows Update setting: 'Receive updates for other Microsoft products when you update Windows'.
For more information please visit https://aka.ms/wsl2kernel.

The WSL 2 kernel file is not found. To update or restore the kernel please run 'wsl --update'.`
	WSL2NotInstalled = `The Windows Subsystem for Linux is not installed. You can install by running 'wsl.exe --install'.
For more information please visit https://aka.ms/wslinstall`
	WSL2InstalledWithWSLDisabled = `Default Version: 2
WSL1 is not supported with your current machine configuration.
Please enable the "Windows Subsystem for Linux" optional component to use WSL1.`
	WSL2InstalledWithVMPDisabled = `Default Version: 2
WSL2 is not supported with your current machine configuration.
Please enable the "Virtual Machine Platform" optional component and ensure virtualization is enabled in the BIOS.
Enable "Virtual Machine Platform" by running: wsl.exe --install --no-distribution
For information please visit https://aka.ms/enablevirtualization`
	WSL2InstalledWithWSLAndVMPDisabled = `Default Version: 2
WSL1 is not supported with your current machine configuration.
Please enable the "Windows Subsystem for Linux" optional component to use WSL1.
WSL2 is not supported with your current machine configuration.
Please enable the "Virtual Machine Platform" optional component and ensure virtualization is enabled in the BIOS.
Enable "Virtual Machine Platform" by running: wsl.exe --install --no-distribution
For information please visit https://aka.ms/enablevirtualization`
	WSL1InstalledWithVMPDisabled = `Default Version: 1
Please enable the Virtual Machine Platform Windows feature and ensure virtualization is enabled in the BIOS.
For information please visit https://aka.ms/enablevirtualization`
	WSL1InstalledWithWSLDisabled = `Default Version: 1
WSL1 is not supported with your current machine configuration.
Please enable the "Windows Subsystem for Linux" optional component to use WSL1.`
)

func TestMatchOutputLine(t *testing.T) {
	tests := []struct {
		winVariant   string
		statusOutput string
		want         wslStatus
	}{
		{
			"WSL1 configured and both Virtual Machine Platform enabled and Windows Subsystem for Linux are enabled",
			WSL1InstalledWithWSLAndVMPEnabled,
			wslStatus{
				installed:         true,
				vmpFeatureEnabled: true,
				wslFeatureEnabled: true,
			},
		},
		{
			"WSL2 configured and both Virtual Machine Platform enabled and Windows Subsystem for Linux enabled",
			WSL2InstalledWithWSLAndVMPEnabled,
			wslStatus{
				installed:         true,
				vmpFeatureEnabled: true,
				wslFeatureEnabled: true,
			},
		},
		{
			"WSL not installed (was previously configured as version 1)",
			WSL1NotInstalled,
			wslStatus{
				installed:         false,
				vmpFeatureEnabled: true,
				wslFeatureEnabled: true,
			},
		},
		{
			"WSL not installed (was previously configured as version 2)",
			WSL2NotInstalled,
			wslStatus{
				installed:         false,
				vmpFeatureEnabled: true,
				wslFeatureEnabled: true,
			},
		},
		{
			"WSL2 configured and Virtual Machine Platform is enabled but Windows Subsystem for Linux is disabled",
			WSL2InstalledWithWSLDisabled,
			wslStatus{
				installed:         true,
				vmpFeatureEnabled: true,
				wslFeatureEnabled: false,
			},
		},
		{
			"WSL2 configured and Virtual Machine Platform is disabled but Windows Subsystem for Linux is enabled",
			WSL2InstalledWithVMPDisabled,
			wslStatus{
				installed:         true,
				vmpFeatureEnabled: false,
				wslFeatureEnabled: true,
			},
		},
		{
			"WSL2 configured and both Virtual Machine Platform and Windows Subsystem for Linux are disabled",
			WSL2InstalledWithWSLAndVMPDisabled,
			wslStatus{
				installed:         true,
				vmpFeatureEnabled: false,
				wslFeatureEnabled: false,
			},
		},
		{
			"WSL1 configured and Virtual Machine Platform is disabled but Windows Subsystem for Linux is enabled",
			WSL1InstalledWithVMPDisabled,
			wslStatus{
				installed:         true,
				vmpFeatureEnabled: false,
				wslFeatureEnabled: true,
			},
		},
		{
			"WSL1 configured and Virtual Machine Platform is enabled but Windows Subsystem for Linux is disabled",
			WSL1InstalledWithWSLDisabled,
			wslStatus{
				installed:         true,
				vmpFeatureEnabled: true,
				wslFeatureEnabled: false,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.winVariant, func(t *testing.T) {
			reader := io.NopCloser(strings.NewReader(tt.statusOutput))
			status, err := matchOutputLine(reader)
			require.NoError(t, err)
			assert.Equal(t, tt.want, status)
		})
	}
}

func TestNewWSLCommand(t *testing.T) {
	cmd := NewWSLCommand("--status")
	assert.Contains(t, cmd.Path, "wsl")
	assert.Equal(t, []string{"--status"}, cmd.Args[1:])
	assert.Contains(t, cmd.Env, "WSL_UTF8=1")
}
