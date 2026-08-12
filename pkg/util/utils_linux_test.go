package util

import (
	"os/user"
	"path/filepath"
	"testing"
)

func TestGetRootlessConfigHomeDirWithRootHome(t *testing.T) {
	u, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "/")

	configDir, err := GetRootlessConfigHomeDir()
	if err != nil {
		t.Fatal(err)
	}

	expected := filepath.Join(u.HomeDir, ".config")
	if configDir != expected {
		t.Fatalf("expected %q, got %q", expected, configDir)
	}
}

func TestIsVirtualConsoleDevice(t *testing.T) {
	testcases := []struct {
		expectedResult bool
		path           string
	}{
		{
			expectedResult: true,
			path:           "/dev/tty10",
		},
		{
			expectedResult: false,
			path:           "/dev/tty",
		},
		{
			expectedResult: false,
			path:           "/dev/ttyUSB0",
		},
		{
			expectedResult: false,
			path:           "/dev/tty0abcd",
		},
		{
			expectedResult: false,
			path:           "1234",
		},
		{
			expectedResult: false,
			path:           "abc",
		},
		{
			expectedResult: false,
			path:           " ",
		},
		{
			expectedResult: false,
			path:           "",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.path, func(t *testing.T) {
			result := isVirtualConsoleDevice(tc.path)
			if result != tc.expectedResult {
				t.Errorf("isVirtualConsoleDevice returned %t, expected %t", result, tc.expectedResult)
			}
		})
	}
}
