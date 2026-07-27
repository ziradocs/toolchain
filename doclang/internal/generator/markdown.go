// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/util"
)

// MarkdownGenerator genera documentos Markdown
type MarkdownGenerator struct {
	logger util.Logger
}

// NewMarkdownGenerator crea un nuevo generador Markdown
func NewMarkdownGenerator(log util.Logger) *MarkdownGenerator {
	return &MarkdownGenerator{
		logger: log,
	}
}

// Generate genera un documento Markdown
func (m *MarkdownGenerator) Generate(doc *ast.AST, outputFile string, opts GeneratorOptions) error {
	m.logger.Info("MARKDOWN", "Building Markdown document...")

	var md strings.Builder

	// Add frontmatter if present
	if doc.FrontMatter != nil {
		md.WriteString("---\n")
		if doc.FrontMatter.Title != "" {
			fmt.Fprintf(&md, "title: %s\n", doc.FrontMatter.Title)
		}
		if doc.FrontMatter.Author != "" {
			fmt.Fprintf(&md, "author: %s\n", doc.FrontMatter.Author)
		}
		if doc.FrontMatter.Date != "" {
			fmt.Fprintf(&md, "date: %s\n", doc.FrontMatter.Date)
		}
		md.WriteString("---\n\n")
	}

	// Add main title
	if doc.FrontMatter != nil && doc.FrontMatter.Title != "" {
		fmt.Fprintf(&md, "# %s\n\n", doc.FrontMatter.Title)
	}

	// Table of contents
	if opts.TOC {
		md.WriteString("## Table of Contents\n\n")
		sectionNum := 1
		for _, block := range doc.ContentBlocks {
			if block.Title != "" {
				anchor := strings.ToLower(strings.ReplaceAll(block.Title, " ", "-"))
				if opts.Numbering {
					fmt.Fprintf(&md, "- [%d. %s](#%s)\n", sectionNum, block.Title, anchor)
				} else {
					fmt.Fprintf(&md, "- [%s](#%s)\n", block.Title, anchor)
				}
				sectionNum++
			}
		}
		md.WriteString("\n")
	}

	// Document body
	sectionNum := 1
	for i, block := range doc.ContentBlocks {
		if block.Title != "" {
			if opts.Numbering {
				fmt.Fprintf(&md, "## %d. %s\n\n", sectionNum, block.Title)
			} else {
				fmt.Fprintf(&md, "## %s\n\n", block.Title)
			}
			sectionNum++
		}

		// Generate content for each element
		for _, element := range block.Elements {
			md.WriteString(m.renderElement(element))
			md.WriteString("\n")
		}

		// Page break marker (HTML comment)
		if opts.PageBreaks && i < len(doc.ContentBlocks)-1 {
			md.WriteString("\n---\n\n")
		}
	}

	// Write to file
	if err := os.WriteFile(outputFile, []byte(md.String()), 0644); err != nil {
		return fmt.Errorf("failed to write Markdown file: %w", err)
	}

	m.logger.Info("MARKDOWN", "Markdown document generated successfully")
	return nil
}

