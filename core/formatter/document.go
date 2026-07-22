// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"fmt"
	"regexp"
	"strings"

	"go.ziradocs.com/core/v2/ast"
)

// FormatDocument serializa doc a la forma canónica de DocLang: `# título`
// por sección (blockType "title" en Heading, "content" en Title — espejo
// de parser.DocumentFlexParser.parseSection), y elementos en su sintaxis
// flex (Markdown): "- item"/"1. item" para POINTS, fenced ``` para CODE,
// "![alt](src)" para IMAGE, "> cita" para QUOTE, "- [x] item" para
// CHECKLIST. DocLang no tiene modo strict — parser.DocumentFlexParser
// SIEMPRE usa el dialecto flex (ver CLAUDE.md) — así que a diferencia de
// FormatStrict no hay que filtrar por "mode:", pero el resto del contrato
// es el mismo: determinista, idempotente, y solo cubre lo que el parser
// realmente puede producir.
//
// Los elementos de bloque (:::especial, :::code-group, <<mermaid>>,
// <<plantuml>>, <<chart:...>>, <<map>>, @directivas) usan EXACTAMENTE la
// misma sintaxis en flex que en strict — confirmado leyendo CanParse() de
// cada parser modular, ninguno rama por ctx.Mode más allá de TEXT/POINTS/
// CODE/IMAGE/QUOTE/CHECKLIST — así que formatDocument reusa los
// serializadores compartidos (formatSpecialBlock, formatCodeGroup,
// formatMermaid, formatPlantUML, formatChart, formatMap, formatDirective)
// en vez de duplicarlos.
func FormatDocument(doc *ast.AST) (string, error) {
	var b strings.Builder

	var fm string
	if doc.FrontMatter != nil {
		var err error
		fm, err = formatFrontMatter(doc.FrontMatter, frontMatterOverrides(doc.FrontMatter, ""))
		if err != nil {
			return "", err
		}
	}
	b.WriteString(fm)

	hasTitleBlock := false
	for i, block := range doc.ContentBlocks {
		if i > 0 || fm != "" {
			b.WriteString("\n")
		}
		blockText, err := formatDocumentSection(&block, &hasTitleBlock)
		if err != nil {
			return "", err
		}
		b.WriteString(blockText)
	}

	return b.String(), nil
}

// formatDocumentSection espeja parser.DocumentFlexParser.parseSection: el
// PRIMER bloque del documento se parsea como blockType "title" (su texto
// va a Heading); todos los siguientes son "content" (texto en Title). El
// formatter no puede leer BlockType para decidir esto de forma confiable
// (el AST puede haberse construido de otras formas), así que sigue la
// MISMA regla posicional que el parser: primer bloque → Heading.
func formatDocumentSection(block *ast.ContentBlock, hasTitleBlock *bool) (string, error) {
	var b strings.Builder

	title := block.Title
	if !*hasTitleBlock {
		title = block.Heading
		*hasTitleBlock = true
	}
	fmt.Fprintf(&b, "# %s\n", title)

	// Separador en blanco entre cada elemento: DocumentFlexParser corre por
	// defecto con normalización AI habilitada (NewDocumentFlexParserWithNormalization),
	// cuyas heurísticas de estructura reclasifican un "## heading" pegado
	// sin línea en blanco al elemento anterior (p. ej. justo después de un
	// "<<end>>" de chart) como texto en negrita en vez de subsection header
	// — confirmado empíricamente con examples/dimensions_test.doclang. El
	// corpus real de DocLang siempre separa elementos con línea en blanco
	// (ver el mismo archivo); replicar esa convención evita el gap.
	for _, el := range block.Elements {
		elText, err := formatDocumentElement(el)
		if err != nil {
			return "", err
		}
		if elText == "" {
			continue
		}
		b.WriteString("\n")
		if isSubsectionHeadingElement(el) {
			// rules/content/headers.go (el normalizer AI que
			// NewDocumentFlexParserWithNormalization corre por defecto) trata
			// "##" como el título de un slide nuevo SOLO justo después de un
			// separador "---", y demueve a "**negrita**" cualquier "##"
			// posterior dentro del mismo bloque lógico — una heurística
			// pensada para slidelang (cada "---" = un slide) que malinterpreta
			// subsection headers legítimos de DocLang. El corpus real (p. ej.
			// examples/dimensions_test.doclang) siempre antepone "---" a cada
			// "##"/"###" — replicar esa convención rodea el bug sin tener que
			// tocar el normalizer compartido (fuera del scope de fmt).
			b.WriteString("---\n\n")
		}
		b.WriteString(elText)
	}

	return b.String(), nil
}

