// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"encoding/json"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
)

func parseIdentityFixture(t *testing.T, source string) *ast.AST {
	t.Helper()
	p := parser.New(util.NewNoop())
	doc, issues := p.Parse(source, "fixture.slidelang")
	for _, issue := range issues {
		if issue.IsError() {
			t.Fatalf("parse: %s", issue.String())
		}
	}
	if doc == nil {
		t.Fatal("nil AST")
	}
	return doc
}

func ids(doc *ast.AST) map[string]ast.NodeType {
	out := map[string]ast.NodeType{}
	_ = ast.Walk(doc, func(node ast.Node) error {
		if n, ok := node.(ast.IdentityNode); ok && n.GetNodeID() != "" {
			out[n.GetNodeID()] = node.GetType()
		}
		return nil
	})
	return out
}

func TestNodeIDsStrictRoundTripsAndEdits(t *testing.T) {
	source := "---\nmode: strict\n---\n\n<!-- node-id: SlideA -->\nSLIDE content\n  title: \"A\"\n  <!-- node-id: First -->\n  TEXT\n    Same\n  <!-- node-id: Second -->\n  TEXT\n    Same\n"
	doc := parseIdentityFixture(t, source)
	if len(ids(doc)) != 3 {
		t.Fatalf("IDs: %v", ids(doc))
	}
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := ast.DecodeAST(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids(restored)) != 3 {
		t.Fatalf("JSON lost IDs: %v", ids(restored))
	}
	first, err := FormatStrict(restored)
	if err != nil {
		t.Fatal(err)
	}
	secondDoc := parseIdentityFixture(t, first)
	second, err := FormatStrict(secondDoc)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("non-idempotent format:\n%s\n---\n%s", first, second)
	}
	if len(ids(secondDoc)) != 3 {
		t.Fatalf("roundtrip lost IDs: %v", ids(secondDoc))
	}
	// Explicit identities follow the authored nodes after insertions/reordering.
	secondDoc.ContentBlocks[0].Elements[0], secondDoc.ContentBlocks[0].Elements[1] = secondDoc.ContentBlocks[0].Elements[1], secondDoc.ContentBlocks[0].Elements[0]
	secondDoc.ContentBlocks[0].Elements = append(secondDoc.ContentBlocks[0].Elements, ast.NewTextElement(diagnostics.NewPosition(50, 1), "New"))
	reordered, err := FormatStrict(secondDoc)
	if err != nil {
		t.Fatal(err)
	}
	parsed := parseIdentityFixture(t, reordered)
	if parsed.ContentBlocks[0].Elements[0].(*ast.TextElement).NodeID != "Second" || parsed.ContentBlocks[0].Elements[1].(*ast.TextElement).NodeID != "First" {
		t.Fatalf("reorder changed identities: %v", ids(parsed))
	}
}

func TestNodeIDDiagnosticsAndLiteralCode(t *testing.T) {
	for _, tc := range []struct{ name, source, want string }{
		{"duplicate", "<!-- node-id: X -->\nSLIDE content\n  <!-- node-id: X -->\n  TEXT\n    A\n", "duplicate nodeId"},
		{"orphan", "SLIDE content\n  <!-- node-id: Missing -->\n", "orphan node-id"},
		{"ambiguous", "<!-- node-id: A -->\n<!-- node-id: B -->\nSLIDE content\n", "ambiguous node-id"},
		{"invalid", "<!-- node-id: bad id -->\nSLIDE content\n", "invalid node-id"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, issues := parser.New(util.NewNoop()).Parse("---\nmode: strict\n---\n"+tc.source, "")
			found := false
			for _, issue := range issues {
				if strings.Contains(issue.Message, tc.want) {
					found = true
					if issue.Position.Line < 5 {
						t.Fatalf("wrong line: %v", issue)
					}
				}
			}
			if !found {
				t.Fatalf("missing %q in %v", tc.want, issues)
			}
		})
	}
	source := "---\nmode: flex\n---\n\n<!-- node-id: Deck -->\n# Title\n\n```text\n<!-- node-id: Literal -->\n```\n"
	doc := parseIdentityFixture(t, source)
	if _, found := ids(doc)["Literal"]; found {
		t.Fatal("literal comment captured as identity")
	}
	if _, found := ids(doc)["Deck"]; !found {
		t.Fatal("flex slide lost identity")
	}
}
