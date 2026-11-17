//go:build !darwin

package qemu

import (
	"reflect"
	"testing"

	"go.podman.io/podman/v6/pkg/machine/define"
)

func TestUSBParsing(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		result  []define.USBConfig
		wantErr bool
	}{
		{
			name: "Good vendor and product",
			args: []string{"vendor=13d3,product=5406", "vendor=08ec,product=0016"},
			result: []define.USBConfig{
				{
					Vendor:  5075,
					Product: 21510,
				},
				{
					Vendor:  2284,
					Product: 22,
				},
			},
			wantErr: false,
		},
		{
			name: "Good bus and device number",
			args: []string{"bus=1,devnum=4", "bus=1,devnum=3"},
			result: []define.USBConfig{
				{
					Bus:       "1",
					DevNumber: "4",
				},
				{
					Bus:       "1",
					DevNumber: "3",
				},
			},
			wantErr: false,
		},
		{
			name:    "Bad vendor and product, not hexa",
			args:    []string{"vendor=13dk,product=5406"},
			result:  []define.USBConfig{},
			wantErr: true,
		},
		{
			name:    "Bad vendor and product, bad separator",
			args:    []string{"vendor=13d3:product=5406"},
			result:  []define.USBConfig{},
			wantErr: true,
		},
		{
			name:    "Bad vendor and product, missing equal",
			args:    []string{"vendor=13d3:product-5406"},
			result:  []define.USBConfig{},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := define.ParseUSBs(test.args)
			if (err != nil) != test.wantErr {
				t.Errorf("parseUUBs error = %v, wantErr %v", err, test.wantErr)
				return
			}
			if !reflect.DeepEqual(got, test.result) {
				t.Errorf("parseUUBs got %v, want %v", got, test.result)
			}
		})
	}
}
