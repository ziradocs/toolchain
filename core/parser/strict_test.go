// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/util"
)

// TestStrictParser_ParseMarkdownTableElement_MalformedHeader_HeadersNotNull cubre
// issue #8: sin un fallback a slice vacío, un header markdown malformado (no
// envuelto en "|...|") dejaba TableElement.Headers en nil, que se serializa como
// JSON null en vez de [] (violando el JSON Schema del contrato).
func TestStrictParser_ParseMarkdownTableElement_MalformedHeader_HeadersNotNull(t *testing.T) {
	p := NewStrictParser("", util.NewNoop())
	p.lines = []string{"| A | B"} // falta el "|" de cierre: no cumple el guard
	p.currentLine = 0

	element := p.parseMarkdownTableElement()
	table, ok := element.(*ast.TableElement)
	if !ok {
		t.Fatalf("expected *ast.TableElement, got %T", element)
	}

	if table.Headers == nil {
		t.Fatal("Headers is nil; want an empty (non-nil) slice")
	}

	data, err := json.Marshal(table)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if decoded["headers"] == nil {
		t.Errorf("serialized headers is null, want []: %s", data)
	}
}

// TestStrictParser_ParseMarkdownTableElement_NoDataRows_RowsNotNull cubre issue
// #8: una tabla markdown con header + separador pero sin filas de datos dejaba
// TableElement.Rows en nil (el loop de filas nunca hace append), serializando
// como JSON null en vez de [].
func TestStrictParser_ParseMarkdownTableElement_NoDataRows_RowsNotNull(t *testing.T) {
	p := NewStrictParser("", util.NewNoop())
	p.lines = []string{
		"| A | B |",
		"|---|---|",
	}
	p.currentLine = 0

	element := p.parseMarkdownTableElement()
	table, ok := element.(*ast.TableElement)
	if !ok {
		t.Fatalf("expected *ast.TableElement, got %T", element)
	}

	if table.Rows == nil {
		t.Fatal("Rows is nil; want an empty (non-nil) slice")
	}
	if len(table.Headers) != 2 {
		t.Errorf("Headers = %v, want 2 entries", table.Headers)
	}

	data, err := json.Marshal(table)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if decoded["rows"] == nil {
		t.Errorf("serialized rows is null, want []: %s", data)
	}
}

// TestStrictParser_ParseMarkdownTableElement_MalformedRowAbsorbedNotFragmented
// cubre issue #155: una fila intermedia malformada ("|xyz", sin "|" de
// cierre) hacía que parseContentBlock re-entrara parseMarkdownTableElement
// desde cero en la línea siguiente, fragmentando UNA tabla legítima en
// 2-3 TableElement separados con pérdida silenciosa de la fila malformada
// Y de las filas que venían después. Con el fix, una fila malformada
// intermedia (con header ya válido) se absorbe (se saltea, con un warning
// visible) y la tabla sigue acumulando filas — el repro exacto de la issue
// debe dar UNA sola tabla con la fila válida posterior incluida.
func TestStrictParser_ParseMarkdownTableElement_MalformedRowAbsorbedNotFragmented(t *testing.T) {
	body := "SLIDE title\n  |a|b|\n  |xyz\n  |c|d|"

	p := NewStrictParser(body, util.NewNoop())
	astNode, diags := p.Parse()

	if n := countErrors(diags); n != 0 {
		t.Fatalf("got %d error diagnostics, want 0: %v", n, diags)
	}

	warned := false
	for _, d := range diags {
		if strings.Contains(d.Message, "malformed table row") {
			warned = true
		}
	}
	if !warned {
		t.Errorf("expected a warning diagnostic about the skipped malformed row, got: %+v", diags)
	}

	if len(astNode.ContentBlocks) != 1 {
		t.Fatalf("ContentBlocks = %d, want 1", len(astNode.ContentBlocks))
	}

	var tables []*ast.TableElement
	for _, el := range astNode.ContentBlocks[0].Elements {
		if table, ok := el.(*ast.TableElement); ok {
			tables = append(tables, table)
		}
	}
	if len(tables) != 1 {
		t.Fatalf("got %d TableElement(s), want 1 (the malformed row must be absorbed into the same table, not fragment it): %+v", len(tables), tables)
	}

	table := tables[0]
	if len(table.Headers) != 2 || table.Headers[0] != "a" || table.Headers[1] != "b" {
		t.Errorf("Headers = %v, want [a b]", table.Headers)
	}
	if len(table.Rows) != 1 || len(table.Rows[0]) != 2 || table.Rows[0][0] != "c" || table.Rows[0][1] != "d" {
		t.Errorf("Rows = %v, want [[c d]] (the row after the malformed one must not be lost)", table.Rows)
	}
}

