// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"reflect"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/util"
)

// Issue #373: columnas de grid tipadas. `<<column typed>>` (strict) y
// `::: column typed` (flex) llenan ColumnElement.Elements; la forma cruda
// sigue llenando Content. Diseño en docs/portable-typed-columns.md.

const typedColStrictHeader = "---\nmode: strict\n---\n"
const typedColFlexHeader = "---\nmode: flex\n---\n"

// parseTypedCols parsea con el normalizador apagado: la equivalencia entre
// dialectos se afirma sobre lo que cada parser produce, no sobre lo que el
// normalizador reescriba.
func parseTypedCols(t *testing.T, src string) (*ast.AST, []diagnostics.Diagnostic) {
	t.Helper()
	p := New(util.NewNoop())
	p.SetNormalization(false)
	doc, diags := p.Parse(src, "typed_columns_test.slidelang")
	if doc == nil {
		t.Fatalf("nil AST; diagnostics: %v", diags)
	}
	return doc, diags
}

func firstGridOf(t *testing.T, doc *ast.AST) *ast.GridElement {
	t.Helper()
	for _, b := range doc.ContentBlocks {
		for _, el := range b.Elements {
			if g, ok := el.(*ast.GridElement); ok {
				return g
			}
		}
	}
	t.Fatal("no GridElement in the parsed document")
	return nil
}

func hasErrorContaining(diags []diagnostics.Diagnostic, want string) bool {
	for _, d := range diags {
		if d.IsError() && strings.Contains(d.Message, want) {
			return true
		}
	}
	return false
}

// El caso de #373: antes, el node-id dentro de una columna strict era un
// orphan node-id y el build fallaba. Con `<<column typed>>` se liga al TEXT.
func TestTypedColumn_Strict_NodeIDBindsToNestedElement(t *testing.T) {
	src := typedColStrictHeader + `SLIDE content
  title: "Grid"
  <<grid>>
  <<column typed>>
    <!-- node-id: ColTextA -->
    TEXT
      Left side.
  <<column>>
  Right side.
  <<end>>
`
	doc, diags := parseTypedCols(t, src)
	if countErrors(diags) != 0 {
		t.Fatalf("unexpected errors: %v", diags)
	}
	grid := firstGridOf(t, doc)
	if len(grid.Columns) != 2 {
		t.Fatalf("columns = %d, want 2", len(grid.Columns))
	}
	typed, raw := grid.Columns[0], grid.Columns[1]
	if typed.Content != "" || len(typed.Elements) != 1 {
		t.Fatalf("typed column: content=%q elements=%d", typed.Content, len(typed.Elements))
	}
	text, ok := typed.Elements[0].(*ast.TextElement)
	if !ok {
		t.Fatalf("element is %T, want *ast.TextElement", typed.Elements[0])
	}
	if text.NodeID != "ColTextA" || text.Content != "Left side." {
		t.Fatalf("text: nodeId=%q content=%q", text.NodeID, text.Content)
	}
	// El ID queda ligado al elemento real, en su línea del archivo (la 9: el
	// TEXT, después de la directiva).
	if text.Position.Line != 9 {
		t.Fatalf("TEXT position line = %d, want 9", text.Position.Line)
	}
	if raw.Content != "Right side." || len(raw.Elements) != 0 {
		t.Fatalf("raw column changed: content=%q elements=%d", raw.Content, len(raw.Elements))
	}
}

// La misma fuente con `<<column>>` (cruda) sigue fallando igual que antes: el
// cuerpo de una columna cruda no es un nodo identificable.
func TestTypedColumn_Strict_RawColumnStillRejectsNodeID(t *testing.T) {
	src := typedColStrictHeader + `SLIDE content
  title: "Grid"
  <<grid>>
  <<column>>
  <!-- node-id: ColTextA -->
  TEXT
    Left side.
  <<column>>
  Right side.
  <<end>>
`
	_, diags := parseTypedCols(t, src)
	if !hasErrorContaining(diags, "orphan node-id") {
		t.Fatalf("expected the orphan node-id error for a raw column; got %v", diags)
	}
}

