// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

// Package modecheck rechaza `mode: strict` en documentos DocLang mientras el
// dialecto strict documental no exista.
//
// El campo `mode:` lo valida el FrontMatterParser compartido de core, que
// acepta "strict" como valor legal porque SlideLang sí lo implementa
// (parser.StrictParser, gramática SLIDE). DocLang, en cambio, parsea SIEMPRE
// con DocumentFlexParser: hasta ahora un `.doclang` que declaraba
// `mode: strict` se construía en silencio como flex —normalizador incluido—
// y encima el linter le disparaba STRICT001/002 (core/linter/rules.go,
// gateada en Mode=="strict") sobre un AST que nunca pasó por un parser
// strict, con mensajes que hablan de "slides".
//
// Ese estado híbrido es justo lo que el modo strict promete no hacer:
// reinterpretar el documento sin avisar. Por eso el diagnóstico es Error y
// no Warning — un artefacto declarado strict que se construye como flex es
// un artefacto que no se puede auditar, y fallar es la única respuesta
// honesta hasta que exista el parser documental strict.
//
// El chequeo vive en doclang y no en core deliberadamente: es temporal, y
// desaparece completo (paquete y llamadas) en cuanto core exponga el
// dispatch documental por modo.
package modecheck

import (
	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// RuleID identifica el diagnóstico. Namespace propio de doclang: los códigos
// FRONT00x son de core (el FrontMatterParser compartido) y no deben crecer
// desde acá.
const RuleID = "MODE001"

// diagnosticSource etiqueta el origen del diagnóstico, en línea con los
// "parser"/"linter" que ya usa core.
const diagnosticSource = "doclang"

// message es el texto del diagnóstico. Dice qué pasó, por qué se rechaza y
// cuál es la salida, sin prometer fechas.
const message = "Invalid mode for a DocLang document: 'strict' is a SlideLang-only dialect (SLIDE blocks) " +
	"and DocLang has no strict parser yet, so this file would be parsed as flex — silently, and with " +
	"normalization applied. Use 'mode: flex' (or omit the key) instead."

// Check devuelve el diagnóstico de rechazo si fm declara modo strict, o nil
// en cualquier otro caso (incluido fm == nil: un documento sin frontmatter
// nunca puede haber declarado un modo).
//
// La posición es (2,1) —la primera línea del YAML— igual que los FRONT001/
// FRONT002 que emite el FrontMatterParser para este mismo campo.
func Check(fm *ast.FrontMatterNode) []diagnostics.Diagnostic {
	if fm == nil || fm.Mode != "strict" {
		return nil
	}
	return []diagnostics.Diagnostic{
		diagnostics.NewError(message, diagnostics.NewPosition(2, 1), diagnosticSource).WithRuleID(RuleID),
	}
}

// CheckAST es el envoltorio para los call sites que tienen el AST completo y
// no el frontmatter suelto. Tolera doc == nil: un parseo que falló antes de
// producir AST ya trae sus propios errores.
func CheckAST(doc *ast.AST) []diagnostics.Diagnostic {
	if doc == nil {
		return nil
	}
	return Check(doc.FrontMatter)
}
