// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"archive/zip"
	"io"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const docxLinksSource = `---
title: "Ligas"
---

# Inicio

Ver [la sección dos](#seccion-dos), [la subsección](#detalle-de-costos) y [la repetida](#repetida). Como en GitHub: [con acento](#sección-dos), [con mayúsculas](#Seccion-Dos) y [codificada](#secci%C3%B3n-dos). Afuera: [sitio](https://example.com/a), **[en negritas](https://example.com/b)**, [` + "`codigo`" + `](https://example.com/c), [correo](mailto:equipo@example.com), [sin destino](#no-existe) y [relativa](otro/archivo.md).

| Columna | Liga |
|---|---|
| uno | [en celda](https://example.com/d) |
| dos | [interna en celda](#seccion-dos) |

- Un punto con [liga en lista](https://example.com/e)

# Sección dos

Texto.

## Detalle de costos

Más texto.

# Repetida

Primera.

# Repetida

Segunda.
`

const docxStrictLinksSource = `---
mode: strict
title: "Ligas"
---

SECTION "Inicio"
  TEXT
    Ver [la explícita](#costos-anuales).

SECTION "Costos"
  TEXT
    Intro.

SECTION "Costos por año"
  level: 2
  id: costos-anuales

  TEXT
    Fin.
`

var (
	docxHyperlinkPattern = regexp.MustCompile(`(?s)<w:hyperlink ([^>]*)>(.*?)</w:hyperlink>`)
	docxAttrPattern      = regexp.MustCompile(`([\w:]+)="([^"]*)"`)
	docxParagraphPattern = regexp.MustCompile(`(?s)<w:p>.*?</w:p>|<w:p [^>]*>.*?</w:p>`)
	docxBookmarkPattern  = regexp.MustCompile(`<w:bookmarkStart [^>]*w:name="([^"]+)"`)
)

type docxLink struct {
	attrs map[string]string
	text  string
}

func docxLinks(xml string) []docxLink {
	var out []docxLink
	for _, m := range docxHyperlinkPattern.FindAllStringSubmatch(xml, -1) {
		attrs := map[string]string{}
		for _, a := range docxAttrPattern.FindAllStringSubmatch(m[1], -1) {
			attrs[a[1]] = a[2]
		}
		out = append(out, docxLink{attrs: attrs, text: docxVisibleText(m[2])})
	}
	return out
}

// docxBookmarkHeadings es el texto del párrafo de cada w:bookmarkStart, por
// nombre del marcador.
func docxBookmarkHeadings(xml string) map[string]string {
	out := map[string]string{}
	for _, p := range docxParagraphPattern.FindAllString(xml, -1) {
		if m := docxBookmarkPattern.FindStringSubmatch(p); m != nil {
			out[m[1]] = docxVisibleText(p)
		}
	}
	return out
}

func docxRelationships(t *testing.T, docxPath string) string {
	t.Helper()
	r, err := zip.OpenReader(docxPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	for _, f := range r.File {
		if f.Name == "word/_rels/document.xml.rels" {
			rc, err := f.Open()
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = rc.Close() }()
			b, _ := io.ReadAll(rc)
			return string(b)
		}
	}
	t.Fatal("no document.xml.rels")
	return ""
}

func generateDocxFromSource(t *testing.T, src string, opts GeneratorOptions) (xml, rels string) {
	t.Helper()
	output := filepath.Join(t.TempDir(), "links.docx")
	opts.Format = "docx"
	if err := New(newTestLogger()).Generate(parseDocLang(t, src), output, opts); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	return docxDocumentXML(t, output), docxRelationships(t, output)
}

