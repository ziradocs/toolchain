// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"regexp"
	"strings"

	"go.ziradocs.com/core/v2/a11y"
	"go.ziradocs.com/core/v2/ast"
)

// langSpanHTMLPattern extracts <span lang="xx">...</span> already
// materialized in a TextElement's raw HTML Content (IsRawHTML=true) — the
// case where the source [texto]{lang=xx} was consumed at PARSE time
// (core/parser/document_flex.go's subsection-heading path, which runs the
// full inline pipeline including the lang-span pass before PopulateLangRuns
// ever sees the raw markdown). Non-greedy: does not correctly separate a
// lang span nested inside another span (e.g. a lang span wrapping a
// class-styled span, reachable via [[x]{.danger}]{lang=fr}) — a rare
// combination inside a heading specifically, left as a known v1 limitation
// rather than growing this into a general HTML parser.
var langSpanHTMLPattern = regexp.MustCompile(`<span lang="([a-zA-Z0-9-]+)">(.*?)</span>`)

// LangSpanHTMLToSource inverts the HTML a lang span materializes to back
// into its [texto]{lang=xx} source syntax (issue #63 code review, finding
// #3): the exact reverse of the <span lang="xx"> that
// ProcessInlineMarkdownFormatsSecure emits for InlineLangSpanPattern.
// Exported for formatter.FormatDocument (a subsection heading's Content is
// already-rendered HTML by the time it reaches the formatter — see
// formatSubsectionHeading) and for --format markdown in doclang/slidelang,
// which faces the identical already-rendered-HTML problem for body prose.
//
// A tag that fails a11y.IsValidLangTag is left as literal HTML rather than
// round-tripped — mirrors PopulateLangRuns's rule that an untrusted-looking
// span (e.g. forged by an external --filter) never gets treated as evidence
// of a real one; the caller's own tag-stripping, if any, is what ends up
// discarding it.
//
// text is also left as literal HTML if it contains "[", "]", or "\n":
// InlineLangSpanPattern's own content class is [^\[\]\n]+ (see that
// pattern's doc comment), so emitting "[" + text + "]{lang=...}" with any of
// those characters inside would produce source that does NOT re-match on
// the next parse — silently dropping the language mark on a reparse, the
// exact defect this function exists to fix. Not reachable from the
// parse-time heading path today (that source can't contain a literal "["
// inside the span to begin with), but this function is also called on
// arbitrary already-rendered body prose, where it is reachable.
func LangSpanHTMLToSource(html string) string {
	return langSpanHTMLPattern.ReplaceAllStringFunc(html, func(match string) string {
		m := langSpanHTMLPattern.FindStringSubmatch(match)
		tag, text := m[1], m[2]
		if !a11y.IsValidLangTag(tag) || strings.ContainsAny(text, "[]\n") {
			return match
		}
		return "[" + text + "]{lang=" + tag + "}"
	})
}

// PopulateLangRuns fills LangRuns on every prose-carrying element reachable
// from doc, deriving it FRESH from each element's own Content every time —
// see LangRun's doc comment for why. Unlike PopulateInlineHTML, this is not
// purely additive: it OVERWRITES LangRuns (never appends, never skips an
// element that already looks populated), because skipping is exactly the
// bypass a hostile --filter would need: forge LangRuns directly, rely on
// this function leaving a "populated-looking" field alone. There is nothing
// here for ast.ClearRenderedHTML to clear (see that function's own doc
// comment on *HTML fields) because there is nothing here to trust from
// upstream in the first place — every call to this function replaces
// LangRuns with what Content, read right now, actually says.
//
// variables is substituted into Markdown Content BEFORE extraction (same
// order as ProcessTextWithVariablesAndMarkdownSecure, see that function),
// so a span reaching this element only via a {{variable}} — e.g.
// {{saludo}} where variables["saludo"] is "[bonjour]{lang=fr}" — is still
// found. It has no effect on the RawHTML path: a heading's Content was
// already materialized to markdown-applied HTML at parse time
// (parser.parseSubsectionHeader calls ProcessInlineMarkdownSecureLine, which
// does NOT substitute {{variables}} — headings never get variable
// substitution at all, a pre-existing behavior unrelated to this function),
// so there is no later point where variables could still apply.
func PopulateLangRuns(doc *ast.AST, variables map[string]interface{}) {
	if doc == nil {
		return
	}
	for i := range doc.ContentBlocks {
		populateContentBlockLangRuns(&doc.ContentBlocks[i], variables)
	}
}

