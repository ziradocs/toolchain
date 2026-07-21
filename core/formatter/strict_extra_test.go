// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"reflect"
	"strings"
	"testing"

	"go.ziradocs.com/core/ast"
	"go.ziradocs.com/core/diagnostics"
)

// TestFormatStrict_RoundTrip_EdgeCases cubre construcciones que el corpus
// real de examples/ no ejercita: IMAGE con caption, TABLE con caption
// (fuerza la forma TABLE/YAML en vez de pipe), chart en JSON mode, y las
// formas de directiva bare/key=value/@notes multi-línea/@delay.
func TestFormatStrict_RoundTrip_EdgeCases(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{
			name: "image with caption",
			content: `---
mode: strict
title: "Edge Cases"
---

SLIDE content
  title: "Image"
  IMAGE "assets/diagram.png" "A diagram"
    caption: "Figure 1: the diagram"
`,
		},
		{
			name: "table with caption",
			content: `---
mode: strict
---

SLIDE content
  title: "Table"
  TABLE
    headers: ["Q1", "Q2"]
    caption: "Ventas trimestrales"
    rows:
        [100, 200]
        [150, 250]
`,
		},
		{
			name: "chart json mode",
			content: `---
mode: strict
---

SLIDE content
  title: "Chart"
  <<chart: bar>>
    {"type": "bar", "data": {"labels": ["a", "b"], "datasets": [{"data": [1, 2]}]}}
    <</chart>>
`,
		},
		{
			name: "directive forms",
			content: `---
mode: strict
---

SLIDE content
  title: "Directives"
  @center
  TEXT
    Centered text.
  @highlight color="warning"
  TEXT
    Highlighted text.
  @delay 2000
  TEXT
    Delayed text.
  @notes
    Remember to mention budget impact.
    Second line of notes.
  TEXT
    Text with notes.
`,
		},
		{
			// Issue #205: QUOTE con AUTHOR/SOURCE, ahora despachado por
			// parser.StrictParser (antes caía al catch-all silencioso).
			name: "quote with author and source",
			content: `---
mode: strict
---

SLIDE content
  title: "Quote"
  QUOTE
    Simplicity is the ultimate sophistication.
    AUTHOR: Leonardo da Vinci
    SOURCE: Notebooks
`,
		},
		{
			// Issue #205: QUOTE sin metadata (Author/Source vacíos no se
			// emiten, ver formatStrictQuote).
			name: "quote without author or source",
			content: `---
mode: strict
---

SLIDE content
  title: "Quote"
  QUOTE
    A quote with no attribution.
`,
		},
		{
			// Issue #205: CHECKLIST con sub-items, ahora despachado por
			// parser.StrictParser.
			name: "checklist with sub-items",
			content: `---
mode: strict
---

SLIDE content
  title: "Checklist"
  CHECKLIST
    [x] Ship the fix
    [ ] Write the follow-up issue
      [x] File it under the right milestone
`,
		},
		{
			// IMAGE con fuente que contiene ":" — URL http(s):// y data-URI.
			// Antes del fix del parser strict, la línea IMAGE se malinterpretaba
			// como propiedad de bloque (el ":" de "https:"/"data:" disparaba la
			// detección de propiedad), así que este AST ni siquiera se podía
			// obtener parseando; ahora round-trip-ea verbatim.
			name: "image with url and data-uri sources",
			content: `---
mode: strict
---

SLIDE content
  title: "URL image"
  IMAGE "https://cdn.example.com/logo.png" "external"

SLIDE content
  title: "Data URI image"
  IMAGE "data:image/png;base64,iVBORw0KGgo=" "inline"

SLIDE content
  title: "URL with caption and label"
  IMAGE "https://x.io/a.png" "alt"
    caption: "Figura 1"
    label: "fig:one"
`,
		},
		{
			// El contenido de código lleva su propia indentación (el cuerpo de
			// def hello() está a 4 espacios). El formatter envuelve todo el
			// bloque a 2 espacios de elemento; si el parser guardara el
			// contenido verbatim, esa indentación de elemento se acumularía en
			// cada pasada de fmt. Este caso lo ejercita end-to-end vía el
			// chequeo de AST-equality + idempotencia del harness.
			name: "code-group with indented multi-line content",
			content: `---
mode: strict
---

SLIDE content
  title: "Code group"
  :::code-group
  ` + "```python [Python]" + `
  def hello():
      print("hi")
  ` + "```" + `
  ` + "```go [Go]" + `
  func hello() {
      fmt.Println("hi")
  }
  ` + "```" + `
  :::
`,
		},
		{
			// GRID strict: dos columnas con contenido markdown, seguido de otro
			// elemento (TEXT) — verifica que <<end>> termina el grid limpiamente
			// en vez de tragarse el elemento siguiente.
			name: "grid with two columns then trailing element",
			content: `---
mode: strict
---

SLIDE content
  title: "Grid"
  <<grid>>
  <<column>>
  ## Left
  - point one
  - point two
  <<column>>
  Right side prose text
  <<end>>
  TEXT
    A paragraph after the grid.
`,
		},
		{
			// GRID strict: prosa suelta antes de la primera columna
			// (GridElement.Content), una columna vacía, y un salto de línea
			// interno dentro del contenido de columna (debe preservarse).
			name: "grid with stray prose, empty column, internal blank line",
			content: `---
mode: strict
---

SLIDE content
  title: "Grid edge"
  <<grid>>
  intro prose before columns
  <<column>>
  first para

  second para after blank
  <<column>>
  <<end>>
`,
		},
		{
			// GRID strict: una línea de contenido que empieza con "SLIDE " NO es
			// un límite de slide (está indentada) — se preserva como texto y
			// round-trip-ea. Regresión de code-review sobre el chequeo de límite
			// de slide demasiado agresivo.
			name: "grid column content starting with SLIDE word",
			content: `---
mode: strict
---

SLIDE content
  title: "Grid slide-text"
  <<grid>>
  <<column>>
  Intro to the SLIDE overview
  SLIDE overview
  <<column>>
  Right
  <<end>>
`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := parseStrict(t, tc.content)
			out, err := FormatStrict(doc)
			if err != nil {
				t.Fatalf("FormatStrict: %v", err)
			}

			reparsed := parseStrict(t, out)
			want := normalizeForComparison(doc)
			got := normalizeForComparison(reparsed)
			if !reflect.DeepEqual(want, got) {
				t.Fatalf("round-trip mismatch for %q\n--- formatted ---\n%s\n--- want ---\n%s\n--- got ---\n%s",
					tc.name, out, toJSON(t, want), toJSON(t, got))
			}

			out2, err := FormatStrict(reparsed)
			if err != nil {
				t.Fatalf("FormatStrict (2nd pass): %v", err)
			}
			if out != out2 {
				t.Fatalf("fmt no es idempotente para %q\n--- 1st ---\n%s\n--- 2nd ---\n%s", tc.name, out, out2)
			}
		})
	}
}

