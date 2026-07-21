// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package inference

import (
	"strings"
	"testing"
)

// Regression coverage for a bug found by gosec (G115): these Reasoning
// strings used string(rune(N)) to render a count, which converts N to a
// single Unicode CODE POINT character (e.g. rune(12) is a control char, not
// the text "12") instead of formatting it as digits.

func TestFormatFloat(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{0.42, "42%"},
		{0.0, "0%"},
		{1.0, "100%"},
		{0.865, "87%"}, // rounds
	}
	for _, c := range cases {
		if got := formatFloat(c.in); got != c.want {
			t.Errorf("formatFloat(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestDiagramDetectionHeuristic_ReasoningHasDigits(t *testing.T) {
	h := NewDiagramDetectionHeuristic()
	result := h.Analyze("flujo flujo flujo", DocumentContext{})

	if !strings.Contains(result.Reasoning, "3 coincidencias") {
		t.Errorf("Reasoning = %q, want it to contain the digit string %q", result.Reasoning, "3 coincidencias")
	}
}

func TestImagePlaceholderHeuristic_ReasoningHasDigits(t *testing.T) {
	h := NewImagePlaceholderHeuristic()
	// "screenshot" and "captura" don't overlap as substrings with each other
	// or any other keyword (unlike e.g. "imagen", which also contains
	// "image" and would double-count), so this yields a clean count of 2.
	result := h.Analyze("screenshot y captura", DocumentContext{})

	if !strings.Contains(result.Reasoning, "2 referencias") {
		t.Errorf("Reasoning = %q, want it to contain the digit string %q", result.Reasoning, "2 referencias")
	}
}
