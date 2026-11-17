// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The Ebitengine Authors

package purego

// CDecl marks a function as being called using the __cdecl calling convention as defined in
// the [MSDocs] when passed to NewCallback. It must be the first argument to the function.
// This is only useful on 386 Windows, but it is safe to use on other platforms.
//
// [MSDocs]: https://learn.microsoft.com/en-us/cpp/cpp/cdecl?view=msvc-170
type CDecl struct{}