// Un `<<end>>` que cierra un elemento DENTRO del cuerpo no cierra el grid: el
// cuerpo se delimita por sangría. En una columna cruda la primera `<<end>>` sí
// cierra el grid, como siempre.
func TestTypedColumn_Strict_NestedEndDoesNotCloseGrid(t *testing.T) {
	src := typedColStrictHeader + `SLIDE content
  title: "Grid"
  <<grid>>
  <<column typed>>
    <<chart: bar>>
      data: [
        ["Q1", 45],
        ["Q2", 52]
      ]
    <<end>>
    TEXT
      After the chart.
  <<column>>
  Right side.
  <<end>>
  TEXT
    After the grid.
`
	doc, diags := parseTypedCols(t, src)
	if countErrors(diags) != 0 {
		t.Fatalf("unexpected errors: %v", diags)
	}
	grid := firstGridOf(t, doc)
	if len(grid.Columns) != 2 {
		t.Fatalf("columns = %d, want 2", len(grid.Columns))
	}
	els := grid.Columns[0].Elements
	if len(els) != 2 {
		t.Fatalf("typed column elements = %d, want 2 (chart, text)", len(els))
	}
	if _, ok := els[0].(*ast.ChartElement); !ok {
		t.Fatalf("first element is %T, want *ast.ChartElement", els[0])
	}
	if _, ok := els[1].(*ast.TextElement); !ok {
		t.Fatalf("second element is %T, want *ast.TextElement", els[1])
	}
	block := doc.ContentBlocks[0]
	if last, ok := block.Elements[len(block.Elements)-1].(*ast.TextElement); !ok || last.Content != "After the grid." {
		t.Fatalf("the element after the grid was swallowed: %+v", block.Elements)
	}
}

// Las columnas crudas, y los documentos que nunca usan el marcador nuevo,
// producen el mismo AST que antes (aditivo).
func TestTypedColumn_RawColumnsUnchanged(t *testing.T) {
	src := typedColStrictHeader + `SLIDE content
  <<grid>>
  <<column>>
  ## Left
  - point one
  <<column>>
  Right side
  <<end>>
`
	doc, diags := parseTypedCols(t, src)
	if countErrors(diags) != 0 {
		t.Fatalf("unexpected errors: %v", diags)
	}
	grid := firstGridOf(t, doc)
	if len(grid.Columns) != 2 {
		t.Fatalf("columns = %d, want 2", len(grid.Columns))
	}
	for i, col := range grid.Columns {
		if len(col.Elements) != 0 || col.Content == "" {
			t.Fatalf("column %d: content=%q elements=%d", i, col.Content, len(col.Elements))
		}
	}
	if grid.Columns[0].Content != "## Left\n- point one" {
		t.Fatalf("raw content = %q", grid.Columns[0].Content)
	}
}

func TestTypedColumn_Strict_Errors(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"property", "  <<grid>>\n  <<column typed>>\n    Nota: algo\n  <<end>>\n", "has no properties"},
		{"unrecognized", "  <<grid>>\n  <<column typed>>\n    TEXXT\n      body\n  <<end>>\n", "not an element"},
		{"heading", "  <<grid>>\n  <<column typed>>\n    SECTION \"Title\"\n  <<end>>\n", "headings are not supported"},
		{"nested grid", "  <<grid>>\n  <<column typed>>\n    <<grid>>\n    <<column>>\n    x\n    <<end>>\n  <<end>>\n", "cannot be nested"},
		{"slide inside", "  <<grid>>\n  <<column typed>>\n    SLIDE content\n  <<end>>\n", "cannot be opened inside a column"},
		{"shallow body", "  <<grid>>\n  <<column typed>>\n   TEXT\n  <<end>>\n", "indented two spaces"},
		{"body at grid indent", "  <<grid>>\n  <<column typed>>\n  TEXT\n    stray\n  <<end>>\n", "must be indented under"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := typedColStrictHeader + "SLIDE content\n  title: \"T\"\n" + tc.body
			_, diags := parseTypedCols(t, src)
			if !hasErrorContaining(diags, tc.want) {
				t.Fatalf("expected an error containing %q; got %v", tc.want, diags)
			}
		})
	}
}

