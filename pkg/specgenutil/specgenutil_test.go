//go:build linux

package specgenutil

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.podman.io/common/pkg/machine"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/specgen"
)

func TestWinPath(t *testing.T) {
	const (
		fail = false
		pass = true
	)
	tests := []struct {
		vol     string
		source  string
		dest    string
		isN     bool
		outcome bool
		mach    string
	}{
		{`C:\Foo:/blah`, "/mnt/c/Foo", "/blah", false, pass, "wsl"},
		{`C:\Foo:/blah`, "/mnt/c/Foo", "/blah", false, fail, ""},
		{`\\?\C:\Foo:/blah`, "/mnt/c/Foo", "/blah", false, pass, "wsl"},
		{`/c/bar:/blah`, "/mnt/c/bar", "/blah", false, pass, "wsl"},
		{`/c/bar:/blah`, "/c/bar", "/blah", false, pass, ""},
		{`/test/this:/blah`, "/test/this", "/blah", false, pass, "wsl"},
		{`c:/bar/something:/other`, "/mnt/c/bar/something", "/other", false, pass, "wsl"},
		{`c:/foo:ro`, "c", "/foo", true, pass, ""},
		{`\\computer\loc:/dest`, "", "", false, fail, "wsl"},
		{`\\.\drive\loc:/target`, "/mnt/wsl/drive/loc", "/target", false, pass, "wsl"},
	}

	f := func(vol string, mach string) (*specgen.SpecGenerator, error) {
		machine := machine.GetMachineMarker()
		oldEnable, oldType := machine.Enabled, machine.Type
		machine.Enabled, machine.Type = len(mach) > 0, mach
		sg := specgen.NewSpecGenerator("nothing", false)
		err := FillOutSpecGen(sg, &entities.ContainerCreateOptions{
			ImageVolume: "ignore",
			Volume:      []string{vol},
		}, []string{},
		)
		machine.Enabled, machine.Type = oldEnable, oldType
		return sg, err
	}

	for _, test := range tests {
		msg := "Checking: " + test.vol
		sg, err := f(test.vol, test.mach)
		if test.outcome == fail {
			assert.NotNil(t, err, msg)
			continue
		}
		if !assert.NoError(t, err, msg) {
			continue
		}
		if test.isN {
			if !assert.Equal(t, 1, len(sg.Volumes), msg) {
				continue
			}
			assert.Equal(t, test.source, sg.Volumes[0].Name, msg)
			assert.Equal(t, test.dest, sg.Volumes[0].Dest, msg)
		} else {
			if !assert.Equal(t, 1, len(sg.Mounts), msg) {
				continue
			}
			assert.Equal(t, test.source, sg.Mounts[0].Source, msg)
			assert.Equal(t, test.dest, sg.Mounts[0].Destination, msg)
		}
	}
}

