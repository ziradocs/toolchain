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

func TestQuizPollResultsInMarkdownAndDOCX(t *testing.T) {
	doc := parseDocLang(t, "---\ntitle: D\n---\n# Doc\n\n<<poll>>\n  question: \"Plan?\"\n  options: [\"Basic\", \"Pro\"]\n  results: [35, 65]\n  responses: 40\n<<end>>\n")
	dir := t.TempDir()
	md := filepath.Join(dir, "d.md")
	if err := New(util.NewNoop()).Generate(doc, md, GeneratorOptions{Format: "markdown"}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(md)
	for _, want := range []string{"1. Basic — 35%", "2. Pro — 65%", "*40 responses*"} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("markdown lacks %q:\n%s", want, data)
		}
	}
	docx := filepath.Join(dir, "d.docx")
	if err := New(util.NewNoop()).Generate(doc, docx, GeneratorOptions{Format: "docx", AssetRoot: dir}); err != nil {
		t.Fatal(err)
	}
	if xml := docxDocumentXML(t, docx); !strings.Contains(xml, "65%") || !strings.Contains(xml, "40 responses") {
		t.Fatal("docx lacks results")
	}
}