// renderElement convierte un elemento AST a Markdown
func (m *MarkdownGenerator) renderElement(element ast.Element) string {
	switch elem := element.(type) {
	case *ast.TextElement:
		// Level (issue #22) es la fuente de verdad cuando está poblado —
		// mismo criterio que docx.go's renderText: evita el acoplamiento
		// frágil de adivinar si Content es un heading mirando su forma.
		// Sin Level (Level == 0, un AST sin ese campo o un párrafo real),
		// Content se vuelca tal cual, comportamiento histórico preservado.
		if elem.Level > 0 {
			text := elem.Content
			anchor := ""
			if m := headingHTMLPattern.FindStringSubmatch(elem.Content); m != nil {
				text = m[1]
				// Preservar el id="..." del <hN> ya renderizado como un
				// anchor explícito (sintaxis {#id}, soportada por
				// Pandoc/kramdown): sin esto, un link en el mismo
				// documento como [ver](#mi-seccion) deja de resolver en
				// cuanto el auto-slug del renderer de Markdown destino
				// difiere del sanitizeAnchor que usó el parser (acentos,
				// emoji, puntuación se limpian distinto) o el auto-slug
				// está deshabilitado.
				if idm := headingIDPattern.FindStringSubmatch(elem.Content); idm != nil && idm[1] != "" {
					anchor = fmt.Sprintf(" {#%s}", idm[1])
				}
			} else if m := markdownHeadingPattern.FindStringSubmatch(elem.Content); m != nil {
				text = m[1]
			}
			return fmt.Sprintf("%s %s%s\n\n", strings.Repeat("#", elem.Level), text, anchor)
		}
		return elem.Content + "\n"

	case *ast.PointsElement:
		var md strings.Builder
		for i, item := range elem.Items {
			if elem.ListType == "ordered" {
				fmt.Fprintf(&md, "%d. %s\n", i+1, item.Content)
			} else {
				fmt.Fprintf(&md, "- %s\n", item.Content)
			}
		}
		return md.String()

	case *ast.CodeElement:
		return fmt.Sprintf("```%s\n%s\n```\n", elem.Language, elem.Content)

	case *ast.ImageElement:
		if elem.Caption != "" {
			return fmt.Sprintf("![%s](%s)\n*%s*\n", elem.Alt, elem.Source, elem.Caption)
		}
		return fmt.Sprintf("![%s](%s)\n", elem.Alt, elem.Source)

	case *ast.MediaElement:
		return renderMediaElementMarkdown(elem)

	case *ast.TableElement:
		var md strings.Builder

		// Headers. escapeMarkdownInline (hallazgo de code-review sobre PR
		// #55, la misma clase de bug que renderMapElementMarkdown más abajo
		// en este archivo): un header/cell con "|" sin escapar desalinea la
		// tabla (columna de más, discrepancia con la fila "| --- |"), y un
		// salto de línea la parte en filas nuevas e inesperadas.
		if len(elem.Headers) > 0 {
			md.WriteString("|")
			for _, header := range elem.Headers {
				fmt.Fprintf(&md, " %s |", escapeMarkdownInline(header))
			}
			md.WriteString("\n|")
			for range elem.Headers {
				md.WriteString(" --- |")
			}
			md.WriteString("\n")
		}

		// Rows
		for _, row := range elem.Rows {
			md.WriteString("|")
			for _, cell := range row {
				fmt.Fprintf(&md, " %s |", escapeMarkdownInline(cell))
			}
			md.WriteString("\n")
		}

		if elem.Caption != "" {
			fmt.Fprintf(&md, "\n*%s*\n", escapeMarkdownInline(elem.Caption))
		}

		return md.String()

	case *ast.QuoteElement:
		return fmt.Sprintf("> %s\n", elem.Content)

	case *ast.ChecklistElement:
		var md strings.Builder
		for _, item := range elem.Items {
			checked := " "
			if item.Checked {
				checked = "x"
			}
			fmt.Fprintf(&md, "- [%s] %s\n", checked, item.Content)
		}
		return md.String()

	case *ast.MermaidElement:
		return fmt.Sprintf("```mermaid\n%s\n```\n", elem.Content)

	case *ast.PlantUMLElement:
		// Same treatment as MermaidElement above: a ```plantuml fence.
		// GitLab and Kroki-backed renderers turn it into the real diagram;
		// renderers that don't still show the source, which beats losing it
		// (issue de cobertura descubierto en #38/#51 — antes caía al
		// default y desaparecía sin rastro).
		var md strings.Builder
		if elem.Title != "" {
			fmt.Fprintf(&md, "**%s**\n\n", elem.Title)
		}
		fmt.Fprintf(&md, "```plantuml\n%s\n```\n", elem.Content)
		return md.String()

	case *ast.MathElement:
		// Content ya es LaTeX crudo (core/ast/nodes.go). $$...$$ es la
		// convención de display math que GitHub, Pandoc y KaTeX/MathJax
		// entienden. Label/Number son el mecanismo de xref — si están
		// poblados, anteponen "Ecuación N" al caption igual que
		// ImageElement antepone "Figura N" (ver ese case arriba).
		var md strings.Builder
		fmt.Fprintf(&md, "$$\n%s\n$$\n", elem.Content)
		switch {
		case elem.Label != "" && elem.Number > 0 && elem.Caption != "":
			fmt.Fprintf(&md, "*Ecuación %d: %s*\n", elem.Number, elem.Caption)
		case elem.Label != "" && elem.Number > 0:
			fmt.Fprintf(&md, "*Ecuación %d*\n", elem.Number)
		case elem.Caption != "":
			fmt.Fprintf(&md, "*%s*\n", elem.Caption)
		}
		return md.String()

	case *ast.CodeGroupElement:
		// Markdown no tiene tabs: emitir los N bloques secuencialmente
		// conserva todo el contenido (hoy se pierden los N, cae al
		// default). Mismo fallback de label que docx.go's renderCodeGroup:
		// Label -> Language -> "Code N".
		var md strings.Builder
		for i, block := range elem.CodeBlocks {
			label := block.Label
			if label == "" {
				label = block.Language
			}
			if label == "" {
				label = fmt.Sprintf("Code %d", i+1)
			}
			fmt.Fprintf(&md, "**%s**\n\n", label)
			fmt.Fprintf(&md, "```%s\n%s\n```\n\n", block.Language, block.Content)
		}
		return md.String()

	case *ast.MapElement:
		return renderMapElementMarkdown(elem)

	case *ast.ChartElement:
		// Represent chart as code block
		return fmt.Sprintf("```chart:%s\n[Chart data would be here]\n```\n", elem.ChartType)

	case *ast.DirectiveNode:
		// Una @directiva (@notes, @timer, …) es metadata de autoría de
		// slidelang: en un documento no hay vista de presentador donde
		// mostrarla, así que no se renderiza. Pero SÍ se avisa —y con el
		// nombre real y la línea—, a diferencia del default genérico que
		// decía "Unknown element type" (falso: el tipo se conoce
		// perfectamente) y no le daba al autor ninguna pista de qué pasó
		// con su contenido.
		m.logger.Warn("MARKDOWN: la directiva @%s (línea %d) no tiene efecto en un documento y se omite; usá un blockquote si querés que su contenido se vea",
			elem.Name, elem.GetPosition().Line)
		return ""

	case *ast.SpecialBlockElement:
		var md strings.Builder
		fmt.Fprintf(&md, "> **%s: %s**\n", strings.ToUpper(elem.BlockType), elem.Title)
		fmt.Fprintf(&md, "> %s\n", elem.Content)
		return md.String()

	case *ast.GridElement:
		// No native grid equivalent in Markdown: each column is rendered as
		// its own section, separated by a divider (issue #56).
		var md strings.Builder
		if elem.Content != "" {
			md.WriteString(elem.Content + "\n\n")
		}
		for i, column := range elem.Columns {
			if i > 0 {
				md.WriteString("\n---\n\n")
			}
			md.WriteString(column.Content + "\n")
		}
		return md.String()

	default:
		m.logger.Warn("MARKDOWN: Unknown element type: %T", element)
		return ""
	}
}