func TestParseLinuxResourcesDeviceAccess(t *testing.T) {
	d, err := parseLinuxResourcesDeviceAccess("a *:* rwm")
	assert.NoError(t, err, "err is nil")
	assert.True(t, d.Allow, "allow is true")
	assert.Equal(t, d.Type, "a", "type is 'a'")
	assert.Nil(t, d.Minor, "minor is nil")
	assert.Nil(t, d.Major, "major is nil")

	d, err = parseLinuxResourcesDeviceAccess("b 3:* rwm")
	assert.NoError(t, err, "err is nil")
	assert.True(t, d.Allow, "allow is true")
	assert.Equal(t, d.Type, "b", "type is 'b'")
	assert.Nil(t, d.Minor, "minor is nil")
	assert.NotNil(t, d.Major, "major is not nil")
	assert.Equal(t, *d.Major, int64(3), "major is 3")

	d, err = parseLinuxResourcesDeviceAccess("a *:3 rwm")
	assert.NoError(t, err, "err is nil")
	assert.True(t, d.Allow, "allow is true")
	assert.Equal(t, d.Type, "a", "type is 'a'")
	assert.Nil(t, d.Major, "major is nil")
	assert.NotNil(t, d.Minor, "minor is not nil")
	assert.Equal(t, *d.Minor, int64(3), "minor is 3")

	d, err = parseLinuxResourcesDeviceAccess("c 1:2 rwm")
	assert.NoError(t, err, "err is nil")
	assert.True(t, d.Allow, "allow is true")
	assert.Equal(t, d.Type, "c", "type is 'c'")
	assert.NotNil(t, d.Major, "minor is not nil")
	assert.Equal(t, *d.Major, int64(1), "minor is 1")
	assert.NotNil(t, d.Minor, "minor is not nil")
	assert.Equal(t, *d.Minor, int64(2), "minor is 2")

	_, err = parseLinuxResourcesDeviceAccess("q *:* rwm")
	assert.NotNil(t, err, "err is not nil")

	_, err = parseLinuxResourcesDeviceAccess("a a:* rwm")
	assert.NotNil(t, err, "err is not nil")

	_, err = parseLinuxResourcesDeviceAccess("a *:a rwm")
	assert.NotNil(t, err, "err is not nil")

	_, err = parseLinuxResourcesDeviceAccess("a *:* abc")
	assert.NotNil(t, err, "err is not nil")

	_, err = parseLinuxResourcesDeviceAccess("* *:* *")
	assert.NotNil(t, err, "err is not nil")

	_, err = parseLinuxResourcesDeviceAccess("* *:a2 *")
	assert.NotNil(t, err, "err is not nil")

	_, err = parseLinuxResourcesDeviceAccess("*")
	assert.NotNil(t, err, "err is not nil")

	_, err = parseLinuxResourcesDeviceAccess("*:*")
	assert.NotNil(t, err, "err is not nil")

	_, err = parseLinuxResourcesDeviceAccess("a *:*")
	assert.NotNil(t, err, "err is not nil")

	_, err = parseLinuxResourcesDeviceAccess("a *:*")
	assert.NotNil(t, err, "err is not nil")

	_, err = parseLinuxResourcesDeviceAccess("a 12a:* r")
	assert.NotNil(t, err, "err is not nil")

	_, err = parseLinuxResourcesDeviceAccess("a a12:* r")
	assert.NotNil(t, err, "err is not nil")

	_, err = parseLinuxResourcesDeviceAccess("a 0x1:* r")
	assert.NotNil(t, err, "err is not nil")

	_, err = parseLinuxResourcesDeviceAccess("a -2:* r")
	assert.NotNil(t, err, "err is not nil")

	_, err = parseLinuxResourcesDeviceAccess("a *:-3 r")
	assert.NotNil(t, err, "err is not nil")
}

func TestGenRlimits(t *testing.T) {
	testLimits := map[string]string{
		"core":       "1:2",
		"cpu":        "1:2",
		"data":       "1:2",
		"fsize":      "1:2",
		"locks":      "1:2",
		"memlock":    "1:2",
		"msgqueue":   "1:2",
		"nice":       "1:2",
		"nofile":     "1:2",
		"nproc":      "1:2",
		"rss":        "1:2",
		"rtprio":     "1:2",
		"rttime":     "1:2",
		"sigpending": "1:2",
		"stack":      "1:2",
	}

	lowerCaseLimits := make([]string, 0, len(testLimits))
	upperCaseLimitsWithPrefix := make([]string, 0, len(testLimits))
	for name, limit := range testLimits {
		lowerCaseLimits = append(lowerCaseLimits, fmt.Sprintf("%s=%s", name, limit))
		upperCaseLimitsWithPrefix = append(upperCaseLimitsWithPrefix, strings.ToUpper(fmt.Sprintf("%s%s=%s", rlimitPrefix, name, limit)))
	}

	parsedLimits, err := GenRlimits(lowerCaseLimits)
	assert.NoError(t, err, "err is nil")

	for _, limit := range parsedLimits {
		val, ok := testLimits[limit.Type]
		assert.True(t, ok, "%s matched", limit.Type)
		assert.Equal(t, val, fmt.Sprintf("%d:%d", limit.Soft, limit.Hard), "soft and hard limits are equal")
	}

	parsedLimits, err = GenRlimits(upperCaseLimitsWithPrefix)
	assert.NoError(t, err, "err is nil")

	for _, limit := range parsedLimits {
		val, ok := testLimits[limit.Type]
		assert.True(t, ok, "%s matched", limit.Type)
		assert.Equal(t, val, fmt.Sprintf("%d:%d", limit.Soft, limit.Hard), "soft and hard limits are equal")
	}

	// go-utils module has no tests for the parser.
	_, err = GenRlimits([]string{"foo=1:1"})
	assert.Error(t, err, "err is not nil")

	_, err = GenRlimits([]string{"RLIMIT_FOO=1:1"})
	assert.Error(t, err, "err is not nil")

	_, err = GenRlimits([]string{"nofile"})
	assert.Error(t, err, "err is not nil")

	_, err = GenRlimits([]string{"nofile=bar"})
	assert.Error(t, err, "err is not nil")

	_, err = GenRlimits([]string{"nofile=bar:buzz"})
	assert.Error(t, err, "err is not nil")
}

