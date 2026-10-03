//go:build !js

// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/util"
)

// slideListHTML parses a flex deck and renders its slides the way the CLI does,
// without nested-list-types-v1, and returns the markup of the lists.
func slideListHTML(t *testing.T, src string) string {
	t.Helper()
	doc, diags := parser.New(util.NewNoop()).Parse(src, "t.slidelang")
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("does not parse: %s", d.String())
		}
	}
	html, err := New(util.NewNoop()).RenderHTMLPreview(doc, GeneratorOptions{}, renderer.NewDefaultRenderContext())
	if err != nil {
		t.Fatal(err)
	}
	return html
}

const flexListHead = "---\nmode: flex\n---\n\n## Lists\n\n"

// Without the capability the AST does not say which marker a sublist used, but
// the parser keeps it in memory so the render can draw what the author wrote: a
// bullet sublist under a numbered item is bullets, not letters.
func TestSlideSublistFollowsTheAuthorsMarker(t *testing.T) {
	for name, tc := range map[string]struct {
		src  string
		want string // markup between the parent and the next top-level item
	}{
		"bullets under a number": {flexListHead + "1. uno\n   - a\n   - b\n2. dos\n", "<ul>"},
		"numbers under a number": {flexListHead + "1. uno\n   1. a\n   2. b\n2. dos\n", "<ol>"},
		"numbers under a bullet": {flexListHead + "- uno\n  1. a\n  2. b\n- dos\n", "<ol>"},
		"bullets under a bullet": {flexListHead + "- uno\n  - a\n  - b\n- dos\n", "<ul>"},
		"letters under a number": {flexListHead + "1. uno\n   a. x\n   b. y\n2. dos\n", "<ol>"},
	} {
		t.Run(name, func(t *testing.T) {
			html := slideListHTML(t, tc.src)
			start := strings.Index(html, "uno")
			end := strings.Index(html, "dos")
			if start < 0 || end <= start {
				t.Fatalf("list not found: %s", html)
			}
			between := html[start:end]
			if !strings.Contains(between, tc.want) {
				t.Errorf("sublist markup between uno and dos has no %s: %s", tc.want, between)
			}
		})
	}
}

// An AST that did not come from the parser (JSON, or rewritten by a filter) has no
// marker, and the render keeps the type of the parent as before.
func TestSlideSublistWithoutMarkerKeepsTheParentType(t *testing.T) {
	points := ast.NewPointsElement(pos())
	points.ListType = "ordered"
	parent := ast.NewPointItem(pos(), "uno")
	parent.SubPoints = append(parent.SubPoints, *ast.NewPointItem(pos(), "a"))
	points.Items = append(points.Items, *parent, *ast.NewPointItem(pos(), "dos"))
	doc := ast.NewAST(pos())
	doc.FrontMatter = ast.NewFrontMatterNode(pos())
	block := ast.NewContentBlock(pos(), "content")
	block.Title = "List"
	block.Elements = append(block.Elements, points)
	doc.ContentBlocks = append(doc.ContentBlocks, *block)
	html, err := New(util.NewNoop()).RenderHTMLPreview(doc, GeneratorOptions{}, renderer.NewDefaultRenderContext())
	if err != nil {
		t.Fatal(err)
	}
	start, end := strings.Index(html, "uno"), strings.Index(html, "dos")
	if !strings.Contains(html[start:end], "<ol>") {
		t.Errorf("a sublist with no recorded marker should follow its ordered parent: %s", html[start:end])
	}
}
