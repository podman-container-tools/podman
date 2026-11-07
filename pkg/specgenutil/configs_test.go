//go:build linux || freebsd

package specgenutil

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseConfigs(t *testing.T) {
	tests := []struct {
		name    string
		input   []string
		want    int
		mode    uint32
		wantErr bool
	}{
		{name: "empty", input: nil},
		{name: "source shorthand", input: []string{"app-config"}, want: 1, mode: 0o444},
		{name: "source and target", input: []string{"source=app-config,target=/etc/app/config,mode=0440"}, want: 1, mode: 0o440},
		{name: "shorthand with target", input: []string{"app-config,target=/etc/app/config"}, want: 1, mode: 0o444},
		{name: "relative target", input: []string{"source=app-config,target=etc/config"}, wantErr: true},
		{name: "environment config", input: []string{"source=app-config,type=env"}, wantErr: true},
		{name: "missing source", input: []string{"target=/etc/app/config"}, wantErr: true},
		{name: "unknown option", input: []string{"source=app-config,unknown=value"}, wantErr: true},
		{name: "option without value", input: []string{"source=app-config,unknown"}, wantErr: true},
		{name: "invalid uid", input: []string{"source=app-config,uid=bad"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configs, err := parseConfigs(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Len(t, configs, tt.want)
			if tt.want > 0 {
				require.Equal(t, tt.mode, configs[0].Mode)
			}
		})
	}
}
