// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"regexp"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/util"
	"go.ziradocs.com/slidelang/v2/internal/generator/css/themes"
)

// classAttrRe finds every class="..." attribute in a rendered HTML string.
var classAttrRe = regexp.MustCompile(`class="([^"]*)"`)

// TestRenderHTMLPreview_EveryElementType_BareClassesAreKnown is the
// code-review finding (second and third rounds, PR #223) that manually
// reading the Go template literals in template/base.go is NOT a reliable
// way to know which classes the real, generated HTML carries bare
// (without the slidelang- prefix): namespaceTemplateClasses() only skips
// auto-prefixing for a class="..." attribute that contains "{{" — a
// static class="tabs" gets auto-prefixed to "slidelang-tabs" even though
// its literal source text has no prefix, while some OTHER attributes
// (class="element map {{range .CSSClasses}}...") were left un-prefixed by
// the template author BECAUSE they contain "{{", with no manual prefix to
// compensate — an inconsistency invisible from the literal alone. This
// test builds one AST document covering every element type template/
// base.go knows how to render, renders it through the real pipeline
// (RenderHTMLPreview — the same code path slidelang build uses), and
// asserts that every class the output actually carries WITHOUT the
// slidelang- prefix is a deliberate, documented entry in
// themes.KnownUnprefixedClasses — not an oversight. A class this test
// finds that ISN'T in that allowlist means either the allowlist is
// stale (a theme has no way to target the real markup) or a template
// forgot to prefix a class that every sibling element type does carry
// prefixed (a real engine bug, not a theme problem) — this test cannot
// tell those two apart, a human has to look, but it stops either from
// going unnoticed.
//
// JS-toggled state classes (classList.add/remove/toggle in template/
// utilities.go and template/directives.go: "expanded", "timer-expired",
// "full-screen") never appear in server-rendered HTML at all — this test
// cannot see them — so they stay in KnownUnprefixedClasses backed by a
// direct grep of that JS source instead of this test.
func TestRenderHTMLPreview_EveryElementType_BareClassesAreKnown(t *testing.T) {
	p := pos()
	block := ast.NewContentBlock(p, "content")
	block.Title = "Every element type"

	textElem := ast.NewTextElement(p, "Some text")
	pointsElem := ast.NewPointsElement(p)
	codeElem := ast.NewCodeElement(p, "go", "package main")
	imageElem := ast.NewImageElement(p, "photo.png", "a photo")
	tableElem := ast.NewTableElement(p)
	specialBlockElem := ast.NewSpecialBlockElement(p, "info", "A note")

	codeGroupElem := ast.NewCodeGroupElement(p)
	codeGroupElem.CodeBlocks = []ast.CodeBlock{
		{Language: "go", Label: "main.go", Content: "package main"},
		{Language: "python", Label: "main.py", Content: "print('hi')"},
	}

	mermaidElem := ast.NewMermaidElement(p, "flowchart", "graph TD; A-->B;")
	plantumlElem := ast.NewPlantUMLElement(p, "sequence", "Alice -> Bob: hi")
	mathElem := ast.NewMathElement(p, "x^2")

	chartElem := ast.NewChartElement(p, "bar")
	chartElem.Data = [][]interface{}{{"Q1", 10.0}}
	chartElem.Series = []string{"A"}

	mediaElem := ast.NewMediaElement(p, "audio", "sound.mp3")

	mapElem := ast.NewMapElement(p, "static")

	quoteElem := ast.NewQuoteElement(p, "A quote worth quoting")
	quoteElem.Author = "Someone"
	quoteElem.Source = "Somewhere"

	checklistElem := ast.NewChecklistElement(p)
	checklistElem.Items = []ast.ChecklistItem{
		{BaseNode: ast.NewBaseNode(ast.NodeTypeText, p), Content: "Done item", Checked: true},
		{BaseNode: ast.NewBaseNode(ast.NodeTypeText, p), Content: "Pending item", Checked: false},
	}

	gridElem := ast.NewGridElement(p)
	gridElem.Columns = append(gridElem.Columns, *ast.NewColumnElement(p, "Column content"))

	block.Elements = append(block.Elements,
		textElem, pointsElem, codeElem, imageElem, tableElem, specialBlockElem,
		codeGroupElem, mermaidElem, plantumlElem, mathElem, chartElem, mediaElem,
		mapElem, quoteElem, checklistElem, gridElem,
	)

	doc := ast.NewAST(p)
	doc.FrontMatter = ast.NewFrontMatterNode(p)
	doc.ContentBlocks = append(doc.ContentBlocks, *block)

	g := New(util.NewNoop())
	html, err := g.RenderHTMLPreview(doc, GeneratorOptions{}, renderer.NewDefaultRenderContext())
	if err != nil {
		t.Fatalf("RenderHTMLPreview: %v", err)
	}

	seen := map[string]bool{}
	var unknown []string
	for _, m := range classAttrRe.FindAllStringSubmatch(html, -1) {
		for _, class := range strings.Fields(m[1]) {
			if strings.Contains(class, "{{") || strings.Contains(class, "}}") {
				// A template expression that didn't fully resolve is not
				// a real class name to check (shouldn't happen in
				// RenderHTMLPreview's output, but not this test's job to
				// diagnose if it does).
				continue
			}
			if strings.HasPrefix(class, "slidelang-") || seen[class] {
				continue
			}
			seen[class] = true
			if themes.KnownUnprefixedClasses[class] {
				continue
			}
			hasKnownPrefix := false
			for _, p := range themes.KnownUnprefixedClassPrefixes {
				if strings.HasPrefix(class, p) {
					hasKnownPrefix = true
					break
				}
			}
			if !hasKnownPrefix {
				unknown = append(unknown, class)
			}
		}
	}

	if len(unknown) > 0 {
		t.Errorf("found %d class(es) without the slidelang- prefix that are NOT in themes.KnownUnprefixedClasses: %v\n\nEither add them there (if the engine deliberately emits them bare) or fix the template to prefix them (if this is an oversight — compare against a sibling element type that gets it right).", len(unknown), unknown)
	}
}
