// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"fmt"
	"strings"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// EarlierFencesRuleID es el aviso de un documento cuyas cercas de código se
// leen con la regla de las versiones anteriores porque con la de CommonMark
// (internal/fence) tiene un error que con esa regla no tiene.
const EarlierFencesRuleID = "CODE005"

// forceEarlierFences, solo para pruebas, lee siempre con la regla anterior.
var forceEarlierFences = false

// withEarlierFences parsea con parse leyendo las cercas con CommonMark
// (internal/fence) y, si el resultado tiene un error y el documento tiene
// alguna línea que empieza con ```, lo parsea otra vez con la regla de las
// versiones anteriores (elements.ParseContext.EarlierFences). Si esa lectura
// no tiene ningún error, la devuelve con un aviso CODE005 en la línea del
// primer error: un documento que compilaba no deja de compilar porque una
// cerca ahora cierra en otra línea (una cerca de cuatro backticks que guarda
// un ```, o una de tres que cierra con una línea de cuatro, deja a la vista
// líneas que antes eran código, o al revés). Si las dos lecturas tienen
// errores, queda la de CommonMark.
func withEarlierFences(content string, parse func(earlier bool) (*ast.AST, []diagnostics.Diagnostic)) (*ast.AST, []diagnostics.Diagnostic) {
	if forceEarlierFences {
		return parse(true)
	}
	doc, diags := parse(false)
	first := firstError(diags)
	if first == nil || !hasFenceLine(content) {
		return doc, diags
	}
	earlierDoc, earlierDiags := parse(true)
	if earlierDoc == nil || firstError(earlierDiags) != nil {
		return doc, diags
	}
	warning := diagnostics.NewWarning(fmt.Sprintf(
		"The code fences of this document are read as in earlier versions: read with CommonMark, this line has an error (%s), and read as before the document has none. A fence of four or more backticks closes at a line of at least as many, and one of three closes at any line of three or more; fix the fences so that the document has no error when read that way.",
		first.Message), first.Position, "code-parser").WithRuleID(EarlierFencesRuleID)
	return earlierDoc, append(earlierDiags, warning)
}

// firstError es el primer diagnóstico de error de diags, o nil.
func firstError(diags []diagnostics.Diagnostic) *diagnostics.Diagnostic {
	for i := range diags {
		if diags[i].IsError() {
			return &diags[i]
		}
	}
	return nil
}

// hasFenceLine dice si content tiene alguna línea que empieza con ```: sin
// ninguna, las dos reglas leen igual.
func hasFenceLine(content string) bool {
	for line := range strings.SplitSeq(content, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			return true
		}
	}
	return false
}