// isSubsectionHeadingElement reporta si el, si es un TextElement RawHTML,
// tiene forma de subsection header (<hN>...</hN>) — sin correr el regex
// completo, solo lo suficiente para decidir si hace falta el separador
// "---" defensivo contra rules/content/headers.go (ver comentario en el
// llamador).
func isSubsectionHeadingElement(el ast.Element) bool {
	t, ok := el.(*ast.TextElement)
	if !ok || !t.IsRawHTML {
		return false
	}
	return strings.HasPrefix(t.Content, "<h") && subsectionHeadingRe.MatchString(t.Content)
}

func formatDocumentElement(el ast.Element) (string, error) {
	var body string
	var err error

	switch e := el.(type) {
	case *ast.TextElement:
		if e.IsRawHTML {
			body, err = formatSubsectionHeading(e)
		} else {
			body = e.Content
		}
	case *ast.PointsElement:
		body = formatPointItems(e.Items, e.ListType)
	case *ast.CodeElement:
		body = formatFlexCode(e)
	case *ast.ImageElement:
		body, err = formatFlexImage(e)
	case *ast.TableElement:
		if e.Caption != "" {
			err = newUnsupported("table", "table.Caption no es representable en el dialecto flex de DocLang (elements.TableParser solo parsea caption en su rama strict/TABLE-keyword, y DocLang nunca la usa)")
		} else {
			body = formatPipeTable(e.Headers, e.Rows)
		}
	case *ast.QuoteElement:
		body, err = formatFlexQuote(e)
	case *ast.ChecklistElement:
		body = formatFlexChecklist(e.Items)
	case *ast.SpecialBlockElement:
		body = formatSpecialBlock(e)
	case *ast.CodeGroupElement:
		body = formatCodeGroup(e)
	case *ast.MermaidElement:
		body = formatMermaid(e)
	case *ast.PlantUMLElement:
		body = formatPlantUML(e)
	case *ast.ChartElement:
		body, err = formatChart(e)
	case *ast.MapElement:
		body, err = formatMap(e)
	case *ast.DirectiveNode:
		body, err = formatDirective(e)
	case *ast.GridElement:
		err = newUnsupported(string(e.GetType()), "DocLang no tiene sintaxis de texto para GRID")
	default:
		err = newUnsupported(string(el.GetType()), "tipo de elemento no reconocido por el formatter de DocLang")
	}
	if err != nil {
		return "", err
	}
	if body == "" {
		return "", nil
	}
	// Ver el comentario equivalente en formatStrictElement: normaliza el
	// trailing "\n" para que el contenido multi-línea no deje una línea en
	// blanco fantasma antes del siguiente elemento.
	return strings.TrimRight(body, "\n") + "\n", nil
}

// subsectionHeadingRe extrae nivel + texto interno de "<h2 id=\"...\">...</h2>"
// — la forma exacta que parser.DocumentFlexParser.parseSubsectionHeader
// produce (ver parser/document_flex.go:348) para cada línea "##"/"###".
var subsectionHeadingRe = regexp.MustCompile(`(?s)^<h([1-6])(?: id="[^"]*")?>(.*)</h[1-6]>$`)

