// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/util"
)

func semanticJSON(t *testing.T, doc *ast.AST) any {
	t.Helper()
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatal(err)
	}
	var scrub func(any)
	scrub = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			delete(x, "position")
			delete(x, "endPosition")
			delete(x, "rowPositions")
			delete(x, "nodeId")
			for _, child := range x {
				scrub(child)
			}
		case []any:
			for _, child := range x {
				scrub(child)
			}
		}
	}
	scrub(value)
	return value
}

func assertNoIdentityErrors(t *testing.T, issues []diagnostics.Diagnostic) {
	t.Helper()
	for _, issue := range issues {
		if issue.Source == "identity" && issue.IsError() {
			t.Fatalf("identity error: %v", issue)
		}
	}
}

func TestNodeIDAnnotationsDoNotChangeParseSemantics(t *testing.T) {
	for _, tc := range []struct{ name, mode, body, annotated string }{
		{"strict", "strict", "SLIDE content\n  title: \"A\"\n  TEXT\n    Value\n", "<!-- node-id: SlideA -->\nSLIDE content\n  title: \"A\"\n  <!-- node-id: ValueA -->\n  TEXT\n    Value\n"},
		{"flex", "flex", "# A\n\nValue\n", "<!-- node-id: SlideA -->\n# A\n\nValue\n"},
		{"full", "flex-full", "# A\n\nValue\n", "<!-- node-id: SlideA -->\n# A\n\nValue\n"},
		{"auto-flex", "auto", "# A\n\nValue\n", "<!-- node-id: SlideA -->\n# A\n\nValue\n"},
		{"auto-strict", "auto", "SLIDE content\n  title: \"A\"\n  TEXT\n    Value\n", "<!-- node-id: SlideA -->\nSLIDE content\n  title: \"A\"\n  TEXT\n    Value\n"},
	} {
		for _, normalize := range []bool{false, true} {
			t.Run(tc.name+map[bool]string{true: "/normalize", false: "/raw"}[normalize], func(t *testing.T) {
				prefix := "---\nmode: " + tc.mode + "\n---\n\n"
				p1, p2 := New(util.NewNoop()), New(util.NewNoop())
				p1.SetNormalization(normalize)
				p2.SetNormalization(normalize)
				plain, _ := p1.Parse(prefix+tc.body, "")
				marked, issues := p2.Parse(prefix+tc.annotated, "")
				if tc.name == "auto-flex" && normalize {
					found := false
					for _, issue := range issues {
						if issue.Source == "identity" && strings.Contains(issue.Message, "normalization") {
							found = true
							if issue.Position.Line != 5 {
								t.Fatalf("normalization diagnostic line = %d", issue.Position.Line)
							}
						}
					}
					if !found {
						t.Fatalf("expected explicit normalization ambiguity, got %v", issues)
					}
					return
				}
				assertNoIdentityErrors(t, issues)
				if plain == nil || marked == nil {
					t.Fatal("nil AST")
				}
				if !reflect.DeepEqual(semanticJSON(t, plain), semanticJSON(t, marked)) {
					t.Fatalf("annotation changed semantics: %s", tc.name)
				}
				if len(marked.ContentBlocks) > 0 && marked.ContentBlocks[0].NodeID != "SlideA" {
					t.Fatalf("missing slide identity: %v", marked.ContentBlocks[0].NodeID)
				}
			})
		}
	}
}

func TestNodeIDInFrontmatterScalarRemainsLiteral(t *testing.T) {
	for _, document := range []bool{false, true} {
		source := "---\nmode: flex\ndescription: |\n  <!-- node-id: Example -->\n---\n\n# Title\n\nText\n"
		p := New(util.NewNoop())
		var doc *ast.AST
		var issues []diagnostics.Diagnostic
		if document {
			doc, issues = p.ParseDocument(source, "")
		} else {
			doc, issues = p.Parse(source, "")
		}
		assertNoIdentityErrors(t, issues)
		if doc == nil || doc.FrontMatter == nil || !strings.Contains(doc.FrontMatter.Raw, "<!-- node-id: Example -->") {
			t.Fatalf("frontmatter scalar changed: %+v", doc)
		}
		if len(doc.ContentBlocks) == 0 || doc.ContentBlocks[0].NodeID != "" {
			t.Fatal("frontmatter scalar became node ID")
		}
	}
}
