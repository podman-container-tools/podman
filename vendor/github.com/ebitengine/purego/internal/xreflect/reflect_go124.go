// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2025 The Ebitengine Authors

//go:build !go1.25

package xreflect

import "reflect"

// TODO: remove this and use Go 1.25's reflect.TypeAssert when minimum go.mod version is 1.25

func TypeAssert[T any](v reflect.Value) (T, bool) {
	v2, ok := v.Interface().(T)
	return v2, ok
}
