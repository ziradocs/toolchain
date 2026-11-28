// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/util"
)

const repeatedAnchorsSource = `---
title: "Ligas repetidas"
---

# A

Ver [primeras](#notas), [segundas](#notas-2), [otra B](#b-2) y [sin letras](#seccion).

## Notas

Primeras notas.

# B

## Notas

Segundas notas.

# B

Otra vez B.

# 概要

Sin letras latinas.
`

var docxBookmarkNamePattern = regexp.MustCompile(`<w:bookmarkStart [^>]*w:name="_Toc(\d+)"`)

// Issue #267 en DOCX: cada w:anchor de un link interno tiene su
// w:bookmarkStart, y el de #notas-2 es el del SEGUNDO encabezado "Notas", no
// el del primero (resolveAnchorLinks toma el primer encabezado con esa ancla,
// y antes los dos tenían notas). Lo mismo para la sección repetida (#b-2) y
// para la que no tiene letras latinas (#seccion).
func TestDOCXLinksToRepeatedHeadings(t *testing.T) {
	for name, opts := range map[string]GeneratorOptions{
		"plain":             {},
		"toc and numbering": {TOC: true, Numbering: true},
	} {
		t.Run(name, func(t *testing.T) {
			xml, _ := generateDocxFromSource(t, repeatedAnchorsSource, opts)
			headings := docxBookmarkHeadings(xml)

			// Orden de los encabezados en el cuerpo: _Toc<n> por posición.
			var order []string
			for _, m := range docxBookmarkNamePattern.FindAllStringSubmatch(xml, -1) {
				order = append(order, "_Toc"+m[1])
			}
			index := map[string]int{}
			for i, name := range order {
				index[name] = i
			}

			got := map[string]string{}
			for _, l := range docxLinks(xml) {
				anchor := l.attrs["w:anchor"]
				if anchor == "" {
					continue
				}
				if _, ok := headings[anchor]; !ok {
					t.Errorf("%q: w:anchor %q no tiene w:bookmarkStart", l.text, anchor)
				}
				got[l.text] = anchor
			}
			for _, text := range []string{"primeras", "segundas", "otra B", "sin letras"} {
				if got[text] == "" {
					t.Fatalf("%q no es un hipervínculo interno; links: %v", text, got)
				}
			}
			if !strings.HasSuffix(headings[got["primeras"]], "Notas") || !strings.HasSuffix(headings[got["segundas"]], "Notas") {
				t.Errorf("los links de notas no llegan a un encabezado Notas: %q, %q", headings[got["primeras"]], headings[got["segundas"]])
			}
			if index[got["segundas"]] <= index[got["primeras"]] {
				t.Errorf("#notas-2 lleva a %s y #notas a %s: el segundo debe ir después", got["segundas"], got["primeras"])
			}
			if !strings.HasSuffix(headings[got["otra B"]], "B") || index[got["otra B"]] <= index[got["segundas"]] {
				t.Errorf("#b-2 lleva a %s (%q), no a la segunda sección B", got["otra B"], headings[got["otra B"]])
			}
			if !strings.HasSuffix(headings[got["sin letras"]], "概要") {
				t.Errorf("#seccion lleva a %q, want 概要", headings[got["sin letras"]])
			}
		})
	}
}

// En Markdown, una sección cuyo id no es el que su título deriva solo (porque
// se repite o no deja caracteres) lo declara con {#id}, y el TOC enlaza ese
// id. Una sección con título único queda como antes, sin {#id}.
func TestMarkdownRepeatedSectionAnchors(t *testing.T) {
	doc := parseDocLang(t, repeatedAnchorsSource)
	out := filepath.Join(t.TempDir(), "doc.md")
	if err := New(util.NewNoop()).Generate(doc, out, GeneratorOptions{Format: "markdown", TOC: true}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	md := string(b)
	for _, want := range []string{
		"## B {#b-2}\n", "## 概要 {#seccion}\n", "{#notas-2}",
		"- [B](#b-2)\n", "- [概要](#seccion)\n",
		"## A\n", "- [A](#a)\n", "- [B](#b)\n",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("falta %q en:\n%s", want, md)
		}
	}
	if strings.Contains(md, "## A {#") || strings.Count(md, "{#b}") > 0 {
		t.Errorf("una sección con id derivado lleva {#id}:\n%s", md)
	}
	// Cada {#id} es único.
	seen := map[string]bool{}
	for _, m := range regexp.MustCompile(`\{#([^}]+)\}`).FindAllStringSubmatch(md, -1) {
		if seen[m[1]] {
			t.Errorf("{#%s} repetido", m[1])
		}
		seen[m[1]] = true
	}
}

