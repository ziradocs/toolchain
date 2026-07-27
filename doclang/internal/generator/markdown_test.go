// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

func TestMarkdownGeneratorGenerate(t *testing.T) {
	logger := newTestLogger()
	gen := NewMarkdownGenerator(logger)
	doc := newTestAST()

	first := doc.ContentBlocks[0]
	first.Elements = append(first.Elements,
		ast.NewCodeElement(diagnostics.NewPosition(6, 1), "go", "fmt.Println(\"hi\")"),
	)
	doc.ContentBlocks[0] = first

	second := ast.NewContentBlock(diagnostics.NewPosition(10, 1), "content")
	second.Title = "Details"
	image := ast.NewImageElement(diagnostics.NewPosition(11, 1), "image.png", "Sample image")
	image.Caption = "Figure 1"
	second.Elements = append(second.Elements, image)

	table := ast.NewTableElement(diagnostics.NewPosition(12, 1))
	table.Headers = []string{"Col1", "Col2"}
	table.Rows = [][]string{{"A", "B"}}
	table.Caption = "Sample table"
	second.Elements = append(second.Elements, table)

	checklist := ast.NewChecklistElement(diagnostics.NewPosition(13, 1))
	checklist.Items = append(checklist.Items,
		*ast.NewChecklistItem(diagnostics.NewPosition(13, 1), "Done", true),
		*ast.NewChecklistItem(diagnostics.NewPosition(14, 1), "Todo", false),
	)
	second.Elements = append(second.Elements, checklist)

	quote := ast.NewQuoteElement(diagnostics.NewPosition(15, 1), "Inspiring words")
	second.Elements = append(second.Elements, quote)

	special := ast.NewSpecialBlockElement(diagnostics.NewPosition(16, 1), "info", "Details")
	special.Title = "Heads up"
	second.Elements = append(second.Elements, special)

	mermaid := ast.NewMermaidElement(diagnostics.NewPosition(17, 1), "graph", "graph TD; A-->B")
	second.Elements = append(second.Elements, mermaid)

	chart := ast.NewChartElement(diagnostics.NewPosition(18, 1), "bar")
	chart.Title = "Sales"
	second.Elements = append(second.Elements, chart)

	doc.ContentBlocks = append(doc.ContentBlocks, *second)

	output := filepath.Join(t.TempDir(), "document.md")
	opts := GeneratorOptions{TOC: true, Numbering: true, PageBreaks: true}
	if err := gen.Generate(doc, output, opts); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}
	content := string(data)

	checks := []string{
		"title: Sample Document",
		"## Table of Contents",
		"- [1. Introduction](#introduction)",
		"## 1. Introduction",
		"```go",
		"![Sample image](image.png)",
		"| Col1 | Col2 |",
		"- [x] Done",
		"> Inspiring words",
		"> **INFO: Heads up**",
		"```mermaid",
		"```chart:bar",
		"\n---\n",
	}
	for _, expected := range checks {
		if !strings.Contains(content, expected) {
			t.Fatalf("generated Markdown missing %q\ncontent:\n%s", expected, content)
		}
	}

	if len(logger.infos) == 0 || !strings.Contains(logger.infos[len(logger.infos)-1], "Markdown document generated successfully") {
		t.Fatalf("expected success log message, got %#v", logger.infos)
	}
}

// TestMarkdownRenderElement_Table_CellAndHeaderWithPipeIsEscaped covers the
// code-review finding that escapeMarkdownInline (added for map markers) was
// never applied to TableElement's own Headers/Rows in the same file, so an
// ordinary table cell containing "|" still desynced the column count from
// the "| --- |" separator row.
func TestMarkdownRenderElement_Table_CellAndHeaderWithPipeIsEscaped(t *testing.T) {
	logger := newTestLogger()
	gen := NewMarkdownGenerator(logger)

	table := ast.NewTableElement(diagnostics.NewPosition(1, 1))
	table.Headers = []string{"Región | Zona", "Total"}
	table.Rows = [][]string{{"Norte | almacén", "42"}}
	table.Caption = "Ver *nota* al pie"

	out := gen.renderElement(table)

	if !strings.Contains(out, `Región \| Zona`) {
		t.Errorf("expected the pipe in the header to be escaped, got:\n%s", out)
	}
	if !strings.Contains(out, `Norte \| almacén`) {
		t.Errorf("expected the pipe in the cell to be escaped, got:\n%s", out)
	}
	if !strings.Contains(out, `Ver \*nota\* al pie`) {
		t.Errorf("expected the asterisks in the caption to be escaped, got:\n%s", out)
	}
}

