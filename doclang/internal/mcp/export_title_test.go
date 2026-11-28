// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"go.ziradocs.com/doclang/v2/internal/pipeline"
)

// El `title` de export es el --title de build: gana sobre el de la fuente y
// no prende nada más.
func TestExport_Title(t *testing.T) {
	f := newExportFixture(t)
	f.write(t, "a.md", plainMarkdown)
	s, ctx := newExportSession(t, Options{Root: f.root})

	exportOK(t, s, ctx, map[string]any{"path": "a.md", "format": "html", "title": "Guía\nQ3"})
	page, err := os.ReadFile(filepath.Join(f.root, "a.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(page, []byte("<title>Guía Q3</title>")) {
		t.Errorf("the export title is not the title argument")
	}
	if bytes.Contains(page, []byte("doclang-viewer")) {
		t.Errorf("a title turned on the interactive viewer of a file without front matter")
	}

	exportOK(t, s, ctx, map[string]any{"path": "a.md", "format": "docx", "title": "Guía Q3"})
	core := zipContents(t, filepath.Join(f.root, "a.docx"))["docProps/core.xml"]
	if !bytes.Contains(core, []byte("<dc:title>Guía Q3</dc:title>")) {
		t.Errorf("docProps/core.xml: %s", core)
	}

	// Sin title manda la fuente.
	exportOK(t, s, ctx, map[string]any{"path": "a.md", "format": "html", "overwrite": true})
	page, err = os.ReadFile(filepath.Join(f.root, "a.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(page, []byte("<title>Informe trimestral</title>")) {
		t.Errorf("without title the export is not titled from the source")
	}
}

// Un title presente tiene que decir algo, igual que --title: "" no vale
// como ausente.
func TestExport_TitleRejections(t *testing.T) {
	f := newExportFixture(t)
	f.write(t, "a.md", plainMarkdown)
	s, ctx := newExportSession(t, Options{Root: f.root})

	for _, value := range []string{"", "  ", "INVISIBLE", strings.Repeat("a", pipeline.MaxTitleRunes+1)} {
		if value == "INVISIBLE" {
			value = string([]rune{0xFEFF, 0x200B})
		}
		exportFail(t, s, ctx, map[string]any{"path": "a.md", "format": "html", "title": value}, "invalid title")
	}
	if _, err := os.Stat(filepath.Join(f.root, "a.html")); err == nil {
		t.Errorf("a refused export wrote its output")
	}
}

// La descripción del parámetro dice el tope de largo que se aplica.
func TestExport_TitleSchemaStatesTheLimit(t *testing.T) {
	f := newExportFixture(t)
	s, ctx := newExportSession(t, Options{Root: f.root})
	got, err := s.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range got.Tools {
		if tool.Name != "export" {
			continue
		}
		schema, err := json.Marshal(tool.InputSchema)
		if err != nil {
			t.Fatal(err)
		}
		var parsed struct {
			Properties map[string]struct {
				Description string `json:"description"`
			} `json:"properties"`
		}
		if err := json.Unmarshal(schema, &parsed); err != nil {
			t.Fatal(err)
		}
		desc := parsed.Properties["title"].Description
		if !strings.Contains(desc, "over "+strconv.Itoa(pipeline.MaxTitleRunes)+" characters") {
			t.Errorf("the title description does not state the %d-character limit: %q", pipeline.MaxTitleRunes, desc)
		}
		return
	}
	t.Fatal("export tool not registered")
}
