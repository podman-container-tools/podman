package util

import (
	"testing"
)

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
