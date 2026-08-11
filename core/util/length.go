// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// lengthUnitsToInches maps every unit ParseLengthInches accepts to its
// inches-per-unit factor. Longest suffix first is not needed here (unlike a
// regex) since the lookup below tries suffixes in decreasing length order
// explicitly — "mm" and "cm" are both 2 chars, "in"/"pt"/"px" are all 2
// chars too, so there is no shorter-prefix-shadows-longer-suffix hazard
// among these five, but the explicit order keeps that invariant visible
// instead of relying on map iteration (which Go does not guarantee).
var lengthUnitOrder = []string{"cm", "mm", "in", "pt", "px"}

var lengthUnitsToInches = map[string]float64{
	"cm": 1 / 2.54, // 1 inch = 2.54 cm
	"mm": 1 / 25.4, // 1 inch = 25.4 mm
	"in": 1,
	"pt": 1.0 / 72.0, // 1 inch = 72 points
	"px": 1.0 / 96.0, // CSS reference pixel: 96px = 1in
}

// ParseLengthInches parses an author-facing length string ("2cm", "0.5in",
// "72pt", "40px") into inches. The parser normalizes FORM (see
// core/parser/frontmatter.go's UnmarshalYAML methods for `page:`); this is
// the renderer-side counterpart that normalizes UNITS — kept as a pure
// function with no side effects so both a PDF renderer (inches) and a
// future DOCX renderer (twips = inches * 1440) can call it without either
// one owning unit-conversion logic the other would have to duplicate.
//
// A bare number with no unit suffix is rejected rather than assumed to mean
// inches or pixels — silently guessing a unit for a value like "2" (author
// meant cm? px? points?) would produce a plausible-looking but wrong
// document instead of a clear error the caller can warn about.
func ParseLengthInches(s string) (float64, error) {
	trimmed := strings.TrimSpace(s)
	lower := strings.ToLower(trimmed)

	for _, unit := range lengthUnitOrder {
		if !strings.HasSuffix(lower, unit) {
			continue
		}
		numPart := strings.TrimSpace(trimmed[:len(trimmed)-len(unit)])
		value, err := strconv.ParseFloat(numPart, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid length %q: %w", s, err)
		}
		// strconv.ParseFloat accepts "NaN"/"Inf"/"+Inf"/"-Inf" as valid
		// float literals (Go's float syntax, not a length unit hazard) —
		// code review finding: "NaNin"/"Infin" parsed with no error into a
		// non-finite inches value, which a renderer converting to
		// pixels/twips downstream would propagate as garbage instead of a
		// caught, warnable error.
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return 0, fmt.Errorf("invalid length %q: value must be finite", s)
		}
		return value * lengthUnitsToInches[unit], nil
	}

	return 0, fmt.Errorf("invalid length %q: no recognized unit (expected one of cm, mm, in, pt, px)", s)
}

// paperSize is a name's dimensions in inches, portrait orientation.
type paperSize struct {
	widthIn, heightIn float64
}

// paperSizesInches uses the ISO 216 / ANSI definitions in millimeters
// (A4 = 210mm x 297mm, Letter = 8.5in x 11in exactly, ...) converted to
// inches at full precision, NOT the rounded 8.27/11.69 that
// core/renderer/chromium's DefaultPDFOptions hardcodes for its A4 default —
// that rounding is a Chromium-renderer implementation detail, and this
// table is the shared source of truth other formats (a future DOCX page
// size) should resolve against too.
var paperSizesInches = map[string]paperSize{
	"a3":      {widthIn: 297.0 / 25.4, heightIn: 420.0 / 25.4},
	"a4":      {widthIn: 210.0 / 25.4, heightIn: 297.0 / 25.4},
	"a5":      {widthIn: 148.0 / 25.4, heightIn: 210.0 / 25.4},
	"letter":  {widthIn: 8.5, heightIn: 11},
	"legal":   {widthIn: 8.5, heightIn: 14},
	"tabloid": {widthIn: 11, heightIn: 17},
}

// PaperSizeInches resolves a case-insensitive paper size name ("A4",
// "letter", "Legal") to its portrait width/height in inches. ok is false
// for an unrecognized name — the caller decides the fallback (today, every
// consumer falls back to A4 with a warning; see doclang's PR 3 for the
// wiring), this function does not silently substitute one.
func PaperSizeInches(name string) (widthIn, heightIn float64, ok bool) {
	size, found := paperSizesInches[strings.ToLower(strings.TrimSpace(name))]
	if !found {
		return 0, 0, false
	}
	return size.widthIn, size.heightIn, true
}
