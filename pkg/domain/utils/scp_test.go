package utils

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

func TestValidateSCPArgs(t *testing.T) {
	type args struct {
		locations []*entities.ScpTransferImageOptions
	}
	tests := []struct {
		name    string
		args    args
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "test args length more than 2",
			args: args{
				locations: []*entities.ScpTransferImageOptions{
					{
						Image: "source image one",
					},
					{
						Image: "source image two",
					},
					{
						Image: "target image one",
					},
					{
						Image: "target image two",
					},
				},
			},
			wantErr: assert.Error,
		},
		{
			name: "test source image is empty",
			args: args{
				locations: []*entities.ScpTransferImageOptions{
					{
						Image: "",
					},
					{
						Image: "target image",
					},
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "test target image is empty",
			args: args{
				locations: []*entities.ScpTransferImageOptions{
					{
						Image: "source image",
					},
					{
						Image: "target image",
					},
				},
			},
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.wantErr(t, ValidateSCPArgs(tt.args.locations), fmt.Sprintf("ValidateSCPArgs(%v)", tt.args.locations))
		})
	}
}

func TestParseImageSCPArg(t *testing.T) {
	tests := []struct {
		name     string
		arg      string
		wantUser string
	}{
		{
			name:     "user without domain",
			arg:      "user@localhost::alpine",
			wantUser: "user",
		},
		{
			name:     "username containing an @ (e.g. Active Directory)",
			arg:      "user@domain@localhost::example.com/foo/bar:latest",
			wantUser: "user@domain",
		},
		{
			name:     "no username before @localhost::",
			arg:      "@localhost::alpine",
			wantUser: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			location, _, err := ParseImageSCPArg(tt.arg)
			assert.NoError(t, err)
			assert.Equal(t, tt.wantUser, location.User)
		})
	}
}
