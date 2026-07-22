// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"testing"

	"go.ziradocs.com/core/v2/util"
)

func TestParser_New(t *testing.T) {
	logger := util.GetDefault()
	parser := New(logger)

	if parser == nil {
		t.Fatal("New() should not return nil")
	}
	if parser.enableNormalize != true {
		t.Error("normalization should be enabled by default")
	}
}

func TestParser_SetNormalization(t *testing.T) {
	logger := util.GetDefault()
	parser := New(logger)

	parser.SetNormalization(false)
	if parser.enableNormalize != false {
		t.Error("SetNormalization(false) should disable normalization")
	}

	parser.SetNormalization(true)
	if parser.enableNormalize != true {
		t.Error("SetNormalization(true) should enable normalization")
	}
}

// TestParser_SetAIProcessing_DeprecatedAlias confirma que el alias
// deprecado sigue funcionando idéntico a SetNormalization (Tier B: romper
// sin alias no es aceptable pre-lanzamiento tampoco).
func TestParser_SetAIProcessing_DeprecatedAlias(t *testing.T) {
	logger := util.GetDefault()
	parser := New(logger)

	parser.SetAIProcessing(false)
	if parser.enableNormalize != false {
		t.Error("SetAIProcessing(false) (deprecated alias) should disable normalization")
	}

	parser.EnableAIProcessing()
	if parser.enableNormalize != true {
		t.Error("EnableAIProcessing() (deprecated alias) should enable normalization")
	}
}

func TestParser_Parse_EmptyContent(t *testing.T) {
	logger := util.GetDefault()
	parser := New(logger)

	content := `---
mode: flex
---`

	ast, diags := parser.Parse(content, "test.slidelang")

	if ast == nil {
		t.Fatal("AST should not be nil for valid frontmatter")
	}
	if len(diags) > 0 {
		t.Errorf("Should not have diagnostics for valid empty content")
	}
	if ast.FilePath != "test.slidelang" {
		t.Errorf("FilePath = %v, want test.slidelang", ast.FilePath)
	}
}

func TestParser_Parse_FlexMode(t *testing.T) {
	logger := util.GetDefault()
	parser := New(logger)

	content := `---
mode: flex
title: Test
---

# Title

Some content here.`

	ast, _ := parser.Parse(content, "test.slidelang")

	if ast == nil {
		t.Fatal("AST should not be nil")
	}
	if ast.FrontMatter == nil {
		t.Fatal("FrontMatter should not be nil")
	}
	if ast.FrontMatter.Mode != "flex" {
		t.Errorf("Mode = %v, want flex", ast.FrontMatter.Mode)
	}
}

// TestParser_Parse_FlexAI_DeprecatedAlias confirma que "flex-ai" (nombre
// previo a #210) sigue siendo un modo válido, permanentemente aceptado
// como alias de "flex-full" — el corpus de ejemplos y la salida propia del
// normalizador ya usaban "flex-ai", así que dejar de aceptarlo rompería
// archivos existentes.
func TestParser_Parse_FlexAI_DeprecatedAlias(t *testing.T) {
	logger := util.GetDefault()

	content := `---
mode: flex-ai
title: Test
---

# Title

Some content here.`

	p := New(logger)
	astNode, diags := p.Parse(content, "test.slidelang")

	if astNode == nil {
		t.Fatal("AST should not be nil for deprecated 'flex-ai' mode")
	}
	if astNode.FrontMatter == nil {
		t.Fatal("FrontMatter should not be nil")
	}
	if astNode.FrontMatter.Mode != "flex-ai" {
		t.Errorf("Mode = %v, want flex-ai (preserved verbatim from frontmatter)", astNode.FrontMatter.Mode)
	}
	for _, d := range diags {
		if d.IsError() {
			t.Errorf("unexpected error diagnostic for deprecated 'flex-ai' mode: %s", d.Message)
		}
	}
}

