// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package diagnostics

import (
	"strings"
	"testing"
)

func TestNewError(t *testing.T) {
	pos := NewPosition(10, 5)
	diag := NewError("test error", pos, "parser")
	
	if diag.Severity != Error {
		t.Errorf("Severity = %v, want Error", diag.Severity)
	}
	
	if diag.Message != "test error" {
		t.Errorf("Message = %s, want test error", diag.Message)
	}
	
	if diag.Source != "parser" {
		t.Errorf("Source = %s, want parser", diag.Source)
	}
	
	if diag.Position.Line != 10 || diag.Position.Column != 5 {
		t.Errorf("Position = %v, want 10:5", diag.Position)
	}
}

func TestNewWarning(t *testing.T) {
	pos := NewPosition(5, 10)
	diag := NewWarning("test warning", pos, "linter")
	
	if diag.Severity != Warning {
		t.Errorf("Severity = %v, want Warning", diag.Severity)
	}
	
	if diag.Message != "test warning" {
		t.Errorf("Message = %s, want test warning", diag.Message)
	}
}

func TestNewInfo(t *testing.T) {
	pos := NewPosition(1, 1)
	diag := NewInfo("test info", pos, "analyzer")
	
	if diag.Severity != Info {
		t.Errorf("Severity = %v, want Info", diag.Severity)
	}
}

func TestDiagnostic_String(t *testing.T) {
	pos := NewPosition(10, 5)
	
	tests := []struct {
		name     string
		diag     Diagnostic
		contains []string
	}{
		{
			name:     "Basic error",
			diag:     NewError("syntax error", pos, "parser"),
			contains: []string{"error", "syntax error", "10:5"},
		},
		{
			name:     "Error with rule ID",
			diag:     NewError("invalid syntax", pos, "parser").WithRuleID("E001"),
			contains: []string{"error[E001]", "invalid syntax", "10:5"},
		},
		{
			name: "Error with end position",
			diag: NewError("invalid range", pos, "parser").WithEndPosition(NewPosition(10, 15)),
			contains: []string{"error", "invalid range", "10:5", "10:15"},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.diag.String()
			for _, substr := range tt.contains {
				if !strings.Contains(result, substr) {
					t.Errorf("String() = %s, should contain %s", result, substr)
				}
			}
		})
	}
}

func TestDiagnostic_IsError(t *testing.T) {
	pos := NewPosition(1, 1)
	
	errorDiag := NewError("error", pos, "test")
	if !errorDiag.IsError() {
		t.Error("IsError() = false for error diagnostic")
	}
	
	warningDiag := NewWarning("warning", pos, "test")
	if warningDiag.IsError() {
		t.Error("IsError() = true for warning diagnostic")
	}
}

func TestDiagnostic_IsWarning(t *testing.T) {
	pos := NewPosition(1, 1)
	
	warningDiag := NewWarning("warning", pos, "test")
	if !warningDiag.IsWarning() {
		t.Error("IsWarning() = false for warning diagnostic")
	}
	
	errorDiag := NewError("error", pos, "test")
	if errorDiag.IsWarning() {
		t.Error("IsWarning() = true for error diagnostic")
	}
}

func TestDiagnostic_WithRuleID(t *testing.T) {
	pos := NewPosition(1, 1)
	diag := NewError("error", pos, "test").WithRuleID("E123")
	
	if diag.RuleID != "E123" {
		t.Errorf("WithRuleID() set RuleID = %s, want E123", diag.RuleID)
	}
}

func TestDiagnostic_WithCode(t *testing.T) {
	pos := NewPosition(1, 1)
	diag := NewError("error", pos, "test").WithCode("SYNTAX_ERROR")
	
	if diag.Code != "SYNTAX_ERROR" {
		t.Errorf("WithCode() set Code = %s, want SYNTAX_ERROR", diag.Code)
	}
}

func TestDiagnostic_WithEndPosition(t *testing.T) {
	pos := NewPosition(1, 1)
	endPos := NewPosition(1, 10)
	diag := NewError("error", pos, "test").WithEndPosition(endPos)
	
	if diag.EndPosition == nil {
		t.Fatal("WithEndPosition() did not set EndPosition")
	}
	
	if diag.EndPosition.Line != 1 || diag.EndPosition.Column != 10 {
		t.Errorf("WithEndPosition() set EndPosition = %v, want 1:10", diag.EndPosition)
	}
}

func TestDiagnostic_Chaining(t *testing.T) {
	pos := NewPosition(5, 10)
	diag := NewError("test error", pos, "parser").
		WithRuleID("E001").
		WithCode("SYNTAX_ERROR").
		WithEndPosition(NewPosition(5, 20))
	
	if diag.RuleID != "E001" {
		t.Errorf("Chained RuleID = %s, want E001", diag.RuleID)
	}
	
	if diag.Code != "SYNTAX_ERROR" {
		t.Errorf("Chained Code = %s, want SYNTAX_ERROR", diag.Code)
	}
	
	if diag.EndPosition == nil {
		t.Fatal("Chained EndPosition is nil")
	}
}
