// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/util"
)

func TestChecklistParser_CanParse(t *testing.T) {
	parser := &ChecklistParser{}

	tests := []struct {
		name     string
		line     string
		mode     string
		expected bool
	}{
		{
			name:     "strict mode - CHECKLIST keyword",
			line:     "CHECKLIST",
			mode:     "strict",
			expected: true,
		},
		{
			name:     "flex mode - unchecked item",
			line:     "- [ ] Task item",
			mode:     "flex",
			expected: true,
		},
		{
			name:     "flex mode - checked item",
			line:     "- [x] Completed task",
			mode:     "flex",
			expected: true,
		},
		{
			name:     "flex mode - checked item uppercase",
			line:     "- [X] Completed task",
			mode:     "flex",
			expected: true,
		},
		{
			name:     "flex mode - with asterisk",
			line:     "* [ ] Task item",
			mode:     "flex",
			expected: true,
		},
		{
			name:     "flex mode - with plus",
			line:     "+ [x] Task item",
			mode:     "flex",
			expected: true,
		},
		{
			name:     "not a checklist - regular list",
			line:     "- Regular list item",
			mode:     "flex",
			expected: false,
		},
		{
			name:     "not a checklist - no space in checkbox",
			line:     "- []Task",
			mode:     "flex",
			expected: false,
		},
		{
			name:     "not a checklist - invalid checkbox",
			line:     "- [y] Invalid checkbox",
			mode:     "flex",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.CanParse(tt.line, tt.mode)
			if result != tt.expected {
				t.Errorf("CanParse() = %v, expected %v for line: %s", result, tt.expected, tt.line)
			}
		})
	}
}

func TestChecklistParser_ParseFlex(t *testing.T) {
	parser := &ChecklistParser{}
	logger := util.NewNoop()

	lines := []string{
		"- [x] Completed task",
		"- [ ] Pending task",
		"- [X] Another completed task",
	}

	ctx := &ParseContext{
		Mode:   "flex",
		Lines:  lines,
		Logger: logger,
	}

	result := parser.Parse(ctx, 0)

	if result.Error != nil {
		t.Fatalf("Parse() error = %v", result.Error)
	}

	if result.Element == nil {
		t.Fatal("Parse() returned nil element")
	}

	checklist, ok := result.Element.(*ast.ChecklistElement)
	if !ok {
		t.Fatalf("Parse() returned wrong element type: %T", result.Element)
	}

	if len(checklist.Items) != 3 {
		t.Errorf("Expected 3 items, got %d", len(checklist.Items))
	}

	// Check first item (checked)
	if !checklist.Items[0].Checked {
		t.Error("First item should be checked")
	}
	if checklist.Items[0].Content != "Completed task" {
		t.Errorf("First item content = %q, expected %q", checklist.Items[0].Content, "Completed task")
	}

	// Check second item (unchecked)
	if checklist.Items[1].Checked {
		t.Error("Second item should not be checked")
	}
	if checklist.Items[1].Content != "Pending task" {
		t.Errorf("Second item content = %q, expected %q", checklist.Items[1].Content, "Pending task")
	}

	// Check third item (checked with uppercase X)
	if !checklist.Items[2].Checked {
		t.Error("Third item should be checked")
	}
	if checklist.Items[2].Content != "Another completed task" {
		t.Errorf("Third item content = %q, expected %q", checklist.Items[2].Content, "Another completed task")
	}

	if result.ConsumedLines != 3 {
		t.Errorf("Expected 3 consumed lines, got %d", result.ConsumedLines)
	}
}

func TestChecklistParser_ParseStrict(t *testing.T) {
	parser := &ChecklistParser{}
	logger := util.NewNoop()

	lines := []string{
		"CHECKLIST",
		"    [x] Completed task",
		"    [ ] Pending task",
		"        [x] Sub-task completed",
		"    [X] Another completed task",
	}

	ctx := &ParseContext{
		Mode:   "strict",
		Lines:  lines,
		Logger: logger,
	}

	result := parser.Parse(ctx, 0)

	if result.Error != nil {
		t.Fatalf("Parse() error = %v", result.Error)
	}

	if result.Element == nil {
		t.Fatal("Parse() returned nil element")
	}

	checklist, ok := result.Element.(*ast.ChecklistElement)
	if !ok {
		t.Fatalf("Parse() returned wrong element type: %T", result.Element)
	}

	if len(checklist.Items) != 3 {
		t.Errorf("Expected 3 main items, got %d", len(checklist.Items))
	}

	// Check first item (checked)
	if !checklist.Items[0].Checked {
		t.Error("First item should be checked")
	}

	// Check second item (unchecked with sub-item)
	if checklist.Items[1].Checked {
		t.Error("Second item should not be checked")
	}
	if len(checklist.Items[1].SubItems) != 1 {
		t.Errorf("Second item should have 1 sub-item, got %d", len(checklist.Items[1].SubItems))
	}
	if len(checklist.Items[1].SubItems) > 0 && !checklist.Items[1].SubItems[0].Checked {
		t.Error("Sub-item should be checked")
	}

	// Check third item (checked)
	if !checklist.Items[2].Checked {
		t.Error("Third item should be checked")
	}
}

