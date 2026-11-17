package util //nolint:revive,nolintlint

import (
	"errors"
	"time"
)

func ReadUptime() (time.Duration, error) {
	return 0, errors.New("readUptime not supported on darwin")
}
