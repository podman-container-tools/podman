//go:build (amd64 && !windows) || (arm64 && !windows)

package define

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"go.podman.io/podman/v6/test/utils"
)

func TestMachineFile_GetPath(t *testing.T) {
	path := "/var/tmp/podman/my.sock"
	sym := "/tmp/podman/my.sock"
	type fields struct {
		Path    string
		Symlink *string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name:   "Original path",
			fields: fields{path, nil},
			want:   path,
		},
		{
			name: "Symlink over path",
			fields: fields{
				Path:    path,
				Symlink: &sym,
			},
			want: sym,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &VMFile{
				Path:    tt.fields.Path,    //nolint: scopelint
				Symlink: tt.fields.Symlink, //nolint: scopelint
			}
			if got := m.GetPath(); got != tt.want { //nolint: scopelint
				t.Errorf("GetPath() = %v, want %v", got, tt.want) //nolint: scopelint
			}
		})
	}
}

func TestNewMachineFile(t *testing.T) {
	empty := ""

	homedir := t.TempDir()
	longTemp := t.TempDir()
	t.Setenv("HOME", homedir)

	p := "/var/tmp/podman/my.sock"
	longp := filepath.Join(longTemp, utils.RandomString(100), "my.sock")
	err := os.MkdirAll(filepath.Dir(longp), 0o755)
	if err != nil {
		panic(err)
	}
	f, err := os.Create(longp)
	if err != nil {
		panic(err)
	}
	_ = f.Close()
	sym := "my.sock"
	longSym := filepath.Join(homedir, ".podman", sym)

	m := VMFile{
		Path:    p,
		Symlink: nil,
	}
	type args struct {
		path    string
		symlink *string
	}
	tests := []struct {
		name    string
		args    args
		want    *VMFile
		wantErr bool
	}{
		{
			name:    "Good",
			args:    args{path: p},
			want:    &m,
			wantErr: false,
		},
		{
			name:    "Good with short symlink",
			args:    args{p, &sym},
			want:    &VMFile{Path: p},
			wantErr: false,
		},
		{
			name:    "Bad path name",
			args:    args{empty, nil},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "Bad symlink name",
			args:    args{p, &empty},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "Good with long symlink",
			args:    args{longp, &sym},
			want:    &VMFile{Path: longp, Symlink: &longSym},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewMachineFile(tt.args.path, tt.args.symlink)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewMachineFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewMachineFile() got = %v, want %v", got, tt.want)
			}
		})
	}
}