// TestMarkdownRenderElement_Table_CellWithNewlineIsNormalized: an embedded
// newline in a cell would otherwise split one table row into malformed
// extra rows — same failure mode as the map marker table.
func TestMarkdownRenderElement_Table_CellWithNewlineIsNormalized(t *testing.T) {
	logger := newTestLogger()
	gen := NewMarkdownGenerator(logger)

	table := ast.NewTableElement(diagnostics.NewPosition(1, 1))
	table.Headers = []string{"Col"}
	table.Rows = [][]string{{"Line1\n| Fake | Row |"}}

	out := gen.renderElement(table)

	dataRows := 0
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "| Line1") {
			dataRows++
		}
	}
	if dataRows != 1 {
		t.Errorf("expected the cell to stay on exactly one table row, got %d:\n%s", dataRows, out)
	}
}

func TestMarkdownRenderElementVariants(t *testing.T) {
	logger := newTestLogger()
	gen := NewMarkdownGenerator(logger)

	cases := []struct {
		name     string
		element  ast.Element
		contains string
	}{
		{
			name:     "text",
			element:  ast.NewTextElement(diagnostics.NewPosition(1, 1), "Hello"),
			contains: "Hello",
		},
		{
			name: "ordered points",
			element: func() ast.Element {
				pts := ast.NewPointsElement(diagnostics.NewPosition(1, 1))
				pts.ListType = "ordered"
				pts.Items = append(pts.Items, *ast.NewPointItem(diagnostics.NewPosition(1, 1), "First"))
				return pts
			}(),
			contains: "1. First",
		},
		{
			name:     "code",
			element:  ast.NewCodeElement(diagnostics.NewPosition(1, 1), "js", "console.log(1)"),
			contains: "```js",
		},
		{
			name: "image caption",
			element: func() ast.Element {
				img := ast.NewImageElement(diagnostics.NewPosition(1, 1), "pic.png", "Alt")
				img.Caption = "Caption"
				return img
			}(),
			contains: "*Caption*",
		},
		{
			name: "checklist",
			element: func() ast.Element {
				cl := ast.NewChecklistElement(diagnostics.NewPosition(1, 1))
				cl.Items = append(cl.Items, *ast.NewChecklistItem(diagnostics.NewPosition(1, 1), "Item", true))
				return cl
			}(),
			contains: "- [x] Item",
		},
		{
			// ColumnElement is the fixture for the `default:` branch —
			// PlantUMLElement used to sit here before it got its own case
			// (issue #38/#51 coverage gap); ColumnElement is a sub-element
			// of GridElement.Columns that never appears bare in
			// block.Elements, so it reliably falls through (see
			// excludedFromElementCoverage in element_coverage_test.go).
			name:     "unsupported",
			element:  ast.NewColumnElement(diagnostics.NewPosition(1, 1), "orphan column"),
			contains: "",
		},
		{
			// Issue #36: MediaElement no tenía case y caía al default.
			name:     "media video",
			element:  ast.NewMediaElement(diagnostics.NewPosition(1, 1), "video", "demo.mp4"),
			contains: "[🎬 video: demo.mp4](demo.mp4)",
		},
		{
			name: "media audio",
			element: func() ast.Element {
				m := ast.NewMediaElement(diagnostics.NewPosition(1, 1), "audio", "song.mp3")
				return m
			}(),
			contains: "[🎵 audio: song.mp3](song.mp3)",
		},
		{
			name:     "media empty source",
			element:  ast.NewMediaElement(diagnostics.NewPosition(1, 1), "video", ""),
			contains: "*[video sin fuente]*",
		},
		{
			name:     "media blocked source",
			element:  ast.NewMediaElement(diagnostics.NewPosition(1, 1), "video", "javascript:alert(1)"),
			contains: "*[video bloqueado por seguridad]*",
		},
		{
			// Issue #38/#51: PlantUMLElement no tenía case y caía al default.
			name:     "plantuml",
			element:  ast.NewPlantUMLElement(diagnostics.NewPosition(1, 1), "sequence", "A->B"),
			contains: "```plantuml\nA->B\n```",
		},
		{
			// Issue #38/#51: MathElement no tenía case y caía al default.
			name:     "math",
			element:  ast.NewMathElement(diagnostics.NewPosition(1, 1), "E = mc^2"),
			contains: "$$\nE = mc^2\n$$",
		},
		{
			// Issue #38/#51: CodeGroupElement no tenía case y caía al default.
			name: "code group",
			element: func() ast.Element {
				cg := ast.NewCodeGroupElement(diagnostics.NewPosition(1, 1))
				cg.CodeBlocks = append(cg.CodeBlocks, ast.CodeBlock{Language: "go", Label: "Go", Content: "fmt.Println(1)"})
				cg.CodeBlocks = append(cg.CodeBlocks, ast.CodeBlock{Language: "js", Label: "JavaScript", Content: "console.log(1)"})
				return cg
			}(),
			contains: "fmt.Println(1)",
		},
		{
			// Issue #38/#51: MapElement no tenía case y caía al default.
			name: "map",
			element: func() ast.Element {
				m := ast.NewMapElement(diagnostics.NewPosition(1, 1), "world")
				m.Markers = append(m.Markers, ast.MapMarker{Label: "CDMX", Lat: 19.43, Lng: -99.13, Value: 42})
				return m
			}(),
			contains: "CDMX",
		},
		{
			// Issue #22: Level, cuando está poblado, debe emitir el heading
			// Markdown real (###) en vez de volcar el <h3 id="..."> crudo.
			name: "heading with Level",
			element: func() ast.Element {
				el := ast.NewRawHTMLTextElement(diagnostics.NewPosition(1, 1), `<h3 id="intro">Introducción</h3>`)
				el.Level = 3
				return el
			}(),
			contains: "### Introducción",
		},
		{
			// Code review de #40: Level>0 pero Content sigue en Markdown
			// crudo (un AST externo vía --filter, o antes de que corra la
			// normalización HTML) no matcheaba headingHTMLPattern, así que
			// el heading se duplicaba a "## ## Resumen". Ver el test
			// dedicado más abajo para la aserción negativa completa.
			name: "heading with Level and raw markdown content",
			element: func() ast.Element {
				el := ast.NewTextElement(diagnostics.NewPosition(1, 1), "## Resumen")
				el.Level = 2
				return el
			}(),
			contains: "## Resumen",
		},
		{
			// Code review de #40: SanitizeURL (escapador de atributo HTML)
			// usado para un link de Markdown escapaba el "&" de un query
			// string a "&amp;", mangling la URL. ValidateURLScheme no
			// escapa.
			name:     "media query string url not html-escaped",
			element:  ast.NewMediaElement(diagnostics.NewPosition(1, 1), "video", "https://cdn.example.com/v.mp4?a=1&b=2"),
			contains: "https://cdn.example.com/v.mp4?a=1&b=2",
		},
		{
			// Issue #56: GridElement no tenía case y caía al default (salida
			// vacía). Cada columna's Content debe aparecer en el Markdown.
			name: "grid",
			element: func() ast.Element {
				grid := ast.NewGridElement(diagnostics.NewPosition(1, 1))
				colA := ast.NewColumnElement(diagnostics.NewPosition(1, 1), "Contenido columna A")
				colB := ast.NewColumnElement(diagnostics.NewPosition(1, 1), "Contenido columna B")
				grid.Columns = append(grid.Columns, *colA, *colB)
				return grid
			}(),
			contains: "Contenido columna A",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := gen.renderElement(tc.element)
			if tc.contains != "" && !strings.Contains(out, tc.contains) {
				t.Fatalf("renderElement output %q does not contain %q", out, tc.contains)
			}
		})
	}

	if len(logger.warns) == 0 || !strings.Contains(logger.warns[0], "Unknown element type") {
		t.Fatalf("expected warning for unknown element, got %#v", logger.warns)
	}
}

