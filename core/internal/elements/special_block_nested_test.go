// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/renderer"
)

// TestSpecialBlockParser_TableNestedInside cubre F9 (audit 2026-09-11): una
// tabla dentro de un ":::info" se reconoce como un TableElement tipado en
// Elements, no como texto crudo sin estructura.
//
// NOTA (hallazgo del advisor sobre PR-9 paso 3): "Antes de la tabla." tiene
// que aparecer TAMBIÉN en Elements, como un TextElement sintético — no solo
// en Content. Si Elements tuviera SOLO la tabla, renderSpecialBlockElement
// (que renderiza Elements en vez de Content en cuanto Elements no está
// vacío) perdería esa prosa en el HTML final. Elements es una
// reconstrucción COMPLETA del cuerpo, no una lista de "solo lo delegado".
func TestSpecialBlockParser_TableNestedInside(t *testing.T) {
	parser := &SpecialBlockParser{}
	ctx := &ParseContext{
		Mode: "flex",
		Lines: []string{
			":::info",
			"Antes de la tabla.",
			"| A | B |",
			"|---|---|",
			"| 1 | 2 |",
			":::",
		},
	}

	result := parser.Parse(ctx, 0)
	if result.Error != nil {
		t.Fatalf("Parse() error = %v", result.Error)
	}
	block, ok := result.Element.(*ast.SpecialBlockElement)
	if !ok {
		t.Fatalf("Element is not SpecialBlockElement: %T", result.Element)
	}
	if result.ConsumedLines != 6 {
		t.Errorf("ConsumedLines = %d, want 6", result.ConsumedLines)
	}
	if len(block.Elements) != 2 {
		t.Fatalf("len(Elements) = %d, want 2 (prosa + tabla): %+v", len(block.Elements), block.Elements)
	}
	prose, ok := block.Elements[0].(*ast.TextElement)
	if !ok || prose.IsRawHTML {
		t.Fatalf("Elements[0] no es un TextElement de prosa: %T", block.Elements[0])
	}
	if prose.Content != "Antes de la tabla." {
		t.Errorf("prose.Content = %q, want %q", prose.Content, "Antes de la tabla.")
	}
	table, ok := block.Elements[1].(*ast.TableElement)
	if !ok {
		t.Fatalf("Elements[1] is not TableElement: %T", block.Elements[1])
	}
	if len(table.Rows) != 1 || table.Rows[0][0] != "1" {
		t.Errorf("table.Rows = %#v, want [[\"1\" \"2\"]]", table.Rows)
	}
	// Content sigue teniendo TODAS las líneas del cuerpo, incluida la tabla
	// (Content y Elements son vistas paralelas, no una partición — ver el
	// comentario de Parse).
	wantContent := "Antes de la tabla.\n| A | B |\n|---|---|\n| 1 | 2 |"
	if block.Content != wantContent {
		t.Errorf("Content = %q, want %q", block.Content, wantContent)
	}
}

