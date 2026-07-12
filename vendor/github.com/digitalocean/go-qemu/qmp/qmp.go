// Copyright 2016 The go-qemu Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package qmp enables interaction with QEMU instances
// via the QEMU Machine Protocol (QMP).
package qmp

import (
	"context"
	"errors"
	"fmt"
)

// ErrEventsNotSupported is returned by Events() if event streams
// are unsupported by either QEMU or libvirt.
var ErrEventsNotSupported = errors.New("event monitor is not supported")

// Monitor represents a QEMU Machine Protocol socket.
// See: http://wiki.qemu.org/QMP
type Monitor interface {
	Connect() error
	Disconnect() error
	Run(command []byte) (out []byte, err error)
	Events(context.Context) (events <-chan Event, err error)
}

// Command represents a QMP command.
type Command struct {
	// Name of the command to run
	Execute string `json:"execute"`

	// Optional arguments for the above command.
	Args interface{} `json:"arguments,omitempty"`
}

type response struct {
	ID     string      `json:"id"`
	Return interface{} `json:"return,omitempty"`
	Error  struct {
		Class string `json:"class"`
		Desc  string `json:"desc"`
	} `json:"error,omitempty"`
}

func (r *response) Err() error {
	if r.Error.Desc == "" {
		return nil
	}

	return errors.New(r.Error.Desc)
}

// Event represents a QEMU QMP event.
// See http://wiki.qemu.org/QMP
type Event struct {
	// Event name, e.g., BLOCK_JOB_COMPLETE
	Event string `json:"event"`

	// Arbitrary event data
	Data map[string]interface{} `json:"data"`

	// Event timestamp, provided by QEMU.
	Timestamp struct {
		Seconds      int64 `json:"seconds"`
		Microseconds int64 `json:"microseconds"`
	} `json:"timestamp"`
}

// Version is the QEMU version structure returned when a QMP connection is
// initiated.
type Version struct {
	Package string `json:"package"`
	QEMU    struct {
		Major int `json:"major"`
		Micro int `json:"micro"`
		Minor int `json:"minor"`
	} `json:"qemu"`
}

func (v Version) String() string {
	q := v.QEMU
	return fmt.Sprintf("%d.%d.%d", q.Major, q.Minor, q.Micro)
}
