// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"fmt"
	"strings"

	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/internal/fence"
)

// LongerClosingFenceRuleID es el aviso de una cerca de tres backticks que
// cierra con una línea de más backticks.
const LongerClosingFenceRuleID = "CODE002"

// FenceFallbackRuleID es el aviso de una cerca que no se lee con la regla de
// CommonMark porque no daría un resultado bien formado: cierra donde cerraba
// en las versiones anteriores, no cierra, o abre con un backtick en el info
// string (fence.ClosedByMain, fence.Unclosed, fence.OpenedByMain).
const FenceFallbackRuleID = "CODE003"

// FenceExtentChangedRuleID es el aviso de una cerca de CommonMark que termina
// en otra línea que en las versiones anteriores, o de una línea que las
// versiones anteriores leían como una cerca que cerraba y ahora es texto (ver
// fenceDiagnostics y TextFenceDiagnostics).
const FenceExtentChangedRuleID = "CODE004"

// fenceAt es la cerca de un bloque de código en ctx.Lines[i]: fence.AtWithin
// acotada a su contenedor (ctx.Limit), o, con ctx.EarlierFences, la regla de
// las versiones anteriores (fence.EarlierAt).
func fenceAt(ctx *ParseContext, i int) (fence.Block, bool) {
	return fenceFor(ctx, i, oldCodeClose)
}

// fenceFor es fenceAt para un parser cuya regla de cierre en las versiones
// anteriores era old (oldCodeClose u oldDiagramClose).
func fenceFor(ctx *ParseContext, i int, old oldFenceRule) (fence.Block, bool) {
	if ctx.EarlierFences {
		return fence.EarlierAt(ctx.Lines, i, old)
	}
	return ctx.fenceIndex().AtWithin(i, ctx.limitFrom(i))
}

// tabAt es fenceAt para una pestaña de un :::code-group (fence.TabWithin).
func tabAt(ctx *ParseContext, i int) (fence.Block, bool) {
	if ctx.EarlierFences {
		return fence.EarlierAt(ctx.Lines, i, oldCodeClose)
	}
	return ctx.fenceIndex().TabWithin(i, ctx.limitFrom(i))
}

// opensFence dice si ctx.Lines[i] abre una cerca de código, con la misma
// lectura que el parser de código (fenceAt). Lo usan los cortes de prosa y de
// listas para no tragarse una cerca que el parser sí va a leer.
func opensFence(ctx *ParseContext, i int) bool {
	_, ok := fenceAt(ctx, i)
	return ok
}

// startsFence es opensFence para una línea que empieza con ```: las que
// no, nunca abren una cerca.
func startsFence(ctx *ParseContext, i int, trimmed string) bool {
	return strings.HasPrefix(trimmed, "```") && opensFence(ctx, i)
}

// oldFenceRule dice cómo cerraba una cerca la regla de su parser en las
// versiones anteriores: un bloque de código, solo en una ``` exacta; una
// cerca de diagrama o de datos (mermaid, plantuml, chart, map), en ``` o en
// ````.
type oldFenceRule func(trimmed string) bool

func oldCodeClose(trimmed string) bool { return trimmed == "```" }

func oldDiagramClose(trimmed string) bool { return trimmed == "```" || trimmed == "````" }

