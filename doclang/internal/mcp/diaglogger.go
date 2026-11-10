// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"fmt"
	"regexp"
	"strconv"
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

// generatorLineRe reconoce la línea que el generador pone en sus warnings:
// "DOCX: line N: …" (imágenes, ecuaciones, mapas, PlantUML) y "DOCX: … at
// line N was not rendered" (mermaid, ecuaciones y gráficas sin Chromium).
// Anclada a esos formatos: un "line N" en otra parte del mensaje (una ruta
// "line 3.png", el error de un renderizador) no es la posición del elemento.
var generatorLineRe = regexp.MustCompile(`^DOCX: (?:(line (\d+)):|(?:mermaid diagram \([^)]*\)|chart \([^)]*\)|equation) at (line (\d+)) was not rendered)`)

// generatorLine devuelve la línea de un warning del generador (0 si no la
// trae, ver generatorLineRe) y dónde está "line N" en msg.
func generatorLine(msg string) (line, start, end int) {
	m := generatorLineRe.FindStringSubmatchIndex(msg)
	if m == nil {
		return 0, 0, 0
	}
	span, digits := m[2:4], m[4:6]
	if span[0] < 0 {
		span, digits = m[6:8], m[8:10]
	}
	n, err := strconv.Atoi(msg[digits[0]:digits[1]])
	if err != nil || n <= 0 {
		return 0, 0, 0
	}
	return n, span[0], span[1]
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
	// El generador no pasa posiciones estructuradas; los mensajes que la
	// conocen (imágenes, ecuaciones, mapas) dicen "line N" (generatorLine).
	// Sin ella queda 0:0, que significa "sin posición".
	var pos diagnostics.Position
	if n, _, _ := generatorLine(msg); n > 0 {
		pos = diagnostics.NewPosition(n, 1)
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.diags = append(l.diags, diagnostics.NewWarning(msg, pos, "generator"))
}

func (l *diagnosticLogger) diagnostics() []diagnostics.Diagnostic {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]diagnostics.Diagnostic(nil), l.diags...)
}
