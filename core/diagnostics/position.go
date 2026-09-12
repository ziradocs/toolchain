// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package diagnostics

import "fmt"

// Position representa una posición en el código fuente. Line y Column son
// 1-based y cuentan desde la primera línea del ARCHIVO, frontmatter
// incluido (issue #245) — nunca desde el cuerpo que un parser en particular
// haya recibido. Todo constructor de una Position para contenido del cuerpo
// tiene que sumar el offset del frontmatter que precede a ese cuerpo; ver
// core/parser/position_offset_guard_test.go, que lo hace cumplir por
// construcción.
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
