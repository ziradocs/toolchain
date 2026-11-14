// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
)

// calloutRoundTrip formatea src (doclang flex) y exige que se relea con el
// mismo AST.
func calloutRoundTrip(t *testing.T, src string, format func(*ast.AST) (string, error)) string {
	t.Helper()
	d := tableDialect{}
	doc, diags := d.parse(src)
	if e, bad := hasErrorDiag(diags); bad {
		t.Fatalf("fixture does not parse: %s", e.String())
	}
	out, err := format(doc)
	if err != nil {
		t.Fatalf("fmt: %v", err)
	}
	doc2, _ := d.parse(out)
	if !reflect.DeepEqual(stripPositions(doc.ContentBlocks), stripPositions(doc2.ContentBlocks)) {
		t.Fatalf("the AST changed through fmt\n before: %s\n after:  %s\n%s",
			jsonOf(t, stripPositions(doc.ContentBlocks)), jsonOf(t, stripPositions(doc2.ContentBlocks)), out)
	}
	return out
}

const calloutDoc = "---\ntitle: A\n---\n# A\n\n%s\n\nDespués.\n"

// Una línea en blanco termina el cuerpo de un TABLE dentro de un bloque: lo
// que sigue (una tabla pipe, una lista, un enlace, un párrafo que empieza con
// "clave:") es del bloque, no de la tabla. Antes se lo tragaba, la lectura de
// vuelta no coincidía y fmt rechazaba el bloque.
func TestFormat_CalloutTableEndsAtBlankLine(t *testing.T) {
	table := "TABLE\n  headers: [\"A\"]\n  rows:\n      [\"1\"]"
	for name, after := range map[string]string{
		"pipe table":  "| C | D |\n|---|---|\n| 3 | 4 |",
		"list":        "- item uno\n- item dos",
		"link":        "[enlace](https://x.test) al final.",
		"key: prose":  "caption: esto es prosa",
		"bracket":     "[nota]",
		"cells: list": "",
	} {
		t.Run(name, func(t *testing.T) {
			body := table + "\n\n" + after
			if name == "cells: list" {
				body = "TABLE\n  cells:\n    - [{content: A, header: true}]\n    - [{content: \"1\"}]\n\n- item uno\n- item dos"
			}
			for _, format := range []func(*ast.AST) (string, error){FormatDocument, FormatDocumentStrict} {
				calloutRoundTrip(t, fmt.Sprintf(calloutDoc, ":::note\n"+body+"\n:::"), format)
			}
		})
	}
}

// El parser sí acepta una línea en blanco entre las propiedades de un TABLE:
// para esa tabla fmt cruza la línea en blanco.
func TestFormat_CalloutTableWithBlankLineInsideItsBody(t *testing.T) {
	block := ":::note\nTABLE\n  headers: [\"A\"]\n\n  rows:\n      [\"1\"]\nDespués de la tabla.\n:::"
	out := calloutRoundTrip(t, fmt.Sprintf(calloutDoc, block), FormatDocument)
	if !strings.Contains(out, ":::note\nTABLE\n  headers: [\"A\"]\n\n  rows:\n      [\"1\"]\nDespués de la tabla.\n:::") {
		t.Errorf("fmt wrote\n%s", out)
	}
	calloutRoundTrip(t, fmt.Sprintf(calloutDoc, block), FormatDocumentStrict)
}

