// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

// Package mcp implementa el servidor MCP (Model Context Protocol) de
// doclang, expuesto vía `doclang mcp` — espejo del servidor de slidelang
// (slidelang/internal/mcp), issue #187/#189. Cada tool es un wrapper
// delgado sobre el mismo pipeline que usa `doclang build`
// (parser/transform/linter de core + internal/generator) — sin
// lógica de parseo/render propia — para que un agente reciba exactamente el
// mismo AST y los mismos diagnósticos que el CLI, sin round-trips por
// archivos.
package mcp

import (
	"fmt"

	"go.ziradocs.com/core/ast"
	"go.ziradocs.com/core/diagnostics"
	"go.ziradocs.com/core/parser"
	"go.ziradocs.com/core/transform"
	"go.ziradocs.com/core/util"
	"go.ziradocs.com/core/xref"
)

// maxConcurrentParses acota cuántas llamadas a parseSource pueden estar en
// vuelo a la vez — mismo motivo y mismo valor que
// slidelang/internal/mcp/parse.go: core no soporta cancelación
// de parseo, así que un servidor MCP de larga vida necesita un tope duro en
// vez de confiar en que el proceso termine pronto (como sí puede asumir
// `doclang build`).
const maxConcurrentParses = 4

var parseSemaphore = make(chan struct{}, maxConcurrentParses)

// parseSource parsea source con los mismos guards que usa `doclang build`
// (doclang/internal/cli/build.go) — cap de tamaño
// (util.CheckInputSize/DefaultMaxInputBytes) antes de tocar el parser, y
// timeout (util.RunGuarded/DefaultParseTimeout) como backstop — y además
// corre la misma etapa de transform built-in (transform.RunBuiltins con
// xref.Transform) que build.go corre entre el parseo y el lint. Sin esto,
// get_ast/preview divergirían de `doclang build --format html`: un doc con
// figuras/tablas/ecuaciones etiquetadas y \ref saldría SIN numerar y con los
// \ref sin resolver, justo el caso que más importa para documentos técnicos
// (a diferencia de slides, donde la numeración pesa menos). NO corre
// include.Expand (no hay base-dir para un string en memoria) ni los
// --filter de usuario (binarios externos configurados por el operador, no
// tienen lugar en una tool call MCP) — ambos gaps deliberados y documentados.
//
// A diferencia de slidelang, DocumentFlexParser no acepta un fileName (ver
// parser.NewDocumentFlexParserWithNormalization) y diagnostics.Diagnostic no
// lleva ruta de archivo — por eso, a diferencia del parseSource de
// slidelang/internal/mcp, este no tiene parámetro fileName: no hay nada
// donde plumbearlo.
func parseSource(logger util.Logger, source string) (*ast.AST, []diagnostics.Diagnostic, error) {
	if err := util.CheckInputSize(len(source), util.DefaultMaxInputBytes); err != nil {
		return nil, nil, err
	}

	// Adquisición no bloqueante — ver el comentario extenso en
	// slidelang/internal/mcp/parse.go: con un `<-` incondicional, tras
	// maxConcurrentParses parses genuinamente colgados TODA llamada
	// subsiguiente se bloquearía para siempre. select+default falla rápido.
	select {
	case parseSemaphore <- struct{}{}:
	default:
		return nil, nil, fmt.Errorf("server busy: parser concurrency limit reached (%d), try again shortly", maxConcurrentParses)
	}

	var astNode *ast.AST
	var diags []diagnostics.Diagnostic
	if err := util.RunGuarded(util.DefaultParseTimeout, func() error {
		// El release va acá adentro, no en un defer a nivel de parseSource —
		// mismo razonamiento que slidelang/internal/mcp/parse.go: si
		// RunGuarded vence el timeout, esta closure (y su goroutine
		// detached) sigue corriendo hasta terminar de verdad; liberar el
		// slot solo cuando ESTA closure retorna hace que el semáforo
		// refleje los parses que siguen realmente en vuelo.
		defer func() { <-parseSemaphore }()

		// La normalización corre dentro del constructor (no en Parse()), así
		// que el guard debe cubrir ambos — ver el mismo comentario en
		// doclang/internal/cli/build.go.
		docParser := parser.NewDocumentFlexParserWithNormalization(source, logger)
		astNode, diags = docParser.Parse()
		return nil
	}); err != nil {
		return nil, nil, err
	}

	if astNode == nil || hasErrorDiagnostic(diags) {
		return astNode, diags, nil
	}

	// Etapa de transform built-in (issue #240/#239) — ver el comentario del
	// paquete arriba. Un error acá (p. ej. label duplicado o \ref sin
	// resolver, xref/numbering.go y xref/resolve.go) es un fallo de
	// contenido real, pero xref.Transform lo reporta como error de Go, no
	// como diagnóstico — se propaga como tal, igual que un error de guard.
	transformed, err := transform.RunBuiltins(astNode, []transform.Transform{xref.Transform})
	if err != nil {
		return astNode, diags, err
	}

	return transformed, diags, nil
}

// hasErrorDiagnostic indica si diags contiene al menos un diagnóstico de
// severidad error (a diferencia de warning/info).
func hasErrorDiagnostic(diags []diagnostics.Diagnostic) bool {
	for _, d := range diags {
		if d.IsError() {
			return true
		}
	}
	return false
}