// TestSpecialBlockParser_NestedSpecialBlock cubre el repro real de
// examples/01_title_and_content/01.7_advanced_inline_syntax_flex.slidelang
// (":::reveal" con varios ":::reveal_content" adentro): antes de este fix,
// un ":::tipo" anidado CERRABA el bloque padre sin consumirlo (tratándolo
// como si fuera un hermano de nivel superior), fragmentando el documento y
// perdiendo el cierre real del padre.
//
// NOTA (PR-9 paso 3): "### Level 1"/"### Level 2" SÍ se promueven ahora a
// encabezados tipados (HeadingParser, agregado a nestedContentParsers) —
// esto invierte a propósito lo que esta misma prueba afirmaba antes de ese
// paso ("no se promueve a heading ... eso lo hace el parser de nivel
// superior"). No es una prueba rota: es la prueba adaptada al cambio de
// diseño que PR-9 paso 3 introduce.
//
// NOTA 2 (hallazgo del advisor sobre el mismo paso 3): "Texto del nivel 1."
// TAMBIÉN tiene que aparecer en Elements, como TextElement sintético entre
// el heading y el reveal_content — Elements es una reconstrucción COMPLETA
// del cuerpo (ver el comentario de flushProseRun en Parse), no solo la
// lista de lo delegado; de lo contrario el renderer (que deja de mirar
// Content en cuanto Elements no está vacío) perdería esa prosa.
func TestSpecialBlockParser_NestedSpecialBlock(t *testing.T) {
	parser := &SpecialBlockParser{}
	ctx := &ParseContext{
		Mode: "flex",
		Lines: []string{
			"::: reveal",
			"### Level 1",
			"Texto del nivel 1.",
			"::: reveal_content",
			"Contenido revelado.",
			":::",
			"### Level 2",
			":::",
		},
	}

	result := parser.Parse(ctx, 0)
	if result.Error != nil {
		t.Fatalf("Parse() error = %v", result.Error)
	}
	block, ok := result.Element.(*ast.SpecialBlockElement)
	if !ok {
		t.Fatalf("Element is not SpecialBlockElement: %T", result.Element)
	}
	if result.ConsumedLines != 8 {
		t.Fatalf("ConsumedLines = %d, want 8 (el bloque completo, incluido su propio cierre)", result.ConsumedLines)
	}
	if block.BlockType != "reveal" {
		t.Errorf("BlockType = %q, want \"reveal\"", block.BlockType)
	}
	if len(block.Elements) != 4 {
		t.Fatalf("len(Elements) = %d, want 4 (heading \"Level 1\", prosa, reveal_content anidado, heading \"Level 2\"): %+v", len(block.Elements), block.Elements)
	}

	heading1, ok := block.Elements[0].(*ast.TextElement)
	if !ok || !heading1.IsRawHTML {
		t.Fatalf("Elements[0] no es un heading RawHTML: %T", block.Elements[0])
	}
	if heading1.Level != 3 {
		t.Errorf("heading1.Level = %d, want 3", heading1.Level)
	}
	if !strings.Contains(heading1.Content, "Level 1") {
		t.Errorf("heading1.Content = %q, quiere contener \"Level 1\"", heading1.Content)
	}

	prose, ok := block.Elements[1].(*ast.TextElement)
	if !ok || prose.IsRawHTML {
		t.Fatalf("Elements[1] no es un TextElement de prosa: %T", block.Elements[1])
	}
	if prose.Content != "Texto del nivel 1." {
		t.Errorf("prose.Content = %q, want %q", prose.Content, "Texto del nivel 1.")
	}

	nested, ok := block.Elements[2].(*ast.SpecialBlockElement)
	if !ok {
		t.Fatalf("Elements[2] is not SpecialBlockElement: %T", block.Elements[2])
	}
	if nested.BlockType != "reveal_content" {
		t.Errorf("nested.BlockType = %q, want \"reveal_content\"", nested.BlockType)
	}
	if nested.Content != "Contenido revelado." {
		t.Errorf("nested.Content = %q, want \"Contenido revelado.\"", nested.Content)
	}

	heading2, ok := block.Elements[3].(*ast.TextElement)
	if !ok || !heading2.IsRawHTML {
		t.Fatalf("Elements[3] no es un heading RawHTML: %T", block.Elements[3])
	}
	if !strings.Contains(heading2.Content, "Level 2") {
		t.Errorf("heading2.Content = %q, quiere contener \"Level 2\"", heading2.Content)
	}

	// Content sigue teniendo TODAS las líneas crudas, incluidas las que
	// también se delegaron a HeadingParser — dos vistas paralelas de la
	// misma fuente, no una partición (ver el comentario de Parse).
	if !containsLine(block.Content, "### Level 2") {
		t.Errorf("Content no incluye \"### Level 2\": %q", block.Content)
	}
}

// TestSpecialBlockParser_SelfClosingOneLiner_DoesNotConsumeFollowingContent
// cubre F13/C25 (audit 2026-09-11): un ":::tipo attrs ... :::" que abre y
// cierra en la MISMA línea no debe escanear hacia adelante buscando un
// cierre que ya pasó — antes de este fix, un bloque de este tipo colgaba
// escaneando el resto del documento y adoptaba como hijos anidados a los
// bloques ":::" siguientes (regresión encontrada contra el fixture real
// 01.7_advanced_inline_syntax_flex.slidelang: "External Widgets" y otro
// ":::embed" completo desaparecían, absorbidos como prosa/hijos del
// primer embed, roto).
func TestSpecialBlockParser_SelfClosingOneLiner_DoesNotConsumeFollowingContent(t *testing.T) {
	parser := &SpecialBlockParser{}
	ctx := &ParseContext{
		Mode: "flex",
		Lines: []string{
			`::: embed type="youtube" video_id="abc123" :::`,
			"",
			"## Next Heading",
			"",
			"Prosa después, no debería tocarse.",
		},
	}

	result := parser.Parse(ctx, 0)
	if result.Error != nil {
		t.Fatalf("Parse() error = %v", result.Error)
	}
	if result.ConsumedLines != 1 {
		t.Fatalf("ConsumedLines = %d, want 1 (el bloque ya cerró en su propia línea)", result.ConsumedLines)
	}
	block, ok := result.Element.(*ast.SpecialBlockElement)
	if !ok {
		t.Fatalf("Element is not SpecialBlockElement: %T", result.Element)
	}
	if block.BlockType != "embed" {
		t.Errorf("BlockType = %q, want \"embed\"", block.BlockType)
	}
	if block.Content != "" {
		t.Errorf("Content = %q, want \"\" (nada que escanear tras el autocierre)", block.Content)
	}
	if len(block.Elements) != 0 {
		t.Errorf("Elements = %+v, want vacío", block.Elements)
	}
}

