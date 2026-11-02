// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2022 The Ebitengine Authors

//go:build !cgo

package fakecgo

import "structs"

type (
	pthread_mutex_t struct {
		_      structs.HostLayout
		sig    int64
		opaque [56]byte
	}
	pthread_cond_t struct {
		_      structs.HostLayout
		sig    int64
		opaque [40]byte
	}
)

var (
	PTHREAD_COND_INITIALIZER  = pthread_cond_t{sig: 0x3CB0B1BB}
	PTHREAD_MUTEX_INITIALIZER = pthread_mutex_t{sig: 0x32AAABA7}
)

// Source: https://github.com/apple/darwin-xnu/blob/2ff845c2e033bd0ff64b5b6aa6063a1f8f65aa32/bsd/sys/signal.h#L488
const (
	SIG_BLOCK   sighow = 1
	SIG_UNBLOCK sighow = 2
	SIG_SETMASK sighow = 3
)

type stack_t struct {
	/* not implemented */
}
