// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"math"
	"testing"
)

func TestParseLengthInches(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    float64
		wantErr bool
	}{
		{"cm", "2cm", 2 / 2.54, false},
		{"mm", "10mm", 10 / 25.4, false},
		{"in", "1in", 1, false},
		{"in decimal", "0.5in", 0.5, false},
		{"pt", "72pt", 1, false},
		{"px", "96px", 1, false},
		{"whitespace tolerated", "  2cm ", 2 / 2.54, false},
		{"no unit", "2", 0, true},
		{"unknown unit", "2furlongs", 0, true},
		{"non-numeric", "twocm", 0, true},
		{"empty", "", 0, true},
		// strconv.ParseFloat accepts Go's own float literals "NaN"/"Inf" —
		// code review finding: these used to parse into a non-finite
		// inches value with no error at all.
		{"NaN", "NaNin", 0, true},
		{"positive infinity", "Infin", 0, true},
		{"negative infinity", "-Infin", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseLengthInches(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseLengthInches(%q) = %v, want an error", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseLengthInches(%q) unexpected error: %v", tt.input, err)
			}
			if math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("ParseLengthInches(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// TestPaperSizeInches_A4NotChromiumsRoundedDefault guards the precision
// this table exists for: core/renderer/chromium's DefaultPDFOptions
// hardcodes A4 as the rounded 8.27in x 11.69in — this table must resolve
// the real ISO 216 dimensions (210mm x 297mm) instead of re-deriving that
// same rounding, or it would just be a slower way to write the same
// approximation.
func TestPaperSizeInches_A4NotChromiumsRoundedDefault(t *testing.T) {
	w, h, ok := PaperSizeInches("A4")
	if !ok {
		t.Fatal("PaperSizeInches(\"A4\") = ok false, want true")
	}
	wantW, wantH := 210.0/25.4, 297.0/25.4
	if math.Abs(w-wantW) > 1e-9 || math.Abs(h-wantH) > 1e-9 {
		t.Errorf("PaperSizeInches(\"A4\") = (%v, %v), want (%v, %v)", w, h, wantW, wantH)
	}
	if w == 8.27 {
		t.Errorf("PaperSizeInches(\"A4\") width = %v, must NOT equal chromium's rounded 8.27", w)
	}
	if h == 11.69 {
		t.Errorf("PaperSizeInches(\"A4\") height = %v, must NOT equal chromium's rounded 11.69", h)
	}
}

func TestPaperSizeInches(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		wantOK bool
		wantW  float64
		wantH  float64
	}{
		{"A4 exact case", "A4", true, 210.0 / 25.4, 297.0 / 25.4},
		{"lowercase", "a4", true, 210.0 / 25.4, 297.0 / 25.4},
		{"letter", "Letter", true, 8.5, 11},
		{"legal", "legal", true, 8.5, 14},
		{"whitespace", " A4 ", true, 210.0 / 25.4, 297.0 / 25.4},
		{"unknown", "Carta", false, 0, 0},
		{"empty", "", false, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, h, ok := PaperSizeInches(tt.input)
			if ok != tt.wantOK {
				t.Fatalf("PaperSizeInches(%q) ok = %v, want %v", tt.input, ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if math.Abs(w-tt.wantW) > 1e-9 || math.Abs(h-tt.wantH) > 1e-9 {
				t.Errorf("PaperSizeInches(%q) = (%v, %v), want (%v, %v)", tt.input, w, h, tt.wantW, tt.wantH)
			}
		})
	}
}
