// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package diagnostics

import "fmt"

type Severity string

const (
	Error   Severity = "error"
	Warning Severity = "warning"
	Info    Severity = "info"
)

type Diagnostic struct {
	Severity    Severity  `json:"severity"`
	Message     string    `json:"message"`
	Position    Position  `json:"position"`
	EndPosition *Position `json:"endPosition,omitempty"`
	RuleID      string    `json:"ruleId,omitempty"`
	Source      string    `json:"source,omitempty"` // "parser", "linter"
	Code        string    `json:"code,omitempty"`
}

func NewError(message string, pos Position, source string) Diagnostic {
	return Diagnostic{
		Severity: Error,
		Message:  message,
		Position: pos,
		Source:   source,
	}
}

func NewWarning(message string, pos Position, source string) Diagnostic {
	return Diagnostic{
		Severity: Warning,
		Message:  message,
		Position: pos,
		Source:   source,
	}
}

func NewInfo(message string, pos Position, source string) Diagnostic {
	return Diagnostic{
		Severity: Info,
		Message:  message,
		Position: pos,
		Source:   source,
	}
}

func (d Diagnostic) String() string {
	severity := string(d.Severity)
	if d.RuleID != "" {
		severity = fmt.Sprintf("%s[%s]", severity, d.RuleID)
	}

	if d.EndPosition != nil {
		return fmt.Sprintf("%s: %s (%s-%s)", severity, d.Message, d.Position, d.EndPosition)
	}
	return fmt.Sprintf("%s: %s (%s)", severity, d.Message, d.Position)
}

// IsError verifica si el diagnóstico es un error
func (d Diagnostic) IsError() bool {
	return d.Severity == Error
}

// IsWarning verifica si el diagnóstico es una advertencia
func (d Diagnostic) IsWarning() bool {
	return d.Severity == Warning
}

// WithRuleID añade un ID de regla al diagnóstico
func (d Diagnostic) WithRuleID(ruleID string) Diagnostic {
	d.RuleID = ruleID
	return d
}

// WithCode añade un código de error al diagnóstico
func (d Diagnostic) WithCode(code string) Diagnostic {
	d.Code = code
	return d
}

// WithEndPosition añade una posición final al diagnóstico
func (d Diagnostic) WithEndPosition(endPos Position) Diagnostic {
	d.EndPosition = &endPos
	return d
}
