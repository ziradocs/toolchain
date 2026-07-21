// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeneratorGenerateHTML(t *testing.T) {
	logger := newTestLogger()
	gen := New(logger)
	doc := newTestAST()

	tmpDir := t.TempDir()
	output := filepath.Join(tmpDir, "document.html")

	if err := gen.Generate(doc, output, GeneratorOptions{Format: "html"}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "Sample Document") {
		t.Fatalf("generated HTML does not contain content block title: %s", content)
	}
	if len(logger.infos) == 0 || !strings.Contains(logger.infos[0], "GENERATOR") {
		t.Fatalf("expected generator log entry, got %#v", logger.infos)
	}
}

func TestGeneratorGenerateDocx(t *testing.T) {
	logger := newTestLogger()
	gen := New(logger)
	doc := newTestAST()
	output := filepath.Join(t.TempDir(), "document.docx")

	err := gen.Generate(doc, output, GeneratorOptions{Format: "docx"})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Verify DOCX file was created
	if _, err := os.Stat(output); os.IsNotExist(err) {
		t.Fatal("DOCX file was not created")
	}

	// Verify DOCX logs were generated
	foundDocxLog := false
	for _, log := range logger.infos {
		if strings.Contains(log, "DOCX") {
			foundDocxLog = true
			break
		}
	}
	if !foundDocxLog {
		t.Fatalf("expected DOCX generator log entry, got %#v", logger.infos)
	}
}

func TestGeneratorGenerateUnsupportedFormat(t *testing.T) {
	gen := New(newTestLogger())
	doc := newTestAST()
	output := filepath.Join(t.TempDir(), "document.txt")

	err := gen.Generate(doc, output, GeneratorOptions{Format: "txt"})
	if err == nil {
		t.Fatal("expected error for unsupported format")
	}
	if !strings.Contains(err.Error(), "unsupported format") {
		t.Fatalf("unexpected error: %v", err)
	}
}
