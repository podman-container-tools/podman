package utils

import (
	"net/url"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

func TestToLibpodFilters(t *testing.T) {
	good := url.Values{}
	good.Set("apple", "red")
	good.Set("banana", "yellow")
	good.Set("pear", "")
	goodResult := []string{"apple=red", "banana=yellow", "pear="}
	sort.Strings(goodResult)

	empty := url.Values{}
	type args struct {
		f url.Values
	}
	tests := []struct {
		name        string
		args        args
		wantFilters []string
	}{
		{
			name: "GoodURLValue",
			args: args{
				f: good,
			},
			wantFilters: goodResult,
		},
		{
			name: "Empty",
			args: args{
				f: empty,
			},
			wantFilters: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.ElementsMatchf(t, ToLibpodFilters(tt.args.f), tt.wantFilters, "ToLibpodFilters() = %v, want %v", ToLibpodFilters(tt.args.f), tt.wantFilters)
		})
	}
}

func TestToURLValues(t *testing.T) {
	good := url.Values{}
	good.Set("apple", "red")
	good.Set("banana", "yellow")
	good.Set("pear", "")
	goodResult := []string{"apple=red", "banana=yellow", "pear="}

	type args struct {
		f []string
	}
	tests := []struct {
		name        string
		args        args
		wantFilters url.Values
	}{
		{
			name:        "Good",
			args:        args{goodResult},
			wantFilters: good,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.EqualValuesf(t, ToURLValues(tt.args.f), tt.wantFilters, "ToURLValues() = %v, want %v", ToURLValues(tt.args.f), tt.wantFilters)
		})
	}
}

func TestParseSCPArgs(t *testing.T) {
	args := []string{"alpine", "root@localhost::"}
	var source *entities.ScpTransferImageOptions
	var dest *entities.ScpTransferImageOptions
	var err error
	source, _, err = ParseImageSCPArg(args[0])
	assert.NoError(t, err)
	assert.Equal(t, source.Image, "alpine")

	dest, _, err = ParseImageSCPArg(args[1])
	assert.NoError(t, err)
	assert.Equal(t, dest.Image, "")
	assert.Equal(t, dest.User, "root")

	args = []string{"root@localhost::alpine"}
	source, _, err = ParseImageSCPArg(args[0])
	assert.NoError(t, err)
	assert.Equal(t, source.User, "root")
	assert.Equal(t, source.Image, "alpine")

	args = []string{"charliedoern@192.168.68.126::alpine", "foobar@192.168.68.126::"}
	source, _, err = ParseImageSCPArg(args[0])
	assert.NoError(t, err)
	assert.True(t, source.Remote)
	assert.Equal(t, source.Image, "alpine")

	dest, _, err = ParseImageSCPArg(args[1])
	assert.NoError(t, err)
	assert.True(t, dest.Remote)
	assert.Equal(t, dest.Image, "")

	args = []string{"charliedoern@192.168.68.126::alpine"}
	source, _, err = ParseImageSCPArg(args[0])
	assert.NoError(t, err)
	assert.True(t, source.Remote)
	assert.Equal(t, source.Image, "alpine")
}