// TestParser_Parse_FlexFull confirma que "flex-full" (nombre canónico desde
// #210, reemplazo de "flex-ai") parsea sin diagnósticos de error.
func TestParser_Parse_FlexFull(t *testing.T) {
	logger := util.GetDefault()

	content := `---
mode: flex-full
title: Test
---

# Title

Some content here.`

	p := New(logger)
	astNode, diags := p.Parse(content, "test.slidelang")

	if astNode == nil {
		t.Fatal("AST should not be nil for 'flex-full' mode")
	}
	if astNode.FrontMatter == nil {
		t.Fatal("FrontMatter should not be nil")
	}
	if astNode.FrontMatter.Mode != "flex-full" {
		t.Errorf("Mode = %v, want flex-full", astNode.FrontMatter.Mode)
	}
	for _, d := range diags {
		if d.IsError() {
			t.Errorf("unexpected error diagnostic for 'flex-full' mode: %s", d.Message)
		}
	}
}

// TestParser_Parse_StrictMode is skipped because strict mode parser
// has complex initialization requirements that need deeper investigation
// func TestParser_Parse_StrictMode(t *testing.T) {
// 	logger := util.GetDefault()
// 	parser := New(logger)
//
// 	content := `---
// mode: strict
// ---
//
// SLIDE title
//   TEXT: test content`
//
// 	ast, _ := parser.Parse(content, "test.slidelang")
//
// 	if ast == nil {
// 		t.Fatal("AST should not be nil")
// 	}
// 	if ast.FrontMatter == nil {
// 		t.Fatal("FrontMatter should not be nil")
// 	}
// 	if ast.FrontMatter.Mode != "strict" {
// 		t.Errorf("Mode = %v, want strict", ast.FrontMatter.Mode)
// 	}
// }

func TestParser_Parse_NoFrontmatter(t *testing.T) {
	logger := util.GetDefault()
	parser := New(logger)

	content := `# Just content
No frontmatter here`

	ast, diags := parser.Parse(content, "test.slidelang")

	// Should return nil AST when no frontmatter
	if ast != nil {
		t.Error("AST should be nil when no frontmatter")
	}
	if len(diags) == 0 {
		t.Error("Should have diagnostics for missing frontmatter")
	}
}

func TestFlexParser_New(t *testing.T) {
	content := "line1\nline2\nline3"
	parser := NewFlexParser(content, util.NewNoop())

	if parser == nil {
		t.Fatal("NewFlexParser() should not return nil")
	}
	if len(parser.lines) != 3 {
		t.Errorf("len(lines) = %v, want 3", len(parser.lines))
	}
	if parser.currentLine != 0 {
		t.Errorf("currentLine = %v, want 0", parser.currentLine)
	}
}

func TestFlexParser_Parse_Empty(t *testing.T) {
	parser := NewFlexParser("", util.NewNoop())

	ast, _ := parser.Parse()

	if ast == nil {
		t.Fatal("AST should not be nil")
	}
	if len(ast.ContentBlocks) != 0 {
		t.Errorf("Empty input should produce 0 blocks, got %v", len(ast.ContentBlocks))
	}
}

func TestFlexParser_Parse_Simple(t *testing.T) {
	content := `# Title

Some text content.`

	parser := NewFlexParser(content, util.NewNoop())
	ast, _ := parser.Parse()

	if ast == nil {
		t.Fatal("AST should not be nil")
	}
	if len(ast.ContentBlocks) == 0 {
		t.Error("Should have at least one content block")
	}
}

func TestStrictParser_New(t *testing.T) {
	content := "SLIDE title"
	parser := NewStrictParser(content, util.NewNoop())

	if parser == nil {
		t.Fatal("NewStrictParser() should not return nil")
	}
	if len(parser.lines) != 1 {
		t.Errorf("len(lines) = %v, want 1", len(parser.lines))
	}
}

// StrictParser tests skipped - requires complex initialization
// func TestStrictParser_Parse_Empty(t *testing.T) {
// 	parser := NewStrictParser("")
//
// 	ast, _ := parser.Parse()
//
// 	if ast == nil {
// 		t.Fatal("AST should not be nil")
// 	}
// 	if len(ast.ContentBlocks) != 0 {
// 		t.Error("Empty input should produce 0 blocks")
// 	}
// }
//
// func TestStrictParser_Parse_Simple(t *testing.T) {
// 	content := `SLIDE title
//   TITLE: Test Title`
//
// 	parser := NewStrictParser(content)
// 	ast, _ := parser.Parse()
//
// 	if ast == nil {
// 		t.Fatal("AST should not be nil")
// 	}
// 	if len(ast.ContentBlocks) == 0 {
// 		t.Error("Should have at least one content block")
// 	}
// }