// TestChecklistParser_ParseStrict_DoesNotSwallowSiblingElement cubre la misma
// clase de bug que el fix de IMAGE (issue: "strict IMAGE deja de tragarse
// elementos hermanos"), pero para CHECKLIST: parseStrictChecklist termina su
// loop de continuación en IsNewElement(line, "strict") (internal/elements/
// common.go), cuya rama strict solo reconoce keywords en mayúsculas
// (TEXT/POINTS/CODE/...) — NO los marcadores simbólicos @ (directiva), :::
// (special block/grid/code-group), << (math/mermaid/plantuml/chart/map) ni |
// (tabla Markdown).
//
// Los items van indentados (2 espacios, igual que el resto del corpus/tests
// de este parser) para que auto-detección de indentación (expectedIndent) se
// active con normalidad; el elemento hermano se coloca a ESE MISMO nivel de
// indentación (no menos), que es precisamente el caso en el que el guard de
// indentación (`currentIndent < expectedIndent`) NO corta el loop por sí
// solo — así el test aísla el gap real: la detección de marcador simbólico
// en IsNewElement, no el guard de indentación (que ya cubre el caso, más
// común, de un hermano desindentado de vuelta al nivel del bloque padre).
func TestChecklistParser_ParseStrict_DoesNotSwallowSiblingElement(t *testing.T) {
	logger := util.NewNoop()

	cases := []struct {
		name         string
		siblingLines []string // líneas del elemento hermano, sin consumir
	}{
		{
			name:         "math block sibling",
			siblingLines: []string{"  <<math>>", "  x^2", "  <<end>>"},
		},
		{
			name:         "special block sibling",
			siblingLines: []string{"  :::info", "  Nota importante.", "  :::"},
		},
		{
			name:         "directive sibling",
			siblingLines: []string{"  @center"},
		},
		{
			name:         "markdown table sibling",
			siblingLines: []string{"  | a | b |", "  |---|---|", "  | 1 | 2 |"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lines := []string{
				"CHECKLIST",
				"  [x] Completed task",
				"  [ ] Pending task",
			}
			lines = append(lines, tc.siblingLines...)

			ctx := &ParseContext{
				Mode:   "strict",
				Lines:  lines,
				Logger: logger,
			}

			parser := &ChecklistParser{}
			result := parser.Parse(ctx, 0)

			if result.Error != nil {
				t.Fatalf("Parse() error = %v", result.Error)
			}

			const checklistLines = 3 // "CHECKLIST" + 2 items
			if result.ConsumedLines != checklistLines {
				t.Errorf("ConsumedLines = %d, want %d (sibling element must not be consumed)", result.ConsumedLines, checklistLines)
			}

			checklist, ok := result.Element.(*ast.ChecklistElement)
			if !ok {
				t.Fatalf("Parse() returned wrong element type: %T", result.Element)
			}
			if len(checklist.Items) != 2 {
				t.Fatalf("Expected 2 items, got %d: %+v", len(checklist.Items), checklist.Items)
			}
			for _, item := range checklist.Items {
				for _, sib := range tc.siblingLines {
					if strings.Contains(item.Content, strings.TrimSpace(sib)) {
						t.Errorf("item content %q leaked sibling element text %q", item.Content, sib)
					}
				}
			}
		})
	}
}

func TestChecklistParser_parseChecklistContent(t *testing.T) {
	parser := &ChecklistParser{}

	tests := []struct {
		line            string
		expectedContent string
		expectedChecked bool
	}{
		{
			line:            "- [x] Completed task",
			expectedContent: "Completed task",
			expectedChecked: true,
		},
		{
			line:            "- [ ] Pending task",
			expectedContent: "Pending task",
			expectedChecked: false,
		},
		{
			line:            "* [X] Uppercase checked",
			expectedContent: "Uppercase checked",
			expectedChecked: true,
		},
		{
			line:            "+ [ ] Plus marker",
			expectedContent: "Plus marker",
			expectedChecked: false,
		},
		{
			line:            "- [x] Task with **bold** text",
			expectedContent: "Task with **bold** text",
			expectedChecked: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			content, checked := parser.parseChecklistContent(tt.line)
			if content != tt.expectedContent {
				t.Errorf("parseChecklistContent() content = %q, expected %q", content, tt.expectedContent)
			}
			if checked != tt.expectedChecked {
				t.Errorf("parseChecklistContent() checked = %v, expected %v", checked, tt.expectedChecked)
			}
		})
	}
}
