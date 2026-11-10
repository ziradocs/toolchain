// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"fmt"
	"strings"

	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/internal/fence"
)

// FenceFallbackRuleID es el aviso de una cerca que no se lee con la regla de
// CommonMark porque no daría un resultado bien formado: cierra donde cerraba
// en las versiones anteriores, no cierra, o abre con un backtick en el info
// string (fence.ClosedByMain, fence.Unclosed, fence.OpenedByMain).
const FenceFallbackRuleID = "CODE003"

// FenceExtentChangedRuleID es el aviso de una cerca de CommonMark que termina
// en otra línea que en las versiones anteriores, o de una línea que las
// versiones anteriores leían como cerca y ahora es texto.
const FenceExtentChangedRuleID = "CODE004"

// fenceAt es fence.AtWithin sobre las líneas del contexto, acotada a su
// contenedor (ctx.Limit).
func fenceAt(ctx *ParseContext, i int) (fence.Block, bool) {
	return fence.AtWithin(ctx.Lines, i, ctx.limit())
}

// opensFence dice si ctx.Lines[i] abre una cerca de código, con la misma
// lectura que el parser de código (fenceAt). Lo usan los cortes de prosa y de
// listas para no tragarse una cerca que el parser sí va a leer.
func opensFence(ctx *ParseContext, i int) bool {
	_, ok := fenceAt(ctx, i)
	return ok
}

// fenceLine es la línea del archivo (1-based) de ctx.Lines[i].
func fenceLine(ctx *ParseContext, i int) int {
	return ctx.Position(i).Line
}

// fenceDiagnostics avisa, si hace falta, cómo se leyó la cerca b: con la regla
// de las versiones anteriores (CODE003) o con la de CommonMark pero con otra
// extensión que antes (CODE002 si cierra en una línea de más backticks,
// CODE004 si no).
func fenceDiagnostics(ctx *ParseContext, b fence.Block) []diagnostics.Diagnostic {
	open := ctx.Position(b.Start)
	warn := func(rule, msg string, pos diagnostics.Position) []diagnostics.Diagnostic {
		return []diagnostics.Diagnostic{diagnostics.NewWarning(msg, pos, "code-parser").WithRuleID(rule)}
	}
	switch b.Reading {
	case fence.ClosedByMain:
		return warn(FenceFallbackRuleID, fmt.Sprintf(
			"the code block opened with %d backticks is closed by a 3-backtick line (line %d), as in earlier versions; CommonMark would not close it there. Close it with %d backticks",
			b.Ticks, fenceLine(ctx, b.End), b.Ticks), open)
	case fence.Unclosed:
		return warn(FenceFallbackRuleID, fmt.Sprintf(
			"the code block opened with %d backticks is never closed and runs to the end of its content. Close it with a line of %d backticks",
			b.Ticks, b.Ticks), open)
	case fence.OpenedByMain:
		closed := "is never closed"
		if b.End >= 0 {
			closed = fmt.Sprintf("closes at line %d", fenceLine(ctx, b.End))
		}
		return warn(FenceFallbackRuleID, fmt.Sprintf(
			"this line has a backtick in its info string, so CommonMark reads it as text; it is read as a code block that %s, as in earlier versions, because as text the rest would leave a code block unclosed. Remove the backtick from the info string",
			closed), open)
	}
	if b.End == b.MainEnd {
		return nil
	}
	if closingLine := strings.TrimSpace(ctx.Lines[b.End]); b.Ticks == 3 && closingLine != "```" {
		return []diagnostics.Diagnostic{longerClosingFenceWarning(ctx, b.End, b.Ticks, closingLine)}
	}
	before := "never closed it"
	if b.MainEnd >= 0 {
		before = fmt.Sprintf("closed it at line %d", fenceLine(ctx, b.MainEnd))
	}
	return warn(FenceExtentChangedRuleID, fmt.Sprintf(
		"the code block opened with %d backticks closes at line %d; earlier versions %s",
		b.Ticks, fenceLine(ctx, b.End), before), open)
}

// TextFenceDiagnostics es el CODE004 de cada línea de ctx.Lines que empieza
// con ``` pero no abre una cerca (un backtick en el info string, fence.Walk) y
// que no cae dentro de otra cerca: las versiones anteriores la leían como una
// cerca y ahora es texto. Lo llaman los parsers flex de documento y de
// presentación al terminar, y ParseMarkdownContent, sobre todas sus líneas,
// porque una línea así puede terminar en cualquier elemento (un párrafo, un
// item, el contenido de un :::, una pestaña de code-group). No corre sobre
// el cuerpo de un bloque `{typed}` de un documento strict: haría falta
// acotarlo a ese cuerpo para no repetir avisos.
func TextFenceDiagnostics(ctx *ParseContext) []diagnostics.Diagnostic {
	var out []diagnostics.Diagnostic
	inFence := fence.Lines(ctx.Lines, 0)
	for i, line := range ctx.Lines {
		if inFence[i] || !strings.HasPrefix(strings.TrimSpace(line), "```") {
			continue
		}
		before := "a code block that was never closed"
		for j := i + 1; j < len(ctx.Lines); j++ {
			if strings.TrimSpace(ctx.Lines[j]) == "```" {
				before = fmt.Sprintf("a code block closed at line %d", fenceLine(ctx, j))
				break
			}
		}
		out = append(out, diagnostics.NewWarning(fmt.Sprintf(
			"this line starts like a code block but has a backtick in its info string, so it is text (CommonMark); earlier versions read it as %s",
			before), ctx.Position(i), "code-parser").WithRuleID(FenceExtentChangedRuleID))
	}
	return out
}

// fenceBody son las líneas del cuerpo de b, sin la apertura ni el cierre.
func fenceBody(lines []string, b fence.Block) []string {
	end := b.End
	if end < 0 {
		end = b.Limit
	}
	return lines[b.Start+1 : end]
}
