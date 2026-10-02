// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
)

// Issue #373: columnas de grid tipadas en el formatter strict. Diseño en
// docs/portable-typed-columns.md.

const typedColumnsSource = "---\nmode: strict\n---\n\n" +
	"SLIDE content\n" +
	"  title: \"Grid\"\n" +
	"  <!-- node-id: TheGrid -->\n" +
	"  <<grid>>\n" +
	"  <!-- node-id: LeftCol -->\n" +
	"  <<column typed>>\n" +
	"    <!-- node-id: ColTextA -->\n" +
	"    TEXT\n" +
	"      Left side.\n" +
	"    <!-- node-id: ColPoints -->\n" +
	"    POINTS\n" +
	"      - one\n" +
	"      - two\n" +
	"    CODE go\n" +
	"      fmt.Println(1)\n" +
	"    <<chart: bar>>\n" +
	"      data: [\n" +
	"        [\"Q1\", 45],\n" +
	"        [\"Q2\", 52]\n" +
	"      ]\n" +
	"    <<end>>\n" +
	"  <<column>>\n" +
	"  Right side.\n" +
	"  <<end>>\n" +
	"  TEXT\n" +
	"    After.\n"

// scrubbedJSON serializa el AST sin posiciones, conservando nodeId: lo que el
// formatter promete es que la identidad y la estructura ven la ida y vuelta.
func scrubbedJSON(t *testing.T, doc *ast.AST) any {
	t.Helper()
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatal(err)
	}
	var scrub func(any)
	scrub = func(x any) {
		switch n := x.(type) {
		case map[string]any:
			delete(n, "position")
			delete(n, "endPosition")
			delete(n, "rowPositions")
			for _, c := range n {
				scrub(c)
			}
		case []any:
			for _, c := range n {
				scrub(c)
			}
		}
	}
	scrub(v)
	return v
}

func TestFormatStrict_TypedColumnRoundTripsWithIdentities(t *testing.T) {
	doc := parseIdentityFixture(t, typedColumnsSource)
	wantIDs := map[string]ast.NodeType{
		"TheGrid": ast.NodeTypeGrid, "LeftCol": ast.NodeTypeColumn,
		"ColTextA": ast.NodeTypeText, "ColPoints": ast.NodeTypePoints,
	}
	if got := ids(doc); !reflect.DeepEqual(got, wantIDs) {
		t.Fatalf("parsed IDs = %v, want %v", got, wantIDs)
	}

	formatted, err := FormatStrict(doc)
	if err != nil {
		t.Fatalf("FormatStrict: %v", err)
	}
	if !strings.Contains(formatted, "<<column typed>>") {
		t.Fatalf("typed column was not written as <<column typed>>:\n%s", formatted)
	}

	parsed := parseIdentityFixture(t, formatted)
	if !reflect.DeepEqual(scrubbedJSON(t, doc), scrubbedJSON(t, parsed)) {
		t.Fatalf("parse, fmt, parse changed the AST (nodeId included)\n%s", formatted)
	}

	again, err := FormatStrict(parsed)
	if err != nil {
		t.Fatalf("second FormatStrict: %v", err)
	}
	if again != formatted {
		t.Fatalf("fmt is not idempotent:\n%s\n---\n%s", formatted, again)
	}
}

// Un grid con una columna tipada y una cruda las conserva a las dos, cada una
// con su forma.
func TestFormatStrict_TypedAndRawColumnsKeepTheirForm(t *testing.T) {
	doc := parseIdentityFixture(t, typedColumnsSource)
	formatted, err := FormatStrict(doc)
	if err != nil {
		t.Fatalf("FormatStrict: %v", err)
	}
	if !strings.Contains(formatted, "<<column>>\n  Right side.") {
		t.Fatalf("raw column lost its form:\n%s", formatted)
	}
	grid := doc.ContentBlocks[0].Elements[0].(*ast.GridElement)
	if len(grid.Columns[0].Elements) != 4 || grid.Columns[0].Content != "" {
		t.Fatalf("typed column: content=%q elements=%d", grid.Columns[0].Content, len(grid.Columns[0].Elements))
	}
	if grid.Columns[1].Content != "Right side." || len(grid.Columns[1].Elements) != 0 {
		t.Fatalf("raw column: %+v", grid.Columns[1])
	}
}

