// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2025 The Ebitengine Authors

//go:build !cgo

package fakecgo

import "structs"

type (
	pthread_cond_t  uintptr
	pthread_mutex_t uintptr
)

var (
	PTHREAD_COND_INITIALIZER  = pthread_cond_t(0)
	PTHREAD_MUTEX_INITIALIZER = pthread_mutex_t(0)
)

// Source: https://github.com/NetBSD/src/blob/613e27c65223fd2283b6ed679da1197e12f50e27/sys/sys/signal.h#L225
type stack_t struct {
	_        structs.HostLayout
	ss_sp    uintptr
	ss_size  uintptr
	ss_flags int32
}

// Source: https://github.com/NetBSD/src/blob/613e27c65223fd2283b6ed679da1197e12f50e27/sys/sys/signal.h#L261
const SS_DISABLE = 0x004