// TestMarkdownRenderElement_Grid cubre issue #56: un GridElement con prosa
// suelta y varias columnas debe renderizar TODO ese contenido en Markdown
// (antes desaparecía por completo, sin ningún case en el switch).
func TestMarkdownRenderElement_Grid(t *testing.T) {
	logger := newTestLogger()
	gen := NewMarkdownGenerator(logger)

	grid := ast.NewGridElement(diagnostics.NewPosition(1, 1))
	grid.Content = "Prosa suelta del grid"
	colA := ast.NewColumnElement(diagnostics.NewPosition(1, 1), "Contenido columna A")
	colB := ast.NewColumnElement(diagnostics.NewPosition(1, 1), "Contenido columna B")
	grid.Columns = append(grid.Columns, *colA, *colB)

	out := gen.renderElement(grid)

	for _, expected := range []string{"Prosa suelta del grid", "Contenido columna A", "Contenido columna B"} {
		if !strings.Contains(out, expected) {
			t.Errorf("grid Markdown output missing %q:\n%s", expected, out)
		}
	}
}

// TestMarkdownRenderElement_PlantUML cubre issue #38/#51: un PlantUMLElement
// no tenía case en markdown.go y caía al default (salida vacía). El Title,
// si viene, se antepone en negrita antes del fence ```plantuml.
func TestMarkdownRenderElement_PlantUML(t *testing.T) {
	logger := newTestLogger()
	gen := NewMarkdownGenerator(logger)

	diagram := ast.NewPlantUMLElement(diagnostics.NewPosition(1, 1), "sequence", "A->B: hola")
	diagram.Title = "Flujo de saludo"

	out := gen.renderElement(diagram)

	for _, expected := range []string{"**Flujo de saludo**", "```plantuml", "A->B: hola", "```"} {
		if !strings.Contains(out, expected) {
			t.Errorf("plantuml Markdown output missing %q:\n%s", expected, out)
		}
	}
}

