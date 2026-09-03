// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/util"
)

// writeExternalThemeWithExtensionTokens writes <dir>/<name>/theme.json
// declaring a diagram-* and a chart-cat-* token, mirroring
// writeMinimalExternalTheme's shape (css/builder_namespace_test.go) but for
// the §2.2 extension tokens instead of §2.1's namespacing.
func writeExternalThemeWithExtensionTokens(t *testing.T, root, name string) {
	t.Helper()
	themeDir := filepath.Join(root, name)
	if err := os.MkdirAll(themeDir, 0755); err != nil {
		t.Fatal(err)
	}
	manifest := `{
  "name": "` + name + `",
  "version": "1.0.0",
  "description": "test theme",
  "author": "test",
  "variables": {
    "--slidelang-background-color": "#ffffff",
    "--slidelang-text-color": "#111111",
    "--slidelang-diagram-node-bg": "#1e293b",
    "--slidelang-chart-cat-1": "#111111",
    "--slidelang-chart-cat-2": "#222222"
  }
}`
	if err := os.WriteFile(filepath.Join(themeDir, "theme.json"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
}

func extractMetadataJSON(t *testing.T, html string) map[string]interface{} {
	t.Helper()
	start := strings.Index(html, `id="slidelang-metadata">`)
	if start == -1 {
		t.Fatal("slidelang-metadata script tag not found")
	}
	start += len(`id="slidelang-metadata">`)
	end := strings.Index(html[start:], "</script>")
	if end == -1 {
		t.Fatal("closing </script> for slidelang-metadata not found")
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(html[start:start+end]), &parsed); err != nil {
		t.Fatalf("slidelang-metadata is not valid JSON: %v\ncontent:\n%s", err, html[start:start+end])
	}
	return parsed
}

// TestRenderHTML_ThemeTokensReachMetadata is the end-to-end regression for
// §2.2's browser path: a theme declaring diagram-*/chart-cat-* tokens must
// have them appear, already resolved to literals, in the
// slidelang-metadata JSON block that mermaid.js/charts.js read.
func TestRenderHTML_ThemeTokensReachMetadata(t *testing.T) {
	dir := t.TempDir()
	writeExternalThemeWithExtensionTokens(t, dir, "tokens-test-theme")
	t.Setenv("SLIDELANG_THEMES_PATH", dir)

	p := pos()
	block := ast.NewContentBlock(p, "content")
	textElem := ast.NewTextElement(p, "hello")
	block.Elements = append(block.Elements, textElem)
	doc := ast.NewAST(p)
	doc.FrontMatter = ast.NewFrontMatterNode(p)
	doc.ContentBlocks = append(doc.ContentBlocks, *block)

	g := New(util.NewNoop())
	html, err := g.RenderHTMLPreview(doc, GeneratorOptions{Theme: "tokens-test-theme"}, renderer.NewDefaultRenderContext())
	if err != nil {
		t.Fatalf("RenderHTMLPreview: %v", err)
	}

	metadata := extractMetadataJSON(t, html)
	themeTokens, ok := metadata["themeTokens"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected themeTokens to be an object, got %#v", metadata["themeTokens"])
	}
	diagram, ok := themeTokens["diagram"].(map[string]interface{})
	if !ok || diagram["diagram-node-bg"] != "#1e293b" {
		t.Errorf("expected diagram.diagram-node-bg = #1e293b, got %#v", themeTokens["diagram"])
	}
	chartCat, ok := themeTokens["chartCategorical"].([]interface{})
	if !ok || len(chartCat) != 2 || chartCat[0] != "#111111" || chartCat[1] != "#222222" {
		t.Errorf("expected chartCategorical = [#111111, #222222], got %#v", themeTokens["chartCategorical"])
	}
}

// TestRenderHTML_NoExtensionTokens_MetadataUnaffected is the byte-for-byte
// non-regression: a theme declaring NO §2.2 tokens (every theme in the
// repo today) must produce an empty themeTokens object and an empty
// themeFontMain-or-resolved-stack, never a synthesized value.
func TestRenderHTML_NoExtensionTokens_MetadataUnaffected(t *testing.T) {
	p := pos()
	block := ast.NewContentBlock(p, "content")
	textElem := ast.NewTextElement(p, "hello")
	block.Elements = append(block.Elements, textElem)
	doc := ast.NewAST(p)
	doc.FrontMatter = ast.NewFrontMatterNode(p)
	doc.ContentBlocks = append(doc.ContentBlocks, *block)

	g := New(util.NewNoop())
	html, err := g.RenderHTMLPreview(doc, GeneratorOptions{}, renderer.NewDefaultRenderContext())
	if err != nil {
		t.Fatalf("RenderHTMLPreview: %v", err)
	}

	metadata := extractMetadataJSON(t, html)
	themeTokens, ok := metadata["themeTokens"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected themeTokens to be an object, got %#v", metadata["themeTokens"])
	}
	for _, key := range []string{"diagram", "chart", "chartCategorical", "chartSequential", "map"} {
		if _, present := themeTokens[key]; present {
			t.Errorf("expected no %q key when the theme declares no §2.2 tokens, got %#v", key, themeTokens[key])
		}
	}
}