// Una columna flex tipada (el caso del issue: la forma con elementos que no
// tenía representación strict) se formatea a strict y, reparseada, da los
// mismos elementos.
func TestFormatStrict_FlexTypedColumnFormatsToStrict(t *testing.T) {
	src := "---\nmode: flex\n---\n# Grid\n\n::: grid\n::: column typed\n<!-- node-id: ColTextA -->\nLeft side.\n\n- one\n- two\n:::\n::: column\nRight side.\n:::\n:::\n"
	p := parser.New(util.NewNoop())
	p.SetNormalization(false)
	flexDoc, diags := p.Parse(src, "typed.slidelang")
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("flex parse: %v", d)
		}
	}
	formatted, err := FormatStrict(flexDoc)
	if err != nil {
		t.Fatalf("FormatStrict of a flex typed column: %v", err)
	}
	strictDoc := parseIdentityFixture(t, formatted)
	// Se compara el grid, no el documento: la primera `# ` de flex es un
	// slide "title" y strict lo escribe como tal, una diferencia ajena a
	// las columnas.
	gridOf := func(doc *ast.AST) *ast.AST {
		for _, b := range doc.ContentBlocks {
			for _, el := range b.Elements {
				if g, ok := el.(*ast.GridElement); ok {
					return &ast.AST{ContentBlocks: []ast.ContentBlock{{Elements: []ast.Element{g}}}}
				}
			}
		}
		t.Fatal("no grid in the document")
		return nil
	}
	if !reflect.DeepEqual(scrubbedJSON(t, gridOf(flexDoc)), scrubbedJSON(t, gridOf(strictDoc))) {
		t.Fatalf("flex to strict changed the grid\n%s", formatted)
	}
	if got := ids(strictDoc)["ColTextA"]; got != ast.NodeTypeText {
		t.Fatalf("identity did not survive as a text node: %v\n%s", got, formatted)
	}
}

// Un AST hecho a mano con Elements (la otra cara del issue) también se
// escribe.
func TestFormatStrict_HandBuiltTypedColumnIsWritten(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	doc := ast.NewAST(pos)
	block := ast.NewContentBlock(pos, "content")
	grid := ast.NewGridElement(pos)
	col := ast.NewColumnElement(pos, "")
	col.Elements = append(col.Elements, ast.NewTextElement(pos, "typed nested text"))
	grid.Columns = append(grid.Columns, *col)
	block.Elements = append(block.Elements, grid)
	doc.ContentBlocks = append(doc.ContentBlocks, *block)

	out, err := FormatStrict(doc)
	if err != nil {
		t.Fatalf("FormatStrict: %v", err)
	}
	if !strings.Contains(out, "<<column typed>>\n") || !strings.Contains(out, "typed nested text") {
		t.Fatalf("typed column not written:\n%s", out)
	}
}

func TestFormatStrict_TypedColumnLimitsFailClosed(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	build := func(col *ast.ColumnElement) *ast.AST {
		doc := ast.NewAST(pos)
		block := ast.NewContentBlock(pos, "content")
		grid := ast.NewGridElement(pos)
		grid.Columns = append(grid.Columns, *col)
		block.Elements = append(block.Elements, grid)
		doc.ContentBlocks = append(doc.ContentBlocks, *block)
		return doc
	}

	nested := ast.NewColumnElement(pos, "")
	nested.Elements = []ast.Element{ast.NewGridElement(pos)}

	heading := ast.NewColumnElement(pos, "")
	heading.Elements = []ast.Element{ast.NewHeadingElement(pos, 3, "Col", "heading-col")}

	rawMarker := ast.NewColumnElement(pos, "before\n<<column typed>>\nafter")

	for name, col := range map[string]*ast.ColumnElement{
		"nested grid":              nested,
		"typed heading":            heading,
		"raw text equal to marker": rawMarker,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := FormatStrict(build(col))
			if err == nil {
				t.Fatal("FormatStrict accepted a column that strict cannot represent")
			}
			var uerr *UnsupportedElementError
			if reflect.TypeOf(err) != reflect.TypeOf(uerr) {
				t.Fatalf("error type = %T, want *UnsupportedElementError: %v", err, err)
			}
		})
	}
}