// TestMarkdownRenderElement_Math cubre issue #38/#51: un MathElement no
// tenía case en markdown.go y caía al default. Content ya es LaTeX crudo, así
// que se envuelve en $$...$$ (display math); con Label+Number poblados (el
// mecanismo de xref), antepone "Ecuación N" al caption, igual que
// ImageElement antepone "Figura N".
func TestMarkdownRenderElement_Math(t *testing.T) {
	logger := newTestLogger()
	gen := NewMarkdownGenerator(logger)

	eq := ast.NewMathElement(diagnostics.NewPosition(1, 1), "E = mc^2")
	eq.Label = "eq:einstein"
	eq.Number = 1
	eq.Caption = "Equivalencia masa-energía"

	out := gen.renderElement(eq)

	for _, expected := range []string{"$$", "E = mc^2", "Ecuación 1: Equivalencia masa-energía"} {
		if !strings.Contains(out, expected) {
			t.Errorf("math Markdown output missing %q:\n%s", expected, out)
		}
	}
}

// TestMarkdownRenderElement_CodeGroup cubre issue #38/#51: un
// CodeGroupElement no tenía case en markdown.go y caía al default. Markdown
// no tiene tabs, así que cada CodeBlock se emite secuencialmente — conserva
// TODO el contenido en vez de perder los N bloques.
func TestMarkdownRenderElement_CodeGroup(t *testing.T) {
	logger := newTestLogger()
	gen := NewMarkdownGenerator(logger)

	cg := ast.NewCodeGroupElement(diagnostics.NewPosition(1, 1))
	cg.CodeBlocks = append(cg.CodeBlocks, ast.CodeBlock{Language: "go", Label: "Go", Content: "fmt.Println(1)"})
	cg.CodeBlocks = append(cg.CodeBlocks, ast.CodeBlock{Language: "js", Label: "JavaScript", Content: "console.log(1)"})

	out := gen.renderElement(cg)

	for _, expected := range []string{"**Go**", "```go", "fmt.Println(1)", "**JavaScript**", "```js", "console.log(1)"} {
		if !strings.Contains(out, expected) {
			t.Errorf("code group Markdown output missing %q:\n%s", expected, out)
		}
	}
}

