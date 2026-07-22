// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package ast

import (
	"regexp"
	"testing"

	"go.ziradocs.com/core/v2/diagnostics"
)

func TestBaseNode(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	node := NewBaseNode(NodeTypeContentBlock, pos)

	if node.GetType() != NodeTypeContentBlock {
		t.Errorf("GetType() = %v, want NodeTypeContentBlock", node.GetType())
	}

	if node.GetPosition().Line != 1 || node.GetPosition().Column != 1 {
		t.Errorf("GetPosition() = %v, want 1:1", node.GetPosition())
	}

	if node.GetEndPosition().Line != 1 || node.GetEndPosition().Column != 1 {
		t.Errorf("GetEndPosition() = %v, want 1:1", node.GetEndPosition())
	}
}

func TestNewAST(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	ast := NewAST(pos)

	if ast == nil {
		t.Fatal("NewAST() returned nil")
	}

	if ast.GetType() != NodeTypePresentation {
		t.Errorf("GetType() = %v, want NodeTypePresentation", ast.GetType())
	}

	if ast.ContentBlocks == nil {
		t.Fatal("NewAST() did not initialize ContentBlocks")
	}

	if len(ast.ContentBlocks) != 0 {
		t.Errorf("NewAST() initialized ContentBlocks with length %d, want 0", len(ast.ContentBlocks))
	}

	// Issue #8: schemaVersion debe poblarse siempre para que los consumidores
	// del contrato JSON (p. ej. el viewer) puedan detectar breaking changes.
	if ast.SchemaVersion == "" {
		t.Error("NewAST() did not populate SchemaVersion")
	}
	if ast.SchemaVersion != SchemaVersion {
		t.Errorf("SchemaVersion = %q, want %q (the package constant)", ast.SchemaVersion, SchemaVersion)
	}
}

var semverPattern = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

// TestSchemaVersion_IsSemver cubre issue #8: la política de compatibilidad
// documentada (breaking change ⇒ incrementar MAJOR) solo tiene sentido si
// SchemaVersion es realmente un semver "MAJOR.MINOR.PATCH", no un placeholder
// como "latest" o "v1". TestNewAST ya verifica que se propaga, pero solo la
// compara contra sí misma (una tautología); esta prueba fija el formato real.
func TestSchemaVersion_IsSemver(t *testing.T) {
	if !semverPattern.MatchString(SchemaVersion) {
		t.Errorf("SchemaVersion = %q, want a MAJOR.MINOR.PATCH semver string", SchemaVersion)
	}
}

func TestNodeTypes(t *testing.T) {
	nodeTypes := []NodeType{
		NodeTypePresentation,
		NodeTypeFrontMatter,
		NodeTypeContentBlock,
		NodeTypeText,
		NodeTypePoints,
		NodeTypeCode,
		NodeTypeImage,
		NodeTypePointItem,
		NodeTypeDirective,
		NodeTypeTable,
		NodeTypeSpecialBlock,
		NodeTypeCodeGroup,
		NodeTypeMermaid,
		NodeTypePlantUML,
		NodeTypeChart,
		NodeTypeMap,
		NodeTypeQuote,
		NodeTypeChecklist,
		NodeTypeChecklistItem,
		NodeTypeGrid,
		NodeTypeColumn,
	}

	// Verify all node types are unique
	seen := make(map[NodeType]bool)
	for _, nt := range nodeTypes {
		if seen[nt] {
			t.Errorf("Duplicate NodeType: %s", nt)
		}
		seen[nt] = true
	}

	if len(seen) != len(nodeTypes) {
		t.Errorf("Expected %d unique node types, got %d", len(nodeTypes), len(seen))
	}
}

func TestBaseNode_Comments(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	node := NewBaseNode(NodeTypeText, pos)

	if len(node.Comments) > 0 {
		t.Error("NewBaseNode should initialize with nil or empty comments")
	}

	node.Comments = []string{"comment1", "comment2"}
	if len(node.Comments) != 2 {
		t.Errorf("Comments length = %d, want 2", len(node.Comments))
	}
}

func TestAST_FilePath(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	ast := NewAST(pos)

	ast.FilePath = "/path/to/file.slidelang"
	if ast.FilePath != "/path/to/file.slidelang" {
		t.Errorf("FilePath = %s, want /path/to/file.slidelang", ast.FilePath)
	}
}