// populateContentBlockLangRuns deliberately does not touch
// block.Title/Heading/Subtitle: those three go through
// renderer.ProcessVariablesSecure only (see populate_inline_html.go's
// TitleHTML/HeadingHTML/SubtitleHTML cases), never through the markdown
// pipeline — a [texto]{lang=xx} typed into one of them is never converted to
// a span in the first place, so there is nothing here for PopulateLangRuns
// to extract. Not an oversight (issue #63 code review raised this as a
// possible fifth gap; it isn't one).
func populateContentBlockLangRuns(block *ast.ContentBlock, variables map[string]interface{}) {
	for _, elem := range block.Elements {
		populateElementLangRuns(elem, variables)
	}
}

// populateElementLangRuns covers every prose carrier reachable from a
// ContentBlock: TextElement, PointItem, ChecklistItem, QuoteElement.Content,
// SpecialBlockElement.Content, GridElement.Content and ColumnElement.Content
// (see the field-by-field scoping note on each of those LangRuns fields for
// why sibling fields like Title/Author are excluded) — table cells remain
// out of scope for #63's v1. GridElement/ColumnElement additionally recurse
// into Columns/Elements to reach any of the other carriers nested inside a
// grid's columns.
//
// Deliberately NOT guarded by a coverage test the way populateElementHTML
// is: LangRuns is a NEW FIELD on an EXISTING type, and every type-coverage
// guard in this repo (verified while researching #63) compares Element
// TYPES against switch cases — none inspects struct fields — so a missing
// case here would fail silently regardless of a guard. The scope above is
// intentional and documented, not something CI can currently enforce.
func populateElementLangRuns(element ast.Element, variables map[string]interface{}) {
	switch elem := element.(type) {
	case *ast.TextElement:
		elem.LangRuns, elem.DiscardedLangRuns = extractLangRuns(elem.Content, elem.IsRawHTML, variables)

	case *ast.PointsElement:
		for i := range elem.Items {
			populatePointItemLangRuns(&elem.Items[i], variables)
		}

	case *ast.QuoteElement:
		elem.LangRuns, elem.DiscardedLangRuns = extractLangRuns(elem.Content, false, variables)

	case *ast.ChecklistElement:
		for i := range elem.Items {
			populateChecklistItemLangRuns(&elem.Items[i], variables)
		}

	case *ast.SpecialBlockElement:
		elem.LangRuns, elem.DiscardedLangRuns = extractLangRuns(elem.Content, false, variables)

	case *ast.GridElement:
		elem.LangRuns, elem.DiscardedLangRuns = extractLangRuns(elem.Content, false, variables)
		for i := range elem.Columns {
			populateColumnLangRuns(&elem.Columns[i], variables)
		}
	}
}

func populateColumnLangRuns(col *ast.ColumnElement, variables map[string]interface{}) {
	col.LangRuns, col.DiscardedLangRuns = extractLangRuns(col.Content, false, variables)
	for _, nested := range col.Elements {
		populateElementLangRuns(nested, variables)
	}
}

func populatePointItemLangRuns(item *ast.PointItem, variables map[string]interface{}) {
	item.LangRuns, item.DiscardedLangRuns = extractLangRuns(item.Content, false, variables)
	for i := range item.SubPoints {
		populatePointItemLangRuns(&item.SubPoints[i], variables)
	}
}

func populateChecklistItemLangRuns(item *ast.ChecklistItem, variables map[string]interface{}) {
	item.LangRuns, item.DiscardedLangRuns = extractLangRuns(item.Content, false, variables)
	for i := range item.SubItems {
		populateChecklistItemLangRuns(&item.SubItems[i], variables)
	}
}

