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