// TestMarkdownRenderElement_CodeGroup_LabelFallback cubre el fallback de
// etiqueta cuando el autor no puso Label: Language, y si tampoco hay
// Language, "Code N" — mismo fallback que docx.go's renderCodeGroup.
func TestMarkdownRenderElement_CodeGroup_LabelFallback(t *testing.T) {
	logger := newTestLogger()
	gen := NewMarkdownGenerator(logger)

	cg := ast.NewCodeGroupElement(diagnostics.NewPosition(1, 1))
	cg.CodeBlocks = append(cg.CodeBlocks, ast.CodeBlock{Content: "sin lenguaje ni label"})

	out := gen.renderElement(cg)

	if !strings.Contains(out, "**Code 1**") {
		t.Errorf("expected the positional fallback label \"Code 1\", got:\n%s", out)
	}
}

// TestMarkdownRenderElement_Map cubre issue #38/#51: un MapElement no tenía
// case en markdown.go y caía al default. Un mapa interactivo no existe en
// Markdown, pero sus marcadores sí son expresables — se degradan a una
// tabla en vez de perderse.
func TestMarkdownRenderElement_Map(t *testing.T) {
	logger := newTestLogger()
	gen := NewMarkdownGenerator(logger)

	m := ast.NewMapElement(diagnostics.NewPosition(1, 1), "world")
	m.Title = "Oficinas"
	m.Markers = append(m.Markers, ast.MapMarker{Label: "CDMX", Lat: 19.43, Lng: -99.13, Value: 42})

	out := gen.renderElement(m)

	for _, expected := range []string{"mapa: world", "Oficinas", "CDMX", "19.43", "-99.13", "42"} {
		if !strings.Contains(out, expected) {
			t.Errorf("map Markdown output missing %q:\n%s", expected, out)
		}
	}
}

// TestMarkdownRenderElement_Map_NoMarkers cubre el caso sin marcadores: la
// línea degradada debe seguir apareciendo, sin una tabla vacía colgando.
func TestMarkdownRenderElement_Map_NoMarkers(t *testing.T) {
	logger := newTestLogger()
	gen := NewMarkdownGenerator(logger)

	m := ast.NewMapElement(diagnostics.NewPosition(1, 1), "world")

	out := gen.renderElement(m)

	if !strings.Contains(out, "mapa: world") {
		t.Errorf("expected the degraded map line, got:\n%s", out)
	}
	if strings.Contains(out, "|") {
		t.Errorf("expected no table when there are no markers, got:\n%s", out)
	}
}

// TestMarkdownRenderElement_Map_LabelWithPipeIsEscaped covers the
// code-review finding on PR #55: an unescaped "|" in marker.Label adds an
// extra column to the marker table (Norte | almacén -> 5 columns instead of
// 4), corrupting its structure.
func TestMarkdownRenderElement_Map_LabelWithPipeIsEscaped(t *testing.T) {
	logger := newTestLogger()
	gen := NewMarkdownGenerator(logger)

	m := ast.NewMapElement(diagnostics.NewPosition(1, 1), "world")
	m.Markers = append(m.Markers, ast.MapMarker{Label: "Norte | almacén", Lat: 1, Lng: 2, Value: 3})

	out := gen.renderElement(m)

	if !strings.Contains(out, `| Norte \| almacén | 1 | 2 | 3 |`) {
		t.Errorf("expected the pipe in the label to be escaped and the row to keep exactly 4 columns, got:\n%s", out)
	}
}

// TestMarkdownRenderElement_Map_LabelWithNewlineIsNormalized covers the
// other half of the same finding: an embedded newline in marker.Label would
// otherwise split the marker across extra, malformed table rows.
func TestMarkdownRenderElement_Map_LabelWithNewlineIsNormalized(t *testing.T) {
	logger := newTestLogger()
	gen := NewMarkdownGenerator(logger)

	m := ast.NewMapElement(diagnostics.NewPosition(1, 1), "world")
	m.Markers = append(m.Markers, ast.MapMarker{Label: "CDMX\n| Fake | Row |", Lat: 1, Lng: 2, Value: 3})

	out := gen.renderElement(m)

	if !strings.Contains(out, `CDMX \| Fake \| Row \|`) {
		t.Errorf("expected the embedded newline to be normalized and the pipes escaped, got:\n%s", out)
	}

	dataRows := 0
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "| CDMX") {
			dataRows++
		}
	}
	if dataRows != 1 {
		t.Errorf("expected the marker to stay on exactly one table row, got %d:\n%s", dataRows, out)
	}
}

