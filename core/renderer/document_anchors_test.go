// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer_test

import (
	"reflect"
	"regexp"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/util"
	"go.ziradocs.com/core/v2/xref"
)

const repeatedHeadingsSource = `---
title: "Manual"
---

# A

Texto.

## Notas

Primeras notas.

# B

## Notas

[ir a la segunda](#notas-2) y [ir a la otra B](#b-2)

# B

Otra vez B.

# 概要

Sin letras latinas.
`

var (
	htmlIDPattern   = regexp.MustCompile(`\bid="([^"]*)"`)
	htmlHrefPattern = regexp.MustCompile(`\bhref="#([^"]*)"`)
	htmlHeadingID   = regexp.MustCompile(`<h[1-6] id="([^"]*)"`)
)

func repeatedHeadingsHTML(t *testing.T, opts renderer.DocumentHTMLOptions) string {
	t.Helper()
	doc, diags := parser.New(util.NewNoop()).ParseDocument(repeatedHeadingsSource, "manual.doclang")
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("parse error: %v", d)
		}
	}
	doc, err := xref.Transform(doc)
	if err != nil {
		t.Fatal(err)
	}
	return renderer.GenerateDocumentHTML(doc, opts, renderer.NewDefaultRenderContext())
}

// Issue #267, de punta a punta en HTML (y en el PDF, que se imprime de este
// mismo HTML): ningún id se repite, cada href="#…" llega a un id que existe,
// y el TOC y la barra lateral llevan cada entrada a su propio encabezado, en
// el flujo normal y en la vista de página.
func TestDocumentHTMLHeadingIDsAreUniqueAndLinked(t *testing.T) {
	for name, opts := range map[string]renderer.DocumentHTMLOptions{
		"toc":             {TOC: true, TOCDepth: 3},
		"toc numerado":    {TOC: true, TOCDepth: 3, Numbering: true},
		"barra lateral":   {TOC: true, TOCDepth: 3, InteractiveViewer: true},
		"vista de página": {TOC: true, TOCDepth: 3, ShowHeaders: true, ShowFooters: true, PageBreaks: true},
	} {
		t.Run(name, func(t *testing.T) {
			html := repeatedHeadingsHTML(t, opts)

			ids := map[string]int{}
			for _, m := range htmlIDPattern.FindAllStringSubmatch(html, -1) {
				ids[m[1]]++
			}
			for id, n := range ids {
				if n > 1 {
					t.Errorf("id %q aparece %d veces", id, n)
				}
				if id == "" {
					t.Errorf("hay un id vacío")
				}
			}

			var headings []string
			for _, m := range htmlHeadingID.FindAllStringSubmatch(html, -1) {
				headings = append(headings, m[1])
			}
			want := []string{"a", "notas", "b", "notas-2", "b-2", "seccion"}
			if !reflect.DeepEqual(headings, want) {
				t.Errorf("ids de encabezado = %v, want %v", headings, want)
			}

			// Cada destino interno existe, y el TOC (o la barra lateral)
			// enlaza cada encabezado exactamente una vez.
			links := map[string]int{}
			for _, m := range htmlHrefPattern.FindAllStringSubmatch(html, -1) {
				if ids[m[1]] == 0 {
					t.Errorf("href=\"#%s\" no llega a ningún id", m[1])
				}
				links[m[1]]++
			}
			for _, id := range want {
				if links[id] == 0 {
					t.Errorf("el TOC no enlaza #%s", id)
				}
			}
			// #notas-2 y #b-2: una vez del TOC y otra del párrafo.
			if links["notas-2"] < 2 || links["b-2"] < 2 {
				t.Errorf("faltan los links del párrafo: %v", links)
			}
		})
	}
}

// Un documento sin repetidos da los mismos ids que antes: el de cada
// encabezado es su id base.
func TestDocumentHTMLHeadingIDsWithoutRepeats(t *testing.T) {
	doc, _ := parser.New(util.NewNoop()).ParseDocument("# Uno\n\n## Dos\n\n# Tres\n", "")
	html := renderer.GenerateDocumentHTML(doc, renderer.DocumentHTMLOptions{TOC: true, TOCDepth: 3}, renderer.NewDefaultRenderContext())
	var headings []string
	for _, m := range htmlHeadingID.FindAllStringSubmatch(html, -1) {
		headings = append(headings, m[1])
	}
	if want := []string{"uno", "dos", "tres"}; !reflect.DeepEqual(headings, want) {
		t.Errorf("ids = %v, want %v", headings, want)
	}
	if !strings.Contains(html, `href="#tres"`) {
		t.Errorf("el TOC no enlaza #tres")
	}
}

// El título de una sección cuenta con sus {{variables}} sustituidas, como su
// id de siempre, también al repartir en el parser: con `# {{p}}` (p: Notas)
// antes de `## Notas` y `# Notas`, el primero en el documento conserva notas
// (revisión de #267, P2).
func TestDocumentSectionAnchorsWithVariables(t *testing.T) {
	src := "---\nvariables:\n  p: Notas\n---\n\n# {{p}}\n\n## Notas\n\n# Notas\n"
	doc, _ := parser.New(util.NewNoop()).ParseDocument(src, "")
	sections := renderer.DocumentSectionAnchors(doc, true)
	heading := doc.ContentBlocks[0].Elements[0].(*ast.TextElement).HeadingAnchor
	got := []string{sections[0], heading, sections[1]}
	if want := []string{"notas", "notas-2", "notas-3"}; !reflect.DeepEqual(got, want) {
		t.Errorf("ids = %v, want %v", got, want)
	}
}
