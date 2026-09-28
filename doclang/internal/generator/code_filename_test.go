// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/util"
)

func TestCodeFilenameInMarkdownAndDOCX(t *testing.T) {
	doc := parseDocLang(t, "---\ntitle: D\n---\n# Doc\n\n```typescript renewals.ts\nconst x = 1;\n```\n")
	dir := t.TempDir()
	md := filepath.Join(dir, "d.md")
	if err := New(util.NewNoop()).Generate(doc, md, GeneratorOptions{Format: "markdown"}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(md)
	if !strings.Contains(string(data), "`renewals.ts`\n\n```typescript\n") {
		t.Fatalf("markdown lacks filename:\n%s", data)
	}
	docx := filepath.Join(dir, "d.docx")
	if err := New(util.NewNoop()).Generate(doc, docx, GeneratorOptions{Format: "docx", AssetRoot: dir}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(docxDocumentXML(t, docx), "renewals.ts") {
		t.Fatal("docx lacks filename")
	}
}
