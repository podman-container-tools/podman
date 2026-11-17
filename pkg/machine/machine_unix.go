//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package machine

import (
	"context"
	"errors"
	"fmt"
	"net"
)

func DialNamedPipe(_ context.Context, _ string) (net.Conn, error) {
	return nil, errors.New("not implemented")
}

func GetEnvSetString(env string, val string) string {
	return fmt.Sprintf("export %s='%s'", env, val)
}