// Un marcador con otro sufijo sigue siendo texto de la columna cruda anterior
// (solo la coincidencia exacta es el marcador nuevo).
func TestTypedColumn_MarkerRequiresExactMatch(t *testing.T) {
	src := typedColStrictHeader + `SLIDE content
  <<grid>>
  <<column>>
  <<column typedx>>
  <<end>>
`
	doc, diags := parseTypedCols(t, src)
	if countErrors(diags) != 0 {
		t.Fatalf("unexpected errors: %v", diags)
	}
	grid := firstGridOf(t, doc)
	if len(grid.Columns) != 1 || grid.Columns[0].Content != "<<column typedx>>" || len(grid.Columns[0].Elements) != 0 {
		t.Fatalf("a near-miss marker changed meaning: %+v", grid.Columns)
	}
}

// El caso de #373 en flex: el node-id antes de un elemento de una columna
// tipada se liga a ese elemento.
func TestTypedColumn_Flex_NodeIDBindsToNestedElement(t *testing.T) {
	src := typedColFlexHeader + `# Grid

::: grid
::: column typed
<!-- node-id: ColTextA -->
Left side.
:::
::: column
Right side.
:::
:::
`
	doc, diags := parseTypedCols(t, src)
	if countErrors(diags) != 0 {
		t.Fatalf("unexpected errors: %v", diags)
	}
	grid := firstGridOf(t, doc)
	if len(grid.Columns) != 2 {
		t.Fatalf("columns = %d, want 2", len(grid.Columns))
	}
	typed := grid.Columns[0]
	if typed.Content != "" || len(typed.Elements) != 1 {
		t.Fatalf("typed column: content=%q elements=%d", typed.Content, len(typed.Elements))
	}
	text, ok := typed.Elements[0].(*ast.TextElement)
	if !ok || text.NodeID != "ColTextA" || text.Content != "Left side." {
		t.Fatalf("typed element: %#v", typed.Elements[0])
	}
	if grid.Columns[1].Content != "Right side." || len(grid.Columns[1].Elements) != 0 {
		t.Fatalf("raw column changed: %+v", grid.Columns[1])
	}
}

// Dentro de una columna flex tipada un `:::` que pertenece a un bloque
// especial o a una valla de código lo consume ese elemento, no cierra la
// columna.
func TestTypedColumn_Flex_InnerBlocksOwnTheirDelimiters(t *testing.T) {
	src := typedColFlexHeader + "# Grid\n\n" +
		"::: grid\n" +
		"::: column typed\n" +
		"Intro line.\n\n" +
		"::: info\nNote body.\n:::\n\n" +
		"```go\nfmt.Println(\":::\")\n```\n" +
		":::\n" +
		"::: column\nRight\n:::\n" +
		":::\n"
	doc, diags := parseTypedCols(t, src)
	if countErrors(diags) != 0 {
		t.Fatalf("unexpected errors: %v", diags)
	}
	grid := firstGridOf(t, doc)
	if len(grid.Columns) != 2 {
		t.Fatalf("columns = %d, want 2", len(grid.Columns))
	}
	var kinds []string
	for _, el := range grid.Columns[0].Elements {
		kinds = append(kinds, string(el.GetType()))
	}
	want := []string{"text", "special_block", "code"}
	if !reflect.DeepEqual(kinds, want) {
		t.Fatalf("typed column element types = %v, want %v", kinds, want)
	}
}

func TestTypedColumn_Flex_NestedGridIsAnError(t *testing.T) {
	src := typedColFlexHeader + "# Grid\n\n" +
		"::: grid\n::: column typed\n::: grid\n::: column\nx\n:::\n:::\n:::\n:::\n"
	_, diags := parseTypedCols(t, src)
	if !hasErrorContaining(diags, "cannot be nested") {
		t.Fatalf("expected a nested grid error; got %v", diags)
	}
}

