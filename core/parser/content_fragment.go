// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/internal/elements"
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
			nodes = append(nodes, ast.NewSpecialBlockElement(ctx.Position(start), "warning", body[0]))
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
	var references []string
	bytes, _ := json.Marshal(nodes)
	var tree any
	_ = json.Unmarshal(bytes, &tree)
	var inspect func(any)
	inspect = func(value any) {
		switch v := value.(type) {
		case []any:
			for _, x := range v {
				inspect(x)
			}
		case map[string]any:
			if v["type"] == "code" {
				return
			}
			for key, x := range v {
				if key != "codeBlocks" {
					inspect(x)
				}
			}
		case string:
			for _, match := range localContentLink.FindAllStringSubmatch(v, -1) {
				references = append(references, match[1])
			}
			for _, match := range localContentReference.FindAllStringSubmatch(v, -1) {
				references = append(references, match[1])
			}
		}
	}
	inspect(tree)
	for _, target := range references {
		if !anchors[target] {
			return nil, append(diags, diagnostics.NewError("unresolved local fragment anchor "+target+"; declare its authoral anchor explicitly", diagnostics.NewPosition(1, 1), "content-markdown"))
		}
	}
	return nodes, diags
}

var localContentLink = regexp.MustCompile(`\]\(\s*<?#([^)>\s]+)>?\s*\)`)
var localContentReference = regexp.MustCompile(`\[[^\]]+\]:[ \t]*<?#([^>\s]+)>?`)
