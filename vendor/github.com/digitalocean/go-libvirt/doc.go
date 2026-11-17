// Copyright 2016 The go-libvirt Authors.
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

// Package libvirt is a pure Go interface to libvirt.
//
// Rather than using Libvirt's C bindings, this package makes use of Libvirt's
// RPC interface, as documented here: https://libvirt.org/internals/rpc.html.
// Connections to the libvirt server may be local, or remote. RPC packets are
// encoded using the XDR standard as defined by RFC 4506.
//
// Example usage:
//
//	package main
//
//	import (
//		"fmt"
//		"log"
//		"net"
//		"time"
//
//		"github.com/digitalocean/go-libvirt"
//	)
//
//	func main() {
//		// This dials libvirt on the local machine, but you can substitute the first
//		// two parameters with "tcp", "<ip address>:<port>" to connect to libvirt on
//		// a remote machine.
//		c, err := net.DialTimeout("unix", "/var/run/libvirt/libvirt-sock", 2*time.Second)
//		if err != nil {
//			log.Fatalf("failed to dial libvirt: %v", err)
//		}
//
//		l := libvirt.New(c)
//		if err := l.Connect(); err != nil {
//			log.Fatalf("failed to connect: %v", err)
//		}
//
//		v, err := l.Version()
//		if err != nil {
//			log.Fatalf("failed to retrieve libvirt version: %v", err)
//		}
//		fmt.Println("Version:", v)
//
//		domains, err := l.Domains()
//		if err != nil {
//			log.Fatalf("failed to retrieve domains: %v", err)
//		}
//
//		fmt.Println("ID\tName\t\tUUID")
//		fmt.Printf("--------------------------------------------------------\n")
//		for _, d := range domains {
//			fmt.Printf("%d\t%s\t%x\n", d.ID, d.Name, d.UUID)
//		}
//
//		if err := l.Disconnect(); err != nil {
//			log.Fatalf("failed to disconnect: %v", err)
//		}
//	}
package libvirt
