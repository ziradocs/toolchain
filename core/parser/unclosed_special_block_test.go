// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/util"
)

func special002(diags []diagnostics.Diagnostic) []diagnostics.Diagnostic {
	var out []diagnostics.Diagnostic
	for _, d := range diags {
		if d.RuleID == "SPECIAL002" {
			out = append(out, d)
		}
	}
	return out
}

// Issue #490: un callout ":::" sin cerrar en un documento strict es un error
// en la línea que lo abrió, y nombra la primera palabra clave a la sangría del
// bloque, que es donde el autor probablemente quería cerrarlo.
func TestUnclosedSpecialBlock_DocumentStrictIsAnError(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		hintLine   string
	}{
		{"single", "  :::info\n  Primer aviso, sin cierre.\n\n  CODE\n    x = 1\n\n  POINTS\n    - uno\n", "line 10 (CODE)"},
		{"nested", "  :::info\n  ::: raro\n  :::\n\n  :::warning\n  Segundo aviso.\n  :::\n\n  CODE\n    x = 1\n\n  POINTS\n    - uno\n\n  CHECKLIST\n    - [ ] tarea\n", "line 15 (CODE)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := "---\nmode: strict\ntitle: \"Demo\"\n---\n\nSECTION \"A\"\n" + tc.body
			_, diags := New(util.NewNoop()).ParseDocument(src, "c.doclang")
			got := special002(diags)
			if len(got) != 1 {
				t.Fatalf("want one SPECIAL002, got %v (all: %v)", got, diags)
			}
			d := got[0]
			if !d.IsError() || d.Position.Line != 7 {
				t.Errorf("want an error at the opening line 7, got %s at %d", d.Severity, d.Position.Line)
			}
			if !strings.Contains(d.Message, `":::info" is never closed`) || !strings.Contains(d.Message, tc.hintLine) {
				t.Errorf("message should name the opener and %s: %q", tc.hintLine, d.Message)
			}
		})
	}
}

// Un callout sin cerrar seguido de otra sección termina en ella; el error
// también sale, y la sección siguiente no se pierde.
func TestUnclosedSpecialBlock_DocumentStrictBeforeNextSection(t *testing.T) {
	src := "---\nmode: strict\ntitle: \"Demo\"\n---\n\nSECTION \"A\"\n  :::info\n  aviso\n\nSECTION \"B\"\n  TEXT\n    b\n"
	doc, diags := New(util.NewNoop()).ParseDocument(src, "c.doclang")
	got := special002(diags)
	if len(got) != 1 || !got[0].IsError() || !strings.Contains(got[0].Message, `the next block (SECTION "B")`) {
		t.Fatalf("want one SPECIAL002 error naming the next section, got %v", diags)
	}
	if len(doc.ContentBlocks) != 2 {
		t.Errorf("the next section must survive, got %d blocks", len(doc.ContentBlocks))
	}
}

// Un callout cerrado que contiene un CODE a su misma sangría es contenido
// válido (el CODE es un elemento anidado): no hay diagnóstico. Tampoco en un
// bloque de una sola línea que se cierra solo.
func TestUnclosedSpecialBlock_ClosedBlocksHaveNoDiagnostic(t *testing.T) {
	for name, body := range map[string]string{
		"closed with code inside": "  :::info\n  Texto\n  CODE\n    x = 1\n  :::\n\n  POINTS\n    - uno\n",
		"nested closed":           "  :::info\n  ::: raro\n  :::\n  :::\n",
		"self-closing":            "  :::embed type=\"youtube\" :::\n",
	} {
		t.Run(name, func(t *testing.T) {
			src := "---\nmode: strict\ntitle: \"Demo\"\n---\n\nSECTION \"A\"\n" + body
			_, diags := New(util.NewNoop()).ParseDocument(src, "c.doclang")
			if got := special002(diags); len(got) != 0 {
				t.Errorf("want no SPECIAL002, got %v", got)
			}
		})
	}
}

// Fuera del dialecto strict de documentos es un warning: un slide strict no
// puede volverse un build roto (mismo criterio que STRICT003), y en flex el
// bloque sin cierre termina en el "---" o en el fin del archivo como siempre.
func TestUnclosedSpecialBlock_WarningElsewhere(t *testing.T) {
	for _, tc := range []struct {
		name, src string
		document  bool
		line      int
	}{
		{"slide strict", "---\nmode: strict\n---\n\nSLIDE content\n  title: \"T\"\n  :::info\n  hola\n\nSLIDE content\n  title: \"U\"\n  TEXT\n    x\n", false, 7},
		{"slide flex", "---\nmode: flex\ntitle: T\n---\n\n# A\n\n::: info\nhola\n\n---\n\n# B\n\nx\n", false, 8},
		{"document flex", "# A\n\n::: info\nhola\n\n## B\n\ntexto\n", true, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := New(util.NewNoop())
			var diags []diagnostics.Diagnostic
			if tc.document {
				_, diags = p.ParseDocument(tc.src, "c.doclang")
			} else {
				_, diags = p.Parse(tc.src, "c.slidelang")
			}
			got := special002(diags)
			if len(got) != 1 || !got[0].IsWarning() || got[0].Position.Line != tc.line {
				t.Errorf("want one SPECIAL002 warning at line %d, got %v", tc.line, diags)
			}
		})
	}
}

// Dentro de una columna tipada de un documento strict también es un error: el
// cuerpo de la columna se parsea con el mismo parser de elementos y el
// diagnóstico sube a error igual que en el cuerpo de la sección.
func TestUnclosedSpecialBlock_DocumentStrictTypedColumn(t *testing.T) {
	src := "---\nmode: strict\ntitle: \"D\"\n---\n\nSECTION \"A\"\n  <<grid>>\n  <<column typed>>\n    :::info\n    aviso\n  <<column typed>>\n    TEXT\n      b\n  <<end>>\n"
	_, diags := New(util.NewNoop()).ParseDocument(src, "c.doclang")
	got := special002(diags)
	if len(got) != 1 || !got[0].IsError() || got[0].Position.Line != 9 {
		t.Errorf("want one SPECIAL002 error at line 9, got %v", diags)
	}
}