// fenceDiagnostics avisa, si hace falta, cómo se leyó la cerca b: con la regla
// de las versiones anteriores (CODE003) o con la de CommonMark pero con otra
// extensión que antes (CODE002 si una cerca de tres cierra en una línea de más
// backticks, CODE004 si no). old es la regla de cierre que tenía su parser, y
// tab dice que b es una pestaña de un :::code-group.
//
// CODE004 sale si la línea que cierra la cerca con CommonMark era, para las
// versiones anteriores, contenido de una cerca que cerraban, o si esas
// versiones leían el resto del documento, desde la cerca, sin dejar ninguna
// abierta. Si no se cumple ninguna de las dos, la leían mal y la de
// CommonMark es la que el autor escribió: una cerca de cuatro que guarda un
// ```, que cerraban en el ``` de adentro, y cuya línea de cierre les abría
// otra cerca hasta el final.
func fenceDiagnostics(ctx *ParseContext, b fence.Block, old oldFenceRule, tab bool) []diagnostics.Diagnostic {
	open := ctx.Position(b.Start)
	warn := func(rule string, pos diagnostics.Position, format string, args ...any) []diagnostics.Diagnostic {
		return []diagnostics.Diagnostic{diagnostics.NewWarning(fmt.Sprintf(format, args...), pos, "code-parser").WithRuleID(rule)}
	}
	switch b.Reading {
	case fence.Earlier:
		return nil
	case fence.ClosedByMain:
		return warn(FenceFallbackRuleID, open,
			"This fence opens with %d backticks and no later line of at least %d backticks closes it, so it closes at the next \"```\" line, as in earlier versions. Close it with a line of %d backticks.",
			b.Ticks, b.Ticks, b.Ticks)
	case fence.Unclosed:
		return warn(FenceFallbackRuleID, open,
			"This fence opens with %d backticks and is never closed, so it runs to the end of its content. Close it with a line of %d backticks.",
			b.Ticks, b.Ticks)
	case fence.OpenedByMain:
		if tab {
			return warn(FenceFallbackRuleID, open,
				"This line has a backtick in its info string, which makes it text in CommonMark; inside a :::code-group it opens a tab, as in earlier versions. Remove the backtick from the info string.")
		}
		return warn(FenceFallbackRuleID, open,
			"This line has a backtick in its info string, which makes it text in CommonMark; it is read as a fence, as in earlier versions, because as text the lines after it would leave a fence unclosed. Remove the backtick from the info string.")
	}
	// Las versiones anteriores no acotaban una cerca a su contenedor: su
	// lectura se compara en todo el documento.
	oldEnd := -1
	for j := b.Start + 1; j < len(ctx.Lines); j++ {
		if old(strings.TrimSpace(ctx.Lines[j])) {
			oldEnd = j
			break
		}
	}
	if b.End == oldEnd {
		return nil
	}
	closingLine := strings.TrimSpace(ctx.Lines[b.End])
	if b.Ticks == 3 && !old(closingLine) {
		return warn(LongerClosingFenceRuleID, ctx.Position(b.End),
			"This line of %d backticks closes the fence opened with 3; earlier versions kept it as content. To keep it as content, open the fence with more backticks than it has.",
			len(closingLine))
	}
	// Avisa si la línea de cierre era, para las versiones anteriores,
	// contenido de una cerca que cerraban, o si esas versiones leían el resto
	// del documento, desde esta cerca, sin dejar ninguna abierta. Si no, la
	// leían mal (una cerca de cuatro que guarda un ```: la cerraban en el ```
	// de adentro y su línea de cierre les abría otra cerca hasta el final) y
	// la de CommonMark es la que el autor escribió.
	idx := ctx.fenceIndex()
	heldBefore := oldEnd >= 0 && (b.End <= oldEnd || idx.MainHolds(oldEnd+1, b.End))
	if !heldBefore && idx.MainLeavesUnclosed(b.Start, len(ctx.Lines)) {
		return nil
	}
	return warn(FenceExtentChangedRuleID, open,
		"This fence opens with %d backticks and closes at the next line of at least %d backticks, as in CommonMark; earlier versions closed it at a different line. Check that it ends where intended.",
		b.Ticks, b.Ticks)
}

// TextFenceDiagnostics es el CODE004 de cada línea de ctx.Lines que empieza
// con ``` pero no abre una cerca (un backtick en el info string, ver
// fence.Walk) y que no cae dentro de otra cerca, cuando las versiones
// anteriores la leían como una cerca que cerraba en una ``` posterior: ahora
// es texto. Lo llaman los parsers
// flex de documento y de presentación al terminar, sobre todas sus líneas,
// porque una línea así puede terminar en cualquier elemento (un párrafo, un
// item, el contenido de un :::, una pestaña de code-group).
func TextFenceDiagnostics(ctx *ParseContext) []diagnostics.Diagnostic {
	if ctx.EarlierFences {
		return nil
	}
	var out []diagnostics.Diagnostic
	idx := ctx.fenceIndex()
	inFence := idx.Lines(0)
	for i, line := range ctx.Lines {
		if inFence[i] || !strings.HasPrefix(strings.TrimSpace(line), "```") {
			continue
		}
		// Si las versiones anteriores no la cerraban, la leían como una
		// cerca hasta el final: esa lectura no era un documento del que
		// alguien dependiera.
		if !idx.HasExactClose(i) {
			continue
		}
		out = append(out, diagnostics.NewWarning(
			"This line starts like a fence but has a backtick in its info string, so it is text, as in CommonMark; earlier versions read it as a fence. To keep it as a fence, remove the backtick from the info string.",
			ctx.Position(i), "code-parser").WithRuleID(FenceExtentChangedRuleID))
	}
	return out
}

// CanParseAt es CanParse con el contexto: en flex, una línea abre un bloque de
// código si fence.AtWithin la lee como cerca, también una con un backtick en
// el info string que se lee como en las versiones anteriores
// (fence.OpenedByMain).
func (p *CodeParser) CanParseAt(ctx *ParseContext, i int) bool {
	if ctx.Mode == "flex" {
		return opensFence(ctx, i)
	}
	return p.CanParse(ctx.Lines[i], ctx.Mode)
}

// canParseAt pregunta a parser si puede leer ctx.Lines[i]: con el contexto si
// lo acepta (CanParseAt, para lo que necesita ver las líneas siguientes, como
// una cerca de código) o con la línea sola.
func canParseAt(parser ElementParser, ctx *ParseContext, i int) bool {
	if at, ok := parser.(interface {
		CanParseAt(ctx *ParseContext, i int) bool
	}); ok {
		return at.CanParseAt(ctx, i)
	}
	return parser.CanParse(ctx.Lines[i], ctx.Mode)
}