// findNotesDirective busca un *ast.DirectiveNode con Name == "notes" entre
// los elementos de un ContentBlock.
func findNotesDirective(elements []ast.Element) *ast.DirectiveNode {
	for _, el := range elements {
		if d, ok := el.(*ast.DirectiveNode); ok && d.Name == "notes" {
			return d
		}
	}
	return nil
}

// countErrors cuenta los diagnósticos de severidad Error.
func countErrors(diags []diagnostics.Diagnostic) int {
	n := 0
	for _, d := range diags {
		if d.IsError() {
			n++
		}
	}
	return n
}

// TestStrictParser_NotesDirective_ParsedNotRejected_Issue153 cubre issue
// #153: antes del fix en f5a02689 (PR #95), la rama de "propiedad de
// bloque" (parseContentBlock, disparada por cualquier línea indentada que
// contuviera ":") interceptaba "@notes: contenido" antes de que pudiera
// llegar al dispatch de directives ("@"-prefix), partiéndola en
// key="@notes" y reportando "Unknown content block property: @notes" — el
// dispatch de directives (línea ~213) era en la práctica inalcanzable para
// la forma "@directiva: valor". El fix excluyó las líneas "@" de esa rama,
// igual que ya se excluían TEXT/POINTS/:::/<<.
//
// Cubre las 2 posiciones que el reporte original probó (antes y después de
// los demás elementos del slide) para blindar contra una regresión que solo
// rompa una de las dos.
func TestStrictParser_NotesDirective_ParsedNotRejected_Issue153(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "notes before TEXT element",
			body: `SLIDE content
  title: "Notes Slide"
  @notes: Remember to breathe.

  TEXT
    Slide body.`,
		},
		{
			name: "notes after TEXT element",
			body: `SLIDE content
  title: "Notes Slide"

  TEXT
    Slide body.

  @notes: Remember to breathe.`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewStrictParser(tt.body, util.NewNoop())
			astNode, diags := p.Parse()

			if n := countErrors(diags); n != 0 {
				t.Fatalf("got %d error diagnostics, want 0: %v", n, diags)
			}

			if len(astNode.ContentBlocks) != 1 {
				t.Fatalf("ContentBlocks = %d, want 1", len(astNode.ContentBlocks))
			}

			notes := findNotesDirective(astNode.ContentBlocks[0].Elements)
			if notes == nil {
				t.Fatalf("no notes DirectiveNode found in elements: %+v", astNode.ContentBlocks[0].Elements)
			}

			want := "Remember to breathe."
			if got, _ := notes.Parameters["content"].(string); got != want {
				t.Errorf("notes content = %q, want %q", got, want)
			}
		})
	}
}

