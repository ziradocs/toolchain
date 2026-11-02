// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
)

// Listas profundas, tareas a profundidad 3 y listas dentro de citas y bloques
// pasan por `slidelang fmt` (flex → strict) y vuelven al mismo AST, de forma
// estable. Antes el tercer nivel volvía como hermano del segundo, y el
// formateador rechazaba una sub-sub-tarea.
func TestFormatStrict_DeepListsRoundTrip(t *testing.T) {
	cases := map[string]string{
		"five levels":       "---\nmode: flex\n---\n\n## One\n\n1. uno\n   - dos\n     1. tres\n        - cuatro\n          1. cinco\n2. otro\n",
		"ordered start":     "---\nmode: flex\n---\n\n## One\n\n3. tres\n4. cuatro\n   7. siete\n",
		"zero start":        "---\nmode: flex\n---\n\n## One\n\n0. cero\n1. uno\n",
		"paren delimiter":   "---\nmode: flex\n---\n\n## One\n\n1) uno\n2) dos\n   - a\n   1. b\n   1) c\n",
		"wide numbers":      "---\nmode: flex\n---\n\n## One\n\n9. nueve\n10. diez\n    - hijo\n",
		"tasks at depth 3":  "---\nmode: flex\n---\n\n## One\n\n- [x] t1\n  - [ ] t1.1\n    - [x] t1.1.1\n- [ ] t2\n",
		"list in a quote":   "---\nmode: flex\n---\n\n## One\n\n> Cita\n> - a\n>   - b\n>     1. c\n",
		"list in a callout": "---\nmode: flex\n---\n\n## One\n\n:::info Nota\nTexto\n- a\n  - b\n    1. c\n:::\n",
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			diffs, out := flexCorpusASTDiffs(t, src)
			if len(diffs) != 0 {
				t.Fatalf("the AST changed:\n  %s\n%s", strings.Join(diffs, "\n  "), out)
			}
			again, err := FormatStrict(parseSlides(t, out))
			if err != nil || again != out {
				t.Fatalf("not idempotent (%v):\n%s\n---\n%s", err, out, again)
			}
		})
	}
}

// Sin list-start-v1 el número con que empieza una lista vive solo en memoria;
// fmt lo vuelve a escribir para que el render siga empezando ahí.
func TestFormatKeepsTheStartOfALegacyList(t *testing.T) {
	out, reparsed := transpile(t, "---\nmode: flex\n---\n\n## One\n\n3. tres\n4. cuatro\n   7. siete\n   8. ocho\n   - x\n   1. uno\n")
	if !strings.Contains(out, "3. tres") || !strings.Contains(out, "7. siete") || !strings.Contains(out, "1. uno") {
		t.Fatalf("starts lost:\n%s", out)
	}
	pts := firstPoints(reparsed)
	if a, _ := pts.Items[0].WrittenOrdinal(); a != 3 {
		t.Fatalf("ordinals after fmt: %+v", pts.Items)
	}
	if b, _ := pts.Items[1].SubPoints[0].WrittenOrdinal(); b != 7 {
		t.Fatalf("ordinals after fmt: %+v", pts.Items)
	}
}

// Una sub-sub-tarea ya no es un caso sin representación en strict.
func TestFormatStrict_DeepChecklistRoundTrips(t *testing.T) {
	out, reparsed := transpile(t, "---\nmode: flex\n---\n\n## One\n\n- [x] t1\n  - [ ] t1.1\n    - [x] t1.1.1\n")
	var cl *ast.ChecklistElement
	_ = ast.Walk(reparsed, func(n ast.Node) error {
		if c, ok := n.(*ast.ChecklistElement); ok {
			cl = c
		}
		return nil
	})
	if cl == nil || len(cl.Items[0].SubItems) != 1 || len(cl.Items[0].SubItems[0].SubItems) != 1 || !cl.Items[0].SubItems[0].SubItems[0].Checked {
		t.Fatalf("depth lost:\n%s", out)
	}
}

// Lo mismo por `doclang fmt`, en los dos dialectos de documento.
func TestFormatDocument_DeepListsRoundTrip(t *testing.T) {
	src := "---\nmode: flex\n---\n\n# S\n\n1. uno\n   - dos\n     1. tres\n\n- [x] t1\n  - [ ] t1.1\n    - [x] t1.1.1\n\n> Cita\n> - a\n>   - b\n\n:::info Nota\nTexto\n- a\n  1. b\n:::\n"
	want := parseDocumentOrFail(t, src)
	for name, format := range map[string]func(*ast.AST) (string, error){"flex": FormatDocument, "strict": FormatDocumentStrict} {
		t.Run(name, func(t *testing.T) {
			out, err := format(want)
			if err != nil {
				t.Fatal(err)
			}
			got := parseDocumentOrFail(t, out)
			if a, b := listSummary(want), listSummary(got); a != b {
				t.Fatalf("lists changed:\n%s\n---\n%s\n%s", a, b, out)
			}
		})
	}
}

func listSummary(doc *ast.AST) string {
	var b strings.Builder
	var points func([]ast.PointItem, string)
	points = func(items []ast.PointItem, indent string) {
		for _, it := range items {
			b.WriteString(indent + it.Content + "/" + it.SubListMarker + "\n")
			points(it.SubPoints, indent+" ")
		}
	}
	var tasks func([]ast.ChecklistItem, string)
	tasks = func(items []ast.ChecklistItem, indent string) {
		for _, it := range items {
			b.WriteString(indent + it.Content + "\n")
			tasks(it.SubItems, indent+" ")
		}
	}
	_ = ast.Walk(doc, func(n ast.Node) error {
		switch e := n.(type) {
		case *ast.PointsElement:
			points(e.Items, "")
		case *ast.ChecklistElement:
			tasks(e.Items, "")
		case *ast.QuoteElement:
			b.WriteString("quote:" + e.Content + "\n")
		case *ast.SpecialBlockElement:
			b.WriteString("block:" + e.Content + "\n")
		}
		return nil
	})
	return b.String()
}

// fmt escribe cada sublista en la columna de contenido de su item, que es
// donde el parser flex (CommonMark) busca a los hijos: tres espacios bajo
// "9. ", cuatro bajo "10. ".
func TestFormatDocument_SublistAtTheContentColumn(t *testing.T) {
	doc := parseDocumentOrFail(t, "---\nmode: flex\n---\n\n# S\n\n9. nueve\n   - a\n10. diez\n    1) b\n")
	out, err := FormatDocument(doc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "9. nueve\n   - a\n") || !strings.Contains(out, "\n    1) b") {
		t.Fatalf("children not at the content column:\n%s", out)
	}
	if a, b := listSummary(doc), listSummary(parseDocumentOrFail(t, out)); a != b {
		t.Fatalf("lists changed:\n%s\n---\n%s", a, b)
	}
}
