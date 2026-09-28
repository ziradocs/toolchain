// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"reflect"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/util"
)

func parseSlides(t *testing.T, src string) (*ast.AST, []diagnostics.Diagnostic) {
	t.Helper()
	return New(util.NewNoop()).Parse(src, "")
}

func parseDoc(t *testing.T, src string) (*ast.AST, []diagnostics.Diagnostic) {
	t.Helper()
	return New(util.NewNoop()).ParseDocument(src, "")
}

func requireNoErrors(t *testing.T, diags []diagnostics.Diagnostic) {
	t.Helper()
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("unexpected error: %v", d)
		}
	}
}

func hasError(diags []diagnostics.Diagnostic, substr string) bool {
	for _, d := range diags {
		if d.IsError() && strings.Contains(d.Message, substr) {
			return true
		}
	}
	return false
}

// headingShape reduce los encabezados de un slide a lo que el contrato
// promete: tipo, nivel, texto/HTML, anchor y nodeId. Sin posiciones, que
// cambian legítimamente entre dialectos.
func typedHeadingShape(t *testing.T, els []ast.Element) []string {
	t.Helper()
	var out []string
	for _, el := range els {
		switch e := el.(type) {
		case *ast.HeadingElement:
			out = append(out, strings.Join([]string{"heading", string(rune('0' + e.Level)), e.Text, e.Anchor, e.NodeID}, "|"))
		case *ast.TextElement:
			if e.IsRawHTML {
				out = append(out, strings.Join([]string{"legacy", string(rune('0' + e.Level)), e.Content, e.NodeID}, "|"))
			} else {
				out = append(out, "text|"+strings.TrimSpace(e.Content))
			}
		}
	}
	return out
}

const flexSlides = `---
mode: flex
%s---
# Deck

## Slide

### Results **now**

Body text.

<!-- node-id: DetailA -->
#### Detail

### Results **now**
`

const strictSlides = `---
mode: strict
%s---
SLIDE content
  title: "Slide"
  SECTION "Results **now**"
    level: 3
  TEXT
    Body text.
  <!-- node-id: DetailA -->
  SECTION "Detail"
    level: 4
  SECTION "Results **now**"
    level: 3
`

func withCaps(tpl, caps string) string {
	return strings.Replace(tpl, "%s", caps, 1)
}

// flex y strict producen los mismos encabezados, con y sin opt-in: mismo
// nivel, misma fuente autoral, misma secuencia de anchors ("heading-…" con
// sufijo en el duplicado) y el mismo nodeId.
func TestTypedHeadingsFlexAndStrictAgree(t *testing.T) {
	for _, tc := range []struct {
		name, caps string
		typed      bool
	}{
		{"legacy", "", false},
		{"typed", "ast_capabilities: [typed-headings-v1]\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			flex, fd := parseSlides(t, withCaps(flexSlides, tc.caps))
			requireNoErrors(t, fd)
			strict, sd := parseSlides(t, withCaps(strictSlides, tc.caps))
			requireNoErrors(t, sd)
			var flexEls []ast.Element
			for _, b := range flex.ContentBlocks {
				flexEls = append(flexEls, b.Elements...)
			}
			got, want := typedHeadingShape(t, strict.ContentBlocks[0].Elements), typedHeadingShape(t, flexEls)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("strict != flex\nstrict: %q\nflex:   %q", got, want)
			}
			if tc.typed {
				wantTyped := []string{
					"heading|3|Results **now**|heading-results-now|",
					"text|Body text.",
					"heading|4|Detail|heading-detail|DetailA",
					"heading|3|Results **now**|heading-results-now-2|",
				}
				if !reflect.DeepEqual(got, wantTyped) {
					t.Fatalf("typed shape = %q, want %q", got, wantTyped)
				}
				if strict.SchemaVersion != ast.TypedHeadingsSchemaVersion || !reflect.DeepEqual(strict.Capabilities, []string{ast.TypedHeadingsCapability}) {
					t.Fatalf("contract = %s %v", strict.SchemaVersion, strict.Capabilities)
				}
			} else {
				if strict.SchemaVersion != ast.LegacySchemaVersion || len(strict.Capabilities) != 0 {
					t.Fatalf("legacy contract changed: %s %v", strict.SchemaVersion, strict.Capabilities)
				}
				if !strings.HasPrefix(got[0], `legacy|3|<h3 id="heading-results-now">Results <strong>now</strong></h3>`) {
					t.Fatalf("legacy heading changed: %q", got[0])
				}
			}
		})
	}
}