// Una cerca de mermaid con sangría dentro de un bloque: el bloque guarda sus
// líneas recortadas, y la cerca vuelve con la sangría del diagrama. En
// 7ce6842 salía sin ella; después, la lectura de vuelta la rechazaba. El
// elemento de plantuml ya guarda sus líneas sin sangría al leerlas, así que
// ahí no hay sangría que devolver: sale como el bloque la guarda.
func TestFormat_CalloutIndentedDiagramFence(t *testing.T) {
	for name, tc := range map[string]struct{ fence, want string }{
		"mermaid":  {"```mermaid\ngraph TD\n    A --> B\n```", "```mermaid\ngraph TD\n    A --> B\n```"},
		"plantuml": {"```plantuml\n@startuml\n  Alice -> Bob\n@enduml\n```", "```plantuml\n@startuml\nAlice -> Bob\n@enduml\n```"},
	} {
		t.Run(name, func(t *testing.T) {
			out := calloutRoundTrip(t, fmt.Sprintf(calloutDoc, ":::note\n"+tc.fence+"\n:::"), FormatDocument)
			if !strings.Contains(out, ":::note\n"+tc.want+"\n:::") {
				t.Errorf("fmt wrote\n%s", out)
			}
		})
	}
}

// Si ni el bloque tal cual ni con la sangría devuelta se relee igual, fmt lo
// rechaza y dice qué elemento cambia. El caso: una fila bajo rows: a la misma
// sangría que rows:, que el parser no lee (TABLE008); con la sangría devuelta
// la leería, y la tabla cambiaría. (Hasta v2.58.0 el caso era un rows: de
// varias líneas con las filas a cuatro espacios, que ahora se leen.)
func TestFormat_CalloutThatDoesNotReadBackIsRefused(t *testing.T) {
	block := ":::note\nTABLE\n  headers: [\"A\", \"B\"]\n  rows:\n  [\"1\", \"2\"]\n:::"
	d := tableDialect{}
	doc, diags := d.parse(fmt.Sprintf(calloutDoc, block))
	if e, bad := hasErrorDiag(diags); bad {
		t.Fatalf("fixture does not parse: %s", e.String())
	}
	if tbl := firstTable(doc); tbl == nil || len(tbl.Rows) != 0 {
		t.Fatalf("fixture: a row at the indentation of rows: must not be read: %s", jsonOf(t, stripPositions(tbl)))
	}
	_, err := FormatDocument(doc)
	var unsupported *UnsupportedElementError
	if !errors.As(err, &unsupported) {
		t.Fatalf("want UnsupportedElementError, got %v", err)
	}
	for _, want := range []string{":::note block (line 6)", "the table with header | A | B |"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the message does not say %q:\n%v", want, err)
		}
	}
}

// Un bloque que ya se relee igual tal cual sale tal cual, byte por byte: la
// sangría devuelta solo se prueba si hace falta. Acá haría daño: con un TABLE
// sin cuerpo seguido de una tabla pipe a la columna 0, la sangría metería la
// tabla pipe en el TABLE.
func TestFormat_CalloutThatReadsBackKeepsItsText(t *testing.T) {
	block := ":::note\nTABLE\n| A | B |\n|---|---|\n| 1 | 2 |\n:::"
	out := calloutRoundTrip(t, fmt.Sprintf(calloutDoc, block), FormatDocument)
	if !strings.Contains(out, block+"\n") {
		t.Errorf("fmt changed the block:\n%s", out)
	}
}

// La lectura de vuelta usa la front matter del documento: con
// typed-headings-v1 un ### dentro del bloque es un encabezado tipado. El
// bloque necesita la sangría devuelta (por el TABLE), así que esto prueba la
// lectura de vuelta de esa forma, no la del bloque tal cual.
func TestFormat_CalloutReadBackUsesTheFrontMatter(t *testing.T) {
	src := "---\ntitle: A\nast_capabilities: [typed-headings-v1]\n---\n# A\n\n:::note\n### Dentro\nTABLE\n  headers: [\"A\"]\n  rows:\n      [\"1\"]\n:::\n\nDespués.\n"
	d := tableDialect{}
	doc, _ := d.parse(src)
	if len(typedHeadings(t, doc)) != 1 {
		t.Fatal("fixture has no typed heading inside the block")
	}
	out := calloutRoundTrip(t, src, FormatDocument)
	if !strings.Contains(out, "### Dentro\nTABLE\n  headers: [\"A\"]\n  rows:\n      [\"1\"]\n:::") {
		t.Errorf("fmt wrote\n%s", out)
	}
}
