// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package ast

import (
	"encoding/json"
	"go.ziradocs.com/core/v2/diagnostics"
	"testing"
)

func TestContentMarkdownContractSchemaAndDecode(t *testing.T) {
	schema := compileASTSchema(t)
	for level := 1; level <= 6; level++ {
		doc := NewAST(diagnostics.NewPosition(1, 1))
		b := NewContentBlock(diagnostics.NewPosition(1, 1), "content")
		b.Elements = []Element{NewHeadingElement(diagnostics.NewPosition(1, 1), level, "Authored", "author-anchor")}
		doc.ContentBlocks = []ContentBlock{*b}
		doc.FrontMatter = NewFrontMatterNode(diagnostics.NewPosition(1, 1))
		doc.FrontMatter.ASTCapabilities = []string{TypedHeadingsCapability, ContentMarkdownCapability}
		SetTableContract(doc)
		if doc.SchemaVersion != ContentMarkdownSchemaVersion {
			t.Fatal(doc.SchemaVersion)
		}
		bytes, _ := json.Marshal(doc)
		if _, err := DecodeAST(bytes); err != nil {
			t.Fatal(err)
		}
		var value any
		_ = json.Unmarshal(bytes, &value)
		if err := schema.Validate(value); err != nil {
			t.Fatal(err)
		}
		root := value.(map[string]any)
		root["capabilities"] = []any{ContentMarkdownCapability}
		bad, _ := json.Marshal(root)
		if _, err := DecodeAST(bad); err == nil {
			t.Fatal("missing typed heading dependency accepted")
		}
		if err := schema.Validate(root); err == nil {
			t.Fatal("schema accepted missing typed heading dependency")
		}
	}
	empty := NewAST(diagnostics.NewPosition(1, 1))
	empty.FrontMatter = NewFrontMatterNode(diagnostics.NewPosition(1, 1))
	empty.FrontMatter.ASTCapabilities = []string{TypedHeadingsCapability, ContentMarkdownCapability}
	SetTableContract(empty)
	if empty.SchemaVersion != LegacySchemaVersion {
		t.Fatal("unused opt-in inflated schema")
	}
}