func TestStrictSlideHeadingExplicitIDAndDefaults(t *testing.T) {
	src := `---
mode: strict
ast_capabilities: [typed-headings-v1]
---
SLIDE content
  title: "S"
  SECTION "Quarter: results"
    id: Quarter Results
  SECTION "Quarter: results"
`
	doc, diags := parseSlides(t, src)
	requireNoErrors(t, diags)
	got := typedHeadingShape(t, doc.ContentBlocks[0].Elements)
	want := []string{
		"heading|3|Quarter: results|quarter-results|",
		"heading|3|Quarter: results|heading-quarter-results|",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestStrictSlideHeadingRejectsInvalidForms(t *testing.T) {
	for _, tc := range []struct{ name, body, want string }{
		{"level too high", "  SECTION \"A\"\n    level: 2\n", "between 3 and 6"},
		{"level not a number", "  SECTION \"A\"\n    level: x\n", "between 3 and 6"},
		{"unknown property", "  SECTION \"A\"\n    color: red\n", "Unknown SECTION property"},
		{"body under heading", "  SECTION \"A\"\n    TEXT\n", "has no body"},
		{"unquoted title", "  SECTION A\n", "must be quoted"},
		{"empty title", "  SECTION \"  \"\n", "non-empty title"},
		{"duplicate id", "  SECTION \"A\"\n    id: same\n  SECTION \"B\"\n    id: same\n", "duplicate heading id"},
		{"id collides with derived", "  SECTION \"Same\"\n  SECTION \"B\"\n    id: heading-same\n", "duplicate heading id"},
		{"id without anchor chars", "  SECTION \"A\"\n    id: ☃\n", "no usable characters"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := "---\nmode: strict\n---\nSLIDE content\n  title: \"S\"\n" + tc.body
			_, diags := parseSlides(t, src)
			if !hasError(diags, tc.want) {
				t.Fatalf("missing error %q in %v", tc.want, diags)
			}
		})
	}
}

// DocLang: `##` en flex y SECTION level 2-6 en strict se promueven igual;
// DocLang no antepone "heading-" (su TOC deriva el anchor del texto).
func TestTypedHeadingsDocLangDialects(t *testing.T) {
	flex := "---\nast_capabilities: [typed-headings-v1]\n---\n# Doc\n\n## Scope *now*\n\nText.\n"
	strict := "---\nmode: strict\nast_capabilities: [typed-headings-v1]\n---\nSECTION \"Doc\"\n\nSECTION \"Scope *now*\"\n  level: 2\n  TEXT\n    Text.\n\nSECTION \"Other\"\n  level: 3\n  id: custom-anchor\n"
	fd, fdiags := parseDoc(t, flex)
	requireNoErrors(t, fdiags)
	if got := typedHeadingShape(t, fd.ContentBlocks[0].Elements); !reflect.DeepEqual(got[:1], []string{"heading|2|Scope *now*|scope-now|"}) {
		t.Fatalf("flex = %q", got)
	}
	sd, sdiags := parseDoc(t, strict)
	requireNoErrors(t, sdiags)
	got := typedHeadingShape(t, sd.ContentBlocks[0].Elements)
	want := []string{"heading|2|Scope *now*|scope-now|", "text|Text.", "heading|3|Other|custom-anchor|"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("strict = %q, want %q", got, want)
	}
	if sd.SchemaVersion != ast.TypedHeadingsSchemaVersion {
		t.Fatalf("version = %s", sd.SchemaVersion)
	}
}

// Un encabezado dentro de un bloque especial también se promueve.
func TestTypedHeadingsNestedInSpecialBlock(t *testing.T) {
	src := "---\nmode: flex\nast_capabilities: [typed-headings-v1]\n---\n# Deck\n\n## Slide\n\n::: note\n### Inside\nBody.\n:::\n"
	doc, diags := parseSlides(t, src)
	requireNoErrors(t, diags)
	found := false
	_ = ast.Walk(doc, func(n ast.Node) error {
		if h, ok := n.(*ast.HeadingElement); ok && h.Text == "Inside" && h.Level == 3 {
			found = true
		}
		return nil
	})
	if !found {
		t.Fatal("nested heading not promoted")
	}
}

// Reordenar encabezados con el mismo texto puede mover el sufijo del anchor
// (depende del orden), pero cada nodeId autoral sigue en su nodo.
func TestTypedHeadingNodeIDsSurviveReorder(t *testing.T) {
	one := "---\nmode: strict\nast_capabilities: [typed-headings-v1]\n---\nSLIDE content\n  title: \"S\"\n  <!-- node-id: First -->\n  SECTION \"Same\"\n    level: 3\n  <!-- node-id: Second -->\n  SECTION \"Same\"\n    level: 4\n"
	two := "---\nmode: strict\nast_capabilities: [typed-headings-v1]\n---\nSLIDE content\n  title: \"S\"\n  <!-- node-id: Second -->\n  SECTION \"Same\"\n    level: 4\n  <!-- node-id: First -->\n  SECTION \"Same\"\n    level: 3\n"
	byID := func(src string) map[string]int {
		doc, diags := parseSlides(t, src)
		requireNoErrors(t, diags)
		out := map[string]int{}
		for _, el := range doc.ContentBlocks[0].Elements {
			if h, ok := el.(*ast.HeadingElement); ok {
				out[h.NodeID] = h.Level
			}
		}
		return out
	}
	a, b := byID(one), byID(two)
	if !reflect.DeepEqual(a, b) || a["First"] != 3 || a["Second"] != 4 {
		t.Fatalf("identity moved on reorder: %v vs %v", a, b)
	}
}