// TestSpecialBlockParser_StopsAtSlideSeparator_WithoutConsuming cubre la
// otra mitad de F9: un ":::bloque" sin su propio cierre no debe tragarse el
// "---" que separa slides/frontmatter en modo flex — antes de este fix solo
// se chequeaba "SLIDE " (el límite de modo strict), así que en flex un
// bloque sin cerrar fusionaba dos slides en una sola en silencio.
func TestSpecialBlockParser_StopsAtSlideSeparator_WithoutConsuming(t *testing.T) {
	parser := &SpecialBlockParser{}
	ctx := &ParseContext{
		Mode: "flex",
		Lines: []string{
			":::info",
			"Contenido sin cierre.",
			"---",
			"# Siguiente slide",
		},
	}

	result := parser.Parse(ctx, 0)
	if result.Error != nil {
		t.Fatalf("Parse() error = %v", result.Error)
	}
	if result.ConsumedLines != 2 {
		t.Fatalf("ConsumedLines = %d, want 2 (no debe consumir el \"---\")", result.ConsumedLines)
	}
}

// TestSpecialBlockParser_NestedChart_PropagatesDiagnostics cubre un hallazgo
// del advisor sobre el propio F9: tryParseNestedContent descartaba
// result.Diagnostics tanto en el camino de éxito como en el de rechazo — un
// chart con JSON inválido anidado dentro de un ":::info" perdía CHART002 en
// silencio (el mismo chart a nivel top emite el warning; anidado, no). El
// bloque especial debe reexportar los diagnósticos de sus hijos delegados en
// su propio ParseResult.Diagnostics, igual que hace con Elements.
func TestSpecialBlockParser_NestedChart_PropagatesDiagnostics(t *testing.T) {
	parser := &SpecialBlockParser{}
	ctx := &ParseContext{
		Mode: "flex",
		Lines: []string{
			":::info",
			"<<chart: bar>>",
			"{ esto no es JSON válido",
			"<</chart>>",
			":::",
		},
	}

	result := parser.Parse(ctx, 0)
	if result.Error != nil {
		t.Fatalf("Parse() error = %v", result.Error)
	}
	block, ok := result.Element.(*ast.SpecialBlockElement)
	if !ok {
		t.Fatalf("Element is not SpecialBlockElement: %T", result.Element)
	}
	// El chart con JSON inválido solo consume su apertura + la línea
	// inválida (no encuentra el "}" de cierre); "<</chart>>" queda como
	// prosa suelta y ahora también se vuelca a Elements como TextElement
	// sintético (ver flushProseRun en Parse) — 2 elementos, no 1.
	if len(block.Elements) != 2 {
		t.Fatalf("len(Elements) = %d, want 2 (chart + prosa \"<</chart>>\"): %+v", len(block.Elements), block.Elements)
	}
	if _, ok := block.Elements[0].(*ast.ChartElement); !ok {
		t.Errorf("Elements[0] is not ChartElement: %T", block.Elements[0])
	}

	found := false
	for _, d := range result.Diagnostics {
		if d.RuleID == "CHART002" {
			found = true
		}
	}
	if !found {
		t.Errorf("Diagnostics = %+v, want CHART002 propagado desde el chart anidado", result.Diagnostics)
	}
}

// TestSpecialBlockParser_EmptySelfClosing_DoesNotPanic cubre un hallazgo del
// advisor: ":::" seguido inmediatamente de su propio autocierre en la misma
// línea (sin tipo) — "::: :::" o "::::::" — dejaba blockContent == "" DESPUÉS
// del CutSuffix del autocierre (el guard `blockContent == ""` de más arriba
// solo atrapa el caso ANTES del corte). `strings.Fields("")` da un slice
// vacío y `parts[0]` entraba en pánico. Antes del autocierre (PR #325) este
// caso no existía: ":::" a secas producía blockContent=":::" (un blockType
// literal ":::"), nunca vacío.
func TestSpecialBlockParser_EmptySelfClosing_DoesNotPanic(t *testing.T) {
	cases := []string{"::: :::", "::::::"}
	for _, line := range cases {
		t.Run(line, func(t *testing.T) {
			parser := &SpecialBlockParser{}
			ctx := &ParseContext{Mode: "flex", Lines: []string{line}}
			result := parser.Parse(ctx, 0)
			if result.Error != nil {
				t.Fatalf("Parse(%q) error = %v", line, result.Error)
			}
			if result.ConsumedLines != 1 {
				t.Errorf("ConsumedLines = %d, want 1", result.ConsumedLines)
			}
		})
	}
}

