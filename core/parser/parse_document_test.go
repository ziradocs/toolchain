// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/util"
)

func TestParseDocument_DispatchesStrict(t *testing.T) {
	doc, diags := New(util.NewNoop()).ParseDocument(`---
mode: strict
title: "Spec"
---

SECTION "Introduction"

  TEXT
    Hello.
`, "spec.doclang")
	assertNoErrors(t, diags)

	if doc.FrontMatter.Mode != "strict" {
		t.Errorf("expected mode strict on the AST, got %q", doc.FrontMatter.Mode)
	}
	if doc.FilePath != "spec.doclang" {
		t.Errorf("expected the file path to be plumbed through, got %q", doc.FilePath)
	}
	if len(doc.ContentBlocks) != 1 || doc.ContentBlocks[0].Heading != "Introduction" {
		t.Fatalf("expected the strict parser to have handled the body, got %+v", doc.ContentBlocks)
	}
}

// La contraprueba del despacho: el MISMO cuerpo SECTION bajo `mode: flex`
// no es Markdown válido y no produce secciones. Sin esto, un test que solo
// mira el camino strict pasaría igual aunque el despacho no existiera.
func TestParseDocument_StrictBodyIsNotParsedByFlex(t *testing.T) {
	doc, _ := New(util.NewNoop()).ParseDocument(`---
mode: flex
title: "Spec"
---

SECTION "Introduction"

  TEXT
    Hello.
`, "spec.doclang")

	if len(doc.ContentBlocks) != 0 {
		t.Errorf("expected the flex parser to find no `# ` sections, got %d blocks", len(doc.ContentBlocks))
	}
}

// flex, flex-full y auto son sinónimos en documentos: una sola gramática,
// nada que autodetectar. Un documento sin frontmatter cae al mismo lugar.
func TestParseDocument_FlexFamilyAndMissingFrontMatter(t *testing.T) {
	body := "# Title\n\nSome prose.\n"
	cases := map[string]string{
		"flex":         "---\nmode: flex\n---\n\n" + body,
		"flex-full":    "---\nmode: flex-full\n---\n\n" + body,
		"auto":         "---\nmode: auto\n---\n\n" + body,
		"sin mode":     "---\ntitle: \"T\"\n---\n\n" + body,
		"sin frontm.":  body,
		"YAML inválid": "---\ntitle: [unclosed\n---\n\n" + body,
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			doc, _ := New(util.NewNoop()).ParseDocument(content, "d.doclang")
			if doc == nil {
				t.Fatal("expected an AST, got nil")
			}
			if len(doc.ContentBlocks) != 1 {
				t.Fatalf("expected the flex parser to produce 1 section, got %d", len(doc.ContentBlocks))
			}
			if doc.ContentBlocks[0].Heading != "Title" {
				t.Errorf("expected Heading %q, got %q", "Title", doc.ContentBlocks[0].Heading)
			}
		})
	}
}

// El contrato central del dialecto: en strict la fase de normalización NO
// corre. El normalizador es lo que reescribe un documento antes de leerlo, y
// un artefacto auditable no puede pasar por ahí.
//
// Se observa vía lastDetectionResult (test de caja blanca, mismo paquete):
// es lo primero que hace esa fase, así que nil prueba que la fase entera se
// salteó — más directo que buscar un cambio de texto, porque el normalizador
// de cuerpo es un no-op para la mayoría del contenido bien formado y un
// "no cambió nada" no distinguiría "no corrió" de "corrió sin efecto".
func TestParseDocument_StrictSkipsTheNormalizationPhase(t *testing.T) {
	strictParser := New(util.NewNoop())
	strictParser.ParseDocument("---\nmode: strict\n---\n\nSECTION \"Intro\"\n\n  TEXT\n    Hello.\n", "d.doclang")
	if strictParser.lastDetectionResult != nil {
		t.Error("the normalization phase ran on a strict document")
	}

	// Control: en flex la misma fase SÍ corre. Sin esto, el assert de
	// arriba pasaría también si la normalización estuviera rota para todos
	// los modos.
	flexParser := New(util.NewNoop())
	flexParser.ParseDocument("---\nmode: flex\n---\n\n# Title\n\nprose\n", "d.doclang")
	if flexParser.lastDetectionResult == nil {
		t.Error("expected the normalization phase to run on the flex path")
	}
}

// SetNormalization(false) sigue mandando en el camino flex, igual que en
// Parse: ParseDocument no inventa su propia política.
func TestParseDocument_RespectsSetNormalization(t *testing.T) {
	p := New(util.NewNoop())
	p.SetNormalization(false)

	doc, _ := p.ParseDocument("---\nmode: flex\n---\n\n# Title\n\nprose\n", "d.doclang")
	if p.lastDetectionResult != nil {
		t.Error("expected the normalization phase to be off")
	}
	if len(doc.ContentBlocks) != 1 {
		t.Errorf("expected the document to still parse, got %d blocks", len(doc.ContentBlocks))
	}
}

// La razón de ser de la forma espejo: el MISMO documento escrito en los dos
// dialectos produce la misma estructura de bloques y encabezados. Es lo que
// permite que renderer, TOC y xref no tengan una sola rama por dialecto.
func TestParseDocument_StrictAndFlexProduceTheSameShape(t *testing.T) {
	flexDoc, flexDiags := New(util.NewNoop()).ParseDocument(`---
mode: flex
title: "T"
---

# Guide

Intro paragraph.

## Install

Run it.

## Configure

Tune it.

# Appendix

Extra notes.
`, "d.doclang")
	assertNoErrors(t, flexDiags)

	strictDoc, strictDiags := New(util.NewNoop()).ParseDocument(`---
mode: strict
title: "T"
---

SECTION "Guide"

  TEXT
    Intro paragraph.

SECTION "Install"
  level: 2

  TEXT
    Run it.

SECTION "Configure"
  level: 2

  TEXT
    Tune it.

SECTION "Appendix"

  TEXT
    Extra notes.
`, "d.doclang")
	assertNoErrors(t, strictDiags)

	if len(flexDoc.ContentBlocks) != len(strictDoc.ContentBlocks) {
		t.Fatalf("block count diverged: flex=%d strict=%d",
			len(flexDoc.ContentBlocks), len(strictDoc.ContentBlocks))
	}

	for i := range flexDoc.ContentBlocks {
		f, s := flexDoc.ContentBlocks[i], strictDoc.ContentBlocks[i]
		if f.BlockType != s.BlockType {
			t.Errorf("block %d: type diverged: flex=%q strict=%q", i, f.BlockType, s.BlockType)
		}
		if f.Heading != s.Heading || f.Title != s.Title {
			t.Errorf("block %d: title diverged: flex=(%q,%q) strict=(%q,%q)",
				i, f.Heading, f.Title, s.Heading, s.Title)
		}
		if got, want := headingShape(s.Elements), headingShape(f.Elements); got != want {
			t.Errorf("block %d: heading elements diverged:\n strict: %s\n   flex: %s", i, got, want)
		}
	}
}

// headingShape resume los encabezados de una lista de elementos —su HTML y
// su posición relativa— para comparar dos dialectos sin atarse al formato
// exacto del texto de los párrafos, que sí difiere legítimamente.
func headingShape(els []ast.Element) string {
	var b strings.Builder
	for i, el := range els {
		te, ok := el.(*ast.TextElement)
		if !ok || te.Level == 0 {
			continue
		}
		b.WriteString(strings.TrimSpace(te.Content))
		b.WriteString("@")
		b.WriteString(string(rune('0' + i)))
		b.WriteString(" ")
	}
	return b.String()
}
