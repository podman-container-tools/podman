// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2022 The Ebitengine Authors

//go:build !cgo

package fakecgo

type (
	pthread_cond_t  uintptr
	pthread_mutex_t uintptr
)

var (
	PTHREAD_COND_INITIALIZER  = pthread_cond_t(0)
	PTHREAD_MUTEX_INITIALIZER = pthread_mutex_t(0)
)

// Source: https://github.com/freebsd/freebsd-src/blob/f84cdc2584305f8b656ce3f3746868c4a58a9a82/sys/sys/signal.h#L470
const (
	SIG_BLOCK   sighow = 1
	SIG_UNBLOCK sighow = 2
	SIG_SETMASK sighow = 3
)

type stack_t struct {
	/* not implemented */
}
