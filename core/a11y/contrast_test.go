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
		{"linear-gradient rejected", "linear-gradient(90deg, #fff, #000)", 0, 0, 0, false},
		{"var() unresolved rejected", "var(--text-color)", 0, 0, 0, false},
		{"empty string rejected", "", 0, 0, 0, false},
		{"garbage hex length rejected", "#12345", 0, 0, 0, false},
		{"non-hex digits rejected", "#gggggg", 0, 0, 0, false},

		// issue #57: rgb()/rgba()/nombres CSS ya no se colapsan en
		// "no evaluable" — son formas de color reales que un tema externo
		// puede legítimamente escribir (core/renderer/css/themes/external.go
		// extrae :root{} verbatim, sin normalizar formato).
		{"named CSS color 'white'", "white", 0xff, 0xff, 0xff, true},
		{"named CSS color mixed case", "ToMaTo", 0xff, 0x63, 0x47, true},
		{"named CSS color 'gray'", "gray", 0x80, 0x80, 0x80, true},
		{"unknown named color rejected", "notacolor", 0, 0, 0, false},
		{"rgb() integer form", "rgb(119, 119, 119)", 119, 119, 119, true},
		{"rgb() with irregular spacing", "rgb(119,  119 ,119)", 119, 119, 119, true},
		{"rgb() percent form", "rgb(50%, 50%, 50%)", 128, 128, 128, true},
		{"rgba() fully opaque accepted", "rgba(255, 0, 0, 1)", 0xff, 0, 0, true},
		{"rgba() translucent still rejected", "rgba(255,255,255,0.5)", 0, 0, 0, false},
		{"rgba() fully transparent still rejected", "rgba(0,0,0,0)", 0, 0, 0, false},
		{"hsl() fully opaque accepted", "hsl(0, 100%, 50%)", 0xff, 0, 0, true},
		{"hsla() translucent rejected", "hsla(0, 100%, 50%, 0.5)", 0, 0, 0, false},

		// Guarda A1: csscolorparser acepta hex SIN "#" como fallback
		// ("ffffff"); ParseColor lo rechaza explícitamente porque no es un
		// color CSS válido dentro de una hoja de estilos.
		{"hex without # prefix still rejected", "ffffff", 0, 0, 0, false},

		// Hallazgo de code-review sobre PR #157: oklab()/oklch()/lab()/lch()
		// cubren un gamut más amplio que sRGB -- un canal fuera de [0,1] es
		// válido en esos espacios pero Color.RGBA255() sin Clamp() truncaba
		// ese float directamente a uint8 sin recortar, dando un canal
		// arbitrario en vez del valor visualmente más cercano dentro de
		// sRGB. Los valores esperados son los que csscolorparser.Color.
		// Clamp().RGBA255() produce -- verificados independientemente con
		// go run, no adivinados.
		{"oklch() out-of-gamut channel gets clamped, not truncated to garbage", "oklch(0.5 0.5 30)", 255, 0, 0, true},
		{"lab() out-of-gamut channels get clamped", "lab(100 150 -150)", 255, 87, 255, true},
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

// TestContrastRatio_RGBFormEvaluatesJustLikeItsHexEquivalent es el test
// discriminante de issue #57: antes de A1, un tema con
// --text-color: rgb(119,119,119) no producía NINGÚN diagnóstico de
// contraste — ni "reprobó" ni "no evaluable", silencio indistinguible de
// "pasó". #777777 sobre blanco da ~4.48:1, justo por debajo del umbral AA
// de 4.5 — es exactamente el caso que el silencio estaba escondiendo.
func TestContrastRatio_RGBFormEvaluatesJustLikeItsHexEquivalent(t *testing.T) {
	hexRatio, hexOK := ContrastRatio("#777777", "#ffffff")
	rgbRatio, rgbOK := ContrastRatio("rgb(119, 119, 119)", "#ffffff")

	if !hexOK || !rgbOK {
		t.Fatalf("expected both forms to parse: hexOK=%v rgbOK=%v", hexOK, rgbOK)
	}
	if math.Abs(hexRatio-rgbRatio) > 0.0001 {
		t.Errorf("rgb() form gave a different ratio than its hex equivalent: hex=%v rgb=%v", hexRatio, rgbRatio)
	}
	if MeetsAA(rgbRatio, false) {
		t.Errorf("ratio %v should fail AA (below 4.5:1) — this is the case the old silent skip was hiding", rgbRatio)
	}
}

// TestContrastRatioDetail_DistinguishesWhichColorFailed es el companion de
// ContrastRatio: un caller (ThemeContrastRule) necesita saber CUÁL de los
// dos colores no parseó para poder nombrarlo en el diagnóstico, no solo
// que el par falló.
func TestContrastRatioDetail_DistinguishesWhichColorFailed(t *testing.T) {
	tests := []struct {
		name       string
		fg, bg     string
		wantStatus ContrastStatus
	}{
		{"both parseable", "#000000", "#ffffff", ContrastOK},
		{"fg is the problem", "linear-gradient(90deg, #fff, #000)", "#ffffff", ContrastFgUnparseable},
		{"bg is the problem", "#000000", "var(--bg)", ContrastBgUnparseable},
		{"both unparseable reports fg first", "var(--fg)", "var(--bg)", ContrastFgUnparseable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, status := ContrastRatioDetail(tt.fg, tt.bg)
			if status != tt.wantStatus {
				t.Errorf("ContrastRatioDetail(%q, %q) status = %v, want %v", tt.fg, tt.bg, status, tt.wantStatus)
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

// TestFormatRatio_TruncatesTowardZero_NeverRoundsUpToThreshold es la
// regresión para el hallazgo de contradicción del code review: un ratio
// que MeetsAA reprobó (por debajo del umbral) no debe formatearse de forma
// que aparente HABER ALCANZADO el umbral. #006ffb sobre blanco da
// ~4.499888:1 — reprueba AA (4.5:1), pero %.2f con redondeo half-up
// mostraría "4.50:1", indistinguible del umbral en el mensaje.
func TestFormatRatio_TruncatesTowardZero_NeverRoundsUpToThreshold(t *testing.T) {
	ratio, ok := ContrastRatio("#006ffb", "#ffffff")
	if !ok {
		t.Fatalf("ContrastRatio(#006ffb, #ffffff) ok = false, want true")
	}
	if MeetsAA(ratio, false) {
		t.Fatalf("ratio %v should fail AA (below 4.5:1)", ratio)
	}

	got := FormatRatio(ratio)
	want := "4.49:1"
	if got != want {
		t.Errorf("FormatRatio(%v) = %q, want %q (a failing ratio must never display as >= the threshold)", ratio, got, want)
	}
}
