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

func TestMediaCaptionInMarkdownAndDOCX(t *testing.T) {
	doc := parseDocLang(t, "---\ntitle: D\n---\n# Doc\n\n<<video src=\"https://example.com/demo.mp4\" poster=\"https://example.com/p.jpg\" caption=\"Product demo\" controls>>\n")
	dir := t.TempDir()
	md := filepath.Join(dir, "d.md")
	if err := New(util.NewNoop()).Generate(doc, md, GeneratorOptions{Format: "markdown"}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(md)
	if !strings.Contains(string(data), "*Product demo*") {
		t.Fatalf("markdown lacks caption:\n%s", data)
	}
	docx := filepath.Join(dir, "d.docx")
	if err := New(util.NewNoop()).Generate(doc, docx, GeneratorOptions{Format: "docx", AssetRoot: dir}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(docxDocumentXML(t, docx), "Product demo") {
		t.Fatal("docx lacks caption")
	}
}
