// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"fmt"
	"strings"
	"sync"

	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/util"
)

// diagnosticLogger reenvía todo al logger del servidor y además guarda cada
// Warn y Error como un diagnóstico "warning" para el resultado de export.
// Los Error del generador no cortan la generación (si la cortaran, Generate
// devolvería error), así que tampoco se marcan como error para el cliente.
// redact quita la ruta absoluta de la raíz de cada mensaje.
type diagnosticLogger struct {
	util.Logger
	redact func(string) string

	mu    sync.Mutex
	diags []diagnostics.Diagnostic
}

func newDiagnosticLogger(next util.Logger, redact func(string) string) *diagnosticLogger {
	return &diagnosticLogger{Logger: next, redact: redact}
}

func (l *diagnosticLogger) Warn(message string, args ...interface{}) {
	l.Logger.Warn(message, args...)
	l.record(message, args...)
}

func (l *diagnosticLogger) Error(message string, args ...interface{}) {
	l.Logger.Error(message, args...)
	l.record(message, args...)
}

func (l *diagnosticLogger) record(message string, args ...interface{}) {
	msg := strings.TrimSpace(fmt.Sprintf(message, args...))
	if l.redact != nil {
		msg = l.redact(msg)
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	// Posición 0:0: el generador no la conoce en general; los mensajes que
	// sí la conocen (imágenes) la citan en el texto.
	l.diags = append(l.diags, diagnostics.NewWarning(msg, diagnostics.Position{}, "generator"))
}

func (l *diagnosticLogger) diagnostics() []diagnostics.Diagnostic {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]diagnostics.Diagnostic(nil), l.diags...)
}
