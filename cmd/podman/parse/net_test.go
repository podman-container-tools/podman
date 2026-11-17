//go:build !windows

// most of these validate and parse functions have been taken from projectatomic/docker
// and modified for cri-o
package parse

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.podman.io/common/libnetwork/etchosts"
)

var Var1 = []string{"ONE=1", "TWO=2"}

func createTmpFile(content []byte) (string, error) {
	tmpfile, err := os.CreateTemp(os.TempDir(), "unittest")
	if err != nil {
		return "", err
	}

	if _, err := tmpfile.Write(content); err != nil {
		return "", err
	}
	if err := tmpfile.Close(); err != nil {
		return "", err
	}
	return tmpfile.Name(), nil
}

func TestValidateExtraHost(t *testing.T) {
	type args struct {
		val string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		// 2001:0db8:85a3:0000:0000:8a2e:0370:7334
		{name: "good-ipv4", args: args{val: "foobar:192.168.1.1"}, want: "foobar:192.168.1.1", wantErr: false},
		{name: "good-multiple-ipv4", args: args{val: "host1;host2;host3:192.168.1.1"}, want: "host1;host2;host3:192.168.1.1", wantErr: false},
		{name: "bad-ipv4", args: args{val: "foobar:999.999.999.99"}, want: "", wantErr: true},
		{name: "bad-ipv4", args: args{val: "foobar:999.999.999"}, want: "", wantErr: true},
		{name: "bad-multiple-ipv4", args: args{val: "host1;host2;host3:999.999.999"}, want: "", wantErr: true},
		{name: "noname-ipv4", args: args{val: "192.168.1.1"}, want: "", wantErr: true},
		{name: "noname-ipv4", args: args{val: ":192.168.1.1"}, want: "", wantErr: true},
		{name: "noname-multiple-ipv4", args: args{val: "host1;;host3:192.168.1.1"}, want: "", wantErr: true},
		{name: "noip", args: args{val: "foobar:"}, want: "", wantErr: true},
		{name: "noip", args: args{val: "foobar"}, want: "", wantErr: true},
		{name: "good-ipv6", args: args{val: "foobar:2001:0db8:85a3:0000:0000:8a2e:0370:7334"}, want: "foobar:2001:0db8:85a3:0000:0000:8a2e:0370:7334", wantErr: false},
		{name: "good-multiple-ipv6", args: args{val: "host1;host2;host3:2001:0db8:85a3:0000:0000:8a2e:0370:7334"}, want: "host1;host2;host3:2001:0db8:85a3:0000:0000:8a2e:0370:7334", wantErr: false},
		{name: "bad-ipv6", args: args{val: "foobar:0db8:85a3:0000:0000:8a2e:0370:7334"}, want: "", wantErr: true},
		{name: "bad-ipv6", args: args{val: "foobar:0db8:85a3:0000:0000:8a2e:0370:7334.0000.0000.000"}, want: "", wantErr: true},
		{name: "bad-multiple-ipv6", args: args{val: "host1;host2;host3:0db8:85a3:0000:0000:8a2e:0370:7334.0000.0000.000"}, want: "", wantErr: true},
		{name: "noname-ipv6", args: args{val: "2001:0db8:85a3:0000:0000:8a2e:0370:7334"}, want: "", wantErr: true},
		{name: "noname-ipv6", args: args{val: ":2001:0db8:85a3:0000:0000:8a2e:0370:7334"}, want: "", wantErr: true},
		{name: "noname-multiple-ipv6", args: args{val: "host1;;host3:2001:0db8:85a3:0000:0000:8a2e:0370:7334"}, want: "", wantErr: true},
		{name: "host-gateway", args: args{val: "foobar:host-gateway"}, want: fmt.Sprintf("foobar:%s", etchosts.HostGateway), wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateExtraHost(tt.args.val)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateExtraHost() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ValidateExtraHost() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_validateIPAddress(t *testing.T) {
	type args struct {
		val string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{name: "ipv4-good", args: args{val: "192.168.1.1"}, want: "192.168.1.1", wantErr: false},
		{name: "ipv4-bad", args: args{val: "192.168.1.1.1"}, want: "", wantErr: true},
		{name: "ipv4-bad", args: args{val: "192."}, want: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validateIPAddress(tt.args.val)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateIPAddress() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("validateIPAddress() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateFileName(t *testing.T) {
	type args struct {
		filename string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{name: "good", args: args{filename: "/some/rand/path"}, wantErr: false},
		{name: "good", args: args{filename: "some/rand/path"}, wantErr: false},
		{name: "good", args: args{filename: "/"}, wantErr: false},
		{name: "bad", args: args{filename: "/:"}, wantErr: true},
		{name: "bad", args: args{filename: ":/"}, wantErr: true},
		{name: "bad", args: args{filename: "/some/rand:/path"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateFileName(tt.args.filename); (err != nil) != tt.wantErr {
				t.Errorf("ValidateFileName() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetAllLabels(t *testing.T) {
	fileLabels := []string{}
	labels, _ := GetAllLabels(fileLabels, Var1)
	assert.Equal(t, len(labels), 2)
}

func TestGetAllLabelsBadKeyValue(t *testing.T) {
	inLabels := []string{"=badValue", "="}
	fileLabels := []string{}
	_, err := GetAllLabels(fileLabels, inLabels)
	assert.Error(t, err, assert.AnError)
}

func TestGetAllLabelsBadLabelFile(t *testing.T) {
	fileLabels := []string{"/foobar5001/be"}
	_, err := GetAllLabels(fileLabels, Var1)
	assert.Error(t, err, assert.AnError)
}

func TestGetAllLabelsFile(t *testing.T) {
	content := []byte("THREE=3")
	tFile, err := createTmpFile(content)
	defer os.Remove(tFile)
	assert.NoError(t, err)
	fileLabels := []string{tFile}
	result, _ := GetAllLabels(fileLabels, Var1)
	assert.Equal(t, len(result), 3)
}

func TestValidWebURL(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "Valid HTTP URL",
			input:   "http://example.com",
			wantErr: false,
		},
		{
			name:    "Valid HTTPS URL",
			input:   "https://example.com",
			wantErr: false,
		},
		{
			name:    "Missing scheme",
			input:   "example.com",
			wantErr: true,
		},
		{
			name:    "Unsupported scheme - FTP",
			input:   "ftp://example.com",
			wantErr: true,
		},
		{
			name:    "Missing host",
			input:   "https://",
			wantErr: true,
		},
		{
			name:    "Local file path - Windows style",
			input:   "C:/hello/world",
			wantErr: true,
		},
		{
			name:    "Local file path - Unix style",
			input:   "/usr/local/bin",
			wantErr: true,
		},
		{
			name:    "Invalid URL characters",
			input:   "https://example.com/%%%",
			wantErr: true,
		},
		{
			name:    "Valid URL with port",
			input:   "https://example.com:8080",
			wantErr: false,
		},
		{
			name:    "Valid URL with path",
			input:   "https://example.com/path/to/resource",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidWebURL(tt.input)
			assert.Equal(t, tt.wantErr, err != nil, "ValidWebURL(%q) = %v, wantErr %v", tt.input, err, tt.wantErr)
		})
	}
}
