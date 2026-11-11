// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/renderer"
)

// calloutProseCases son avisos sin elementos anidados: con un párrafo, con
// varios, con listas numeradas y anidadas y mezclados.
var calloutProseCases = []struct{ name, body string }{
	{"un párrafo", "Solo un párrafo con **negrita**."},
	{"varios párrafos", "Primer párrafo.\n\nSegundo párrafo."},
	{"lista numerada", "1. a\n2. b"},
	{"lista anidada", "- a\n  - b"},
	{"mixto", "Intro.\n\n1. uno\n   - sub\n2. dos\n\nCierre."},
}

func calloutProseSource() string {
	var src strings.Builder
	src.WriteString("---\ntitle: Avisos\n---\n\n# Avisos\n\n")
	for _, c := range calloutProseCases {
		src.WriteString(":::note\n" + c.body + "\n:::\n\n")
	}
	return src.String()
}

// El contentHTML de cada aviso en get_ast (RenderASTJSON, lo que devuelve el
// tool del MCP) es el interior de su .alert-content en el HTML de doclang
// build. Necesita un core con InlineHTMLOptions.SetBlockProse: contra el core
// publicado que todavía no la trae (build-test de CI) se salta, y corre en
// workspace-integration, que compila contra el core del árbol.
func TestRenderASTJSON_CalloutContentHTMLMatchesBuildHTML(t *testing.T) {
	if _, ok := any(&renderer.InlineHTMLOptions{}).(interface{ SetBlockProse(bool) }); !ok {
		t.Skip("el core publicado no trae InlineHTMLOptions.SetBlockProse")
	}

	out := filepath.Join(t.TempDir(), "avisos.html")
	if err := New(newTestLogger()).Generate(parseInline(t, calloutProseSource()), out, GeneratorOptions{Format: "html"}); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	const open = `<div class="alert-content">`
	var fragments []string
	for _, part := range strings.Split(string(raw), open)[1:] {
		end := strings.Index(part, "</div>")
		if end < 0 {
			t.Fatalf("un .alert-content sin cerrar:\n%s", part)
		}
		fragments = append(fragments, part[:end])
	}

	astJSON, err := New(newTestLogger()).RenderASTJSON(parseInline(t, calloutProseSource()))
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		ContentBlocks []struct {
			Elements []struct {
				Type        string `json:"type"`
				ContentHTML string `json:"contentHTML"`
			} `json:"elements"`
		} `json:"contentBlocks"`
	}
	if err := json.Unmarshal(astJSON, &got); err != nil {
		t.Fatal(err)
	}
	var contentHTML []string
	for _, block := range got.ContentBlocks {
		for _, elem := range block.Elements {
			if elem.Type == "special_block" {
				contentHTML = append(contentHTML, elem.ContentHTML)
			}
		}
	}
	if len(fragments) != len(calloutProseCases) || len(contentHTML) != len(calloutProseCases) {
		t.Fatalf("%d .alert-content y %d special_block, se esperaban %d", len(fragments), len(contentHTML), len(calloutProseCases))
	}
	for i, c := range calloutProseCases {
		if contentHTML[i] != fragments[i] {
			t.Errorf("%s:\nget_ast contentHTML: %s\nHTML del build:      %s", c.name, contentHTML[i], fragments[i])
		}
	}
}

// El DOCX ya escribía cada párrafo de un aviso aparte y sus listas como listas
// de Word, numeradas y anidadas; esta prueba lo fija ahora que el HTML y
// get_ast lo hacen también.
func TestDOCXCalloutParagraphsAndLists(t *testing.T) {
	src := "# Avisos\n\n:::note\nPrimer párrafo.\n\nSegundo párrafo.\n\n1. uno\n2. dos\n   - sub\n:::\n"
	path := buildDOCXFile(t, parseInline(t, src))
	paras := docxParagraphs(docxZipPart(t, path, "word/document.xml"))
	var texts []string
	for _, p := range paras {
		if p.numID == 0 && strings.Contains(p.text, "párrafo") {
			texts = append(texts, p.text)
		}
	}
	if strings.Join(texts, "|") != "Primer párrafo.|Segundo párrafo." {
		t.Errorf("párrafos del aviso = %q, se esperaban dos párrafos separados", texts)
	}
	checkListItems(t, path, []wantListItem{
		{"uno", 0, "decimal", 1},
		{"dos", 0, "decimal", 0},
		{"sub", 1, "bullet", 0},
	})
}