// Un link interno [texto](#ancla) es un w:hyperlink con w:anchor hacia el
// marcador del encabezado cuyo id en el HTML es esa ancla; uno con esquema
// es un w:hyperlink externo con su relación. Antes los dos eran texto con
// color y subrayado, sin destino. Vale con y sin TOC y numeración (que
// agregan encabezados y números al texto), en celdas de tabla y en listas.
func TestDOCXInternalLinksTargetTheirHeading(t *testing.T) {
	for name, opts := range map[string]GeneratorOptions{
		"plain":             {},
		"toc and numbering": {TOC: true, Numbering: true},
	} {
		t.Run(name, func(t *testing.T) {
			xml, rels := generateDocxFromSource(t, docxLinksSource, opts)
			headings := docxBookmarkHeadings(xml)
			wantAnchor := map[string]string{
				"la sección dos":   "Sección dos",
				"la subsección":    "Detalle de costos",
				"interna en celda": "Sección dos",
				"la repetida":      "Repetida",
				"con acento":       "Sección dos",
				"con mayúsculas":   "Sección dos",
				"codificada":       "Sección dos",
			}
			wantURL := map[string]string{
				"sitio":       "https://example.com/a",
				"en negritas": "https://example.com/b",
				// La etiqueta sale con las comillas invertidas: el .docx no
				// lee código dentro de la etiqueta de un link.
				"`codigo`":      "https://example.com/c",
				"correo":        "mailto:equipo@example.com",
				"en celda":      "https://example.com/d",
				"liga en lista": "https://example.com/e",
			}
			seen := map[string]bool{}
			for _, l := range docxLinks(xml) {
				seen[l.text] = true
				if heading, ok := wantAnchor[l.text]; ok {
					got := headings[l.attrs["w:anchor"]]
					if !strings.HasSuffix(got, heading) {
						t.Errorf("%q: w:anchor %q is the bookmark of %q, want the heading %q", l.text, l.attrs["w:anchor"], got, heading)
					}
					continue
				}
				if url, ok := wantURL[l.text]; ok {
					id := l.attrs["r:id"]
					if id == "" || !strings.Contains(rels, `Id="`+id+`"`) || !strings.Contains(rels, `Target="`+url+`"`) || !strings.Contains(rels, `TargetMode="External"`) {
						t.Errorf("%q: r:id %q does not reach %q in the relationships:\n%s", l.text, id, url, rels)
					}
					continue
				}
				t.Errorf("unexpected hyperlink %q %v", l.text, l.attrs)
			}
			for text := range wantAnchor {
				if !seen[text] {
					t.Errorf("%q is not a hyperlink", text)
				}
			}
			for text := range wantURL {
				if !seen[text] {
					t.Errorf("%q is not a hyperlink", text)
				}
			}
			// Un ancla sin encabezado y un destino relativo siguen como texto
			// con estilo de link: un w:anchor sin marcador, o un destino que
			// Word resolvería contra la carpeta del .docx, no llevan a nada.
			for _, text := range []string{"sin destino", "relativa"} {
				if seen[text] {
					t.Errorf("%q should stay styled text", text)
				}
				if run := docxRunContaining(t, xml, text); !strings.Contains(run, "<w:u ") {
					t.Errorf("%q lost its link styling:\n%s", text, run)
				}
			}
			// "la repetida" va al primero de los dos encabezados iguales,
			// como en el HTML.
			for _, l := range docxLinks(xml) {
				if l.text != "la repetida" {
					continue
				}
				first := ""
				for _, m := range docxParagraphPattern.FindAllString(xml, -1) {
					if b := docxBookmarkPattern.FindStringSubmatch(m); b != nil && strings.HasSuffix(docxVisibleText(m), "Repetida") {
						first = b[1]
						break
					}
				}
				if l.attrs["w:anchor"] != first {
					t.Errorf("the repeated heading link goes to %q, want the first one %q", l.attrs["w:anchor"], first)
				}
			}
		})
	}
}

// El id: de un SECTION strict (el ancla explícita de DocLang) funciona igual
// que el derivado del título.
func TestDOCXInternalLinkToAnExplicitID(t *testing.T) {
	xml, _ := generateDocxFromSource(t, docxStrictLinksSource, GeneratorOptions{})
	headings := docxBookmarkHeadings(xml)
	links := docxLinks(xml)
	if len(links) != 1 || links[0].text != "la explícita" || !strings.HasSuffix(headings[links[0].attrs["w:anchor"]], "Costos por año") {
		t.Fatalf("want one link to the heading with id: costos-anuales, got %+v (bookmarks %v)", links, headings)
	}
}
