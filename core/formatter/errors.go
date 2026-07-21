// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import "fmt"

// UnsupportedElementError se devuelve cuando el AST contiene un elemento
// que el dialecto de destino no puede representar hoy — un hueco
// pre-existente del parser (p. ej. el modo strict de slidelang no tiene
// sintaxis para GRID: el keyword existe en el parser modular pero
// parser.StrictParser nunca lo despacha — ver issue #214; QUOTE/CHECKLIST
// tenían el mismo hueco hasta que se cerró en issue #205), no una omisión
// del formatter. Se reporta en vez de emitir texto que luego no re-parsea
// al mismo AST (violaría el contrato de round-trip).
type UnsupportedElementError struct {
	NodeType string
	Reason   string
}

func (e *UnsupportedElementError) Error() string {
	return fmt.Sprintf("formatter: no se puede representar un elemento %q: %s", e.NodeType, e.Reason)
}

func newUnsupported(nodeType, reason string) error {
	return &UnsupportedElementError{NodeType: nodeType, Reason: reason}
}
