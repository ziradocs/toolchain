// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// TestDOCXGenerator_CodeBlock_HasThemeBackground: cada línea de un bloque de
// código lleva el fondo CodeBlockBg del tema. Sin él, el D4D4D4 de
// professional quedaba gris claro sobre blanco.
func TestDOCXGenerator_CodeBlock_HasThemeBackground(t *testing.T) {
	doc := newTestAST()
	block := &doc.ContentBlocks[0]
	block.Elements = append(block.Elements,
		ast.NewCodeElement(diagnostics.NewPosition(6, 1), "go", "package main\n\nfunc main() {}\n"),
	)

	for _, theme := range []string{"professional", "academic", "technical", "page-view"} {
		style := GetStyleForTheme(theme, newTestLogger())
		xml := buildDOCXXML(t, doc, GeneratorOptions{Theme: theme})
		fill := `<w:shd w:val="clear" w:color="auto" w:fill="` + style.CodeBlockBg + `">`
		// Tres líneas (la vacía incluida, sin la del salto final).
		if n := strings.Count(xml, fill); n != 3 {
			t.Errorf("%s: want 3 shaded code lines with fill %s, got %d", theme, style.CodeBlockBg, n)
		}
	}
}

// TestDOCXGenerator_CodeGroup_HasThemeBackground: los bloques de un grupo de
// código usan el mismo fondo.
func TestDOCXGenerator_CodeGroup_HasThemeBackground(t *testing.T) {
	doc := newTestAST()
	group := ast.NewCodeGroupElement(diagnostics.NewPosition(6, 1))
	group.CodeBlocks = []ast.CodeBlock{
		{Language: "go", Content: "a := 1\nb := 2"},
		{Language: "py", Content: "a = 1"},
	}
	doc.ContentBlocks[0].Elements = append(doc.ContentBlocks[0].Elements, group)

	xml := buildDOCXXML(t, doc, GeneratorOptions{})
	if n := strings.Count(xml, `w:fill="1E1E1E"`); n != 3 {
		t.Errorf("want 3 shaded lines in the code group, got %d", n)
	}
}

// TestDOCXStyles_CodeBlockContrast exige contraste WCAG AA (4.5:1) entre el
// texto y el fondo de los bloques de código en cada tema.
func TestDOCXStyles_CodeBlockContrast(t *testing.T) {
	for _, theme := range []string{"professional", "academic", "technical", "page-view"} {
		s := GetStyleForTheme(theme, newTestLogger())
		if ratio := contrastRatio(t, s.CodeBlockColor, s.CodeBlockBg); ratio < 4.5 {
			t.Errorf("%s: code block contrast %s on %s = %.2f, want >= 4.5", theme, s.CodeBlockColor, s.CodeBlockBg, ratio)
		}
	}
}

var hexColor = regexp.MustCompile(`^[0-9A-Fa-f]{6}$`)

func contrastRatio(t *testing.T, fg, bg string) float64 {
	t.Helper()
	l1, l2 := relativeLuminance(t, fg), relativeLuminance(t, bg)
	if l1 < l2 {
		l1, l2 = l2, l1
	}
	return (l1 + 0.05) / (l2 + 0.05)
}

func relativeLuminance(t *testing.T, hex string) float64 {
	t.Helper()
	if !hexColor.MatchString(hex) {
		t.Fatalf("not a hex color: %q", hex)
	}
	channel := func(i int) float64 {
		v, _ := strconv.ParseUint(hex[i:i+2], 16, 8)
		c := float64(v) / 255
		if c <= 0.03928 {
			return c / 12.92
		}
		return math.Pow((c+0.055)/1.055, 2.4)
	}
	return 0.2126*channel(0) + 0.7152*channel(2) + 0.0722*channel(4)
}
