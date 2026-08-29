// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/util"
)

// El normalizador corre detrás de un gate: normalizer.Detector le pone
// puntaje al contenido y solo con más de 0.3 se aplica alguna regla. Un
// <<mermaid>> cuyo contenido no está indentado 2 espacios puntúa 0.8
// (detectMalformedMermaidDiagrams), que es una forma perfectamente razonable
// de escribir un diagrama, y eso encendía el set ENTERO de reglas sobre un
// documento de doclang.
//
// Ahí HeadersRule — una heurística de slidelang: un solo "##" titula el slide,
// cualquier otro de la misma región delimitada por "---" es contenido —
// degradaba a "**negrita**" el segundo y siguientes "##" de cada sección. El
// daño caía en `doclang build`, no solo en `fmt`: el heading desaparecía de la
// jerarquía y del TOC sin ningún diagnóstico.
//
// Los dos tests de acá son las dos mitades del arreglo, y hacen falta las dos:
// el primero prueba que la regla ya no toca documentos, el segundo que sigue
// tocando slides. Sin el segundo, un dialecto mal cableado (por ejemplo
// quedando en "documents" para todo el mundo) pasa el primero en verde
// mientras apaga la regla también para slidelang.

// unindentedMermaidDoc es el documento mínimo que dispara la cadena completa:
// el mermaid sin indentar enciende el normalizador, y "## 1.2 Scope" es el
// segundo "##" bajo el mismo "#".
const unindentedMermaidDoc = `---
title: T
mode: flex
---

# 1. Introduction

## 1.1 Purpose

Text.

## 1.2 Scope

Text.

---

# 2. Architecture

<<mermaid>>
graph TD
A --> B
<<end>>
`

func TestParseDocument_SubsectionsSurviveNormalization(t *testing.T) {
	doc, diags := New(util.NewNoop()).ParseDocument(unindentedMermaidDoc, "doc.doclang")
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("diagnóstico de error inesperado: %v", d)
		}
	}

	for _, want := range []string{"1.1 Purpose", "1.2 Scope"} {
		if !documentHasSubsectionHeading(doc, want) {
			t.Errorf("el subtítulo %q dejó de ser un heading al parsear.\n"+
				"Es HeadersRule (internal/normalize/normalizer/rules/content/headers.go) corriendo sobre un documento: "+
				"su AppliesTo debe devolver false para base.DialectDocuments, y parser.DocumentFlexParser debe declarar "+
				"ese dialecto al llamar a normalize.ProcessWithDetection.", want)
		}
	}
}

// TestNormalization_StillAppliesToSlides es la otra mitad, y hace falta: el
// mismo cableado no puede apagar HeadersRule para slidelang, que es el
// dialecto cuyo modelo la regla describe. Sin este test, pasar el dialecto
// equivocado en la ruta de slides (o dejarlo en DialectDocuments para todo el
// mundo) deja el primer test en verde mientras apaga la regla de más.
//
// Sobre la aserción: en slidelang un "##" abre un slide nuevo, así que su
// texto va a ContentBlock.Title y no a un <hN> como en doclang. Cuando
// HeadersRule degrada el segundo "##", ese slide desaparece y su título queda
// como un TextElement "**1.2 Scope**" dentro del slide anterior — esa es la
// forma observable, y es la que se asierta. Una versión de este test escrita
// contra <hN> (como el de doclang) pasaría siempre, sin importar si la regla
// corrió: en slides nunca hay <hN>.
func TestNormalization_StillAppliesToSlides(t *testing.T) {
	doc, diags := New(util.NewNoop()).Parse(unindentedMermaidDoc, "deck.slidelang")
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("diagnóstico de error inesperado: %v", d)
		}
	}
	if doc == nil {
		t.Fatal("Parse devolvió nil")
	}

	if !documentHasPlainText(doc, "**1.2 Scope**") {
		t.Error("HeadersRule dejó de aplicarse a slidelang: el segundo \"##\" no se degradó a negrita.\n" +
			"Suele ser el dialecto mal cableado — parser.Parser.Parse debe pasar base.DialectSlides. Ojo que " +
			"Parser.ParseDocument (doclang) y Parser.Parse (slidelang) tienen llamadas a " +
			"normalize.ProcessWithDetection casi idénticas, y confundirlas es exactamente el error que este test caza.")
	}
}

// documentHasPlainText reporta si algún elemento del AST es un TextElement
// (no RawHTML) cuyo contenido es exactamente want.
func documentHasPlainText(doc *ast.AST, want string) bool {
	if doc == nil {
		return false
	}
	for _, block := range doc.ContentBlocks {
		for _, el := range block.Elements {
			text, ok := el.(*ast.TextElement)
			if ok && !text.IsRawHTML && strings.TrimSpace(text.Content) == want {
				return true
			}
		}
	}
	return false
}

// documentHasSubsectionHeading reporta si algún elemento del AST es un
// subsection heading (<hN>) cuyo texto es title. El parser flex emite los
// "##"/"###" como TextElement con IsRawHTML, no como un tipo propio.
func documentHasSubsectionHeading(doc *ast.AST, title string) bool {
	if doc == nil {
		return false
	}
	for _, block := range doc.ContentBlocks {
		for _, el := range block.Elements {
			text, ok := el.(*ast.TextElement)
			if !ok || !text.IsRawHTML {
				continue
			}
			if !strings.HasPrefix(strings.TrimSpace(text.Content), "<h") {
				continue
			}
			if strings.Contains(text.Content, ">"+title+"<") {
				return true
			}
		}
	}
	return false
}
