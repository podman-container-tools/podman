//go:build !remote && linux

package libpod

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.podman.io/podman/v6/libpod/define"
)

func Test_statToPercent(t *testing.T) {
	type args struct {
		in0 []string
	}
	tests := []struct {
		name    string
		args    args
		want    *define.CPUUsage
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "GoodParse",
			args: args{in0: []string{"cpu", "33628064", "27537", "9696996", "1314806705", "588142", "4775073", "2789228", "0", "598711", "0"}},
			want: &define.CPUUsage{
				UserPercent:   2.48,
				SystemPercent: 0.71,
				IdlePercent:   96.81,
			},
			wantErr: assert.NoError,
		},
		{
			name:    "BadUserValue",
			args:    args{in0: []string{"cpu", "k", "27537", "9696996", "1314806705", "588142", "4775073", "2789228", "0", "598711", "0"}},
			want:    nil,
			wantErr: assert.Error,
		},
		{
			name:    "BadSystemValue",
			args:    args{in0: []string{"cpu", "33628064", "27537", "k", "1314806705", "588142", "4775073", "2789228", "0", "598711", "0"}},
			want:    nil,
			wantErr: assert.Error,
		},
		{
			name:    "BadIdleValue",
			args:    args{in0: []string{"cpu", "33628064", "27537", "9696996", "k", "588142", "4775073", "2789228", "0", "598711", "0"}},
			want:    nil,
			wantErr: assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := statToPercent(tt.args.in0)
			if !tt.wantErr(t, err, fmt.Sprintf("statToPercent(%v)", tt.args.in0)) {
				return
			}
			assert.Equalf(t, tt.want, got, "statToPercent(%v)", tt.args.in0)
		})
	}
}