// `::: column` con otro sufijo sigue siendo una columna cruda.
func TestTypedColumn_Flex_OtherSuffixStaysRaw(t *testing.T) {
	src := typedColFlexHeader + "# Grid\n\n::: grid\n::: column wide\nbody\n:::\n:::\n"
	doc, diags := parseTypedCols(t, src)
	if countErrors(diags) != 0 {
		t.Fatalf("unexpected errors: %v", diags)
	}
	col := firstGridOf(t, doc).Columns[0]
	if col.Content != "body" || len(col.Elements) != 0 {
		t.Fatalf("column changed meaning: %+v", col)
	}
}

// Strict y flex dan el mismo AST (tipos y campos, sin posiciones ni nodeId)
// para una columna tipada. El conjunto cubierto es el del diseño.
func TestTypedColumn_StrictAndFlex_SameElements(t *testing.T) {
	cases := []struct {
		name   string
		strict string // cuerpo bajo `<<column typed>>`, ya a 4 espacios
		flex   string // cuerpo bajo `::: column typed`
	}{
		{"text", "    TEXT\n      Left side.", "Left side."},
		{"points", "    POINTS\n      - one\n      - two", "- one\n- two"},
		{"code", "    CODE go\n      fmt.Println(1)", "```go\nfmt.Println(1)\n```"},
		{"image", "    IMAGE \"a.png\" \"alt a\"", "![alt a](a.png)"},
		{"quote", "    QUOTE\n      Simplicity.", "> Simplicity."},
		{"checklist", "    CHECKLIST\n      [x] Done\n      [ ] Todo", "- [x] Done\n- [ ] Todo"},
		{"table", "    TABLE\n      headers: [\"A\", \"B\"]\n      rows:\n        [\"1\", \"2\"]", "\n| A | B |\n|---|---|\n| 1 | 2 |\n"}, // la línea en blanco ya es necesaria en una columna cruda: sin ella flex lee `::: grid` como fila de tabla
		{"chart", "    <<chart: bar>>\n      data: [\n        [\"Q1\", 45],\n        [\"Q2\", 52]\n      ]\n    <<end>>", "<<chart: bar>>\n  data: [\n    [\"Q1\", 45],\n    [\"Q2\", 52]\n  ]\n<<end>>"},
		{"heading line", "    TEXT\n      Intro\n    TEXT\n      ## Sub heading\n    TEXT\n      after heading", "Intro\n\n## Sub heading\nafter heading"},
		{"rule line", "    TEXT\n      Intro\n    TEXT\n      ---\n    TEXT\n      after rule", "Intro\n\n---\n\nafter rule"},
		{"two elements", "    TEXT\n      Intro.\n    POINTS\n      - one", "Intro.\n\n- one"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			strictSrc := typedColStrictHeader + "SLIDE content\n  <<grid>>\n  <<column typed>>\n" + tc.strict + "\n  <<end>>\n"
			flexSrc := typedColFlexHeader + "# Slide\n\n::: grid\n::: column typed\n" + tc.flex + "\n:::\n:::\n"
			strictDoc, sd := parseTypedCols(t, strictSrc)
			flexDoc, fd := parseTypedCols(t, flexSrc)
			if countErrors(sd) != 0 || countErrors(fd) != 0 {
				t.Fatalf("errors: strict=%v flex=%v", sd, fd)
			}
			sg, fg := firstGridOf(t, strictDoc), firstGridOf(t, flexDoc)
			if len(sg.Columns) != 1 || len(fg.Columns) != 1 {
				t.Fatalf("columns: strict=%d flex=%d", len(sg.Columns), len(fg.Columns))
			}
			if len(sg.Columns[0].Elements) == 0 {
				t.Fatal("strict typed column has no elements")
			}
			got := semanticJSON(t, &ast.AST{ContentBlocks: []ast.ContentBlock{{Elements: []ast.Element{&sg.Columns[0]}}}})
			want := semanticJSON(t, &ast.AST{ContentBlocks: []ast.ContentBlock{{Elements: []ast.Element{&fg.Columns[0]}}}})
			if tc.name == "image" {
				// ImageParser trata la fuente y el contexto distinto en cada
				// dialecto también a nivel de slide (flex antepone
				// assets/images/ y infiere el contexto); la columna hereda esa
				// diferencia, no la introduce.
				dropImageDialectFields(got)
				dropImageDialectFields(want)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("strict and flex columns differ:\nstrict: %v\nflex:   %v", got, want)
			}
		})
	}
}

