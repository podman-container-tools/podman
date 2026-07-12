package signal

import (
	"syscall"
	"testing"
)

func TestParseSignal(t *testing.T) {
	type args struct {
		rawSignal string
	}
	tests := []struct {
		name    string
		args    args
		want    syscall.Signal
		wantErr bool
	}{
		{
			name: "KILL to SIGKILL",
			args: args{
				rawSignal: "KILL",
			},
			want:    syscall.SIGKILL,
			wantErr: false,
		},
		{
			name: "Case does not matter",
			args: args{
				rawSignal: "kIlL",
			},
			want:    syscall.SIGKILL,
			wantErr: false,
		},
		{
			name: "Garbage signal",
			args: args{
				rawSignal: "FOO",
			},
			want:    -1,
			wantErr: true,
		},
		{
			name: "Signal with prepended SIG",
			args: args{
				rawSignal: "SIGKILL",
			},
			want:    syscall.SIGKILL,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSignal(tt.args.rawSignal)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseSignal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseSignal() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseSignalNameOrNumber(t *testing.T) {
	type args struct {
		rawSignal string
	}
	tests := []struct {
		name    string
		args    args
		want    syscall.Signal
		wantErr bool
	}{
		{
			name: "Kill should work",
			args: args{
				rawSignal: "kill",
			},
			want:    syscall.SIGKILL,
			wantErr: false,
		},
		{
			name: "9 for kill should work",
			args: args{
				rawSignal: "9",
			},
			want:    syscall.SIGKILL,
			wantErr: false,
		},
		{
			name: "Non-defined signal number should work",
			args: args{
				rawSignal: "923",
			},
			want:    923,
			wantErr: false,
		},
		{
			name: "garbage should fail",
			args: args{
				rawSignal: "foo",
			},
			want:    -1,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSignalNameOrNumber(tt.args.rawSignal)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseSignalNameOrNumber() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseSignalNameOrNumber() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseSysSignalToName(t *testing.T) {
	type args struct {
		signal syscall.Signal
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "Kill should work",
			args: args{
				signal: syscall.SIGKILL,
			},
			want:    "KILL",
			wantErr: false,
		},
		{
			name: "Non-defined signal number should not work",
			args: args{
				signal: 923,
			},
			want:    "",
			wantErr: true,
		},
		{
			name: "garbage should fail",
			args: args{
				signal: -1,
			},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSysSignalToName(tt.args.signal)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseSysSignalToName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseSysSignalToName() got = %v, want %v", got, tt.want)
			}
		})
	}
}