// TestMarkdownRenderElement_Map_TitleNewlineIsNormalized covers the Title/
// MapType line (not a table, but still a single Markdown line that a raw
// embedded newline could otherwise split, e.g. into a fake heading).
func TestMarkdownRenderElement_Map_TitleNewlineIsNormalized(t *testing.T) {
	logger := newTestLogger()
	gen := NewMarkdownGenerator(logger)

	m := ast.NewMapElement(diagnostics.NewPosition(1, 1), "world")
	m.Title = "Oficinas\n\n# Fake Heading"

	out := gen.renderElement(m)

	if strings.Contains(out, "\n\n# Fake Heading") {
		t.Errorf("expected the Title's embedded newlines to be normalized, got:\n%s", out)
	}
	if !strings.Contains(out, "*[mapa: world — Oficinas # Fake Heading]*") {
		t.Errorf("expected the normalized single-line title, got:\n%s", out)
	}
}

// TestMarkdownRenderElement_Map_TitleWithBracketAndAsteriskIsEscaped covers
// the code-review finding that normalizeMarkdownLine alone only closed the
// newline vector: a Title containing "]" or "*" could still break out of
// the *[mapa: … — …]* wrapper early (closing the emphasis, then continuing
// as unrelated Markdown body text).
func TestMarkdownRenderElement_Map_TitleWithBracketAndAsteriskIsEscaped(t *testing.T) {
	logger := newTestLogger()
	gen := NewMarkdownGenerator(logger)

	m := ast.NewMapElement(diagnostics.NewPosition(1, 1), "world")
	m.Title = "Zona ]* **PRECIO OCULTO**"

	out := gen.renderElement(m)

	if !strings.Contains(out, `Zona \]\* \*\*PRECIO OCULTO\*\*`) {
		t.Errorf("expected ] and * in the title to be escaped, got:\n%s", out)
	}
	if strings.Contains(out, "**PRECIO OCULTO**") {
		t.Errorf("expected no unescaped bold markers to survive, got:\n%s", out)
	}
}

// TestMarkdownRenderElement_HeadingPreservesAnchorID covers a finding from
// the #40 code review: re-emitting a heading as a "#" prefix from
// `<hN id="...">...</hN>` content discarded the id attribute entirely, so
// an in-document link like `[ver](#mi-seccion)` stopped resolving once this
// diff started stripping the wrapper down to plain text. The id must
// survive as an explicit {#id} anchor (Pandoc/kramdown syntax).
func TestMarkdownRenderElement_HeadingPreservesAnchorID(t *testing.T) {
	logger := newTestLogger()
	gen := NewMarkdownGenerator(logger)

	el := ast.NewRawHTMLTextElement(diagnostics.NewPosition(1, 1), `<h3 id="mi-seccion">Mi Sección</h3>`)
	el.Level = 3

	out := gen.renderElement(el)

	if !strings.Contains(out, "{#mi-seccion}") {
		t.Errorf("expected the heading anchor id to survive as {#mi-seccion}, got %q", out)
	}
	if !strings.Contains(out, "### Mi Sección") {
		t.Errorf("expected the extracted heading text, got %q", out)
	}
}

// TestMarkdownRenderElement_HeadingWithRawMarkdownDoesNotDoubleHash covers
// the same #40 finding as docx.go's
// TestDOCXGenerator_RenderText_LevelWithRawMarkdownContent: Level > 0 but
// Content still in raw Markdown form (reachable via an external --filter
// AST, or before HTML normalization runs) must not double the "#" prefix.
func TestMarkdownRenderElement_HeadingWithRawMarkdownDoesNotDoubleHash(t *testing.T) {
	logger := newTestLogger()
	gen := NewMarkdownGenerator(logger)

	el := ast.NewTextElement(diagnostics.NewPosition(1, 1), "## Resumen")
	el.Level = 2

	out := gen.renderElement(el)

	if strings.Contains(out, "## ## Resumen") {
		t.Errorf("heading prefix was doubled: %q", out)
	}
	if !strings.Contains(out, "## Resumen") {
		t.Errorf("expected \"## Resumen\", got %q", out)
	}
}
