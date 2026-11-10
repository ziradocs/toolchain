// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"encoding/json"
	"testing"
)

// El título de una sección sale escapado y sin Markdown en el <h1> de
// doclang, así que el titleHTML de get_ast (RenderASTJSON) dice lo mismo:
// texto plano escapado, aunque los títulos de slide sí lleven Markdown.
func TestRenderASTJSON_SectionTitleStaysPlain(t *testing.T) {
	src := "---\ntitle: Doc\nmode: flex\n---\n\n# Ventas **18%** & más\n\nTexto con **negrita**.\n"
	doc := parseInline(t, src)
	out, err := New(newTestLogger()).RenderASTJSON(doc)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		ContentBlocks []struct {
			Title       string `json:"title"`
			TitleHTML   string `json:"titleHTML"`
			Heading     string `json:"heading"`
			HeadingHTML string `json:"headingHTML"`
		} `json:"contentBlocks"`
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.ContentBlocks) == 0 {
		t.Fatal("no content blocks")
	}
	b := got.ContentBlocks[0]
	raw, html := b.Title, b.TitleHTML
	if raw == "" {
		raw, html = b.Heading, b.HeadingHTML
	}
	if raw != "Ventas **18%** & más" || html != "Ventas **18%** &amp; más" {
		t.Errorf("section title %q, html %q", raw, html)
	}
}
