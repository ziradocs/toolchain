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
	"io"
	"os/exec"
	"reflect"
	"time"

	"go.ziradocs.com/core/v2/ast"
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
		if issues := ast.ValidateNodeIDs(doc); len(issues) != 0 {
			return nil, fmt.Errorf("built-in transform #%d: %s", i, issues[0].String())
		}
		if ast.UsesTableRows(doc) || doc.SchemaVersion == ast.SchemaVersion || len(doc.Capabilities) > 0 {
			if err := ast.ValidateTableContract(doc); err != nil {
				return nil, fmt.Errorf("built-in transform #%d: %w", i, err)
			}
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
		if ast.UsesTableRows(doc) {
			if err := ast.ValidateTableContract(doc); err != nil {
				return nil, fmt.Errorf("filter %q input: %w", path, err)
			}
			if err := ast.FilterTableIdentityReady(doc); err != nil {
				return nil, fmt.Errorf("filter %q: %w", path, err)
			}
			if err := negotiateTableRows(path, timeout); err != nil {
				return nil, fmt.Errorf("filter %q: %w", path, err)
			}
		}
		var err error
		before := doc
		doc, err = runExternalFilter(doc, path, timeout)
		if err != nil {
			return nil, fmt.Errorf("filter %q: %w", path, err)
		}
		if ast.UsesTableRows(before) {
			if !ast.UsesTableRows(doc) || !reflect.DeepEqual(ast.TableIdentityOwners(before), ast.TableIdentityOwners(doc)) {
				return nil, fmt.Errorf("filter %q: table row/cell identities or capability were removed or changed", path)
			}
		}
		if issues := ast.ValidateNodeIDs(doc); len(issues) != 0 {
			return nil, fmt.Errorf("filter %q: %s", path, issues[0].String())
		}
	}
	return doc, nil
}

// negotiateTableRows sends no AST bytes. This is a compatibility gate, not
// trust in a filter: decoded output and identity ownership are checked after
// the filter runs too. Legacy 2.14 documents never invoke this handshake.
func negotiateTableRows(path string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "--ziradocs-capabilities")
	cmd.Stdin = bytes.NewReader(nil)
	stdout, stderr := &limitedWriter{limit: 4096}, &limitedWriter{limit: 4096}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("tableRows capability handshake timed out")
		}
		return fmt.Errorf("tableRows capability handshake failed: %w (%s)", err, stderr.buf.String())
	}
	if stdout.exceeded || stderr.exceeded {
		return fmt.Errorf("tableRows capability handshake response exceeds 4096 bytes")
	}
	var response struct {
		ASTSchemaVersions []string `json:"astSchemaVersions"`
		Features          []string `json:"features"`
	}
	decoder := json.NewDecoder(bytes.NewReader(stdout.buf.Bytes()))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&response); err != nil {
		return fmt.Errorf("invalid tableRows capability response: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return fmt.Errorf("invalid trailing capability response")
	}
	if len(response.ASTSchemaVersions) != 1 || response.ASTSchemaVersions[0] != ast.SchemaVersion || len(response.Features) != 1 || response.Features[0] != ast.TableRowsCapability {
		return fmt.Errorf("filter does not support schemaVersion %s and %s", ast.SchemaVersion, ast.TableRowsCapability)
	}
	return nil
}

type limitedWriter struct {
	buf      bytes.Buffer
	limit    int
	exceeded bool
}

func (w *limitedWriter) Write(p []byte) (int, error) {
	if w.buf.Len()+len(p) > w.limit {
		w.exceeded = true
		return 0, fmt.Errorf("capability response exceeds %d bytes", w.limit)
	}
	return w.buf.Write(p)
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
