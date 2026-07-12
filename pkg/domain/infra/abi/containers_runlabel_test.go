//go:build !remote && (linux || freebsd)

package abi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReplaceName(t *testing.T) {
	tests := [][]string{
		{"NAME=$NAME", "test1", "NAME=test1"},
		{"NAME=${NAME}", "test2", "NAME=test2"},
		{"NAME=NAME", "test3", "NAME=test3"},
		{"NAME=NAMEFOO", "test3", "NAME=NAMEFOO"},
		{"NAME", "test4", "test4"},
		{"FNAME", "test5", "FNAME"},
		{"NAME=foo", "test6", "NAME=foo"},
		{"This is my NAME", "test7", "This is my NAME"},
	}
	for _, args := range tests {
		val := replaceName(args[0], args[1])
		assert.Equal(t, val, args[2])
	}
}

func TestReplaceImage(t *testing.T) {
	tests := [][]string{
		{"IMAGE=$IMAGE", "test1", "IMAGE=test1"},
		{"IMAGE=${IMAGE}", "test2", "IMAGE=test2"},
		{"IMAGE=IMAGE", "test3", "IMAGE=test3"},
		{"IMAGE=IMAGEFOO", "test3", "IMAGE=IMAGEFOO"},
		{"IMAGE", "test4", "test4"},
		{"FIMAGE", "test5", "FIMAGE"},
		{"IMAGE=foo", "test6", "IMAGE=foo"},
	}
	for _, args := range tests {
		val := replaceImage(args[0], args[1])
		assert.Equal(t, val, args[2])
	}
}
