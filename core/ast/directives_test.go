// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package ast

import (
	"testing"

	"go.ziradocs.com/core/diagnostics"
)

func TestNewDirectiveNode(t *testing.T) {
	pos := diagnostics.NewPosition(5, 10)
	directive := NewDirectiveNode(pos, "layout")
	
	if directive == nil {
		t.Fatal("NewDirectiveNode() returned nil")
	}
	
	if directive.GetType() != NodeTypeDirective {
		t.Errorf("GetType() = %v, want NodeTypeDirective", directive.GetType())
	}
	
	if directive.Name != "layout" {
		t.Errorf("Name = %s, want layout", directive.Name)
	}
	
	if directive.Parameters == nil {
		t.Fatal("NewDirectiveNode() did not initialize Parameters")
	}
	
	if len(directive.Parameters) != 0 {
		t.Errorf("Parameters length = %d, want 0", len(directive.Parameters))
	}
}

func TestDirectiveNode_Position(t *testing.T) {
	pos := diagnostics.NewPosition(10, 5)
	directive := NewDirectiveNode(pos, "test")
	
	if directive.GetPosition().Line != 10 {
		t.Errorf("GetPosition().Line = %d, want 10", directive.GetPosition().Line)
	}
	
	if directive.GetPosition().Column != 5 {
		t.Errorf("GetPosition().Column = %d, want 5", directive.GetPosition().Column)
	}
}

func TestDirectiveNode_Parameters(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	directive := NewDirectiveNode(pos, "config")
	
	// Add string parameter
	directive.Parameters["color"] = "#FF0000"
	if directive.Parameters["color"] != "#FF0000" {
		t.Errorf("Parameters[color] = %v, want #FF0000", directive.Parameters["color"])
	}
	
	// Add numeric parameter
	directive.Parameters["size"] = 42
	if directive.Parameters["size"] != 42 {
		t.Errorf("Parameters[size] = %v, want 42", directive.Parameters["size"])
	}
	
	// Add boolean parameter
	directive.Parameters["enabled"] = true
	if directive.Parameters["enabled"] != true {
		t.Errorf("Parameters[enabled] = %v, want true", directive.Parameters["enabled"])
	}
	
	// Verify count
	if len(directive.Parameters) != 3 {
		t.Errorf("Parameters length = %d, want 3", len(directive.Parameters))
	}
}

func TestDirectiveNode_MultipleDirectives(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	
	directives := []*DirectiveNode{
		NewDirectiveNode(pos, "layout"),
		NewDirectiveNode(pos, "theme"),
		NewDirectiveNode(pos, "transition"),
	}
	
	expectedNames := []string{"layout", "theme", "transition"}
	
	for i, directive := range directives {
		if directive.Name != expectedNames[i] {
			t.Errorf("Directive %d name = %s, want %s", i, directive.Name, expectedNames[i])
		}
	}
}

func TestDirectiveNode_ComplexParameters(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	directive := NewDirectiveNode(pos, "style")
	
	// Add map parameter
	styleConfig := map[string]interface{}{
		"background": "#FFFFFF",
		"color":      "#000000",
		"fontSize":   "16px",
	}
	directive.Parameters["config"] = styleConfig
	
	// Verify nested map
	if config, ok := directive.Parameters["config"].(map[string]interface{}); ok {
		if config["background"] != "#FFFFFF" {
			t.Errorf("Nested config[background] = %v, want #FFFFFF", config["background"])
		}
	} else {
		t.Error("Parameters[config] is not a map[string]interface{}")
	}
	
	// Add array parameter
	colors := []string{"red", "green", "blue"}
	directive.Parameters["colors"] = colors
	
	if colorArray, ok := directive.Parameters["colors"].([]string); ok {
		if len(colorArray) != 3 {
			t.Errorf("Color array length = %d, want 3", len(colorArray))
		}
	} else {
		t.Error("Parameters[colors] is not a []string")
	}
}
