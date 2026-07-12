package specgenutil

import "testing"

func Test_verifyExpose(t *testing.T) {
	type args struct {
		expose []string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "single port with tcp",
			args: args{
				expose: []string{"53/tcp"},
			},
			wantErr: false,
		},
		{
			name: "single port with udp",
			args: args{
				expose: []string{"53/udp"},
			},
			wantErr: false,
		},
		{
			name: "good port range",
			args: args{
				expose: []string{"100-133"},
			},
			wantErr: false,
		},
		{
			name: "high to low should fail",
			args: args{
				expose: []string{"100-99"},
			},
			wantErr: true,
		},
		{
			name: "range with protocol",
			args: args{
				expose: []string{"53/tcp-55/tcp"},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := verifyExpose(tt.args.expose); (err != nil) != tt.wantErr {
				t.Errorf("verifyExpose() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