// TestFormatStrict_GridEmptySupported: un GridElement vacío (sin columnas ni
// prosa) ya es representable con la sintaxis strict <<grid>>/<<column>>/<<end>>
// — antes devolvía UnsupportedElementError. Se construye a mano porque el
// parser nunca produce un grid completamente vacío, pero un AST de otro origen
// sí podría.
func TestFormatStrict_GridEmptySupported(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	doc := ast.NewAST(pos)
	block := ast.NewContentBlock(pos, "content")
	block.Elements = append(block.Elements, ast.NewGridElement(pos))
	doc.ContentBlocks = append(doc.ContentBlocks, *block)

	out, err := FormatStrict(doc)
	if err != nil {
		t.Fatalf("FormatStrict: unexpected error for empty *ast.GridElement: %v", err)
	}
	if !strings.Contains(out, "<<grid>>") || !strings.Contains(out, "<<end>>") {
		t.Errorf("FormatStrict output missing grid markers:\n%s", out)
	}
}

// TestFormatStrict_GridWithTypedColumnElementsUnsupported: la forma strict de
// grid guarda el cuerpo de cada columna como Content en bruto (lo que produce
// el parser y consume el renderer). Una columna con Elements TIPADOS anidados
// —forma que ningún parser produce hoy, pero que un AST de otro origen (p. ej.
// un transpiler) podría traer— no tiene representación en la forma de texto
// Content-based, así que se reporta en vez de perder esos Elements en silencio.
func TestFormatStrict_GridWithTypedColumnElementsUnsupported(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	doc := ast.NewAST(pos)
	block := ast.NewContentBlock(pos, "content")
	grid := ast.NewGridElement(pos)
	col := ast.NewColumnElement(pos, "")
	col.Elements = append(col.Elements, ast.NewTextElement(pos, "typed nested text"))
	grid.Columns = append(grid.Columns, *col)
	block.Elements = append(block.Elements, grid)
	doc.ContentBlocks = append(doc.ContentBlocks, *block)

	_, err := FormatStrict(doc)
	if err == nil {
		t.Fatal("FormatStrict: expected UnsupportedElementError for a column with typed Elements, got nil")
	}
	var uerr *UnsupportedElementError
	if reflect.TypeOf(err) != reflect.TypeOf(uerr) {
		t.Fatalf("FormatStrict error type = %T, want *UnsupportedElementError: %v", err, err)
	}
}