func TestFillOutSpecGenRecorsUserNs(t *testing.T) {
	sg := specgen.NewSpecGenerator("nothing", false)
	err := FillOutSpecGen(sg, &entities.ContainerCreateOptions{
		ImageVolume: "ignore",
		UserNS:      "keep-id",
	}, []string{})
	assert.NoError(t, err)

	v, ok := sg.Annotations[define.UserNsAnnotation]
	assert.True(t, ok, "UserNsAnnotation is set")
	assert.Equal(t, "keep-id", v, "UserNsAnnotation is keep-id")
}

func TestLogOptMaxFile(t *testing.T) {
	tests := []struct {
		desc      string
		logOpts   []string
		wantVal   string
		wantErr   bool
	}{
		{
			desc:    "valid max-file integer",
			logOpts: []string{"max-file=5"},
			wantVal: "5",
		},
		{
			desc:    "max-file of 1",
			logOpts: []string{"max-file=1"},
			wantVal: "1",
		},
		{
			desc:    "invalid max-file string",
			logOpts: []string{"max-file=notanumber"},
			wantErr: true,
		},
		{
			desc:    "invalid max-file negative",
			logOpts: []string{"max-file=-3"},
			wantErr: true,
		},
		{
			desc:    "invalid max-file zero",
			logOpts: []string{"max-file=0"},
			wantErr: true,
		},
		{
			desc:    "conflicting max-file with log-rotate=false",
			logOpts: []string{"max-file=5", "log-rotate=false"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			sg := specgen.NewSpecGenerator("nothing", false)
			sg.LogConfiguration = &specgen.LogConfig{}
			err := FillOutSpecGen(sg, &entities.ContainerCreateOptions{
				ImageVolume: "ignore",
				LogOptions:  tt.logOpts,
			}, []string{})
			if tt.wantErr {
				assert.Error(t, err, "expected an error for %q", tt.logOpts)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.wantVal, sg.LogConfiguration.Options["max-file"],
				"max-file stored in LogConfiguration.Options")
		})
	}
}

func TestLogOptLogRotate(t *testing.T) {
	tests := []struct {
		desc    string
		logOpts []string
		wantVal string
		wantErr bool
	}{
		{
			desc:    "log-rotate true",
			logOpts: []string{"log-rotate=true"},
			wantVal: "true",
		},
		{
			desc:    "log-rotate false",
			logOpts: []string{"log-rotate=false"},
			wantVal: "false",
		},
		{
			desc:    "log-rotate 1",
			logOpts: []string{"log-rotate=1"},
			wantVal: "1",
		},
		{
			desc:    "invalid log-rotate value",
			logOpts: []string{"log-rotate=maybe"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			sg := specgen.NewSpecGenerator("nothing", false)
			sg.LogConfiguration = &specgen.LogConfig{}
			err := FillOutSpecGen(sg, &entities.ContainerCreateOptions{
				ImageVolume: "ignore",
				LogOptions:  tt.logOpts,
			}, []string{})
			if tt.wantErr {
				assert.Error(t, err, "expected an error for %q", tt.logOpts)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.wantVal, sg.LogConfiguration.Options["log-rotate"],
				"log-rotate stored in LogConfiguration.Options")
		})
	}
}