func TestNewContentBlock(t *testing.T) {
	pos := diagnostics.NewPosition(5, 10)
	block := NewContentBlock(pos, "title")

	if block == nil {
		t.Fatal("NewContentBlock() should not return nil")
	}
	if block.BlockType != "title" {
		t.Errorf("BlockType = %v, want title", block.BlockType)
	}
	if len(block.Elements) != 0 {
		t.Error("New content block should have 0 elements")
	}
}

func TestContentBlock_AddElements(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	block := NewContentBlock(pos, "content")

	text := &TextElement{Content: "test"}
	block.Elements = append(block.Elements, text)

	if len(block.Elements) != 1 {
		t.Errorf("len(Elements) = %v, want 1", len(block.Elements))
	}
}

func TestFrontMatterNode_DataFields(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	fm := NewFrontMatterNode(pos)

	if fm == nil {
		t.Fatal("NewFrontMatterNode() should not return nil")
	}
	if fm.Raw != "" {
		t.Error("Raw should be empty initially")
	}

	fm.Mode = "flex"
	fm.Title = "Test"
	fm.Author = "Someone"

	if fm.Mode != "flex" {
		t.Errorf("Mode = %v, want flex", fm.Mode)
	}
	if fm.Title != "Test" {
		t.Errorf("Title = %v, want Test", fm.Title)
	}
}

func TestTextElement(t *testing.T) {
	text := &TextElement{Content: "Hello, World!"}
	if text.Content != "Hello, World!" {
		t.Errorf("Content = %v", text.Content)
	}
}

func TestCodeElement(t *testing.T) {
	code := &CodeElement{
		Language: "go",
		Content:  "fmt.Println(\"Hello\")",
	}
	if code.Language != "go" {
		t.Errorf("Language = %v, want go", code.Language)
	}
	if code.Content != "fmt.Println(\"Hello\")" {
		t.Errorf("Content = %v", code.Content)
	}
}

func TestImageElement(t *testing.T) {
	img := &ImageElement{
		Source: "/path/to/image.png",
		Alt:    "Test",
	}
	if img.Source != "/path/to/image.png" {
		t.Errorf("Source = %v", img.Source)
	}
	if img.Alt != "Test" {
		t.Errorf("Alt = %v, want Test", img.Alt)
	}
}

func TestChartElement(t *testing.T) {
	chart := &ChartElement{
		ChartType: "bar",
		Labels:    []string{"Q1", "Q2", "Q3"},
	}
	if len(chart.Labels) != 3 {
		t.Errorf("len(Labels) = %v, want 3", len(chart.Labels))
	}
	if chart.ChartType != "bar" {
		t.Errorf("ChartType = %v, want bar", chart.ChartType)
	}
}

func TestMermaidElement(t *testing.T) {
	mermaid := &MermaidElement{
		DiagramType: "flowchart",
		Content:     "graph TD",
	}
	if mermaid.DiagramType != "flowchart" {
		t.Errorf("DiagramType = %v, want flowchart", mermaid.DiagramType)
	}
	if mermaid.Content != "graph TD" {
		t.Errorf("Content = %v, want 'graph TD'", mermaid.Content)
	}
}

func TestMapElement(t *testing.T) {
	mapElem := &MapElement{
		Center: &MapCoordinate{Lat: 40.7128, Lng: -74.0060},
		Zoom:   12,
	}
	if mapElem.Zoom != 12 {
		t.Errorf("Zoom = %v, want 12", mapElem.Zoom)
	}
	if mapElem.Center == nil {
		t.Error("Center should not be nil")
	}
	if mapElem.Center.Lat != 40.7128 {
		t.Errorf("Lat = %v, want 40.7128", mapElem.Center.Lat)
	}
}

func TestTableElement(t *testing.T) {
	table := &TableElement{
		Headers: []string{"Name", "Age"},
		Rows:    [][]string{{"Alice", "30"}},
	}
	if len(table.Headers) != 2 {
		t.Errorf("len(Headers) = %v, want 2", len(table.Headers))
	}
	if len(table.Rows) != 1 {
		t.Errorf("len(Rows) = %v, want 1", len(table.Rows))
	}
}

func TestQuoteElement(t *testing.T) {
	quote := &QuoteElement{
		Content: "To be or not to be",
		Author:  "Shakespeare",
	}
	if quote.Author != "Shakespeare" {
		t.Errorf("Author = %v, want Shakespeare", quote.Author)
	}
	if quote.Content != "To be or not to be" {
		t.Errorf("Content = %v", quote.Content)
	}
}