// TestFormatStrict_QuoteContentUnsupported cubre la guarda de issue #205:
// elements.QuoteParser.parseStrict termina la cita en la primera línea
// vacía, "---", o AUTHOR:/SOURCE:/keyword-de-elemento — un QuoteElement.Content
// que contenga alguna de esas formas (posible si el elemento vino de un parse
// flex, donde el markdown ">" no tiene esas restricciones) no es
// representable sin pérdida en modo strict. Este contenido no se puede
// obtener parseando texto strict real (el propio parser strict nunca
// produciría un QuoteElement con estas formas), así que se construye a mano.
func TestFormatStrict_QuoteContentUnsupported(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{"blank line", "First line.\n\nSecond line."},
		{"separator line", "First line.\n---\nSecond line."},
		{"embedded AUTHOR metadata line", "First line.\nAUTHOR: not really the author."},
		{"embedded element keyword line", "First line.\nTEXT this looks like a new element."},
		// Símbolos, no keywords en mayúsculas: espeja el fix de
		// internal/elements/common.go IsNewElement (rama strict) que agregó
		// detección de @ directiva, ::: special block, << diagrama/chart/
		// map/math y | tabla Markdown — antes de ese fix, elements.QuoteParser
		// también se tragaba estas líneas como contenido de la cita, así que
		// esta guarda estaba incompleta de forma consistente con el parser;
		// ahora que el parser SÍ corta ahí, esta guarda debe rechazarlas.
		{"embedded directive marker line", "First line.\n@center"},
		{"embedded special block marker line", "First line.\n:::info"},
		{"embedded diagram/chart/math marker line", "First line.\n<<math>>"},
		{"embedded markdown table marker line", "First line.\n| a | b |"},
		// Code-review finding on this PR: elements.QuoteParser.parseStrict
		// TrimSpace-ea cada línea al reparsear, así que espacio en blanco
		// al inicio/final de una línea (p.ej. código indentado citado
		// dentro de un QUOTE) se perdería en silencio sin esta guarda.
		{"leading whitespace on a content line", "First line.\n    indented second line."},
		{"trailing whitespace on a content line", "First line.   \nSecond line."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pos := diagnostics.NewPosition(1, 1)
			doc := ast.NewAST(pos)
			block := ast.NewContentBlock(pos, "content")
			block.Elements = append(block.Elements, ast.NewQuoteElement(pos, tc.content))
			doc.ContentBlocks = append(doc.ContentBlocks, *block)

			_, err := FormatStrict(doc)
			if err == nil {
				t.Fatalf("FormatStrict: expected UnsupportedElementError for content %q, got nil", tc.content)
			}
			var uerr *UnsupportedElementError
			if reflect.TypeOf(err) != reflect.TypeOf(uerr) {
				t.Fatalf("FormatStrict error type = %T, want *UnsupportedElementError: %v", err, err)
			}
		})
	}
}