// TestStrictParser_QuoteAndChecklist_Dispatched_Issue205 cubre issue #205:
// parseContentBlock no tenía branches para los keywords QUOTE/CHECKLIST,
// aunque ambos ya tenían soporte de modo strict implementado en sus parsers
// modulares (internal/elements/quote.go, internal/elements/checklist.go).
// Antes del fix, "QUOTE"/"CHECKLIST" caían al catch-all final (drop
// silencioso, currentLine++) y sus líneas de metadata "AUTHOR:"/"SOURCE:"
// disparaban "Unknown content block property" espurios vía la rama de
// propiedades del bloque (cualquier línea indentada con ":").
func TestStrictParser_QuoteAndChecklist_Dispatched_Issue205(t *testing.T) {
	body := `SLIDE content
  title: "Quote and Checklist"
  QUOTE
    Simplicity is the ultimate sophistication.
    AUTHOR: Leonardo da Vinci
    SOURCE: Notebooks

  CHECKLIST
    [x] Ship the fix
    [ ] Write the follow-up issue
      [x] File it under the right milestone`

	p := NewStrictParser(body, util.NewNoop())
	astNode, diags := p.Parse()

	if n := countErrors(diags); n != 0 {
		t.Fatalf("got %d error diagnostics, want 0: %v", n, diags)
	}
	if len(astNode.ContentBlocks) != 1 {
		t.Fatalf("ContentBlocks = %d, want 1", len(astNode.ContentBlocks))
	}

	elements := astNode.ContentBlocks[0].Elements
	if len(elements) != 2 {
		t.Fatalf("Elements = %d, want 2 (QuoteElement + ChecklistElement): %+v", len(elements), elements)
	}

	quote, ok := elements[0].(*ast.QuoteElement)
	if !ok {
		t.Fatalf("elements[0] = %T, want *ast.QuoteElement", elements[0])
	}
	if want := "Simplicity is the ultimate sophistication."; quote.Content != want {
		t.Errorf("quote.Content = %q, want %q", quote.Content, want)
	}
	if want := "Leonardo da Vinci"; quote.Author != want {
		t.Errorf("quote.Author = %q, want %q", quote.Author, want)
	}
	if want := "Notebooks"; quote.Source != want {
		t.Errorf("quote.Source = %q, want %q", quote.Source, want)
	}

	checklist, ok := elements[1].(*ast.ChecklistElement)
	if !ok {
		t.Fatalf("elements[1] = %T, want *ast.ChecklistElement", elements[1])
	}
	if len(checklist.Items) != 2 {
		t.Fatalf("checklist.Items = %d, want 2: %+v", len(checklist.Items), checklist.Items)
	}
	if !checklist.Items[0].Checked || checklist.Items[0].Content != "Ship the fix" {
		t.Errorf("checklist.Items[0] = %+v, want checked=true content=%q", checklist.Items[0], "Ship the fix")
	}
	if checklist.Items[1].Checked || checklist.Items[1].Content != "Write the follow-up issue" {
		t.Errorf("checklist.Items[1] = %+v, want checked=false content=%q", checklist.Items[1], "Write the follow-up issue")
	}
	if len(checklist.Items[1].SubItems) != 1 || !checklist.Items[1].SubItems[0].Checked {
		t.Fatalf("checklist.Items[1].SubItems = %+v, want 1 checked sub-item", checklist.Items[1].SubItems)
	}
}

// TestStrictParser_NotesWithoutAtPrefix_StillRejected documenta que "notes:"
// sin el prefijo "@" no es un directive y debe seguir siendo rechazado como
// "Unknown content block property" — el fix de issue #153 excluye líneas
// "@" de esa rama, no la palabra "notes" en sí.
func TestStrictParser_NotesWithoutAtPrefix_StillRejected(t *testing.T) {
	body := `SLIDE content
  title: "Notes Slide"
  notes: Remember to breathe.

  TEXT
    Slide body.`

	p := NewStrictParser(body, util.NewNoop())
	_, diags := p.Parse()

	// Aserta el mensaje específico, no solo "hubo algún error": así el test
	// falla si un refactor futuro hace que "notes:" falle por otra razón
	// (o deja de tratarse como propiedad desconocida), en vez de pasar por
	// el motivo equivocado.
	const want = "Unknown content block property: notes"
	found := false
	for _, d := range diags {
		if d.IsError() && strings.Contains(d.Message, want) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("no error diagnostic containing %q; got %v", want, diags)
	}
}

