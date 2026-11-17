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

package qmp

import (
	"context"
	"encoding/json"
	"net"

	"github.com/digitalocean/go-libvirt"
)

var _ Monitor = &LibvirtRPCMonitor{}

// A LibvirtRPCMonitor implements LibVirt's remote procedure call protocol.
type LibvirtRPCMonitor struct {
	l *libvirt.Libvirt
	// Domain name as seen by libvirt, e.g., stage-lb-1
	Domain string
}

// NewLibvirtRPCMonitor configures a new Libvirt RPC Monitor connection.
// The provided domain should be the name of the domain as seen
// by libvirt, e.g., stage-lb-1.
func NewLibvirtRPCMonitor(domain string, conn net.Conn) *LibvirtRPCMonitor {
	l := libvirt.New(conn)

	return &LibvirtRPCMonitor{
		l:      l,
		Domain: domain,
	}
}

// Connect establishes communication with the libvirt server.
// The underlying libvirt socket connection must be previously established.
func (rpc *LibvirtRPCMonitor) Connect() error {
	return rpc.l.Connect()
}

// Disconnect shuts down communication with the libvirt server
// and closes the underlying net.Conn.
func (rpc *LibvirtRPCMonitor) Disconnect() error {
	return rpc.l.Disconnect()
}

// Events streams QEMU QMP Events until the provided context is cancelled.
// If a problem is encountered setting up the event monitor connection
// an error will be returned. Errors encountered during streaming will
// cause the returned event channel to be closed.
func (rpc *LibvirtRPCMonitor) Events(ctx context.Context) (<-chan Event, error) {
	events, err := rpc.l.SubscribeQEMUEvents(ctx, rpc.Domain)
	if err != nil {
		return nil, err
	}

	c := make(chan Event)
	go func() {
		defer close(c)
		// process events
		for e := range events {
			qe, err := qmpEvent(&e)
			if err != nil {
				break
			}

			c <- *qe
		}
	}()

	return c, nil
}

// Run executes the given QAPI command against a domain's QEMU instance.
// For a list of available QAPI commands, see:
//	http://git.qemu.org/?p=qemu.git;a=blob;f=qapi-schema.json;hb=HEAD
func (rpc *LibvirtRPCMonitor) Run(cmd []byte) ([]byte, error) {
	return rpc.l.Run(rpc.Domain, cmd)
}

// qmpEvent takes a libvirt DomainEvent and returns the QMP equivalent.
func qmpEvent(e *libvirt.DomainEvent) (*Event, error) {
	var qe Event

	if e.Details != nil {
		if err := json.Unmarshal(e.Details, &qe.Data); err != nil {
			return nil, err
		}
	}

	qe.Event = e.Event
	qe.Timestamp.Seconds = int64(e.Seconds)
	qe.Timestamp.Microseconds = int64(e.Microseconds)

	return &qe, nil
}
