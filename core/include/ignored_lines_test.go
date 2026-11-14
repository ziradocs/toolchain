// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package include

import (
	"regexp"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
)

// lineNumberInMessage es un número de línea escrito en el texto de un aviso.
// Se calcula antes del remapeo de @include, que solo corrige Position y
// EndPosition, así que en un archivo incluido nombraría una línea del
// documento expandido.
var lineNumberInMessage = regexp.MustCompile(`\bline \d+`)

// Los avisos de líneas ignoradas de un archivo incluido apuntan a su línea en
// ese archivo, con el rango entero, y su texto no nombra ninguna línea por
// número.
func TestIgnoredLineDiagnosticsInsideAnInclude(t *testing.T) {
	part := "  QUOTE\n    cita\n    AUTHOR: Ana\n    AUTHOR: Beto\n  <<metric>>\n    label: L\n    notas:\n      - x\n      - y\n  <<end>>\n"
	files := map[string]string{abs(t, "/base/part.doclang"): part}
	root := "---\nmode: strict\ntitle: \"D\"\n---\n\nSECTION \"A\"\n  TEXT\n    antes\n@include part.doclang\n"
	text, sm, err := ExpandWithSourceMap(root, abs(t, "/base"), memFS(files))
	if err != nil {
		t.Fatal(err)
	}
	_, diags := parser.New(util.NewNoop()).ParseDocument(text, "d.doclang")
	mapped := sm.MapDiagnostics(diags, abs(t, "/base"))
	want := map[string][2]int{"QUOTE001": {3, 3}, "METRIC006": {7, 9}}
	for _, d := range mapped {
		w, ok := want[d.RuleID]
		if !ok {
			continue
		}
		delete(want, d.RuleID)
		end := d.Position.Line
		if d.EndPosition != nil {
			end = d.EndPosition.Line
		}
		if d.File != "part.doclang" || d.Position.Line != w[0] || end != w[1] {
			t.Errorf("%s: want part.doclang:%d-%d, got %s:%d-%d", d.RuleID, w[0], w[1], d.File, d.Position.Line, end)
		}
		if m := lineNumberInMessage.FindString(d.Message); m != "" {
			t.Errorf("%s names %q in its text: %s", d.RuleID, m, d.Message)
		}
		if d.RuleID == "QUOTE001" && !strings.Contains(d.Message, `"AUTHOR: Beto"`) {
			t.Errorf("QUOTE001 does not name the line that replaces it: %s", d.Message)
		}
	}
	for id := range want {
		t.Errorf("missing %s in %v", id, mapped)
	}
}

// SPECIAL002 de un archivo incluido: la línea que sugiere como cierre se
// nombra por su contenido, no por el número del documento expandido (que no
// es el del archivo), y Position apunta a la apertura en el archivo.
func TestUnclosedSpecialBlockInsideAnInclude(t *testing.T) {
	part := "  :::info\n  aviso sin cierre\n\n  CODE\n    x = 1\n"
	files := map[string]string{abs(t, "/base/part.doclang"): part}
	root := "---\nmode: strict\ntitle: \"D\"\n---\n\nSECTION \"A\"\n  TEXT\n    uno\n\n  TEXT\n    dos\n\n@include part.doclang\n"
	text, sm, err := ExpandWithSourceMap(root, abs(t, "/base"), memFS(files))
	if err != nil {
		t.Fatal(err)
	}
	_, diags := parser.New(util.NewNoop()).ParseDocument(text, "d.doclang")
	var found int
	for _, d := range sm.MapDiagnostics(diags, abs(t, "/base")) {
		if d.RuleID != "SPECIAL002" {
			continue
		}
		found++
		if d.File != "part.doclang" || d.Position.Line != 1 {
			t.Errorf("want part.doclang:1, got %s:%d", d.File, d.Position.Line)
		}
		if m := lineNumberInMessage.FindString(d.Message); m != "" {
			t.Errorf("SPECIAL002 names %q in its text: %s", m, d.Message)
		}
		if !strings.Contains(d.Message, `the line "CODE" is at the block's own indentation`) {
			t.Errorf("SPECIAL002 does not name the line by its content: %s", d.Message)
		}
	}
	if found != 1 {
		t.Errorf("want one SPECIAL002, got %d in %v", found, diags)
	}
}
