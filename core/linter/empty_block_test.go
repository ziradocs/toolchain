// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package linter

import (
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
)

// lintSource parsea con el mismo punto de entrada que `slidelang build`
// y devuelve los diagnósticos del linter por defecto. Pasar por el parser
// real importa: en flex es el parser el que llena Heading/Subtitle a partir
// de `#`/`##`, y un AST armado a mano no demostraría eso. Devuelve juntos
// los diagnósticos del parser y los del linter, como los reporta el CLI.
func lintSource(t *testing.T, src string) []diagnostics.Diagnostic {
	t.Helper()
	doc, parseDiags := parser.New(util.NewNoop()).Parse(src, "test.slidelang")
	if doc == nil {
		t.Fatalf("parse returned no AST: %v", parseDiags)
	}
	return append(parseDiags, New().Lint(doc)...)
}

func codesAt(diags []diagnostics.Diagnostic, code string) []int {
	var lines []int
	for _, d := range diags {
		if d.Code == code || d.RuleID == code {
			lines = append(lines, d.Position.Line)
		}
	}
	return lines
}

// Issue #257: SYNTAX001, PARSE001 y PARSE002 decidían si un slide estaba
// vacío mirando solo Title y Elements, así que un slide de título escrito
// con `heading:` (strict) o con `# H1` (flex) contaba como vacío.
func TestEmptySlideRulesConsiderHeading(t *testing.T) {
	for _, tc := range []struct {
		name   string
		src    string
		absent []string
	}{
		{"strict heading-only title slide", `---
mode: strict
title: "T"
---

SLIDE title
  heading: "Main Presentation Title"
  subtitle: "This is the descriptive subtitle"

SLIDE content
  title: "Content"
  TEXT
    Body.
`, []string{"SLIDE002", "SYNTAX001", "PARSE001"}},
		// El deck de dos slides del issue: los dos layouts admiten cero
		// elementos, y antes PARSE001 (error) rompía el build.
		{"strict title followed by closing", `---
mode: strict
title: "T"
---

SLIDE title
  heading: "Mi presentación"
  subtitle: "Un subtítulo"

SLIDE closing
  heading: "Gracias"
`, []string{"SLIDE002", "SYNTAX001", "PARSE001", "PARSE002"}},
		{"flex H1-only cover", `---
mode: flex
title: "Deck"
---

# Title

---

## Slide two

- point
`, []string{"SLIDE002", "SYNTAX001", "PARSE001"}},
		{"flex H1 and H2 cover", `---
mode: flex
title: "Deck"
---

# Title
## Sub

---

## Slide two

- point
`, []string{"SLIDE002", "SYNTAX001", "PARSE001"}},
		{"flex single H1-only slide", `---
mode: flex
title: "Deck"
---

# Title
`, []string{"SLIDE002", "SYNTAX001", "PARSE002"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			diags := lintSource(t, tc.src)
			for _, code := range tc.absent {
				if lines := codesAt(diags, code); len(lines) > 0 {
					t.Errorf("false positive %s at lines %v", code, lines)
				}
			}
			for _, d := range diags {
				if d.Severity == diagnostics.Error {
					t.Errorf("unexpected error %s: %s", d.RuleID, d.Message)
				}
			}
		})
	}
}

// Los casos para los que existen las reglas siguen reportándose.
func TestEmptySlideRulesStillReportEmptySlides(t *testing.T) {
	t.Run("strict slide with unindented body", func(t *testing.T) {
		// El cuerpo sin indentar corta el bloque en su declaración: el slide
		// queda vacío, que es exactamente lo que SYNTAX001 avisa.
		diags := lintSource(t, `---
mode: strict
title: "T"
---

SLIDE title
  heading: "Deck"

SLIDE content
title: "Mal indentado"
TEXT
  Body.
`)
		if lines := codesAt(diags, "SYNTAX001"); len(lines) != 1 || lines[0] != 9 {
			t.Errorf("SYNTAX001 at %v, want exactly [9]", lines)
		}
		if lines := codesAt(diags, "PARSE001"); len(lines) > 0 {
			t.Errorf("PARSE001 at %v after a heading-only slide", lines)
		}
	})

	pos := func(line int) diagnostics.Position { return diagnostics.NewPosition(line, 1) }
	heading := ast.NewContentBlock(pos(1), "title")
	heading.Heading = "Deck"
	empty := func(line int) *ast.ContentBlock { return ast.NewContentBlock(pos(line), "content") }
	doc := func(blocks ...*ast.ContentBlock) *ast.AST {
		a := ast.NewAST(pos(1))
		a.FrontMatter = ast.NewFrontMatterNode(pos(1))
		a.FrontMatter.Mode = "strict"
		for _, b := range blocks {
			a.ContentBlocks = append(a.ContentBlocks, *b)
		}
		return a
	}

	for _, tc := range []struct {
		name string
		ast  *ast.AST
		want map[string][]int
	}{
		{"single empty slide", doc(empty(2)),
			map[string][]int{"SYNTAX001": {2}, "SLIDE002": {2}, "PARSE002": {1}, "PARSE001": nil}},
		{"two consecutive empty slides", doc(empty(2), empty(3)),
			map[string][]int{"SYNTAX001": {2, 3}, "SLIDE002": {2, 3}, "PARSE001": {3}}},
		{"heading-only then empty", doc(heading, empty(2)),
			map[string][]int{"SYNTAX001": {2}, "SLIDE002": {2}, "PARSE001": nil, "PARSE002": nil}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			diags := New().Lint(tc.ast)
			for code, want := range tc.want {
				got := codesAt(diags, code)
				if len(got) != len(want) {
					t.Errorf("%s at %v, want %v", code, got, want)
					continue
				}
				for i := range want {
					if got[i] != want[i] {
						t.Errorf("%s at %v, want %v", code, got, want)
						break
					}
				}
			}
		})
	}
}