// TestFormatStrict_ChecklistUnsupported cubre la guarda simétrica a
// TestFormatStrict_QuoteContentUnsupported, agregada tras un hallazgo de
// code-review en esta misma PR: antes de este fix, formatStrictChecklist no
// validaba nada (a diferencia de Quote), así que un ChecklistItem con
// Content vacío, con un salto de línea embebido, o con SubItems anidados
// dentro de otro SubItem se serializaba en texto que reparseaba a un AST
// DISTINTO en silencio (perdía items, fusionaba contenido, o aplanaba un
// nivel de anidamiento) en vez de fallar — una regresión de postura de
// error respecto al comportamiento previo (todo ChecklistElement devolvía
// UnsupportedElementError incondicionalmente). Igual que con Quote, este
// contenido no se puede obtener parseando texto strict real, así que se
// construye a mano.
func TestFormatStrict_ChecklistUnsupported(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)

	newChecklistDoc := func(items []ast.ChecklistItem) *ast.AST {
		doc := ast.NewAST(pos)
		block := ast.NewContentBlock(pos, "content")
		checklist := ast.NewChecklistElement(pos)
		checklist.Items = items
		block.Elements = append(block.Elements, checklist)
		doc.ContentBlocks = append(doc.ContentBlocks, *block)
		return doc
	}

	cases := []struct {
		name  string
		items []ast.ChecklistItem
	}{
		{
			name:  "empty content",
			items: []ast.ChecklistItem{{Content: "", Checked: false}},
		},
		{
			name:  "embedded newline in content",
			items: []ast.ChecklistItem{{Content: "first line\nsecond line", Checked: true}},
		},
		{
			name: "sub-item nested inside another sub-item",
			items: []ast.ChecklistItem{
				{
					Content: "top",
					SubItems: []ast.ChecklistItem{
						{
							Content: "sub",
							SubItems: []ast.ChecklistItem{
								{Content: "sub-sub"},
							},
						},
					},
				},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := FormatStrict(newChecklistDoc(tc.items))
			if err == nil {
				t.Fatalf("FormatStrict: expected UnsupportedElementError for items %+v, got nil", tc.items)
			}
			var uerr *UnsupportedElementError
			if reflect.TypeOf(err) != reflect.TypeOf(uerr) {
				t.Fatalf("FormatStrict error type = %T, want *UnsupportedElementError: %v", err, err)
			}
		})
	}
}

