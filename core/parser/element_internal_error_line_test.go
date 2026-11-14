// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/util"
)

const (
	mapMarkerMsg    = `map marker lng " abc" is not a number`
	mapNoLngMsg     = "map marker at lat 10 has no lng"
	mathIndentMsg   = "literal math line requires structural indentation"
	mathUnclosedMsg = "literal math requires a closing delimiter"
)

// findDiag devuelve el único diagnóstico cuyo mensaje contiene sub.
func onlyDiagWith(t *testing.T, diags []diagnostics.Diagnostic, sub string) diagnostics.Diagnostic {
	t.Helper()
	var got []diagnostics.Diagnostic
	for _, d := range diags {
		if strings.Contains(d.Message, sub) {
			got = append(got, d)
		}
	}
	if len(got) != 1 {
		t.Fatalf("want one diagnostic with %q, got %d (diags: %+v)", sub, len(got), diags)
	}
	return got[0]
}

func wantDiagAt(t *testing.T, diags []diagnostics.Diagnostic, sub string, sev diagnostics.Severity, line, end int) {
	t.Helper()
	d := onlyDiagWith(t, diags, sub)
	if d.Severity != sev {
		t.Errorf("%q: want severity %s, got %s", sub, sev, d.Severity)
	}
	if d.Position.Line != line {
		t.Errorf("%q: want line %d, got %d", sub, line, d.Position.Line)
	}
	gotEnd := 0
	if d.EndPosition != nil {
		gotEnd = d.EndPosition.Line
	}
	if gotEnd != end {
		t.Errorf("%q: want end line %d, got %d", sub, end, gotEnd)
	}
}

// Un error que un parser de elemento reporta sobre una línea interna (un
// marcador de <<map>>, una línea de un <<math>> literal) sale en esa línea,
// no en la que abre el elemento. Mismo texto y misma severidad que antes;
// antes todos salían en la línea del <<map>> o del <<math>>.
func TestElementInternalErrorLine(t *testing.T) {
	const slideMap = "<<map>>\ncenter: 10, 20\nzoom: 3\nmarker: 10, abc, X\n<</map>>\n"
	const slideMapNoLng = "<<map>>\nmarkers:\n- lat: 10\nlabel: \"A\"\n<</map>>\n"
	const strictMap = "  <<map>>\n  center: 10, 20\n  marker: 10, abc, X\n  <</map>>\n"
	const strictMath = "  <<math>>\n    x^2\n   y\n    z\n  <<end>>\n"

	type parse func(src string) []diagnostics.Diagnostic
	slides := func(src string) []diagnostics.Diagnostic {
		_, d := New(util.NewNoop()).Parse(src, "d.slidelang")
		return d
	}
	docs := func(src string) []diagnostics.Diagnostic {
		_, d := New(util.NewNoop()).ParseDocument(src, "d.doclang")
		return d
	}
	for _, tc := range []struct {
		name      string
		parse     parse
		src       string
		msg       string
		sev       diagnostics.Severity
		line, end int
	}{
		{"flex slides map marker", slides, "---\nmode: flex\ntitle: T\n---\n\n# S\n\n" + slideMap, mapMarkerMsg, diagnostics.Error, 11, 0},
		{"flex slides map without lng", slides, "---\nmode: flex\ntitle: T\n---\n\n# S\n\n" + slideMapNoLng, mapNoLngMsg, diagnostics.Error, 10, 0},
		{"flex documents map marker", docs, "---\nmode: flex\ntitle: T\n---\n\n# S\n\n" + slideMap, mapMarkerMsg, diagnostics.Error, 11, 0},
		{"strict slides map marker", slides, "---\nmode: strict\ntitle: T\n---\n\nSLIDE \"S\"\n" + strictMap, mapMarkerMsg, diagnostics.Error, 9, 0},
		{"strict documents map marker", docs, "---\nmode: strict\ntitle: T\n---\n\nSECTION \"S\"\n  level: 1\n" + strictMap, mapMarkerMsg, diagnostics.Error, 10, 0},
		// Dentro de un callout el error del elemento es el aviso SPECIAL003, y
		// se queda aviso: solo cambia la línea.
		{"flex callout map marker", slides, "---\nmode: flex\ntitle: T\n---\n\n# S\n\n:::note\n" + slideMap + ":::\n", mapMarkerMsg, diagnostics.Warning, 12, 0},
		{"strict callout map marker", slides, "---\nmode: strict\ntitle: T\n---\n\nSLIDE \"S\"\n  :::note\n" + strictMap + "  :::\n", mapMarkerMsg, diagnostics.Warning, 10, 0},
		{"flex typed column map marker", docs, "---\nmode: flex\ntitle: T\n---\n\n# S\n\n::: grid\n::: column typed\n" + slideMap + ":::\n:::\n", mapMarkerMsg, diagnostics.Error, 13, 0},
		{"strict typed column map marker", slides, "---\nmode: strict\ntitle: T\n---\n\nSLIDE \"S\"\n  <<grid>>\n  <<column typed>>\n    <<map>>\n    center: 10, 20\n    marker: 10, abc, X\n    <</map>>\n  <<end>>\n  <<end>>\n", mapMarkerMsg, diagnostics.Error, 11, 0},
		{"typed callout map marker", slides, "---\nmode: flex\ntitle: T\n---\n\n# S\n\n:::note{typed}\n" + slideMap + ":::\n", mapMarkerMsg, diagnostics.Warning, 12, 0},
		{"strict slides math indentation", slides, "---\nmode: strict\ntitle: T\nast_capabilities: [math-source-v1]\n---\n\nSLIDE \"S\"\n" + strictMath, mathIndentMsg, diagnostics.Error, 10, 0},
		{"strict documents math indentation", docs, "---\nmode: strict\ntitle: T\nast_capabilities: [math-source-v1]\n---\n\nSECTION \"S\"\n  level: 1\n" + strictMath, mathIndentMsg, diagnostics.Error, 11, 0},
		// Lo que no cierra es el bloque: el error se queda en la línea que lo
		// abre y cubre lo que tomó como suyo, sin las líneas en blanco del
		// final.
		{"flex math unclosed", slides, "---\nmode: flex\ntitle: T\nast_capabilities: [math-source-v1]\n---\n\n# S\n\n<<math>>\nx^2\ny\n\n", mathUnclosedMsg, diagnostics.Error, 9, 11},
		{"flex documents math unclosed", docs, "---\nmode: flex\ntitle: T\nast_capabilities: [math-source-v1]\n---\n\n# S\n\n$$\nx^2\n", mathUnclosedMsg, diagnostics.Error, 9, 10},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wantDiagAt(t, tc.parse(tc.src), tc.msg, tc.sev, tc.line, tc.end)
		})
	}
}