// formatSubsectionHeading reconstruye "## texto" desde el TextElement
// RawHTML que el parser produce para subsection headers. Es best-effort:
// parseSubsectionHeader ya corrió el texto original por
// ProcessInlineMarkdownSecureLine (escapa HTML y aplica **bold**/*italic*/
// `code` a tags reales) antes de guardarlo — esa transformación no es
// invertible en general (p. ej. "**bold**" y un "<strong>bold</strong>"
// tecleado a mano por el usuario producen el MISMO TextElement, así que no
// hay forma de saber cuál escribir de vuelta). Para headers de texto plano
// (el caso común) esto round-trip-ea exacto; para headers con
// formato inline Markdown, reconstruye la versión con las tags HTML
// crudas visibles en el texto en vez del Markdown original — un
// canonicalizador legítimo (ver "no verbatim" en el comentario de
// paquete), no un bug, pero documentado aquí porque es la única pérdida
// real de fidelidad de todo el formatter de DocLang.
func formatSubsectionHeading(e *ast.TextElement) (string, error) {
	m := subsectionHeadingRe.FindStringSubmatch(e.Content)
	if m == nil {
		// No es un header de subsección — es un TextElement RawHTML "genérico"
		// (contenido HTML explícito del autor). No hay sintaxis flex para
		// reinyectar HTML crudo de vuelta de forma que el parser lo re-detecte
		// como tal (el fallback de TEXT en flex no reconoce HTML).
		return "", newUnsupported("text", "TextElement con IsRawHTML=true que no es un subsection header <hN> no es representable en el dialecto flex de DocLang")
	}
	level := m[1]
	inner := stripTags(m[2])
	return strings.Repeat("#", int(level[0]-'0')) + " " + inner, nil
}

var tagRe = regexp.MustCompile(`<[^>]*>`)

func stripTags(s string) string {
	s = tagRe.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", "\"")
	s = strings.ReplaceAll(s, "&#39;", "'")
	return s
}

func formatFlexCode(e *ast.CodeElement) string {
	return "```" + e.Language + "\n" + e.Content + "\n```"
}

// formatFlexImage serializa "![alt](source)". No hay forma de portar
// Caption ni Context en la sintaxis Markdown de imagen — ambos se
// re-derivan en el parser a partir de heurísticas de posición
// (detectImageContext), no de texto explícito, así que Context nunca causa
// una pérdida real (se recalcula igual en el reparse); Caption sí, y se
// reporta en vez de perderse en silencio.
func formatFlexImage(e *ast.ImageElement) (string, error) {
	if e.Caption != "" {
		return "", newUnsupported("image", "image.Caption no es representable en la sintaxis Markdown de imagen de DocLang")
	}
	return fmt.Sprintf("![%s](%s)", e.Alt, e.Source), nil
}

// formatFlexQuote serializa "> línea" por línea, con "-- Autor" final si
// Author está poblado (ver QuoteParser.parseFlex). Source no tiene forma
// flex (solo el keyword SOURCE: existe en modo strict, que DocLang nunca
// usa) — se reporta en vez de perderse.
func formatFlexQuote(e *ast.QuoteElement) (string, error) {
	if e.Source != "" {
		return "", newUnsupported("quote", "quote.Source no es representable en el dialecto flex de DocLang (SOURCE: es sintaxis exclusiva de modo strict)")
	}
	var b strings.Builder
	for i, line := range strings.Split(e.Content, "\n") {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString("> " + line)
	}
	if e.Author != "" {
		fmt.Fprintf(&b, "\n-- %s", e.Author)
	}
	return b.String(), nil
}

func formatFlexChecklist(items []ast.ChecklistItem) string {
	var b strings.Builder
	for i, item := range items {
		if i > 0 {
			b.WriteString("\n")
		}
		mark := " "
		if item.Checked {
			mark = "x"
		}
		fmt.Fprintf(&b, "- [%s] %s", mark, item.Content)
		if len(item.SubItems) > 0 {
			b.WriteString("\n")
			b.WriteString(indent(formatFlexChecklist(item.SubItems), 2))
		}
	}
	return b.String()
}
