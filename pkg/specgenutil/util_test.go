package specgenutil

import (
	"reflect"
	"testing"
)

func TestCreateExpose(t *testing.T) {
	single := make(map[uint16]string, 0)
	single[99] = "tcp"

	simpleRange := make(map[uint16]string, 0)
	simpleRange[99] = "tcp"
	simpleRange[100] = "tcp"

	simpleRangeUDP := make(map[uint16]string, 0)
	simpleRangeUDP[99] = "udp"
	simpleRangeUDP[100] = "udp"
	type args struct {
		expose []string
	}
	tests := []struct {
		name    string
		args    args
		want    map[uint16]string
		wantErr bool
	}{
		{
			name: "single port",
			args: args{
				expose: []string{"99"},
			},
			want:    single,
			wantErr: false,
		},
		{
			name: "simple range tcp",
			args: args{
				expose: []string{"99-100"},
			},
			want:    simpleRange,
			wantErr: false,
		},
		{
			name: "simple range udp",
			args: args{
				expose: []string{"99-100/udp"},
			},
			want:    simpleRangeUDP,
			wantErr: false,
		},
		{
			name: "range inverted should fail",
			args: args{
				expose: []string{"100-99"},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "specifying protocol twice should fail",
			args: args{
				expose: []string{"99/tcp-100/tcp"},
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CreateExpose(tt.args.expose)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateExpose() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CreateExpose() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_parseAndValidatePort(t *testing.T) {
	type args struct {
		port string
	}
	tests := []struct {
		name    string
		args    args
		want    uint16
		wantErr bool
	}{
		{
			name: "0 should fail",
			args: args{
				port: "0",
			},
			want:    0,
			wantErr: true,
		},
		{
			name: "over 65535 should fail",
			args: args{
				port: "66666",
			},
			want:    0,
			wantErr: true,
		},
		{
			name: "",
			args: args{
				port: "99",
			},
			want:    99,
			wantErr: false,
		},
		{
			name: "negative values should fail",
			args: args{
				port: "-1",
			},
			want:    0,
			wantErr: true,
		},
		{
			name: "protocol should fail",
			args: args{
				port: "99/tcp",
			},
			want:    0,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseAndValidatePort(tt.args.port)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseAndValidatePort() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseAndValidatePort() got = %v, want %v", got, tt.want)
			}
		})
	}
}