// TestSpecialBlockParser_RenderedHTML_KeepsProseAlongsideNestedElements
// cubre el hallazgo del advisor sobre PR-9 paso 3: renderSpecialBlockElement
// (core/renderer/html.go) renderiza SOLO block.Elements en cuanto no está
// vacío — Content deja de mirarse por completo. Antes de este fix, la prosa
// que rodeaba a un elemento anidado (un heading, una tabla, un chart) vivía
// solo en Content y desaparecía del HTML en cuanto CUALQUIER elemento se
// delegaba, sin que ningún test de este mismo paquete lo notara (los tests
// de Parse solo miran el AST, nunca el HTML). Se prueba acá, no en
// core/renderer, porque este paquete ya importa renderer (para
// BuildHeadingElement) y renderer no puede importar de vuelta a este
// paquete — así que renderer no puede tener un test que construya el AST
// con el parser real sin duplicar la fixture a mano.
func TestSpecialBlockParser_RenderedHTML_KeepsProseAlongsideNestedElements(t *testing.T) {
	parser := &SpecialBlockParser{}
	ctx := &ParseContext{
		Mode: "flex",
		Lines: []string{
			":::info",
			"### Information",
			"Antes considerada prosa suelta, ahora tiene que sobrevivir al render.",
			":::",
		},
	}

	result := parser.Parse(ctx, 0)
	if result.Error != nil {
		t.Fatalf("Parse() error = %v", result.Error)
	}
	block := result.Element.(*ast.SpecialBlockElement)
	if len(block.Elements) == 0 {
		t.Fatalf("Elements vacío — este test no está probando lo que dice si no hay nada delegado")
	}

	html := renderer.RenderElementToHTML(block, nil, nil)
	if !strings.Contains(html, "<h3") || !strings.Contains(html, "Information") {
		t.Errorf("HTML no incluye el heading anidado: %s", html)
	}
	if !strings.Contains(html, "Antes considerada prosa suelta") {
		t.Errorf("HTML perdió la prosa que acompañaba al heading anidado (el bug que este test cubre): %s", html)
	}
}

// TestSpecialBlockParser_RenderedHTML_ListAlongsideHeadingIsValidHTML cubre
// un segundo hallazgo de revisión sobre el mismo mecanismo: la prosa
// sintética que flushProseRun agrega a Elements puede mezclar una línea
// suelta con líneas "- item" en el MISMO TextElement (Points/Checklist
// quedan fuera de nestedContentParsers a propósito — ver el comentario de
// esa var), algo que a nivel top nunca pasa (PointsParser se adelanta a
// TextParser en el registry). renderTextElement envolvía TODO el contenido
// en <p>...</p> sin mirar si el markdown adentro ya trae un <ul> — <ul> es
// contenido de flujo, <p> solo admite fraseo, así que el resultado era HTML
// inválido (<p>...<ul>...</ul></p>). Cubierto acá con el parser real (no
// solo la llamada directa a renderTextElement en core/renderer) porque es
// el camino end-to-end real: un ":::bloque" con un heading Y una lista.
func TestSpecialBlockParser_RenderedHTML_ListAlongsideHeadingIsValidHTML(t *testing.T) {
	parser := &SpecialBlockParser{}
	ctx := &ParseContext{
		Mode: "flex",
		Lines: []string{
			":::info",
			"### Resumen",
			"Cifras del trimestre:",
			"- Q1: 45",
			"- Q2: 32",
			":::",
		},
	}

	result := parser.Parse(ctx, 0)
	if result.Error != nil {
		t.Fatalf("Parse() error = %v", result.Error)
	}
	block := result.Element.(*ast.SpecialBlockElement)

	html := renderer.RenderElementToHTML(block, nil, nil)
	if strings.Contains(html, "<p>") {
		t.Errorf("un <ul> quedó envuelto en <p>, HTML inválido: %s", html)
	}
	if !strings.Contains(html, "<ul>") || !strings.Contains(html, "<li>Q1: 45</li>") {
		t.Errorf("la lista no se procesó como se esperaba: %s", html)
	}
	if !strings.Contains(html, "Cifras del trimestre:") {
		t.Errorf("se perdió la prosa que precedía a la lista: %s", html)
	}
}

func containsLine(content, want string) bool {
	for _, line := range splitLinesForTest(content) {
		if line == want {
			return true
		}
	}
	return false
}

func splitLinesForTest(s string) []string {
	var lines []string
	start := 0
	for i, r := range s {
		if r == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	lines = append(lines, s[start:])
	return lines
}
