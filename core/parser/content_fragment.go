// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"golang.org/x/net/html"
	"io"
	"regexp"
	"strings"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/internal/elements"
	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/util"
)

// ContentOptions selects an explicit Markdown body context, not a document.
// Namespace is stable authoral identity (for example document/slot/block), not
// a global line offset. It applies only to automatically generated anchors.
type ContentOptions struct{ AnchorNamespace string }

var explicitContentAnchor = regexp.MustCompile(`\s+\{#([a-z0-9_-]+)\}$`)

// ParseMarkdownContent parses original Markdown as ordered content elements.
// It never runs document normalization, interprets frontmatter, opens a slide,
// or promotes raw HTML to a typed heading. The existing element registry owns
// CODE, tables, lists, quotes and embedded elements.
func ParseMarkdownContent(markdown string, options ContentOptions) ([]ast.Element, []diagnostics.Diagnostic) {
	if options.AnchorNamespace == "" || len(options.AnchorNamespace) > 512 || strings.ContainsAny(options.AnchorNamespace, "\r\n\x00") {
		return nil, []diagnostics.Diagnostic{diagnostics.NewError("a nonempty stable fragment namespace (at most 512 bytes) is required", diagnostics.NewPosition(1, 1), "content-markdown")}
	}
	if strings.Contains(markdown, "\r") {
		return nil, []diagnostics.Diagnostic{diagnostics.NewError("CR line endings are not losslessly representable in this fragment profile", diagnostics.NewPosition(1, 1), "content-markdown")}
	}
	lines := strings.Split(markdown, "\n")
	registry := elements.GetDefaultRegistry()
	ctx := &elements.ParseContext{Mode: "flex", ContentMarkdown: true, NestedListTypes: true, ListStart: true, Lines: lines, Logger: util.NewNoop()}
	var nodes []ast.Element
	var diags []diagnostics.Diagnostic
	anchors := map[string]bool{}
	namespace := fmt.Sprintf("%x", sha256.Sum256([]byte(options.AnchorNamespace)))[:20]
	headingIndex := 0
	for i := 0; i < len(lines); {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			i++
			continue
		}
		n := 0
		for n < len(line) && line[n] == '#' {
			n++
		}
		if n >= 1 && n <= 6 && n < len(line) && (line[n] == ' ' || line[n] == '\t') {
			if len(lines[i])-len(strings.TrimLeft(lines[i], " ")) >= 4 || strings.HasPrefix(lines[i], "\t") {
				return nil, append(diags, diagnostics.NewError("indented ATX may be literal code; use an explicit CODE fence", ctx.Position(i), "content-markdown"))
			}
			text := strings.TrimSpace(line[n:])
			anchor := ""
			if match := explicitContentAnchor.FindStringSubmatchIndex(text); match != nil {
				anchor = text[match[2]:match[3]]
				text = strings.TrimSpace(text[:match[0]])
			} else if strings.Contains(text, "{#") {
				diags = append(diags, diagnostics.NewError("invalid explicit heading anchor; use [a-z0-9_-]", ctx.Position(i), "content-markdown"))
				return nil, diags
			}
			headingIndex++
			if anchor == "" {
				anchor = fmt.Sprintf("heading-fragment-%s-%d", namespace, headingIndex)
			}
			if anchors[anchor] {
				diags = append(diags, diagnostics.NewError("duplicate explicit heading anchor "+anchor, ctx.Position(i), "content-markdown"))
				return nil, diags
			}
			anchors[anchor] = true
			heading := ast.NewHeadingElement(ctx.Position(i), n, text, anchor)
			if err := ast.ValidateHeading(heading); err != nil {
				diags = append(diags, diagnostics.NewError(err.Error(), ctx.Position(i), "content-markdown"))
				return nil, diags
			}
			nodes = append(nodes, heading)
			i++
			continue
		}
		// Document metadata and slide/section markers have no fragment authority.
		if line == "---" || strings.HasPrefix(line, "SLIDE ") || strings.HasPrefix(line, "SECTION ") || strings.HasPrefix(line, "<!-- node-id:") {
			diags = append(diags, diagnostics.NewError("document/identity syntax is not Markdown fragment content", ctx.Position(i), "content-markdown"))
			return nil, diags
		}
		// Only the explicit standard WARNING marker is normalized. Its body
		// remains literal and CODE fences are consumed before their inner lines.
		if line == "> [!WARNING]" {
			start := i
			i++
			var body []string
			for i < len(lines) && strings.HasPrefix(lines[i], "> ") {
				body = append(body, strings.TrimPrefix(lines[i], "> "))
				i++
			}
			if len(body) != 1 || strings.TrimSpace(body[0]) == "" {
				diags = append(diags, diagnostics.NewError("WARNING fragment profile requires one nonempty literal quoted body line", ctx.Position(start), "content-markdown"))
				return nil, diags
			}
			warning := ast.NewSpecialBlockElement(ctx.Position(start), "warning", body[0])
			warning.Icon = elements.KnownSpecialBlockTypes()["warning"]
			nodes = append(nodes, warning)
			continue
		}
		result := registry.Parse(ctx, i)
		diags = append(diags, result.Diagnostics...)
		if result.Error != nil {
			diags = append(diags, diagnostics.NewError(result.Error.Error(), ctx.Position(i), "content-markdown"))
		}
		if result.Error != nil || result.ConsumedLines <= 0 || result.Element == nil {
			diags = append(diags, diagnostics.NewError("fragment syntax did not produce a content element", ctx.Position(i), "content-markdown"))
			return nil, diags
		}
		if code, ok := result.Element.(*ast.CodeElement); ok {
			// The native flex parser owns the payload; an unclosed fence has no
			// authority to consume the rest of an arbitrary fragment silently.
			end := i + result.ConsumedLines - 1
			if end >= len(lines) || strings.TrimSpace(lines[end]) != "```" {
				diags = append(diags, diagnostics.NewError("unclosed CODE fence", ctx.Position(i), "content-markdown"))
				return nil, diags
			}
			literal := strings.Join(lines[i+1:end], "\n")
			if literal != code.Content {
				diags = append(diags, diagnostics.NewError("CODE bytes changed during fragment parsing", ctx.Position(i), "content-markdown"))
				return nil, diags
			}
		}
		nodes = append(nodes, result.Element)
		i += result.ConsumedLines
	}
	for _, d := range diags {
		if d.IsError() {
			return nil, diags
		}
	}
	// A generated anchor is deliberately namespaced. Local links may retain
	// only targets whose explicit/generated authority is actually resolvable;
	// never silently keep a broken pre-namespace auto-anchor destination.
	references, err := contentReferences(nodes)
	if err != nil {
		return nil, append(diags, diagnostics.NewError(err.Error(), diagnostics.NewPosition(1, 1), "content-markdown"))
	}
	for _, target := range references {
		if !anchors[target] {
			return nil, append(diags, diagnostics.NewError("unresolved local fragment anchor "+target+"; declare its authoral anchor explicitly", diagnostics.NewPosition(1, 1), "content-markdown"))
		}
	}
	return nodes, diags
}