// renderMediaElementMarkdown degrades a MediaElement to a link (issue #36) —
// Markdown has no native <video>/<audio> equivalent, unlike doclang's HTML
// output (core/renderer's renderMediaElement, which this mirrors). Same
// security rule as core, with one difference: Source goes through
// renderer.ValidateURLScheme, NOT SanitizeURL. SanitizeURL layers
// EscapeHTMLAttribute on top of the scheme allowlist — correct for an HTML
// attribute, wrong here, since both the Markdown link target and its
// visible text are not HTML; that escaping only leaves literal `&amp;`
// entities in any URL with a query string. An empty source is reported
// distinctly from a source ValidateURLScheme blocked — an empty src is
// missing data, not a rejected dangerous scheme, and conflating the two
// would mislead the author into thinking something was blocked when
// nothing was ever provided.
func renderMediaElementMarkdown(elem *ast.MediaElement) string {
	label := "video"
	icon := "🎬"
	if elem.MediaType == "audio" {
		label = "audio"
		icon = "🎵"
	}

	source := strings.TrimSpace(elem.Source)
	if source == "" {
		return fmt.Sprintf("*[%s sin fuente]*\n", label)
	}
	safeSource := renderer.ValidateURLScheme(source)
	if safeSource == "" {
		return fmt.Sprintf("*[%s bloqueado por seguridad]*\n", label)
	}
	return fmt.Sprintf("[%s %s: %s](%s)\n", icon, label, safeSource, safeSource)
}