// `###` dentro de una columna no es un encabezado tipado en ningún dialecto:
// flex lo deja como texto y un TEXT strict con esa línea da lo mismo.
func TestTypedColumn_HeadingLineIsTextInBothDialects(t *testing.T) {
	strictSrc := typedColStrictHeader + "SLIDE content\n  <<grid>>\n  <<column typed>>\n    TEXT\n      ### Title\n  <<end>>\n"
	flexSrc := typedColFlexHeader + "# Slide\n\n::: grid\n::: column typed\n### Title\n:::\n:::\n"
	sd, _ := parseTypedCols(t, strictSrc)
	fd, _ := parseTypedCols(t, flexSrc)
	for name, doc := range map[string]*ast.AST{"strict": sd, "flex": fd} {
		els := firstGridOf(t, doc).Columns[0].Elements
		if len(els) != 1 {
			t.Fatalf("%s: elements = %d, want 1", name, len(els))
		}
		if text, ok := els[0].(*ast.TextElement); !ok || !strings.Contains(text.Content, "### Title") {
			t.Fatalf("%s: element is %#v", name, els[0])
		}
	}
}

// Posiciones reales: el ligado de node-id es por línea, así que los
// elementos anidados deben llevar la línea del archivo, también con
// frontmatter delante (flex).
func TestTypedColumn_Flex_NestedPositionsAreFileLines(t *testing.T) {
	src := typedColFlexHeader + "# Grid\n\n::: grid\n::: column typed\nLeft side.\n:::\n:::\n"
	doc, _ := parseTypedCols(t, src)
	col := firstGridOf(t, doc).Columns[0]
	if col.Position.Line != 7 {
		t.Fatalf("column line = %d, want 7", col.Position.Line)
	}
	if got := col.Elements[0].GetPosition().Line; got != 8 {
		t.Fatalf("nested text line = %d, want 8", got)
	}
}

// El identificador de la columna misma: un node-id antes del marcador se liga
// al ColumnElement (en ambos dialectos).
func TestTypedColumn_NodeIDOnTheColumnItself(t *testing.T) {
	strictSrc := typedColStrictHeader + "SLIDE content\n  <<grid>>\n  <!-- node-id: LeftCol -->\n  <<column typed>>\n    TEXT\n      Left.\n  <<end>>\n"
	flexSrc := typedColFlexHeader + "# Slide\n\n::: grid\n<!-- node-id: LeftCol -->\n::: column typed\nLeft.\n:::\n:::\n"
	for name, src := range map[string]string{"strict": strictSrc, "flex": flexSrc} {
		doc, diags := parseTypedCols(t, src)
		if countErrors(diags) != 0 {
			t.Fatalf("%s: unexpected errors: %v", name, diags)
		}
		if got := firstGridOf(t, doc).Columns[0].NodeID; got != "LeftCol" {
			t.Fatalf("%s: column nodeId = %q", name, got)
		}
	}
}

// dropImageDialectFields quita source y context de los nodos image de un árbol
// ya serializado: son las dos diferencias de dialecto que ImageParser ya tiene
// fuera de las columnas.
func dropImageDialectFields(v any) {
	switch n := v.(type) {
	case map[string]any:
		if n["type"] == "image" {
			delete(n, "source")
			delete(n, "context")
		}
		for _, c := range n {
			dropImageDialectFields(c)
		}
	case []any:
		for _, c := range n {
			dropImageDialectFields(c)
		}
	}
}