// TestStrictParser_ImageSourceWithColon_NotMisparsedAsProperty cubre el bug
// donde una fuente de IMAGE que contiene ":" (URLs http(s):// y data-URIs) se
// malinterpretaba como una propiedad de bloque. La detección de propiedad de
// parseContentBlock disparaba con "cualquier línea con ':' que no empiece con
// TEXT/POINTS/:::/<</@", así que `IMAGE "https://cdn.example.com/x.png"` se
// partía en key=`IMAGE "https` y emitía "Unknown content block property".
// El fix restringe esa rama a `<identificador>: <valor>`, dejando que el
// opener de IMAGE caiga al dispatch de elementos donde su cadena entrecomillada
// (que ya soporta ":") se preserva verbatim. Cada IMAGE va en su propio SLIDE
// porque el loop de continuación caption:/label: de parseStrictImage consume
// líneas indentadas siguientes (limitación pre-existente, ajena a este fix).
func TestStrictParser_ImageSourceWithColon_NotMisparsedAsProperty(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{"local path", "images/logo.png"},
		{"http url", "http://example.com/a.png"},
		{"https url", "https://cdn.example.com/logo.png"},
		{"data uri", "data:image/png;base64,iVBORw0KGgo="},
		{"windows drive path", "C:/win/path.png"},
		{"query string with colon", "https://x.io/a.png?t=12:30&u=a:b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := "SLIDE content\n  heading: \"H\"\n  IMAGE \"" + tt.source + "\" \"alt text\""
			p := NewStrictParser(body, util.NewNoop())
			astNode, diags := p.Parse()

			if n := countErrors(diags); n != 0 {
				t.Fatalf("got %d error diagnostics, want 0: %v", n, diags)
			}
			if len(astNode.ContentBlocks) != 1 {
				t.Fatalf("ContentBlocks = %d, want 1", len(astNode.ContentBlocks))
			}
			els := astNode.ContentBlocks[0].Elements
			if len(els) != 1 {
				t.Fatalf("Elements = %d, want 1 (an ImageElement): %+v", len(els), els)
			}
			img, ok := els[0].(*ast.ImageElement)
			if !ok {
				t.Fatalf("elements[0] = %T, want *ast.ImageElement", els[0])
			}
			if img.Source != tt.source {
				t.Errorf("img.Source = %q, want %q (verbatim, ':' preserved)", img.Source, tt.source)
			}
			if img.Alt != "alt text" {
				t.Errorf("img.Alt = %q, want %q", img.Alt, "alt text")
			}
		})
	}
}

// TestParseBlockPropertyLine cubre la lógica del guard directamente: una línea
// con ":" es una propiedad de bloque a menos que abra un elemento strict (una
// IMAGE con URL, un <<...>>, etc.). Una clave malformada (no-ASCII, con punto)
// SIGUE siendo una propiedad — se diagnostica como desconocida, no se descarta.
func TestParseBlockPropertyLine(t *testing.T) {
	tests := []struct {
		line      string
		wantOK    bool
		wantKey   string
		wantValue string
	}{
		{`title: "Hello"`, true, "title", "Hello"},
		{`heading: Plain value`, true, "heading", "Plain value"},
		{`notes: x`, true, "notes", "x"},                 // desconocida, pero sigue siendo forma de propiedad
		{`sub-title: y`, true, "sub-title", "y"},         // desconocida
		{`título: "Hola"`, true, "título", "Hola"},       // clave no-ASCII → propiedad malformada (se diagnostica)
		{`title.es: "Intro"`, true, "title.es", "Intro"}, // clave con punto → propiedad malformada
		{`IMAGE "https://cdn.example.com/x.png"`, false, "", ""},
		{`IMAGE "data:image/png;base64,AAAA"`, false, "", ""},
		{`:::code-group`, false, "", ""},
		{`<<chart: bar>>`, false, "", ""},
		{`no colon here`, false, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			key, value, ok := parseBlockPropertyLine(tt.line)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v (key=%q value=%q)", ok, tt.wantOK, key, value)
			}
			if ok && (key != tt.wantKey || value != tt.wantValue) {
				t.Errorf("got (%q, %q), want (%q, %q)", key, value, tt.wantKey, tt.wantValue)
			}
		})
	}
}