// TestFormatStrict_UnescapableQuoteRejected cubre el hallazgo de
// code-review en issue #206 (fmt --strict transpile flex→strict): quote()
// (formatter/util.go) envuelve valores en comillas dobles SIN escapar nada
// -- documentado, aceptado como limitación pre-existente del dialecto
// strict-nativo, donde un autor rara vez tipea comillas literales en un
// campo entrecomillado. El transpiler expone esos MISMOS campos a texto
// flex/markdown de forma libre (heading/título/alt/caption/label/etc.),
// donde una comilla literal es común (p.ej. "cita" o contracciones) -- sin
// una guarda, esto corrompía el valor EN SILENCIO en vez de fallar
// (confirmado end-to-end: internal/elements/*.go cierra el valor
// entrecomillado en la PRÓXIMA comilla literal, truncando el resto).
//
// Repro real encontrado corriendo el corpus completo tras agregar la
// guarda: examples/maps_special_characters_test.doclang tiene un
// marker.details con `"Flavian Amphitheatre"` embebido -- antes de esta
// guarda, TestFormatDocument_RoundTrip_Corpus lo daba por bueno (PASS)
// pese a la corrupción, porque el truncamiento es estable entre pasadas
// (compara reparse-tras-formatear contra sí mismo, no contra el texto
// fuente original) -- exactamente la trampa "zero diff ≠ seguro" ya
// documentada en este proyecto. Con la guarda, ahora produce
// UnsupportedElementError y el mismo test lo SKIPea explícitamente en vez
// de reportar un falso "canónico".
//
// Este test cubre cada campo entrecomillado alcanzable desde un AST de
// origen flex (property de bloque, IMAGE, TABLE con caption/headers/filas,
// chart title/series/labels/data, map marker.label, parámetro de
// directiva) -- no exhaustivo campo-por-campo dentro de cada función (eso
// ya lo cubre validateStrictQuoteContent/Checklist para Quote/Checklist),
// sino una prueba de que CADA función que pasa por quote() en este archivo
// tiene la guarda, no solo algunas.
func TestFormatStrict_UnescapableQuoteRejected(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	const q = `a "quoted" value`

	newDocWithBlock := func(block *ast.ContentBlock) *ast.AST {
		doc := ast.NewAST(pos)
		doc.ContentBlocks = append(doc.ContentBlocks, *block)
		return doc
	}

	cases := []struct {
		name string
		doc  *ast.AST
	}{
		{
			name: "content block title",
			doc: func() *ast.AST {
				block := ast.NewContentBlock(pos, "content")
				block.Title = q
				return newDocWithBlock(block)
			}(),
		},
		{
			name: "image source",
			doc: func() *ast.AST {
				block := ast.NewContentBlock(pos, "content")
				block.Elements = append(block.Elements, ast.NewImageElement(pos, q, "alt"))
				return newDocWithBlock(block)
			}(),
		},
		{
			name: "table caption",
			doc: func() *ast.AST {
				table := ast.NewTableElement(pos)
				table.Headers = []string{"A"}
				table.Rows = [][]string{{"1"}}
				table.Caption = q
				block := ast.NewContentBlock(pos, "content")
				block.Elements = append(block.Elements, table)
				return newDocWithBlock(block)
			}(),
		},
		{
			name: "table row value (forces TABLE/YAML form via caption)",
			doc: func() *ast.AST {
				table := ast.NewTableElement(pos)
				table.Headers = []string{"A"}
				table.Rows = [][]string{{q}}
				table.Caption = "safe caption"
				block := ast.NewContentBlock(pos, "content")
				block.Elements = append(block.Elements, table)
				return newDocWithBlock(block)
			}(),
		},
		{
			name: "chart title",
			doc: func() *ast.AST {
				chart := ast.NewChartElement(pos, "bar")
				chart.Title = q
				block := ast.NewContentBlock(pos, "content")
				block.Elements = append(block.Elements, chart)
				return newDocWithBlock(block)
			}(),
		},
		{
			name: "chart labels",
			doc: func() *ast.AST {
				chart := ast.NewChartElement(pos, "bar")
				chart.Labels = []string{q}
				block := ast.NewContentBlock(pos, "content")
				block.Elements = append(block.Elements, chart)
				return newDocWithBlock(block)
			}(),
		},
		{
			name: "map marker label",
			doc: func() *ast.AST {
				m := ast.NewMapElement(pos, "leaflet")
				m.Markers = []ast.MapMarker{{Lat: 1, Lng: 2, Label: q}}
				block := ast.NewContentBlock(pos, "content")
				block.Elements = append(block.Elements, m)
				return newDocWithBlock(block)
			}(),
		},
		{
			name: "directive parameter value",
			doc: func() *ast.AST {
				d := ast.NewDirectiveNode(pos, "highlight")
				d.Parameters["color"] = q
				block := ast.NewContentBlock(pos, "content")
				block.Elements = append(block.Elements, d)
				return newDocWithBlock(block)
			}(),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := FormatStrict(tc.doc)
			if err == nil {
				t.Fatalf("FormatStrict: expected UnsupportedElementError for a value containing a literal quote, got nil")
			}
			var uerr *UnsupportedElementError
			if reflect.TypeOf(err) != reflect.TypeOf(uerr) {
				t.Fatalf("FormatStrict error type = %T, want *UnsupportedElementError: %v", err, err)
			}
		})
	}
}