// Con el normalizador activo (el valor por defecto, y flex-full/auto lo
// fuerzan) una fuente ya bien formada sigue dando columnas tipadas.
func TestTypedColumn_Flex_SurvivesNormalization(t *testing.T) {
	for _, mode := range []string{"flex", "flex-full", "auto"} {
		t.Run(mode, func(t *testing.T) {
			src := "---\nmode: " + mode + "\n---\n# Grid\n\n::: grid\n::: column typed\nLeft side.\n\n- one\n- two\n:::\n::: column\nRight side.\n:::\n:::\n"
			doc, diags := New(util.NewNoop()).Parse(src, "typed.slidelang")
			if countErrors(diags) != 0 {
				t.Fatalf("unexpected errors: %v", diags)
			}
			grid := firstGridOf(t, doc)
			if len(grid.Columns) != 2 || len(grid.Columns[0].Elements) != 2 || grid.Columns[0].Content != "" {
				t.Fatalf("normalization changed the typed column: %+v", grid.Columns)
			}
			if grid.Columns[1].Content != "Right side." {
				t.Fatalf("raw column = %q", grid.Columns[1].Content)
			}
		})
	}
}

// DocLang comparte el mismo camino: `::: column typed` en el dialecto flex y
// `<<column typed>>` en el strict de documentos.
func TestTypedColumn_DocumentDialects(t *testing.T) {
	cases := map[string]string{
		"flex":   "---\nmode: flex\n---\n# Report\n\n::: grid\n::: column typed\n<!-- node-id: ColTextA -->\nLeft side.\n:::\n::: column\nRight side.\n:::\n:::\n",
		"strict": "---\nmode: strict\n---\nSECTION \"Report\"\n  <<grid>>\n  <<column typed>>\n    <!-- node-id: ColTextA -->\n    TEXT\n      Left side.\n  <<column>>\n  Right side.\n  <<end>>\n",
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			p := New(util.NewNoop())
			p.SetNormalization(false)
			doc, diags := p.ParseDocument(src, "typed.doclang")
			if countErrors(diags) != 0 {
				t.Fatalf("unexpected errors: %v", diags)
			}
			grid := firstGridOf(t, doc)
			if len(grid.Columns) != 2 || len(grid.Columns[0].Elements) != 1 {
				t.Fatalf("columns: %+v", grid.Columns)
			}
			text, ok := grid.Columns[0].Elements[0].(*ast.TextElement)
			if !ok || text.NodeID != "ColTextA" || text.Content != "Left side." {
				t.Fatalf("typed element: %#v", grid.Columns[0].Elements[0])
			}
			if grid.Columns[1].Content != "Right side." {
				t.Fatalf("raw column = %q", grid.Columns[1].Content)
			}
		})
	}
}

// Un `## Heading` o un `---` dentro de una columna flex tipada no cortan el
// cuerpo: se quedan en la columna (como en una cruda), nunca pasan a la prosa
// suelta del grid sin avisar.
func TestTypedColumn_Flex_HeadingAndRuleStayInsideTheColumn(t *testing.T) {
	for name, body := range map[string]string{
		"heading": "Intro\n\n## Sub heading\nafter heading",
		"rule":    "Intro\n\n---\n\nafter rule",
		"h1":      "Intro\n\n# Top heading\n\nafter top",
	} {
		t.Run(name, func(t *testing.T) {
			src := typedColFlexHeader + "# Grid\n\n::: grid\n::: column typed\n" + body + "\n:::\n::: column\nRight\n:::\n:::\n"
			doc, diags := parseTypedCols(t, src)
			if countErrors(diags) != 0 {
				t.Fatalf("unexpected errors: %v", diags)
			}
			grid := firstGridOf(t, doc)
			if grid.Content != "" {
				t.Fatalf("text leaked into the grid's loose prose: %q", grid.Content)
			}
			if len(grid.Columns) != 2 {
				t.Fatalf("columns = %d, want 2", len(grid.Columns))
			}
			var all strings.Builder
			for _, el := range grid.Columns[0].Elements {
				if text, ok := el.(*ast.TextElement); ok {
					all.WriteString(text.Content + "\n")
				}
			}
			for _, want := range strings.Fields(strings.NewReplacer("#", "", "-", "").Replace(body)) {
				if !strings.Contains(all.String(), want) {
					t.Fatalf("word %q is not in the typed column: %v", want, grid.Columns[0].Elements)
				}
			}
			if grid.Columns[1].Content != "Right" {
				t.Fatalf("raw column = %q", grid.Columns[1].Content)
			}
		})
	}
}
