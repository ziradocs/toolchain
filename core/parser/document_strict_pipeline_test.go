// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser_test

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/util"
	"go.ziradocs.com/core/v2/xref"
)

// Estos tests viven en parser_test (paquete externo) a propósito: consumen
// el AST strict desde AFUERA, con el mismo renderer y el mismo transform que
// usa `doclang build`. Son la prueba de que espejar la forma del dialecto
// flex rinde de verdad, y no solo en la estructura que el parser cree
// producir.

func parseStrict(t *testing.T, src string) (*ast.AST, []string) {
	t.Helper()
	doc, diags := parser.New(util.NewNoop()).ParseDocument(src, "d.doclang")
	var errs []string
	for _, d := range diags {
		if d.IsError() {
			errs = append(errs, d.Message)
		}
	}
	return doc, errs
}

// El TOC del renderer re-extrae las subsecciones con una expresión regular
// sobre el `<hN id=...>` que produce el parser. Si el dialecto strict
// emitiera ese HTML aunque sea con otro orden de atributos, el TOC saldría
// vacío sin un solo error.
func TestStrictDocument_RendersTOCLikeFlex(t *testing.T) {
	doc, errs := parseStrict(t, `---
mode: strict
title: "Guide"
---

SECTION "Guide"

  TEXT
    Intro.

SECTION "Installation"
  level: 2

  TEXT
    Steps here.

SECTION "Configuration"
  level: 2
  id: config

  TEXT
    Options here.
`)
	if len(errs) > 0 {
		t.Fatalf("unexpected parse errors: %v", errs)
	}

	html := renderer.GenerateDocumentHTML(doc, renderer.DocumentHTMLOptions{
		TOC:      true,
		TOCDepth: 3,
	}, nil)

	for _, want := range []string{`id="installation"`, `id="config"`, "Installation", "Configuration"} {
		if !strings.Contains(html, want) {
			t.Errorf("expected the rendered document to contain %q", want)
		}
	}
}

// La etapa de transform (numeración de figuras/tablas + resolución de
// \ref) corre sobre el AST sin saber de dialectos. En strict, además,
// `label:` es alcanzable de verdad — en flex no lo es para tablas/figuras,
// que es justo la carencia que el dialecto declarativo cierra.
func TestStrictDocument_ResolvesCrossReferences(t *testing.T) {
	doc, errs := parseStrict(t, `---
mode: strict
title: "Report"
---

SECTION "Results"

  TABLE
    headers: ["A", "B"]
    rows: [["1", "2"]]
    caption: "Measured throughput"
    label: tbl-throughput

  TEXT
    See \ref{tbl-throughput} for the numbers.
`)
	if len(errs) > 0 {
		t.Fatalf("unexpected parse errors: %v", errs)
	}

	transformed, err := xref.Transform(doc)
	if err != nil {
		t.Fatalf("xref transform failed on a strict document: %v", err)
	}

	html := renderer.GenerateDocumentHTML(transformed, renderer.DocumentHTMLOptions{}, nil)
	if strings.Contains(html, `\ref{`) {
		t.Error("the cross-reference was left unresolved in a strict document")
	}
	if !strings.Contains(html, "Table 1") && !strings.Contains(html, "Tabla 1") {
		t.Errorf("expected the table to have been numbered; rendered output:\n%s", truncate(html, 800))
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
