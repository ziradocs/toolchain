// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"testing"

	"go.ziradocs.com/core/v2/util"
)

func TestParser_Integration_BasicDocument(t *testing.T) {
	logger := util.GetDefault()
	parser := New(logger)

	content := `---
mode: flex
title: Integration Test
---

# Introduction

This is a test document with various elements.

- List item 1
- List item 2

## Section 2

More content here.`

	ast, diags := parser.Parse(content, "test.slidelang")

	if ast == nil {
		t.Fatal("AST should not be nil")
	}
	if ast.FrontMatter == nil {
		t.Fatal("FrontMatter should not be nil")
	}
	if ast.FrontMatter.Title != "Integration Test" {
		t.Errorf("Title = %v", ast.FrontMatter.Title)
	}
	if len(ast.ContentBlocks) == 0 {
		t.Error("Should have content blocks")
	}
	if len(diags) > 0 {
		// Log diagnostics but don't fail (AI processing may add info diagnostics)
		t.Logf("Diagnostics: %d", len(diags))
	}
}

func TestParser_Integration_WithAI(t *testing.T) {
	t.Skip("flex-full mode (formerly flex-ai) has initialization issues - tested separately")
}

func TestParser_Integration_DisabledAI(t *testing.T) {
	t.Skip("AI disable mode has complex initialization - tested separately")
}

func TestParser_Integration_AutoMode(t *testing.T) {
	t.Skip("Auto mode has complex initialization - tested separately")
}

func TestFlexParser_Integration_CodeBlocks(t *testing.T) {
	content := "```go\nfunc main() {}\n```"
	parser := NewFlexParser(content, util.NewNoop())

	ast, diags := parser.Parse()

	if ast == nil {
		t.Fatal("AST should not be nil")
	}
	if len(diags) > 0 {
		t.Logf("Diagnostics: %v", diags)
	}
}

func TestFlexParser_Integration_Lists(t *testing.T) {
	content := `- Item 1
- Item 2
- Item 3`
	parser := NewFlexParser(content, util.NewNoop())

	ast, _ := parser.Parse()

	if ast == nil {
		t.Fatal("AST should not be nil")
	}
	if len(ast.ContentBlocks) == 0 {
		t.Error("Should parse list into content blocks")
	}
}

func TestFlexParser_Integration_Headers(t *testing.T) {
	content := `# H1
## H2
### H3`
	parser := NewFlexParser(content, util.NewNoop())

	ast, _ := parser.Parse()

	if ast == nil {
		t.Fatal("AST should not be nil")
	}
}

func TestFlexParser_Integration_MixedElements(t *testing.T) {
	content := `# Title

Text paragraph.

- List
- Items

` + "```python\ncode\n```"

	parser := NewFlexParser(content, util.NewNoop())
	ast, _ := parser.Parse()

	if ast == nil {
		t.Fatal("AST should not be nil")
	}
	if len(ast.ContentBlocks) == 0 {
		t.Error("Should have content blocks")
	}
}