// extractLangRuns deliberately derives LangRuns from raw Content, never from
// the pre-rendered *HTML fields — so it doesn't matter which one of
// PopulateLangRuns/PopulateInlineHTML a caller runs first. In the build CLIs
// (doclang/internal/cli/build.go, slidelang/internal/cli/build.go),
// PopulateLangRuns runs before the linter on purpose (the linter needs
// LangRuns already populated) and before PopulateInlineHTML too, so the
// *HTML fields genuinely are still empty there. But RenderASTJSON in both
// generator packages (the MCP-server path) calls PopulateInlineHTML first —
// there the *HTML fields ARE already populated when this runs, and
// extractLangRuns still ignores them, reading Content instead.
//
// isRawHTML selects which source shape to scan: false for ordinary Markdown
// Content (the [texto]{lang=xx} span, matched via InlineLangSpanPattern —
// shared with ProcessInlineMarkdownFormatsSecure so the two can never drift
// on what a valid span looks like); true for a TextElement whose Content was
// already materialized to HTML at parse time (## headings), where the span
// already became a literal <span lang="xx"> by the time this runs.
//
// Every extracted tag is re-validated against a11y.IsValidLangTag regardless
// of source shape — an element flowing through an external --filter could
// set IsRawHTML on any TextElement and forge a <span lang> of its own, so
// trusting the RawHTML path without validation would reopen exactly the
// bypass PopulateLangRuns's "always re-derive, never skip" design exists to
// close.
// extractLangRuns's second return value, discarded, is every span whose tag
// failed a11y.IsValidLangTag — populated into DiscardedLangRuns so a
// consumer (a WCAG 3.1.2 linter rule, a formatter round-tripping content)
// can see that an author's language mark existed and didn't take, instead of
// having to re-derive that fact by hand-rolling its own copy of this same
// extraction (issue #92). A span suppressed for a DIFFERENT reason — e.g.
// crossesCode's "this span crosses a code boundary, the HTML pipeline never
// emits a <span lang> here either" — is not a discard and never appears
// here; only IsValidLangTag failures do.
func extractLangRuns(content string, isRawHTML bool, variables map[string]interface{}) (runs, discarded []ast.LangRun) {
	if isRawHTML {
		return extractLangRunsFromHTML(content)
	}
	return extractLangRunsFromMarkdown(content, variables)
}