// Navigation authority comes only from the existing renderer's actual HTML
// fields. Literal chart/MATH/metric values are never parsed as Markdown here.
func contentReferences(nodes []ast.Element) ([]string, error) {
	doc := ast.NewAST(diagnostics.NewPosition(1, 1))
	block := ast.NewContentBlock(diagnostics.NewPosition(1, 1), "content")
	block.Elements = nodes
	doc.ContentBlocks = []ast.ContentBlock{*block}
	doc.FrontMatter = &ast.FrontMatterNode{Mode: "strict", ASTCapabilities: []string{ast.TypedHeadingsCapability, ast.ContentMarkdownCapability}}
	ast.SetTableContract(doc)
	encoded, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	copy, err := ast.DecodeAST(encoded)
	if err != nil {
		return nil, err
	}
	renderer.PopulateInlineHTML(copy, nil)
	encoded, err = json.Marshal(copy.ContentBlocks[0].Elements)
	if err != nil {
		return nil, err
	}
	var tree any
	_ = json.Unmarshal(encoded, &tree)
	var references []string
	tokenize := func(value string) {
		tokenizer := html.NewTokenizer(strings.NewReader(value))
		codeDepth := 0
		for {
			kind := tokenizer.Next()
			if kind == html.ErrorToken {
				if tokenizer.Err() != io.EOF {
					references = append(references, "invalid-link-tokenization")
				}
				break
			}
			token := tokenizer.Token()
			if kind == html.StartTagToken && (token.Data == "code" || token.Data == "pre") {
				codeDepth++
			}
			if kind == html.EndTagToken && (token.Data == "code" || token.Data == "pre") && codeDepth > 0 {
				codeDepth--
			}
			if codeDepth == 0 && (kind == html.StartTagToken || kind == html.SelfClosingTagToken) && token.Data == "a" {
				for _, attr := range token.Attr {
					if attr.Key == "href" && strings.HasPrefix(attr.Val, "#") {
						references = append(references, strings.TrimPrefix(attr.Val, "#"))
					}
				}
			}
		}
	}
	var inspectHTML func(any)
	inspectHTML = func(value any) {
		switch v := value.(type) {
		case string:
			tokenize(v)
		case []any:
			for _, x := range v {
				inspectHTML(x)
			}
		}
	}
	var inspect func(any)
	inspect = func(value any) {
		switch v := value.(type) {
		case []any:
			for _, x := range v {
				inspect(x)
			}
		case map[string]any:
			// Fixed field order and array order make diagnostics reproducible.
			for _, key := range []string{"textHTML", "titleHTML", "contentHTML", "headersHTML", "rowsHTML", "captionHTML", "authorHTML", "sourceHTML", "labelHTML", "valueHTML", "questionHTML", "optionsHTML", "explanationHTML"} {
				if x, ok := v[key]; ok {
					inspectHTML(x)
				}
			}
			for _, key := range []string{"elements", "items", "subPoints", "subItems", "columns", "codeBlocks"} {
				if x, ok := v[key]; ok {
					inspect(x)
				}
			}
		}
	}
	inspect(tree)
	return references, nil
}
