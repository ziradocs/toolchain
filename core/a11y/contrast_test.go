// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package a11y

import (
	"math"
	"testing"
)

func TestParseColor(t *testing.T) {
	tests := []struct {
		name   string
		in     string
		wantR  uint8
		wantG  uint8
		wantB  uint8
		wantOK bool
	}{
		{"6-digit hex", "#ffffff", 0xff, 0xff, 0xff, true},
		{"6-digit hex black", "#000000", 0, 0, 0, true},
		{"3-digit shorthand equals 6-digit expansion", "#fff", 0xff, 0xff, 0xff, true},
		{"3-digit shorthand mid value", "#abc", 0xaa, 0xbb, 0xcc, true},
		{"8-digit hex with fully opaque alpha (ff) parses", "#ff0000ff", 0xff, 0x00, 0x00, true},
		{"4-digit shorthand with fully opaque alpha (f) parses", "#f00f", 0xff, 0x00, 0x00, true},
		{"8-digit hex with translucent alpha is rejected", "#ff000080", 0, 0, 0, false},
		{"4-digit shorthand with translucent alpha is rejected", "#f008", 0, 0, 0, false},
		{"8-digit hex with fully transparent alpha is rejected", "#ffffff00", 0, 0, 0, false},
		{"named CSS color rejected", "white", 0, 0, 0, false},
		{"rgba() rejected", "rgba(255,255,255,0.5)", 0, 0, 0, false},
		{"linear-gradient rejected", "linear-gradient(90deg, #fff, #000)", 0, 0, 0, false},
		{"var() unresolved rejected", "var(--text-color)", 0, 0, 0, false},
		{"empty string rejected", "", 0, 0, 0, false},
		{"garbage hex length rejected", "#12345", 0, 0, 0, false},
		{"non-hex digits rejected", "#gggggg", 0, 0, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, g, b, ok := ParseColor(tt.in)
			if ok != tt.wantOK {
				t.Fatalf("ParseColor(%q) ok = %v, want %v", tt.in, ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if r != tt.wantR || g != tt.wantG || b != tt.wantB {
				t.Errorf("ParseColor(%q) = (%d,%d,%d), want (%d,%d,%d)", tt.in, r, g, b, tt.wantR, tt.wantG, tt.wantB)
			}
		})
	}
}

func TestContrastRatio_KnownReferenceValues(t *testing.T) {
	tests := []struct {
		name      string
		fg, bg    string
		wantRatio float64
		tolerance float64
	}{
		{"black on white is maximum contrast 21:1", "#000000", "#ffffff", 21.0, 0.01},
		{"white on white is minimum contrast 1:1", "#ffffff", "#ffffff", 1.0, 0.001},
		{"black on black is minimum contrast 1:1", "#000000", "#000000", 1.0, 0.001},
		{"mid-gray on white (#767676) is ~4.54:1 (WCAG's own AA-boundary example)", "#767676", "#ffffff", 4.54, 0.05},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ratio, ok := ContrastRatio(tt.fg, tt.bg)
			if !ok {
				t.Fatalf("ContrastRatio(%q, %q) ok = false, want true", tt.fg, tt.bg)
			}
			if math.Abs(ratio-tt.wantRatio) > tt.tolerance {
				t.Errorf("ContrastRatio(%q, %q) = %v, want ~%v (+/- %v)", tt.fg, tt.bg, ratio, tt.wantRatio, tt.tolerance)
			}
		})
	}
}

func TestContrastRatio_OrderIndependent(t *testing.T) {
	r1, ok1 := ContrastRatio("#000000", "#ffffff")
	r2, ok2 := ContrastRatio("#ffffff", "#000000")
	if !ok1 || !ok2 {
		t.Fatalf("expected both directions to parse: ok1=%v ok2=%v", ok1, ok2)
	}
	if math.Abs(r1-r2) > 0.0001 {
		t.Errorf("ContrastRatio should be symmetric: fg/bg=%v, bg/fg=%v", r1, r2)
	}
}

func TestContrastRatio_UnparseableInputsFailGracefully(t *testing.T) {
	tests := []struct {
		name   string
		fg, bg string
	}{
		{"fg is a gradient", "linear-gradient(90deg, #fff, #000)", "#ffffff"},
		{"bg is rgba()", "#000000", "rgba(255,255,255,0.5)"},
		{"both unparseable", "var(--fg)", "var(--bg)"},
		// Regression (code review): a translucent/transparent hex background
		// must be treated the same as its rgba() equivalent — skipped, not
		// silently composited against an assumed-opaque backdrop this
		// package never actually has.
		{"bg is a translucent 8-digit hex", "#333333", "#ffffff80"},
		{"bg is a fully transparent 8-digit hex", "#333333", "#ffffff00"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ratio, ok := ContrastRatio(tt.fg, tt.bg)
			if ok {
				t.Fatalf("ContrastRatio(%q, %q) ok = true (ratio=%v), want false — must not fabricate a ratio", tt.fg, tt.bg, ratio)
			}
		})
	}
}

func TestMeetsAA(t *testing.T) {
	if !MeetsAA(21.0, false) {
		t.Error("21:1 should meet AA normal text")
	}
	if MeetsAA(4.4, false) {
		t.Error("4.4:1 should NOT meet AA normal text (needs 4.5:1)")
	}
	if !MeetsAA(4.5, false) {
		t.Error("4.5:1 should meet AA normal text (boundary)")
	}
	if !MeetsAA(3.0, true) {
		t.Error("3.0:1 should meet AA large text (boundary)")
	}
	if MeetsAA(2.9, true) {
		t.Error("2.9:1 should NOT meet AA large text")
	}
}

func TestMeetsAAA(t *testing.T) {
	if !MeetsAAA(7.0, false) {
		t.Error("7.0:1 should meet AAA normal text (boundary)")
	}
	if MeetsAAA(6.9, false) {
		t.Error("6.9:1 should NOT meet AAA normal text")
	}
	if !MeetsAAA(4.5, true) {
		t.Error("4.5:1 should meet AAA large text (boundary)")
	}
}

func TestFormatRatio(t *testing.T) {
	got := FormatRatio(4.5)
	want := "4.50:1"
	if got != want {
		t.Errorf("FormatRatio(4.5) = %q, want %q", got, want)
	}
}
