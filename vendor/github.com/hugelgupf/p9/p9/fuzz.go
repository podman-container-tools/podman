//go:build gofuzz

package p9

import (
	"bytes"

	"github.com/u-root/uio/ulog"
)

func Fuzz(data []byte) int {
	buf := bytes.NewBuffer(data)
	tag, msg, err := recv(ulog.Null, buf, DefaultMessageSize, msgDotLRegistry.get)
	if err != nil {
		if msg != nil {
			panic("msg !=nil on error")
		}
		return 0
	}
	buf.Reset()
	send(ulog.Null, buf, tag, msg)
	if err != nil {
		panic(err)
	}
	return 1
}
