// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"testing"

	"go.ziradocs.com/core/v2/ast"
)

func parsePointsLines(t *testing.T, lines []string, startIndex int) (*ast.PointsElement, int) {
	t.Helper()
	result := (&PointsParser{}).Parse(&ParseContext{Lines: lines, Mode: "flex"}, startIndex)
	if result.Error != nil {
		t.Fatalf("Parse() error = %v", result.Error)
	}
	el, ok := result.Element.(*ast.PointsElement)
	if !ok {
		t.Fatalf("Element is not *ast.PointsElement, got %T", result.Element)
	}
	return el, result.ConsumedLines
}

func itemContents(el *ast.PointsElement) []string {
	out := make([]string, 0, len(el.Items))
	for _, it := range el.Items {
		out = append(out, it.Content)
	}
	return out
}

// Issue #241: "los puntos clave, y luego los pasos" es el patrón más común de
// un deck, y terminaba en UNA sola lista `unordered` con los cuatro items
// pegados — la numeración se perdía sin diagnóstico. El tipo se detectaba en
// el primer item y no se volvía a mirar nunca.
func TestPointsParser_OrderedAfterUnordered_SplitsIntoTwoLists(t *testing.T) {
	lines := []string{
		"- alpha", // 0
		"- beta",  // 1
		"",        // 2
		"1. one",  // 3
		"2. two",  // 4
	}

	first, consumed := parsePointsLines(t, lines, 0)

	if first.ListType != "unordered" {
		t.Errorf("ListType = %q, want %q", first.ListType, "unordered")
	}
	if got := itemContents(first); len(got) != 2 {
		t.Fatalf("items = %v, want solo [alpha beta] — la lista numerada no pertenece a esta", got)
	}
	if consumed > 3 {
		t.Fatalf("ConsumedLines = %d, want <= 3 — %q abre la lista siguiente y no puede consumirse", consumed, lines[3])
	}

	second, _ := parsePointsLines(t, lines, consumed)
	if second.ListType != "ordered" {
		t.Errorf("segunda lista: ListType = %q, want %q", second.ListType, "ordered")
	}
	if got := itemContents(second); len(got) != 2 {
		t.Errorf("segunda lista: items = %v, want [one two]", got)
	}
}

// Sin línea en blanco de por medio el corte es el mismo: lo que separa es el
// cambio de marcador, no la blanca (que ya se toleraba dentro de una lista).
func TestPointsParser_OrderedAfterUnordered_NoBlankLine(t *testing.T) {
	lines := []string{"- alpha", "1. one"}

	first, consumed := parsePointsLines(t, lines, 0)

	if got := itemContents(first); len(got) != 1 || got[0] != "alpha" {
		t.Errorf("items = %v, want [alpha]", got)
	}
	if consumed != 1 {
		t.Errorf("ConsumedLines = %d, want 1", consumed)
	}
}

// Lo que el corte NO debe romper: un item más indentado sigue siendo
// sub-punto de la lista de arriba aunque cambie de marcador.
func TestPointsParser_NestedOrderedStaysASubPoint(t *testing.T) {
	lines := []string{
		"- alpha",
		"  1. anidado",
		"- beta",
	}

	el, consumed := parsePointsLines(t, lines, 0)

	if consumed != 3 {
		t.Fatalf("ConsumedLines = %d, want 3 — el anidado no abre lista nueva", consumed)
	}
	if len(el.Items) != 2 {
		t.Fatalf("items de nivel base = %d, want 2", len(el.Items))
	}
	if len(el.Items[0].SubPoints) != 1 {
		t.Errorf("SubPoints de %q = %d, want 1", el.Items[0].Content, len(el.Items[0].SubPoints))
	}
}

// Un cambio de carácter dentro del mismo tipo no separa: los dos son
// "unordered". CommonMark sí abriría lista nueva; replicarlo es otro cambio.
func TestPointsParser_DashToStarDoesNotSplit(t *testing.T) {
	lines := []string{"- alpha", "* beta"}

	el, consumed := parsePointsLines(t, lines, 0)

	if consumed != 2 || len(el.Items) != 2 {
		t.Errorf("ConsumedLines = %d, items = %d; want 2 y 2 — mismo tipo, una sola lista", consumed, len(el.Items))
	}
}

// Issue #242: "* * *" es un thematic break, no una viñeta. PointsParser lo
// reclamaba por el prefijo "* " y la diapositiva mostraba un bullet con el
// texto "* *". El guard vive en CanParse (que es lo que consulta el registry)
// además de en isListItem.
func TestPointsParser_ThematicBreakIsNotAListItem(t *testing.T) {
	p := &PointsParser{}

	for _, line := range []string{"* * *", "***", "___"} {
		if p.CanParse(line, "flex") {
			t.Errorf("CanParse(%q) = true, want false — es un thematic break", line)
		}
	}

	// Una viñeta de verdad se sigue reclamando.
	for _, line := range []string{"- alpha", "* beta", "+ gamma", "1. one"} {
		if !p.CanParse(line, "flex") {
			t.Errorf("CanParse(%q) = false, want true", line)
		}
	}
}
