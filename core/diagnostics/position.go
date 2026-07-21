// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package diagnostics

import "fmt"

// Position representa una posición en el código fuente (1-indexed)
type Position struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

func NewPosition(line, column int) Position {
	return Position{Line: line, Column: column}
}

func (p Position) String() string {
	return fmt.Sprintf("%d:%d", p.Line, p.Column)
}

// IsValid verifica si la posición es válida
func (p Position) IsValid() bool {
	return p.Line > 0 && p.Column > 0
}

// Before verifica si esta posición está antes que otra
func (p Position) Before(other Position) bool {
	if p.Line < other.Line {
		return true
	}
	return p.Line == other.Line && p.Column < other.Column
}
