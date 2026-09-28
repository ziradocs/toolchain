// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
)

const typedHeadingsDocBody = `# Report

Intro text.

## Scope **now**

Body with [hola]{lang=es}.

### Details

More.

## Scope **now**

Repeated title.
`

var cspNonce = regexp.MustCompile(`nonce(-|=")[A-Za-z0-9+/=]+`)

func parseDocLang(t *testing.T, src string) *ast.AST {
	t.Helper()
	doc, diags := parser.New(util.NewNoop()).ParseDocument(src, "report.doclang")
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("parse error: %v", d)
		}
	}
	return doc
}

// Oráculo de la bajada: el mismo documento con y sin typed-headings-v1 produce
// salida idéntica en HTML (con TOC y numeración), Markdown y DOCX, y el AST
// tipado del caller no se modifica.
func TestTypedHeadingsDocLangOutputsMatchLegacy(t *testing.T) {
	legacy := parseDocLang(t, "---\ntitle: Report\n---\n"+typedHeadingsDocBody)
	typed := parseDocLang(t, "---\ntitle: Report\nast_capabilities: [typed-headings-v1]\n---\n"+typedHeadingsDocBody)
	if !ast.UsesTypedHeadings(typed) || ast.UsesTypedHeadings(legacy) {
		t.Fatal("fixture did not produce the expected heading forms")
	}
	for _, tc := range []struct {
		format, ext string
		toc, num    bool
	}{
		{"html", "html", false, false},
		{"html", "html", true, true},
		{"markdown", "md", false, false},
		{"docx", "docx", true, false},
	} {
		dir := t.TempDir()
		outputs := map[string]string{}
		for name, doc := range map[string]*ast.AST{"legacy": legacy, "typed": typed} {
			out := filepath.Join(dir, name+"."+tc.ext)
			opts := GeneratorOptions{Format: tc.format, TOC: tc.toc, Numbering: tc.num, AssetRoot: dir}
			if err := New(util.NewNoop()).Generate(doc, out, opts); err != nil {
				t.Fatalf("%s %s: %v", tc.format, name, err)
			}
			if tc.format == "docx" {
				outputs[name] = docxDocumentXML(t, out)
			} else {
				data, err := os.ReadFile(out)
				if err != nil {
					t.Fatal(err)
				}
				// El nonce CSP es aleatorio por render; no es parte del oráculo.
				outputs[name] = cspNonce.ReplaceAllString(string(data), "nonce-X")
			}
		}
		if l, ty := outputs["legacy"], outputs["typed"]; l != ty {
			i := 0
			for i < len(l) && i < len(ty) && l[i] == ty[i] {
				i++
			}
			lo := max(0, i-200)
			t.Fatalf("%s (toc=%v numbering=%v) differs between legacy and typed headings at byte %d\nlegacy: %q\n typed: %q", tc.format, tc.toc, tc.num, i, l[lo:min(i+200, len(l))], ty[lo:min(i+200, len(ty))])
		}
		if !strings.Contains(outputs["typed"], "Scope") {
			t.Fatalf("%s output lost the heading text", tc.format)
		}
	}
	if !ast.UsesTypedHeadings(typed) {
		t.Fatal("generation mutated the typed AST")
	}
}
