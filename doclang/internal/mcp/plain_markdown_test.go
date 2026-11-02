// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestLintTool_PlainMarkdown: Markdown sin frontmatter y solo con `##` es
// válido para la tool `lint`, igual que para `doclang build` — sin
// FRONT003, sin reglas de layout de slides, con una sección inicial sin
// encabezado.
func TestLintTool_PlainMarkdown(t *testing.T) {
	session, ctx := newTestSession(t)

	res := callTool(t, session, ctx, "lint", map[string]any{"source": "## Solo dos\n\nTexto SENTINEL_C.\n"})
	var out lintOutput
	if err := json.Unmarshal([]byte(resultText(t, res)), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if res.IsError || !out.Valid {
		t.Fatalf("expected Valid=true, diagnostics: %+v", out.Diagnostics)
	}
	if out.SectionCount != 1 {
		t.Errorf("expected SectionCount=1, got %d", out.SectionCount)
	}
	for _, d := range out.Diagnostics {
		for _, id := range []string{"FRONT003", "LAYOUT002", "LAYOUT_FORBIDDEN_ELEMENT", "LAYOUT_AUTO_CLOSING"} {
			if d.RuleID == id || d.Code == id {
				t.Errorf("regla de slides sobre un documento: %+v", d)
			}
		}
	}
}

// TestLintTool_EmptyDocument: un documento vacío sigue siendo inválido,
// con el CORE001 de documentos.
func TestLintTool_EmptyDocument(t *testing.T) {
	session, ctx := newTestSession(t)

	res := callTool(t, session, ctx, "lint", map[string]any{"source": "\n"})
	var out lintOutput
	if err := json.Unmarshal([]byte(resultText(t, res)), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Valid {
		t.Fatal("un documento vacío no es válido")
	}
	found := false
	for _, d := range out.Diagnostics {
		if d.RuleID == "CORE001" {
			found = true
			if strings.Contains(d.Message, "slide") {
				t.Errorf("el mensaje no debe hablar de slides: %q", d.Message)
			}
		}
	}
	if !found {
		t.Errorf("expected CORE001, got %+v", out.Diagnostics)
	}
}

// TestGetASTTool_PlainMarkdown: get_ast conserva el texto previo al primer
// `#` en una sección sin encabezado y no sintetiza frontmatter.
func TestGetASTTool_PlainMarkdown(t *testing.T) {
	session, ctx := newTestSession(t)

	src := "Intro SENTINEL_F.\n\n# Título\n\nCuerpo SENTINEL_G.\n"
	res := callTool(t, session, ctx, "get_ast", map[string]any{"source": src})
	if res.IsError {
		t.Fatalf("expected success, got: %s", resultText(t, res))
	}
	var doc struct {
		FrontMatter   json.RawMessage `json:"frontMatter"`
		ContentBlocks []struct {
			BlockType string `json:"blockType"`
			Heading   string `json:"heading"`
			Title     string `json:"title"`
		} `json:"contentBlocks"`
	}
	text := resultText(t, res)
	if err := json.Unmarshal([]byte(text), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(doc.FrontMatter) > 0 && string(doc.FrontMatter) != "null" {
		t.Errorf("no se sintetiza frontmatter, got %s", doc.FrontMatter)
	}
	if len(doc.ContentBlocks) != 2 {
		t.Fatalf("want preámbulo + sección, got %+v", doc.ContentBlocks)
	}
	if b := doc.ContentBlocks[0]; b.BlockType != "content" || b.Heading != "" || b.Title != "" {
		t.Errorf("preámbulo sin encabezado, got %+v", b)
	}
	if b := doc.ContentBlocks[1]; b.BlockType != "title" || b.Heading != "Título" {
		t.Errorf("sección del `#`, got %+v", b)
	}
	for _, s := range []string{"SENTINEL_F", "SENTINEL_G"} {
		if !strings.Contains(text, s) {
			t.Errorf("falta %s en el AST", s)
		}
	}
}

// TestPreviewTool_TitleFromFirstH1: sin frontmatter, el <title> de la
// vista previa sale del primer `#` (no hay nombre de archivo en memoria).
func TestPreviewTool_TitleFromFirstH1(t *testing.T) {
	session, ctx := newTestSession(t)

	res := callTool(t, session, ctx, "preview", map[string]any{"source": "Intro.\n\n# Del H1\n\nCuerpo.\n"})
	if res.IsError {
		t.Fatalf("expected success, got: %s", resultText(t, res))
	}
	if html := resultText(t, res); !strings.Contains(html, "<title>Del H1</title>") {
		t.Errorf("expected <title>Del H1</title> in the preview")
	}
}
