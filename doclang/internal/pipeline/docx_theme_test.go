// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package pipeline

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/util"
)

type discardLogger struct{}

func (discardLogger) Error(string, ...interface{})           {}
func (discardLogger) Warn(string, ...interface{})            {}
func (discardLogger) Info(string, string, ...interface{})    {}
func (discardLogger) Debug(string, string, ...interface{})   {}
func (discardLogger) Progress(string, string, int)           {}
func (discardLogger) Summary(string, map[string]interface{}) {}
func (discardLogger) SetLevel(util.LogLevel)                 {}

// TestGenerate_ExternalThemeReachesDOCX: un tema externo cargado desde
// ./themes llega al DOCX con sus variables. Falla si Generate deja de pasar
// ThemeExternal al generador (el DOCX caería a professional).
func TestGenerate_ExternalThemeReachesDOCX(t *testing.T) {
	dir := t.TempDir()
	themes := filepath.Join(dir, "themes")
	if err := os.MkdirAll(themes, 0o755); err != nil {
		t.Fatal(err)
	}
	themeJSON := `{"name":"ext-docx","version":"1.0.0","variables":{
		"--doclang-page-bg":"#ffffff","--doclang-font-main":"'Palatino Linotype', serif",
		"--doclang-h1-color":"#aa0011","--doclang-text-color":"#223344",
		"--doclang-code-bg":"#101820","--doclang-table-border":"#abcdef"}}`
	if err := os.WriteFile(filepath.Join(themes, "ext-docx.json"), []byte(themeJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	log := discardLogger{}
	prepared, err := Prepare(log, []byte("# Titulo\n\nTexto del documento.\n"), PrepareOptions{ThemeName: "ext-docx", ThemeTrusted: true, MaxInputBytes: 1 << 20, IncludeRoot: dir})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if !prepared.Theme.IsExternal {
		t.Fatalf("theme should load as external, got %+v", prepared.Theme.Name)
	}

	out := filepath.Join(dir, "out.docx")
	if err := Generate(log, prepared, "doc.doclang", GenerateOptions{Format: "docx", OutputFile: out, RenderMode: "browser", DiagramBackend: "chromium"}); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	r, err := zip.OpenReader(out)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	var docXML string
	for _, f := range r.File {
		if f.Name == "word/document.xml" {
			rc, _ := f.Open()
			b, _ := io.ReadAll(rc)
			_ = rc.Close()
			docXML = string(b)
		}
	}
	for _, want := range []string{`w:ascii="Palatino Linotype"`, "AA0011", "223344"} {
		if !strings.Contains(docXML, want) {
			t.Errorf("document.xml missing %q: the external theme did not reach the DOCX", want)
		}
	}
}