// TestStrictParser_MalformedPropertyKey_StillDiagnosed: una línea "key: value"
// cuya clave no es válida (no-ASCII, con punto, mal escrita, con espacio) debe
// seguir surfaceando "Unknown content block property" — NO descartarse en
// silencio. Regresión de code-review sobre el fix de IMAGE-con-":": un guard
// que exigía una clave "identificador simple" hacía caer estas líneas al
// catch-all silencioso, ocultando metadata de slide inválida.
func TestStrictParser_MalformedPropertyKey_StillDiagnosed(t *testing.T) {
	for _, key := range []string{"título", "title.es", "titel", "sub title"} {
		t.Run(key, func(t *testing.T) {
			body := "SLIDE content\n  " + key + `: "value"`
			p := NewStrictParser(body, util.NewNoop())
			_, diags := p.Parse()

			want := "Unknown content block property: " + key
			found := false
			for _, d := range diags {
				if d.IsError() && strings.Contains(d.Message, want) {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("no error diagnostic containing %q; got %v", want, diags)
			}
		})
	}
}

// TestStrictParser_Grid_TypedWithNestedColumnContent cubre la sintaxis strict
// de grid <<grid>>/<<column>>/<<end>>: produce un GridElement tipado con
// columnas cuyo Content en bruto conserva la indentación relativa (sub-puntos)
// y donde <<end>> termina el grid sin tragarse el elemento siguiente.
func TestStrictParser_Grid_TypedWithNestedColumnContent(t *testing.T) {
	body := `SLIDE content
  title: "Grid"
  <<grid>>
  intro prose
  <<column>>
  ## Left
  - point one
    - nested point
  <<column>>
  Right side
  <<end>>
  TEXT
    After the grid.`

	p := NewStrictParser(body, util.NewNoop())
	astNode, diags := p.Parse()

	if n := countErrors(diags); n != 0 {
		t.Fatalf("got %d error diagnostics, want 0: %v", n, diags)
	}
	if len(astNode.ContentBlocks) != 1 {
		t.Fatalf("ContentBlocks = %d, want 1", len(astNode.ContentBlocks))
	}
	els := astNode.ContentBlocks[0].Elements
	if len(els) != 2 {
		t.Fatalf("Elements = %d, want 2 (grid + text): %+v", len(els), els)
	}

	grid, ok := els[0].(*ast.GridElement)
	if !ok {
		t.Fatalf("elements[0] = %T, want *ast.GridElement", els[0])
	}
	if grid.Content != "intro prose" {
		t.Errorf("grid.Content = %q, want %q", grid.Content, "intro prose")
	}
	if len(grid.Columns) != 2 {
		t.Fatalf("grid.Columns = %d, want 2: %+v", len(grid.Columns), grid.Columns)
	}
	// La indentación relativa del sub-punto se preserva; la sangría base
	// (2 espacios del nivel de elemento) se quita.
	if want := "## Left\n- point one\n  - nested point"; grid.Columns[0].Content != want {
		t.Errorf("grid.Columns[0].Content = %q, want %q", grid.Columns[0].Content, want)
	}
	if grid.Columns[1].Content != "Right side" {
		t.Errorf("grid.Columns[1].Content = %q, want %q", grid.Columns[1].Content, "Right side")
	}

	// <<end>> terminó el grid: el TEXT siguiente es su propio elemento.
	if _, ok := els[1].(*ast.TextElement); !ok {
		t.Fatalf("elements[1] = %T, want *ast.TextElement (grid did not swallow it)", els[1])
	}
}

// TestStrictParser_Grid_IndentedSlideTextIsContent cubre el hallazgo de
// code-review: la guarda defensiva "límite de slide" solo debe disparar en un
// marcador de slide REAL (columna 0, sin sangría), no en una línea de contenido
// indentada que casualmente empieza con "SLIDE ". Un `SLIDE overview` indentado
// dentro de una columna se preserva como texto; un `SLIDE next` sin sangría
// tras el grid (aquí SIN <<end>>, input malformado) sí lo termina.
func TestStrictParser_Grid_IndentedSlideTextIsContent(t *testing.T) {
	body := `SLIDE content
  <<grid>>
  <<column>>
  Intro to the SLIDE overview
  SLIDE overview
  <<column>>
  Right
  <<end>>

SLIDE content
  title: "Real next slide"
  TEXT
    Body.`

	p := NewStrictParser(body, util.NewNoop())
	astNode, diags := p.Parse()
	if n := countErrors(diags); n != 0 {
		t.Fatalf("got %d error diagnostics, want 0: %v", n, diags)
	}
	if len(astNode.ContentBlocks) != 2 {
		t.Fatalf("ContentBlocks = %d, want 2 (grid slide + real next slide)", len(astNode.ContentBlocks))
	}

	grid, ok := astNode.ContentBlocks[0].Elements[0].(*ast.GridElement)
	if !ok {
		t.Fatalf("first element = %T, want *ast.GridElement", astNode.ContentBlocks[0].Elements[0])
	}
	if len(grid.Columns) != 2 {
		t.Fatalf("grid.Columns = %d, want 2", len(grid.Columns))
	}
	// La línea "SLIDE overview" indentada NO terminó la columna: es contenido.
	if want := "Intro to the SLIDE overview\nSLIDE overview"; grid.Columns[0].Content != want {
		t.Errorf("grid.Columns[0].Content = %q, want %q", grid.Columns[0].Content, want)
	}

	// El SLIDE real, sin sangría, sí abrió un segundo bloque.
	if astNode.ContentBlocks[1].Title != "Real next slide" {
		t.Errorf("ContentBlocks[1].Title = %q, want %q", astNode.ContentBlocks[1].Title, "Real next slide")
	}
}

// TestGrid_StrictAndFlex_SameAST verifica que la forma strict
// (<<grid>>/<<column>>/<<end>>) y la forma flex (::: grid/::: column) de la
// MISMA estructura lógica producen GridElements idénticos — el requisito de
// equivalencia build→AST entre dialectos. Se comparan tras poner en cero
// posiciones y ContentHTML (que dependen del layout de texto, no de la
// estructura).
func TestGrid_StrictAndFlex_SameAST(t *testing.T) {
	strictBody := `---
mode: strict
---
SLIDE content
  <<grid>>
  <<column>>
  ## Left
  - point one
  - point two
  <<column>>
  Right side prose text
  <<end>>`

	flexBody := `---
mode: flex
---
# Slide

::: grid
::: column
## Left
- point one
- point two
:::
::: column
Right side prose text
:::
:::`

	strictGrid := firstGrid(t, strictBody)
	flexGrid := firstGrid(t, flexBody)

	if !reflect.DeepEqual(strictGrid, flexGrid) {
		t.Fatalf("strict and flex grid ASTs differ:\nstrict: %+v\nflex:   %+v", strictGrid, flexGrid)
	}
}

// firstGrid parsea content con el parser completo (que enruta por mode) y
// devuelve el primer GridElement con posiciones y ContentHTML en cero.
func firstGrid(t *testing.T, content string) *ast.GridElement {
	t.Helper()
	p := New(util.NewNoop())
	doc, diags := p.Parse(content, "grid_test.slidelang")
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("parse error: %v", d.Message)
		}
	}
	for _, b := range doc.ContentBlocks {
		for _, el := range b.Elements {
			if g, ok := el.(*ast.GridElement); ok {
				gc := *g
				gc.Position, gc.EndPosition = diagnostics.Position{}, diagnostics.Position{}
				gc.ContentHTML = ""
				cols := make([]ast.ColumnElement, len(g.Columns))
				for i, col := range g.Columns {
					col.Position, col.EndPosition = diagnostics.Position{}, diagnostics.Position{}
					col.ContentHTML = ""
					cols[i] = col
				}
				gc.Columns = cols
				return &gc
			}
		}
	}
	t.Fatalf("no GridElement found in parsed content")
	return nil
}
