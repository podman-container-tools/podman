// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package godoclink

import (
	"regexp"
	"strings"
)

// Markers bridge the two halves of idiom recomposition.
//
// At the consumption seam a resolvable doc-link is encoded as a marker carrying the referenced
// schema's fully-qualified definition key (plus an exposed field-chain suffix, a humanized
// fallback, and a sentence-initial titleize bit).
//
// After the spec builder reduces definition names, SubstituteMarkers rewrites each marker to the
// schema's final exposed name.
//
// The delimiters are NUL (\x00) and Unit Separator (\x1f): neither can appear in a Go source
// comment, so a marker never collides with real prose.
// The format is.
//
//	\x00gl\x1f<defKey>\x1f<suffix>\x1f<fallback>\x1f<0|1>\x00
const (
	markerOpen  = "\x00gl\x1f"
	markerClose = "\x00"
	markerSep   = "\x1f"
)

// markerRE matches one encoded marker, capturing defKey, suffix, fallback and the titleize bit.
//
// The field bodies exclude the two delimiter runes so a marker can never swallow an adjacent one.
var markerRE = regexp.MustCompile("\x00gl\x1f([^\x1f\x00]*)\x1f([^\x1f\x00]*)\x1f([^\x1f\x00]*)\x1f([01])\x00")

// encodeMarker builds a marker for a resolvable doc-link. defKey is the referenced type's
// definition key; suffix is the exposed field chain (already resolved, e.g. ".customer_name") or
// ""; fallback is the humanized leaf used when the key turns out not to be an emitted definition;
// titleize records sentence-initial position.
func encodeMarker(defKey, suffix, fallback string, titleize bool) string {
	bit := "0"
	if titleize {
		bit = "1"
	}

	return markerOpen + defKey + markerSep + suffix + markerSep + fallback + markerSep + bit + markerClose
}

// HasMarkers reports whether s contains any godoclink marker — a cheap guard so callers can skip
// the substitution walk for marker-free prose.
func HasMarkers(s string) bool {
	return strings.Contains(s, markerOpen)
}

// SubstituteMarkers rewrites every marker in text to its final exposed name. finalName maps a
// definition key to the name it is ultimately emitted under, returning ok=false when the key is not
// an emitted definition (pruned or unresolved); in that case the marker collapses to its humanized
// fallback.
//
// A resolved marker yields finalName+suffix.
// The sentence-initial bit, when set, upper-cases the first rune of the result.
// No marker ever survives this pass.
func SubstituteMarkers(text string, finalName func(defKey string) (string, bool)) string {
	if !HasMarkers(text) {
		return text
	}

	return markerRE.ReplaceAllStringFunc(text, func(m string) string {
		groups := markerRE.FindStringSubmatch(m)
		defKey, suffix, fallback, bit := groups[1], groups[2], groups[3], groups[4]

		var out string
		if name, ok := finalName(defKey); ok {
			out = name + suffix
		} else {
			out = fallback
		}
		if bit == "1" {
			out = capitalizeFirst(out)
		}

		return out
	})
}
