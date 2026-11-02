// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"testing"

	"go.ziradocs.com/core/v2/ast"
)

// El fence ```math (GitHub/GitLab) era un bloque de código: la ecuación
// salía como LaTeX crudo en HTML, PDF y DOCX, con red o sin ella.

func TestMathParser_CanParseFence(t *testing.T) {
	p := &MathParser{}
	for _, tc := range []struct {
		line string
		mode string
		want bool
	}{
		{"```math", "flex", true},
		{"````math", "flex", true},
		{"  ```math  ", "flex", true},
		{"``` math", "flex", true},
		{"```math", "strict", false},
		{"```mathematica", "flex", false},
		{"```math-notes", "flex", false},
		{"```latex", "flex", false},
		{"```", "flex", false},
	} {
		if got := p.CanParse(tc.line, tc.mode); got != tc.want {
			t.Errorf("CanParse(%q, %q) = %v, want %v", tc.line, tc.mode, got, tc.want)
		}
	}
}

func TestMathParser_ParseFence(t *testing.T) {
	for _, tc := range []struct {
		name     string
		lines    []string
		source   bool
		want     string
		consumed int
		wantErr  bool
	}{
		{
			name:     "three backticks",
			lines:    []string{"```math", `\int_0^1 x^2 dx = \frac{1}{3}`, "```", "after"},
			want:     `\int_0^1 x^2 dx = \frac{1}{3}`,
			consumed: 3,
		},
		{
			name:     "four backticks close only on four",
			lines:    []string{"````math", "a", "```", "b", "````"},
			want:     "a\n```\nb",
			consumed: 5,
		},
		{
			name:     "lines trimmed, blank edges dropped",
			lines:    []string{"```math", "", "  a = b \\\\", "  c = d", "", "```"},
			want:     "a = b \\\\\nc = d",
			consumed: 6,
		},
		{
			name:     "unclosed runs to EOF",
			lines:    []string{"```math", "x"},
			want:     "x",
			consumed: 2,
		},
		{
			name:     "math-source keeps the body verbatim",
			lines:    []string{"```math", "  a", "", "b  ", "```"},
			source:   true,
			want:     "  a\n\nb  ",
			consumed: 5,
		},
		{
			name:     "math-source requires the closing fence",
			lines:    []string{"```math", "x"},
			source:   true,
			want:     "x",
			consumed: 2,
			wantErr:  true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := &ParseContext{Mode: "flex", Lines: tc.lines, MathSource: tc.source}
			res := (&MathParser{}).Parse(ctx, 0)
			if (res.Error != nil) != tc.wantErr {
				t.Fatalf("error = %v, wantErr %v", res.Error, tc.wantErr)
			}
			m, ok := res.Element.(*ast.MathElement)
			if !ok {
				t.Fatalf("element = %T, want *ast.MathElement", res.Element)
			}
			if m.Content != tc.want {
				t.Errorf("content = %q, want %q", m.Content, tc.want)
			}
			if res.ConsumedLines != tc.consumed {
				t.Errorf("consumed = %d, want %d", res.ConsumedLines, tc.consumed)
			}
		})
	}
}
