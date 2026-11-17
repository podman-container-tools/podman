// Copyright 2018 The gVisor Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package p9

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	// highestSupportedVersion is the highest supported version X in a
	// version string of the format 9P2000.L.Google.X.
	//
	// Clients are expected to start requesting this version number and
	// to continuously decrement it until a Tversion request succeeds.
	highestSupportedVersion uint32 = 7

	// lowestSupportedVersion is the lowest supported version X in a
	// version string of the format 9P2000.L.Google.X.
	//
	// Clients are free to send a Tversion request at a version below this
	// value but are expected to encounter an Rlerror in response.
	lowestSupportedVersion uint32 = 0
)

type baseVersion string

const (
	undetermined   baseVersion = ""
	version9P2000  baseVersion = "9P2000"
	version9P2000U baseVersion = "9P2000.u"
	version9P2000L baseVersion = "9P2000.L"
)

// HighestVersionString returns the highest possible version string that a client
// may request or a server may support.
func HighestVersionString() string {
	return versionString(version9P2000L, highestSupportedVersion)
}

// parseVersion parses a Tversion version string into a numeric version number
// if the version string is supported by p9.  Otherwise returns (0, false).
//
// From Tversion(9P): "Version strings are defined such that, if the client string
// contains one or more period characters, the initial substring up to but not
// including any single period in the version string defines a version of the protocol."
//
// p9 intentionally diverges from this and always requires that the version string
// start with 9P2000.L to express that it is always compatible with 9P2000.L.  The
// only supported versions extensions are of the format 9p2000.L.Google.X where X
// is an ever increasing version counter.
//
// Version 9P2000.L.Google.0 implies 9P2000.L.
//
// New versions must always be a strict superset of 9P2000.L. A version increase must
// define a predicate representing the feature extension introduced by that version. The
// predicate must be commented and should take the format:
//
// // VersionSupportsX returns true if version v supports X and must be checked when ...
//
//	func VersionSupportsX(v int32) bool {
//		...
//
// )
func parseVersion(str string) (baseVersion, uint32, bool) {
	switch str {
	case "9P2000.L":
		return version9P2000L, 0, true
	case "9P2000.u":
		return version9P2000U, 0, true
	case "9P2000":
		return version9P2000, 0, true
	default:
		substr := strings.Split(str, ".")
		if len(substr) != 4 {
			return "", 0, false
		}
		if substr[0] != "9P2000" || substr[1] != "L" || substr[2] != "Google" || len(substr[3]) == 0 {
			return "", 0, false
		}
		version, err := strconv.ParseUint(substr[3], 10, 32)
		if err != nil {
			return "", 0, false
		}
		return version9P2000L, uint32(version), true
	}
}

// versionString formats a p9 version number into a Tversion version string.
func versionString(baseVersion baseVersion, version uint32) string {
	// Special case the base version so that clients expecting this string
	// instead of the 9P2000.L.Google.0 equivalent get it.  This is important
	// for backwards compatibility with legacy servers that check for exactly
	// the baseVersion and allow nothing else.
	if version == 0 {
		return string(baseVersion)
	}
	return fmt.Sprintf("9P2000.L.Google.%d", version)
}

// versionSupportsTwalkgetattr returns true if version v supports the
// Twalkgetattr message. This predicate must be checked by clients before
// attempting to make a Twalkgetattr request.
func versionSupportsTwalkgetattr(v uint32) bool {
	return v >= 2
}

// versionSupportsTucreation returns true if version v supports the Tucreation
// messages (Tucreate, Tusymlink, Tumkdir, Tumknod). This predicate must be
// checked by clients before attempting to make a Tucreation request.
// If Tucreation messages are not supported, their non-UID supporting
// counterparts (Tlcreate, Tsymlink, Tmkdir, Tmknod) should be used.
func versionSupportsTucreation(v uint32) bool {
	return v >= 3
}

// VersionSupportsMultiUser returns true if version v supports multi-user fake
// directory permissions and ID values.
func VersionSupportsMultiUser(v uint32) bool {
	return v >= 6
}
