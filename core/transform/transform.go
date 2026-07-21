// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

// Package transform implementa la etapa de transformación del AST (issue
// #240, decisión C del plan OSS): un pase ordenado que corre entre parse y
// lint, formado por transforms BUILT-IN (registrados por core — p. ej. la
// numeración de refs cruzadas de #239) seguidos de FILTROS DE TERCEROS
// (--filter, procesos externos estilo Pandoc Lua-filters).
//
// # Por qué filtros de terceros = proceso externo, no una API Go
//
// core/doc.go es explícito: el único contrato de terceros
// versionado por semver es el AST serializado vía --format json
// (ast.SchemaVersion). No hay compromiso de estabilidad sobre ninguna firma
// de Go. Un filtro de terceros que importara este paquete y recibiera un
// *ast.AST en proceso estaría atado a un contrato Go inestable — en cambio,
// un filtro externo que habla JSON por stdin/stdout monta sobre el contrato
// que SÍ está versionado y es el que el ecosistema puede consumir sin
// nuestras herramientas.
//
// # Garantía de seguridad — por qué NO se usa el JSON "de salida"
//
// El AST que un filtro recibe y devuelve es el AST SEMÁNTICO CRUDO —serializado
// con encoding/json estándar (json.Marshal(doc)), NUNCA vía el camino que
// usa RenderASTJSON (BuildVariables + PopulateInlineHTML + serialize)—.
// Ese camino hornea los campos "*HTML" ya pre-renderizados y pre-sanitizados
// en el JSON; si un filtro los recibiera y los reenviara (o los mutara) sin
// pasar por el sanitizador, --filter se volvería una vía directa de
// inyección de HTML no sanitizado hacia --format json y el viewer — un
// bypass del gate de seguridad XSS del MVP. Por eso:
//  1. La etapa corre ANTES de PopulateInlineHTML (los *HTML de origen ya
//     están vacíos en este punto del pipeline).
//  2. Al decodificar la respuesta del subproceso (que igual podría rellenar
//     esos campos por su cuenta), RunFilters llama ast.ClearRenderedHTML
//     como defensa en profundidad — nunca se confía en el *HTML que devuelve
//     un proceso externo. *HTML se re-deriva después por el camino normal.
package transform

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"go.ziradocs.com/core/ast"
)

// Transform es la firma de un transform BUILT-IN: recibe el AST ya parseado
// (en el slot post-parse/pre-lint) y devuelve el AST transformado, o un
// error que aborta el build. Registrado por core (p. ej. el pase de
// numeración de #239), no por terceros — para terceros ver RunFilters.
type Transform func(*ast.AST) (*ast.AST, error)

// DefaultFilterTimeout es el presupuesto de tiempo por invocación de un
// filtro externo — mismo orden de magnitud que util.DefaultParseTimeout
// (30s), pero un valor propio: un filtro de terceros es un proceso
// completamente fuera de nuestro control, y ese es exactamente el caso que
// justifica no compartir la constante con el parser.
const DefaultFilterTimeout = 30 * time.Second

// RunBuiltins aplica cada Transform de builtins en orden, pasando la salida
// de uno como entrada del siguiente. Se detiene en el primer error.
func RunBuiltins(doc *ast.AST, builtins []Transform) (*ast.AST, error) {
	for i, t := range builtins {
		var err error
		doc, err = t(doc)
		if err != nil {
			return nil, fmt.Errorf("built-in transform #%d: %w", i, err)
		}
		if doc == nil {
			return nil, fmt.Errorf("built-in transform #%d devolvió un AST nil", i)
		}
	}
	return doc, nil
}

// RunFilters ejecuta cada binario en filterPaths, en orden, como subproceso:
// serializa doc a JSON crudo (SIN *HTML — ver el docstring del paquete),
// lo escribe al stdin del filtro, lee el AST transformado de su stdout, lo
// decodifica y blanquea cualquier *HTML que el filtro haya dejado antes de
// pasarlo al siguiente filtro o de devolverlo. Se detiene en el primer error
// (exit code no-cero, timeout, o JSON inválido) — el mensaje incluye stderr
// del filtro para diagnóstico.
func RunFilters(doc *ast.AST, filterPaths []string, timeout time.Duration) (*ast.AST, error) {
	for _, path := range filterPaths {
		var err error
		doc, err = runExternalFilter(doc, path, timeout)
		if err != nil {
			return nil, fmt.Errorf("filter %q: %w", path, err)
		}
	}
	return doc, nil
}

func runExternalFilter(doc *ast.AST, binaryPath string, timeout time.Duration) (*ast.AST, error) {
	// AST crudo: json.Marshal directo sobre *ast.AST, NUNCA RenderASTJSON —
	// ver el docstring del paquete. En este punto del pipeline (post-parse,
	// pre-PopulateInlineHTML) los campos *HTML del doc de entrada ya están
	// vacíos por construcción.
	originalFilePath := doc.FilePath
	input, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("serializing AST for filter: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binaryPath)
	cmd.Stdin = bytes.NewReader(input)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("timed out after %s", timeout)
		}
		return nil, fmt.Errorf("exited with error: %w (stderr: %s)", err, stderr.String())
	}

	decoded, err := ast.DecodeAST(stdout.Bytes())
	if err != nil {
		return nil, fmt.Errorf("decoding filter output: %w", err)
	}

	// Defensa en profundidad: nunca confiar en *HTML que devuelva un
	// subproceso, sin importar qué haya hecho el filtro.
	ast.ClearRenderedHTML(decoded)

	// FilePath no se serializa (json:"-"); preservarlo explícitamente, no es
	// responsabilidad del filtro reconstruirlo.
	decoded.FilePath = originalFilePath

	return decoded, nil
}
