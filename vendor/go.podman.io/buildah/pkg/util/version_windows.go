package util //nolint:revive,nolintlint

import (
	"errors"
)

func ReadKernelVersion() (string, error) {
	return "", errors.New("readKernelVersion not supported on windows")
}