// TestFormatStrict_CodeGroupIndentIdempotent es la regresión directa del bug
// pre-existente encontrado al agregar la sintaxis strict de GRID: el `fmt`
// strict de un :::code-group NO era idempotente — cada pasada acumulaba 2
// espacios de indentación en el cuerpo de cada fence. Causa raíz:
// formatCodeGroup emite el contenido a ras de margen y formatStrictElement
// envuelve el elemento con indent(body, 2); al re-parsear,
// elements.CodeGroupParser guardaba cada línea verbatim, incluyendo esa
// indentación de elemento, así que CodeBlock.Content crecía 2 espacios por
// round-trip. El fix dedenta el contenido por la indentación base del
// marcador :::code-group en el parser strict (internal/elements/code_group.go).
//
// Este test fija (1) que `fmt` dos veces produce bytes idénticos y (2) que
// el contenido almacenado tras un build→fmt→build conserva su indentación
// natural exacta (4 espacios para el cuerpo de la función), sin ganar ni
// perder espacios.
func TestFormatStrict_CodeGroupIndentIdempotent(t *testing.T) {
	src := "---\nmode: strict\n---\n\nSLIDE content\n  title: \"CG\"\n  :::code-group\n" +
		"  ```python [Python]\n  def hello():\n      print(\"hi\")\n  ```\n  :::\n"

	// build → fmt (1ª pasada)
	doc := parseStrict(t, src)
	out1, err := FormatStrict(doc)
	if err != nil {
		t.Fatalf("FormatStrict (1st): %v", err)
	}

	// build(out1) → fmt (2ª pasada): debe ser byte-idéntico.
	reparsed := parseStrict(t, out1)
	out2, err := FormatStrict(reparsed)
	if err != nil {
		t.Fatalf("FormatStrict (2nd): %v", err)
	}
	if out1 != out2 {
		t.Fatalf("fmt de :::code-group no es idempotente\n--- 1st ---\n%q\n--- 2nd ---\n%q", out1, out2)
	}

	// El contenido almacenado del bloque debe conservar SOLO su indentación
	// natural (4 espacios), tanto en el AST original como en el re-parseado.
	const wantContent = "def hello():\n    print(\"hi\")"
	for _, tc := range []struct {
		label string
		doc   *ast.AST
	}{
		{"original", doc},
		{"reparsed", reparsed},
	} {
		cg := firstCodeGroup(t, tc.doc)
		if len(cg.CodeBlocks) != 1 {
			t.Fatalf("%s: CodeBlocks = %d, want 1", tc.label, len(cg.CodeBlocks))
		}
		if got := cg.CodeBlocks[0].Content; got != wantContent {
			t.Fatalf("%s: CodeBlock.Content = %q, want %q", tc.label, got, wantContent)
		}
	}
}

func firstCodeGroup(t *testing.T, doc *ast.AST) *ast.CodeGroupElement {
	t.Helper()
	for _, block := range doc.ContentBlocks {
		for _, el := range block.Elements {
			if cg, ok := el.(*ast.CodeGroupElement); ok {
				return cg
			}
		}
	}
	t.Fatalf("no CodeGroupElement found in AST")
	return nil
}