func extractLangRunsFromMarkdown(content string, variables map[string]interface{}) (runs, discarded []ast.LangRun) {
	// ProcessVariables primero, igual que ProcessTextWithVariablesAndMarkdownSecure
	// — un span que solo existe tras sustituir una {{variable}} debe seguir
	// siendo encontrado (issue #63 code review, hallazgo #10).
	content = ProcessVariables(content, variables)
	// Excluir spans de idioma que caen DENTRO de un `código` (issue #63 code
	// review, hallazgo #7 — advisor follow-up): desde el fix de #7,
	// ProcessInlineMarkdownFormatsSecure protege código antes que cualquier
	// otra pasada, así que "`[a]{lang=fr}`" ya NO se convierte en
	// <span lang="fr">, queda <code>[a]{lang=fr}</code> literal. Sin esto,
	// LangRuns reportaría un run que el HTML nunca produjo — la misma
	// divergencia HTML/LangRuns que el contrato de esta función existe para
	// evitar (ver LangRun's doc comment).
	//
	// NO se puede lograr borrando los code spans de `content` antes de
	// matchear: eso también borra Texto de un span de idioma que CONTIENE un
	// code span (`[texto con \`code\`]{lang=fr}`), violando el contrato de
	// que Text es el texto verbatim del span. En vez de eso, se matchea
	// sobre el `content` SIN modificar y se descarta cualquier match del
	// span de idioma cuyo rango CRUCE la frontera de un code span (advisor
	// follow-up #2): ni contención simple del match dentro del código
	// (dejaría pasar el cruce parcial "[a`b]{lang=fr}`c", donde el code
	// span de sanitizer.go se traga "]{lang=fr}" completo — el pipeline
	// jamás emite <span lang> ahí) ni un overlap ingenuo (rechazaría
	// también el caso legítimo de un code span anidado DENTRO del texto del
	// span de idioma). Un match se acepta solo si, para cada code range, o
	// bien es disjunto de él, o bien el code range vive enteramente DENTRO
	// del match (anidamiento válido) — se rechaza en cualquier otro cruce.
	//
	// matches primero, codeRanges después (code-review de esta misma PR,
	// hallazgo confirmado): la inmensa mayoría de los elementos de prosa de
	// un documento no tienen NINGÚN span de idioma, así que calcular
	// codeRanges (otro barrido completo con regex) es trabajo tirado en el
	// caso común — se pospone hasta saber que hace falta.
	matches := InlineLangSpanPattern.FindAllStringSubmatchIndex(content, -1)
	if len(matches) == 0 {
		return nil, nil
	}
	// codeRanges se calcula LÍNEA POR LÍNEA, no sobre `content` completo
	// (code-review de esta misma PR, hallazgo confirmado): todo llamador
	// real del pipeline HTML (ProcessInlineMarkdownSecure/Multiline, en
	// sanitizer.go) parte el contenido por "\n" ANTES de proteger código,
	// así que un backtick jamás empareja cruzando un salto de línea en el
	// HTML real. Si esta función buscara backticks sobre `content` sin
	// partir, "[^`]+" (que sí matchea "\n") podría emparejar un backtick
	// suelto de una línea con uno no relacionado de una línea posterior,
	// fabricando un rango de código que el pipeline real jamás produce, y
	// rechazando de más un span de idioma legítimo. Los offsets se
	// recalculan por línea para que sigan siendo válidos contra `content`.
	var codeRanges [][2]int
	offset := 0
	for _, line := range strings.Split(content, "\n") {
		for _, r := range inlineCodePattern.FindAllStringIndex(line, -1) {
			codeRanges = append(codeRanges, [2]int{r[0] + offset, r[1] + offset})
		}
		offset += len(line) + 1 // +1 por el "\n" que Split consumió
	}
	// Dos punteros, no O(matches × codeRanges) (code-review de esta misma
	// PR, hallazgo confirmado): matches (de FindAllStringSubmatchIndex) y
	// codeRanges (construido línea por línea con offset creciente) están
	// AMBOS ordenados por posición de inicio. `ri` solo avanza, nunca
	// retrocede: una vez que un rango de código termina antes de que
	// empiece el match ACTUAL, ningún match POSTERIOR (que empieza en una
	// posición igual o mayor) puede solaparlo tampoco, así que es seguro
	// descartarlo para siempre. Esto asume que crossesCode se llama en el
	// mismo orden creciente en que aparecen `matches` — ver el for de abajo.
	ri := 0
	crossesCode := func(start, end int) bool {
		for ri < len(codeRanges) && codeRanges[ri][1] <= start {
			ri++
		}
		for j := ri; j < len(codeRanges) && codeRanges[j][0] < end; j++ {
			r := codeRanges[j]
			if start <= r[0] && end >= r[1] {
				continue // el code span vive DENTRO del match: se conserva
			}
			return true
		}
		return false
	}
	for _, m := range matches {
		if crossesCode(m[0], m[1]) {
			// Supresión correcta, no un descarte (issue #92): el pipeline HTML
			// tampoco emite <span lang> para un span que cruza un code span, así
			// que esto NO debe aparecer en DiscardedLangRuns — DiscardedLangRuns
			// es para tags que SÍ tenían intención de ser un run pero fallaron
			// validación, no para spans que el pipeline nunca considera válidos.
			continue
		}
		text, tag := content[m[2]:m[3]], content[m[4]:m[5]]
		if !a11y.IsValidLangTag(tag) {
			discarded = append(discarded, ast.LangRun{Text: text, Lang: tag})
			continue
		}
		runs = append(runs, ast.LangRun{Text: text, Lang: tag})
	}
	return runs, discarded
}

func extractLangRunsFromHTML(content string) (runs, discarded []ast.LangRun) {
	matches := langSpanHTMLPattern.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil, nil
	}
	for _, m := range matches {
		tag, text := m[1], m[2]
		// stripHTML + UnescapeHTML: Text es plain text en las dos rutas de
		// extracción (ver el doc comment de LangRun) — sin stripHTML, un span
		// que envuelve markup ya renderizado (p.ej. <span lang="fr">a
		// <strong>b</strong> c</span>, producido por [a *b* c]{lang=fr} en
		// una ruta que ya pasó por el pipeline inline) filtraría tags HTML
		// crudos a Text; sin UnescapeHTML, un "&" o "<" literal del autor
		// (ya escapado por EscapeHTML al materializarse el heading) quedaría
		// como "&amp;"/"&lt;" en vez del carácter real, divergiendo de la
		// ruta Markdown (que nunca escapa Content). Se aplica por igual al
		// texto de un run válido y al de uno descartado (issue #92): los dos
		// paths deben normalizar Text de la misma forma o reportarían textos
		// distintos para el mismo input según terminen en LangRuns o en
		// DiscardedLangRuns.
		run := ast.LangRun{Text: UnescapeHTML(stripHTML(text)), Lang: tag}
		if !a11y.IsValidLangTag(tag) {
			discarded = append(discarded, run)
			continue
		}
		runs = append(runs, run)
	}
	return runs, discarded
}
