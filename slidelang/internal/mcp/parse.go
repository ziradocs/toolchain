// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

// Package mcp implementa el servidor MCP (Model Context Protocol) de
// slidelang, expuesto vía `slidelang mcp`. Cada tool es un wrapper delgado
// sobre el mismo pipeline que usa `slidelang build` (parser/linter/generator
// de core y slidelang/internal/generator) — sin lógica de
// parseo/render propia — para que un agente reciba exactamente los mismos
// diagnósticos y el mismo AST que el CLI, sin round-trips por archivos.
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

// defaultFileName se usa cuando el caller no provee uno — solo afecta el
// texto de los diagnósticos y el nombre implícito que el AST expone en
// FilePath, ningún tool escribe a disco.
const defaultFileName = "input.slidelang"

// maxConcurrentParses acota cuántas llamadas a parseSource pueden estar en
// vuelo a la vez. core no soporta cancelación de parseo: cuando
// util.RunGuarded vence el timeout, la goroutine del parse sigue corriendo
// en segundo plano indefinidamente (documentado en util.RunWithTimeout).
// build.go tolera esto porque el proceso del CLI termina poco después; el
// servidor MCP es un proceso stdio de larga vida, y el SDK despacha cada
// tool call de forma concurrente — sin este límite, un cliente enviando
// repetidamente contenido que agota el timeout acumularía una goroutine
// detached por llamada durante toda la vida del proceso. Este semáforo no
// elimina la fuga (eso requeriría que el parser soporte context.Context real
// — issue de seguimiento), pero acota la tasa máxima a la que puede crecer.
const maxConcurrentParses = 4

var parseSemaphore = make(chan struct{}, maxConcurrentParses)

// parseSource parsea source con los mismos dos guards que usa `slidelang
// build` (slidelang/internal/cli/build.go): un cap de tamaño
// (util.CheckInputSize/DefaultMaxInputBytes) ANTES de tocar el parser —la
// defensa primaria contra la amplificación del normalizer AI y loops
// patológicos, ver docs/SECURITY_AUDIT_2026-07.md ME-8— y el timeout
// (util.RunGuarded/DefaultParseTimeout) como backstop de defensa en
// profundidad. El contenido de un tool call MCP es tan no confiable como un
// archivo de entrada de línea de comandos (issue #45, fuzzing encontró
// cuelgues reales del parser en el pasado) — sin el cap de tamaño, un
// cliente MCP podía enviar un `source` sin límite (el CLI sí lo tenía vía
// --max-size/SLIDELANG_MAX_SIZE).
func parseSource(logger util.Logger, source, fileName string) (*ast.AST, []diagnostics.Diagnostic, error) {
	if fileName == "" {
		fileName = defaultFileName
	}

	if err := util.CheckInputSize(len(source), util.DefaultMaxInputBytes); err != nil {
		return nil, nil, err
	}

	// Adquisición no bloqueante: si un parse queda REALMENTE colgado (no solo
	// vence el timeout, sino que nunca retorna — un loop infinito genuino),
	// su slot nunca se libera. Un `<-` incondicional acá significaría que
	// después de maxConcurrentParses parses genuinamente colgados, TODA
	// llamada subsiguiente se bloquearía para siempre esperando un slot que
	// nunca se libera — convirtiendo la fuga lenta y acotada que este
	// semáforo intenta mitigar en una parálisis total del servidor a partir
	// del (maxConcurrentParses+1)-ésimo request. Con `select`+`default`
	// fallamos rápido y devolvemos un error claro en vez de colgar al
	// caller indefinidamente; el servidor sigue respondiendo (con "busy")
	// a nuevos requests aunque los slots estén agotados.
	select {
	case parseSemaphore <- struct{}{}:
	default:
		return nil, nil, fmt.Errorf("server busy: parser concurrency limit reached (%d), try again shortly", maxConcurrentParses)
	}

	p := parser.New(logger)

	var astNode *ast.AST
	var diags []diagnostics.Diagnostic
	if err := util.RunGuarded(util.DefaultParseTimeout, func() error {
		// El release va acá adentro, no en un defer a nivel de parseSource:
		// RunGuarded retorna en cuanto vence el timeout, pero esta closure
		// (y la goroutine detached que la corre) sigue ejecutando p.Parse
		// hasta que de verdad termine. Liberar el slot recién cuando ESTA
		// closure retorna hace que el semáforo refleje cuántos parses siguen
		// realmente corriendo, incluidos los detached — liberar en un defer
		// de parseSource liberaría el slot en el momento del timeout, sin
		// acotar nada de lo que el hallazgo señala.
		defer func() { <-parseSemaphore }()
		astNode, diags = p.Parse(source, fileName)
		return nil
	}); err != nil {
		return nil, nil, err
	}

	if astNode == nil || hasErrorDiagnostic(diags) {
		return astNode, diags, nil
	}

	// Etapa de transform built-in (issue #240/#239) — la misma que corre
	// `slidelang build` entre el parseo y el lint (build.go:402). Sin esto,
	// este parseSource queda parse-only y get_ast/preview divergen de lo que
	// el propio comentario de este paquete promete ("el mismo AST que el
	// CLI"): un doc con figuras/tablas/ecuaciones etiquetadas y \ref
	// aparecería SIN numerar y con los \ref sin resolver. Ese era el estado
	// real hasta ahora — este tool se creó el 2026-07-14 (#133), 5 días
	// antes de que #239/#240 introdujeran esta etapa en build.go
	// (2026-07-19): quedó parse-only por orden cronológico, no por diseño.
	// No corre los --filter de usuario (binarios externos configurados por
	// el operador, no tienen lugar en una tool call MCP) — mismo gap
	// deliberado que doclang/internal/mcp/parse.go.
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