// Con {{variables}} en el título de una sección, el HTML, el DOCX y el
// Markdown reparten igual (revisión de #267, P2): `# {{p}}` (p: Notas) es el
// primero en el documento y conserva notas; `## Notas` queda notas-2 y
// `# Notas`, notas-3. Antes el DOCX y el Markdown derivaban del título sin
// sustituir y llevaban #notas-2 y #notas-3 a otro lado.
func TestRepeatedAnchorsWithVariablesAgreeAcrossFormats(t *testing.T) {
	const src = "---\ntitle: \"Variables\"\nvariables:\n  p: Notas\n---\n\n# {{p}}\n\nVer [uno](#notas), [dos](#notas-2) y [tres](#notas-3).\n\n## Notas\n\nTexto.\n\n# Notas\n\nOtro.\n"
	dir := t.TempDir()

	htmlOut := filepath.Join(dir, "doc.html")
	if err := New(util.NewNoop()).Generate(parseDocLang(t, src), htmlOut, GeneratorOptions{Format: "html"}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(htmlOut)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, m := range regexp.MustCompile(`<h[1-6] id="([^"]*)"`).FindAllStringSubmatch(string(b), -1) {
		ids = append(ids, m[1])
	}
	if want := []string{"notas", "notas-2", "notas-3"}; strings.Join(ids, " ") != strings.Join(want, " ") {
		t.Fatalf("ids del HTML = %v, want %v", ids, want)
	}

	xml, _ := generateDocxFromSource(t, src, GeneratorOptions{})
	var order []string
	for _, m := range docxBookmarkNamePattern.FindAllStringSubmatch(xml, -1) {
		order = append(order, "_Toc"+m[1])
	}
	position := map[string]int{}
	for i, name := range order {
		position[name] = i
	}
	got := map[string]int{}
	for _, l := range docxLinks(xml) {
		if a := l.attrs["w:anchor"]; a != "" {
			got[l.text] = position[a]
		}
	}
	// El mismo orden que los ids del HTML: uno al primero, dos al segundo,
	// tres al tercero.
	for i, text := range []string{"uno", "dos", "tres"} {
		if p, ok := got[text]; !ok || p != i {
			t.Errorf("DOCX: %q va al encabezado %d (ok=%v), want %d; links: %v", text, p, ok, i, got)
		}
	}

	mdOut := filepath.Join(dir, "doc.md")
	if err := New(util.NewNoop()).Generate(parseDocLang(t, src), mdOut, GeneratorOptions{Format: "markdown"}); err != nil {
		t.Fatal(err)
	}
	mb, err := os.ReadFile(mdOut)
	if err != nil {
		t.Fatal(err)
	}
	md := string(mb)
	// El Markdown deja `{{p}}` sin sustituir, así que el visor derivaría p:
	// cada encabezado declara su id, y los tres enlaces llegan, en orden, al
	// mismo encabezado que en el HTML y el DOCX.
	var mdIDs []string
	for _, m := range regexp.MustCompile(`(?m)^#+ (.*) \{#([^}]+)\}$`).FindAllStringSubmatch(md, -1) {
		mdIDs = append(mdIDs, m[2])
	}
	if want := []string{"notas", "notas-2", "notas-3"}; strings.Join(mdIDs, " ") != strings.Join(want, " ") {
		t.Errorf("ids declarados en el Markdown = %v, want %v:\n%s", mdIDs, want, md)
	}
	if !strings.Contains(md, "## {{p}} {#notas}\n") {
		t.Errorf("la sección {{p}} no declara su id:\n%s", md)
	}
	for _, link := range []string{"[uno](#notas)", "[dos](#notas-2)", "[tres](#notas-3)"} {
		if !strings.Contains(md, link) {
			t.Errorf("Markdown sin el link %s:\n%s", link, md)
		}
	}
}
