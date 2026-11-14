// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package ast

import (
	"encoding/json"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/diagnostics"
)

func TestDecodeASTValidatesNestedNodeIDs(t *testing.T) {
	pos := diagnostics.NewPosition(2, 1)
	doc := NewAST(pos)
	block := NewContentBlock(pos, "content")
	block.NodeID = "Same"
	callout := NewSpecialBlockElement(pos, "info", "")
	nested := NewTextElement(pos, "text")
	nested.NodeID = "Same"
	callout.Elements = append(callout.Elements, nested)
	block.Elements = append(block.Elements, callout)
	doc.ContentBlocks = append(doc.ContentBlocks, *block)
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeAST(data); err == nil || !strings.Contains(err.Error(), "duplicate nodeId") {
		t.Fatalf("duplicate nested ID accepted: %v", err)
	}
	nested.NodeID = "bad id"
	if data, err = json.Marshal(doc); err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeAST(data); err == nil || !strings.Contains(err.Error(), "invalid nodeId") {
		t.Fatalf("invalid nested ID accepted: %v", err)
	}
	nested.NodeID = "Nested"
	if data, err = json.Marshal(doc); err != nil {
		t.Fatal(err)
	}
	got, err := DecodeAST(data)
	if err != nil {
		t.Fatal(err)
	}
	if got.ContentBlocks[0].Elements[0].(*SpecialBlockElement).Elements[0].(*TextElement).NodeID != "Nested" {
		t.Fatal("nested ID lost in JSON")
	}
}

// El aviso de un nodeId repetido no escribe dónde está el primero: ese
// número se calcula antes de llevar las posiciones a su archivo (@include) y
// saldría distinto del real. El ID entre comillas basta para encontrarlo.
func TestValidateNodeIDsDuplicateMessageHasNoLineNumber(t *testing.T) {
	doc := NewAST(diagnostics.NewPosition(1, 1))
	block := NewContentBlock(diagnostics.NewPosition(2, 1), "content")
	first := NewTextElement(diagnostics.NewPosition(3, 1), "a")
	first.NodeID = "Same"
	second := NewTextElement(diagnostics.NewPosition(9, 1), "b")
	second.NodeID = "Same"
	block.Elements = append(block.Elements, first, second)
	doc.ContentBlocks = append(doc.ContentBlocks, *block)
	issues := ValidateNodeIDs(doc)
	if len(issues) == 0 || issues[0].Message != `duplicate nodeId "Same" (also on the text "a")` || issues[0].Position.Line != 9 {
		t.Fatalf("want the error at line 9 naming the first node, got %v", issues)
	}
	if len(issues) != 2 || issues[1].Position.Line != 3 || issues[1].Severity != diagnostics.Info {
		t.Fatalf("want an info at the first node (line 3), got %v", issues)
	}
}
