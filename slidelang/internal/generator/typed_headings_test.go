//go:build !js

// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/util"
)

const typedHeadingsDeckBody = `# Deck

## Slide

### Results **now**

Body with [hola]{lang=es}.

#### Detail

### Results **now**
`

var cspNonce = regexp.MustCompile(`nonce(-|=")[A-Za-z0-9+/=]+`)

func parseSlideLang(t *testing.T, src string) *ast.AST {
	t.Helper()
	doc, diags := parser.New(util.NewNoop()).Parse(src, "deck.slidelang")
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("parse error: %v", d)
		}
	}
	return doc
}

func pptxSlideXML(t *testing.T, path string) string {
	t.Helper()
	r, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	var names []string
	parts := map[string]string{}
	for _, f := range r.File {
		if !strings.HasPrefix(f.Name, "ppt/slides/") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, _ := io.ReadAll(rc)
		_ = rc.Close()
		names = append(names, f.Name)
		parts[f.Name] = string(data)
	}
	sort.Strings(names)
	var b strings.Builder
	for _, n := range names {
		b.WriteString(n + "\n" + parts[n] + "\n")
	}
	return b.String()
}

// Oráculo de la bajada en SlideLang: HTML y PPTX idénticos con y sin
// typed-headings-v1; el JSON sí conserva el nodo tipado.
func TestTypedHeadingsSlideLangOutputsMatchLegacy(t *testing.T) {
	legacy := parseSlideLang(t, "---\nmode: flex\n---\n"+typedHeadingsDeckBody)
	typed := parseSlideLang(t, "---\nmode: flex\nast_capabilities: [typed-headings-v1]\n---\n"+typedHeadingsDeckBody)
	if !ast.UsesTypedHeadings(typed) || ast.UsesTypedHeadings(legacy) {
		t.Fatal("fixture did not produce the expected heading forms")
	}
	for _, format := range []string{"html", "pptx"} {
		outputs := map[string]string{}
		for name, doc := range map[string]*ast.AST{"legacy": legacy, "typed": typed} {
			dir := t.TempDir()
			opts := GeneratorOptions{AssetRoot: dir, EmbedAssets: true}
			if err := New(util.NewNoop()).GenerateWithOptions(doc, format, dir, opts, renderer.NewDefaultRenderContext()); err != nil {
				t.Fatalf("%s %s: %v", format, name, err)
			}
			path := filepath.Join(dir, "deck."+format)
			if format == "pptx" {
				outputs[name] = pptxSlideXML(t, path)
				continue
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			// El nonce CSP es aleatorio por render; no es parte del oráculo.
			outputs[name] = cspNonce.ReplaceAllString(string(data), "nonce-X")
		}
		if l, ty := outputs["legacy"], outputs["typed"]; l != ty {
			i := 0
			for i < len(l) && i < len(ty) && l[i] == ty[i] {
				i++
			}
			lo, hi := max(0, i-200), i+200
			t.Fatalf("%s differs between legacy and typed headings at byte %d\nlegacy: %q\n typed: %q", format, i, l[lo:min(hi, len(l))], ty[lo:min(hi, len(ty))])
		}
		if !strings.Contains(outputs["typed"], "Results") {
			t.Fatalf("%s lost the heading text", format)
		}
	}
	data, err := New(util.NewNoop()).RenderASTJSON(typed)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"type":"heading"`) || !strings.Contains(string(data), `"text":"Results **now**"`) {
		t.Fatalf("JSON lost the typed heading: %s", data)
	}
}