// newlineRun matches one or more consecutive line-break characters. Usado
// SOLO por normalizeMarkdownLine — a diferencia de un strings.Fields/Join
// (que colapsa CUALQUIER corrida de whitespace, incluyendo espacios
// consecutivos deliberados o un NBSP U+00A0), esto toca únicamente saltos de
// línea, dejando el resto del contenido de autor intacto (hallazgo de
// code-review sobre PR #55: strings.Fields mutaba más de lo que el nombre
// de la función prometía).
var newlineRun = regexp.MustCompile(`[\r\n]+`)

// normalizeMarkdownLine colapsa cualquier corrida de saltos de línea a un
// solo espacio, para valores de autor que van interpolados en una sola
// línea de Markdown (no en una tabla) — sin esto, un Title/MapType con un
// "\n\n# Heading" incrustado podría partir la línea e inyectar estructura
// Markdown inesperada.
func normalizeMarkdownLine(s string) string {
	return newlineRun.ReplaceAllString(s, " ")
}

// mdInlineEscaper escapa los metacaracteres de Markdown que romperían la
// sintaxis alrededor de donde se interpola un valor de autor: \, *, _, `,
// [, ] (emphasis/link/code-span) y | (estructura de tabla). Un solo
// strings.Replacer —sustitución SIMULTÁNEA de una sola pasada sobre el
// string original, no llamadas encadenadas a ReplaceAll— para que un "\"
// del contenido de entrada nunca vuelva a matchear la regla de "\" que la
// sustitución de OTRO carácter (p. ej. "|" → "\|") acaba de introducir; ese
// es justo el riesgo de doble-escapado que un orden manual de ReplaceAll
// tendría que evitar a mano.
var mdInlineEscaper = strings.NewReplacer(
	`\`, `\\`,
	"*", `\*`,
	"_", `\_`,
	"`", "\\`",
	"[", `\[`,
	"]", `\]`,
	"|", `\|`,
)

// escapeMarkdownInline escapa metacaracteres de Markdown Y normaliza saltos
// de línea, para valores de autor interpolados dentro de sintaxis Markdown
// que ya existe alrededor (una celda de tabla, o el wrapper *[...]* de
// renderMapElementMarkdown) — hallazgo de code-review sobre PR #55: sin
// esto, "|" desalinea una tabla (columna de más o menos), "]"/"*" cierran el
// wrapper de énfasis/corchetes antes de tiempo, y un salto de línea parte la
// fila/línea donde se interpola el valor.
func escapeMarkdownInline(s string) string {
	return normalizeMarkdownLine(mdInlineEscaper.Replace(s))
}

// renderMapElementMarkdown degrades a MapElement to its data (issue #38/#51
// coverage gap) — an interactive map has no Markdown equivalent, but its
// markers ARE expressable and are what the author actually wrote; losing
// them entirely (the previous default: behavior) is worse than degrading to
// a table, same rationale as renderMediaElementMarkdown above.
func renderMapElementMarkdown(elem *ast.MapElement) string {
	var md strings.Builder
	// escapeMarkdownInline (no solo normalizeMarkdownLine): Title/MapType
	// se interpolan dentro del wrapper *[mapa: ... — ...]* — sin escapar
	// "]"/"*" un valor como `Zona ]* **PRECIO OCULTO**` cerraría ese
	// wrapper antes de tiempo e inyectaría texto en negrita fuera de él
	// (hallazgo de code-review sobre PR #55).
	mapType := escapeMarkdownInline(elem.MapType)
	if elem.Title != "" {
		fmt.Fprintf(&md, "*[mapa: %s — %s]*\n", mapType, escapeMarkdownInline(elem.Title))
	} else {
		fmt.Fprintf(&md, "*[mapa: %s]*\n", mapType)
	}

	if len(elem.Markers) == 0 {
		return md.String()
	}

	md.WriteString("\n| Label | Lat | Lng | Value |\n")
	md.WriteString("| --- | --- | --- | --- |\n")
	for _, marker := range elem.Markers {
		fmt.Fprintf(&md, "| %s | %g | %g | %g |\n", escapeMarkdownInline(marker.Label), marker.Lat, marker.Lng, marker.Value)
	}
	return md.String()
}
