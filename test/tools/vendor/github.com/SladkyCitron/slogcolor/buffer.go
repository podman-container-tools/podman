package slogcolor

import (
	"bytes"
	"sync"
)

var bufPool = sync.Pool{
	New: func() any {
		return &bytes.Buffer{}
	},
}

func getBuffer() *bytes.Buffer {
	return bufPool.Get().(*bytes.Buffer)
}

func freeBuffer(bf *bytes.Buffer) {
	bufPool.Put(bf)
}