// Un node-id delante del elemento corre todas las líneas; la del error se
// cuenta en el archivo igual.
func TestElementInternalErrorLineWithNodeID(t *testing.T) {
	src := "---\nmode: flex\ntitle: T\n---\n\n# S\n\n<!-- node-id: Mapa -->\n<<map>>\ncenter: 10, 20\nmarker: 10, abc, X\n<</map>>\n"
	_, diags := New(util.NewNoop()).ParseDocument(src, "d.doclang")
	wantDiagAt(t, diags, mapMarkerMsg, diagnostics.Error, 11, 0)
}

// El primer marcador malo es el que se reporta, como antes, y el mapa se
// queda con los marcadores buenos.
func TestElementInternalErrorLineKeepsTheFirstMarker(t *testing.T) {
	src := "---\nmode: flex\ntitle: T\n---\n\n# S\n\n<<map>>\nmarker: 1, 2, A\nmarker: 10, abc, X\nmarker: 3, 4, B\nmarker: 5, xyz, Y\n<</map>>\n"
	doc, diags := New(util.NewNoop()).Parse(src, "d.slidelang")
	wantDiagAt(t, diags, mapMarkerMsg, diagnostics.Error, 10, 0)
	if got := errorLines(diags, "map marker"); len(got) != 1 {
		t.Errorf("want one map marker error, got lines %v", got)
	}
	var markers int
	_ = ast.Walk(doc, func(n ast.Node) error {
		if m, ok := n.(*ast.MapElement); ok {
			markers += len(m.Markers)
		}
		return nil
	})
	if markers != 2 {
		t.Errorf("want the two good markers, got %d", markers)
	}
}
