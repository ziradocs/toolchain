// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/linter"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
)

// tableDialect es un dialecto de entrada con su fmt: cómo se envuelve una
// tabla en un documento, cómo se lee y cómo se formatea.
type tableDialect struct {
	name    string
	slides  bool
	strict  bool // la entrada es strict (las filas | sin celdas se descartan)
	grid    bool // la tabla va en una columna tipada de un grid
	flexOut bool // fmt escribe flex (el de documento)
	// callout, si no está vacío, es el tipo del bloque ::: en el que va la
	// tabla, seguida de un párrafo adentro del bloque (`note`, `tip{typed}`).
	callout string
	format  func(*ast.AST) (string, error)
}

// wrap envuelve la tabla, escrita con TABLE en la columna 0, en un documento
// del dialecto, seguida de un párrafo: si fmt escribiera una tabla que se
// traga lo que sigue, el AST cambiaría.
func (d tableDialect) wrap(table string) string {
	if d.callout != "" {
		table = ":::" + d.callout + "\n" + table + "\nDespués.\n:::"
	}
	front := "---\nmode: strict\ntitle: A\n---\n"
	switch {
	case d.strict && d.grid && d.slides:
		return front + "SLIDE content\n  title: \"A\"\n  <<grid>>\n  <<column typed>>\n" + indent(table, 4) + "\n  <<end>>\n  TEXT\n    after\n"
	case d.strict && d.grid:
		return front + "SECTION \"A\"\n  <<grid>>\n  <<column typed>>\n" + indent(table, 4) + "\n  <<end>>\n  TEXT\n    after\n"
	case d.strict && d.slides:
		return front + "SLIDE content\n  title: \"A\"\n" + indent(table, 2) + "\n  TEXT\n    after\n"
	case d.strict:
		return front + "SECTION \"A\"\n" + indent(table, 2) + "\n  TEXT\n    after\n"
	case d.slides:
		return "---\nmode: flex\ntitle: A\n---\n# A\n\n" + table + "\n\nafter\n"
	default:
		return "---\ntitle: A\n---\n# A\n\n" + table + "\n\nafter\n"
	}
}

// rowsIndent es la sangría, dentro de la tabla, de las filas bajo rows: para
// que caigan a seis espacios del comienzo de la línea que lee el parser.
func (d tableDialect) rowsIndent() int {
	if d.strict && !strings.HasSuffix(d.callout, "{typed}") {
		return 4
	}
	// Flex, o un bloque strict {typed}, que lee su cuerpo como flex: la
	// tabla está a la sangría del bloque, que el parser le quita.
	return 6
}

func (d tableDialect) parse(src string) (*ast.AST, []diagnostics.Diagnostic) {
	p := parser.New(util.NewNoop())
	if d.slides {
		return p.Parse(src, "t.slidelang")
	}
	return p.ParseDocument(src, "t.doclang")
}

var tableDialects = []tableDialect{
	{name: "slidelang strict", slides: true, strict: true, format: FormatStrict},
	{name: "slidelang strict grid", slides: true, strict: true, grid: true, format: FormatStrict},
	{name: "slidelang flex", slides: true, format: FormatStrict},
	{name: "doclang strict", strict: true, format: FormatDocumentStrict},
	{name: "doclang strict grid", strict: true, grid: true, format: FormatDocumentStrict},
	{name: "doclang flex", flexOut: true, format: FormatDocument},
	{name: "doclang flex to strict", format: FormatDocumentStrict},
}

// calloutTableDialects ponen la tabla dentro de un bloque :::. Main escribía
// el cuerpo de un TABLE sin sangría (el bloque guarda sus líneas recortadas),
// y al releerlo la tabla salía vacía, con headers:/rows: como texto.
var calloutTableDialects = []tableDialect{
	{name: "slidelang strict :::note{typed}", slides: true, strict: true, callout: "note{typed}", format: FormatStrict},
	{name: "slidelang strict :::tip", slides: true, strict: true, callout: "tip", format: FormatStrict},
	{name: "slidelang flex :::note", slides: true, callout: "note", format: FormatStrict},
	{name: "doclang strict :::tip{typed}", strict: true, callout: "tip{typed}", format: FormatDocumentStrict},
	{name: "doclang strict :::note", strict: true, callout: "note", format: FormatDocumentStrict},
	{name: "doclang flex :::note", flexOut: true, callout: "note", format: FormatDocument},
	{name: "doclang flex :::tip", flexOut: true, callout: "tip", format: FormatDocument},
	{name: "doclang flex :::note to strict", callout: "note", format: FormatDocumentStrict},
}

// tableShape es una tabla generada: encabezado (nil = sin encabezado), filas,
// y caption/label.
type tableShape struct {
	headers        []string
	rows           [][]string
	caption, label string
}

func (s tableShape) String() string {
	return fmt.Sprintf("headers=%q rows=%q caption=%q label=%q", s.headers, s.rows, s.caption, s.label)
}

func hasEmptyRow(rows [][]string) bool {
	for _, r := range rows {
		if len(r) == 0 {
			return true
		}
	}
	return false
}

func generatedTableShapes() []tableShape {
	headerOpts := [][]string{nil, {""}, {"A"}, {"A", "B"}, {"", ""}}
	rowOpts := [][]string{{}, {""}, {"x"}, {"", ""}, {"x", "y"}, {"x", "y", "z"}}
	// Pares de filas: una fila sin celdas, de una celda vacía o de otro largo
	// junto a otra, en los dos órdenes.
	pairOpts := [][]string{{}, {""}, {"x", "y"}, {"x", "y", "z"}}
	var rowSets [][][]string
	rowSets = append(rowSets, [][]string{})
	for _, a := range rowOpts {
		rowSets = append(rowSets, [][]string{a})
	}
	for _, a := range pairOpts {
		for _, b := range pairOpts {
			rowSets = append(rowSets, [][]string{a, b})
		}
	}
	var shapes []tableShape
	for _, h := range headerOpts {
		for _, rs := range rowSets {
			for _, extra := range [][2]string{{"", ""}, {"Tabla", ""}, {"", "tbl:a"}} {
				shapes = append(shapes, tableShape{headers: h, rows: rs, caption: extra[0], label: extra[1]})
			}
		}
	}
	return shapes
}

// specialCellShapes son las tablas de generatedTableShapes (sin encabezado o
// con ["A", "B"], sin label) con la celda "x" cambiada por un contenido que
// alguna forma no lleva: una comilla doble (headers:/rows:), espacios en los
// bordes y una celda "---" (pipe), y un backslash al final.
func specialCellShapes() []tableShape {
	var shapes []tableShape
	for _, special := range []string{`Café "premium"`, " A ", "---", `a\`} {
		for _, sh := range generatedTableShapes() {
			if sh.label != "" || !(sh.headers == nil || len(sh.headers) == 2 && sh.headers[0] == "A") {
				continue
			}
			found := false
			rows := make([][]string, len(sh.rows))
			for i, r := range sh.rows {
				rows[i] = make([]string, len(r))
				for j, c := range r {
					if c == "x" {
						c = special
						found = true
					}
					rows[i][j] = c
				}
			}
			if found {
				shapes = append(shapes, tableShape{headers: sh.headers, rows: rows, caption: sh.caption})
			}
		}
	}
	return shapes
}

func shapeHas(s tableShape, f func(string) bool) bool {
	for _, c := range s.headers {
		if f(c) {
			return true
		}
	}
	for _, r := range s.rows {
		for _, c := range r {
			if f(c) {
				return true
			}
		}
	}
	return false
}

func hasQuote(c string) bool { return strings.Contains(c, `"`) }

func quotedCells(cells []string) string {
	q := make([]string, len(cells))
	for i, c := range cells {
		q[i] = `"` + c + `"`
	}
	return "[" + strings.Join(q, ", ") + "]"
}

func tableProps(s tableShape) string {
	var b strings.Builder
	if s.caption != "" {
		fmt.Fprintf(&b, "  caption: %q\n", s.caption)
	}
	if s.label != "" {
		fmt.Fprintf(&b, "  label: %q\n", s.label)
	}
	return b.String()
}

// tableSourceForm escribe una tableShape en una forma de entrada. ok es false
// si la forma no tiene cómo escribirla. want es la vista plana que el parser
// tiene que leer de ella, para que la prueba no pase en vacío por una tabla
// que nunca se leyó como se pensaba.
type tableSourceForm struct {
	name  string
	write func(s tableShape, d tableDialect) (src string, wantHeaders []string, wantRows [][]string, ok bool)
}

func yamlCell(content string, header bool) string {
	if header {
		return fmt.Sprintf("{content: %q, header: true, scope: col}", content)
	}
	return fmt.Sprintf("{content: %q}", content)
}

var tableSourceForms = []tableSourceForm{
	{"TABLE rows:", func(s tableShape, d tableDialect) (string, []string, [][]string, bool) {
		if shapeHas(s, hasQuote) {
			// headers:/rows: no tiene cómo escribir una comilla doble.
			return "", nil, nil, false
		}
		var b strings.Builder
		b.WriteString("TABLE\n" + tableProps(s))
		if s.headers != nil {
			b.WriteString("  headers: " + quotedCells(s.headers) + "\n")
		}
		b.WriteString("  rows:")
		for _, r := range s.rows {
			b.WriteString("\n" + strings.Repeat(" ", d.rowsIndent()) + quotedCells(r))
		}
		h := s.headers
		if h == nil {
			h = []string{}
		}
		return b.String(), h, s.rows, true
	}},
	{"TABLE cells:", func(s tableShape, _ tableDialect) (string, []string, [][]string, bool) {
		var cells [][]ast.TableCell
		var b strings.Builder
		b.WriteString("TABLE\n" + tableProps(s) + "  cells:")
		if s.headers != nil {
			var row []string
			var hc []ast.TableCell
			for _, h := range s.headers {
				row = append(row, yamlCell(h, true))
				hc = append(hc, ast.TableCell{Content: h, IsHeader: true})
			}
			cells = append(cells, hc)
			b.WriteString("\n    - [" + strings.Join(row, ", ") + "]")
		}
		for _, r := range s.rows {
			row := []string{}
			rc := []ast.TableCell{}
			for _, c := range r {
				row = append(row, yamlCell(c, false))
				rc = append(rc, ast.TableCell{Content: c})
			}
			cells = append(cells, rc)
			b.WriteString("\n    - [" + strings.Join(row, ", ") + "]")
		}
		h, rows := ast.FlattenCellsToRows(cells)
		return b.String(), h, rows, true
	}},
	{"TABLE tableRows:", func(s tableShape, _ tableDialect) (string, []string, [][]string, bool) {
		var b strings.Builder
		b.WriteString("TABLE\n" + tableProps(s) + "  tableRows:")
		var cells [][]ast.TableCell
		if s.headers != nil {
			var row []string
			var hc []ast.TableCell
			for _, h := range s.headers {
				row = append(row, fmt.Sprintf("{content: %q, header: true}", h))
				hc = append(hc, ast.TableCell{Content: h, IsHeader: true})
			}
			cells = append(cells, hc)
			b.WriteString("\n    - section: header\n      cells: [" + strings.Join(row, ", ") + "]")
		}
		for _, r := range s.rows {
			row := []string{}
			rc := []ast.TableCell{}
			for _, c := range r {
				row = append(row, yamlCell(c, false))
				rc = append(rc, ast.TableCell{Content: c})
			}
			cells = append(cells, rc)
			b.WriteString("\n    - cells: [" + strings.Join(row, ", ") + "]")
		}
		h, rows := ast.FlattenCellsToRows(cells)
		return b.String(), h, rows, len(cells) > 0
	}},
	{"Markdown", markdownTableForm(false)},
	{"Markdown aligned", markdownTableForm(true)},
}

// markdownTableForm escribe la tabla con pipes, con o sin alineación en la
// fila delimitadora.
func markdownTableForm(aligned bool) func(s tableShape, d tableDialect) (string, []string, [][]string, bool) {
	return func(s tableShape, d tableDialect) (string, []string, [][]string, bool) {
		if s.headers == nil || s.caption != "" || s.label != "" {
			return "", nil, nil, false
		}
		if shapeHas(s, func(c string) bool { return c != strings.TrimSpace(c) || strings.Contains(c, "---") }) {
			// La forma pipe recorta los bordes de una celda, y flex salta
			// como separador una fila con "---".
			return "", nil, nil, false
		}
		pipeRow := func(cells []string) string {
			if len(cells) == 0 {
				return "|"
			}
			return "| " + strings.Join(cells, " | ") + " |"
		}
		var b strings.Builder
		b.WriteString(pipeRow(s.headers) + "\n|")
		for i := range s.headers {
			sep := "---"
			if aligned {
				sep = []string{":---", ":---:", "---:"}[i%3]
			}
			b.WriteString(sep + "|")
		}
		rows := [][]string{}
		for _, r := range s.rows {
			b.WriteString("\n" + pipeRow(r))
			// Strict descarta la fila "|" (sin celdas) al leerla. Dentro de un
			// bloque ::: la tabla la lee el parser de elementos, que la deja.
			if len(r) > 0 || !d.strict || d.callout != "" {
				rows = append(rows, r)
			}
		}
		return b.String(), s.headers, rows, true
	}
}

func firstTable(doc *ast.AST) *ast.TableElement {
	var found *ast.TableElement
	_ = ast.Walk(doc, func(n ast.Node) error {
		if t, ok := n.(*ast.TableElement); ok && found == nil {
			found = t
		}
		return nil
	})
	return found
}

func hasErrorDiag(diags []diagnostics.Diagnostic) (diagnostics.Diagnostic, bool) {
	for _, d := range diags {
		if d.IsError() {
			return d, true
		}
	}
	return diagnostics.Diagnostic{}, false
}

// isAlignmentRefusal dice si err es el rechazo de una tabla alineada con una
// fila sin celdas cuando fmt escribe strict.
func isAlignmentRefusal(err error) bool {
	var unsupported *UnsupportedElementError
	return errors.As(err, &unsupported) && strings.Contains(err.Error(), "column alignment and a row without cells")
}

// newRuleIDs devuelve los RuleID de after que before no tiene.
func newRuleIDs(before, after []diagnostics.Diagnostic) []string {
	seen := map[string]bool{}
	for _, d := range before {
		seen[d.RuleID] = true
	}
	var extra []string
	for _, d := range after {
		if !seen[d.RuleID] {
			extra = append(extra, fmt.Sprintf("%s %s", d.RuleID, d.Message))
			seen[d.RuleID] = true
		}
	}
	return extra
}

func jsonOf(t *testing.T, v interface{}) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// La propiedad de ida y vuelta de fmt sobre tablas generadas: para cada
// entrada que fmt acepta, releer lo que fmt escribe da el mismo AST (sin
// posiciones) sin ningún aviso nuevo del parser, y fmt es idempotente. Las
// tablas combinan encabezados vacíos, sin encabezado, filas sin celdas, filas
// de celdas vacías, filas de otro largo que el encabezado (TABLE003) y
// caption/label, escritas en headers:/rows:, cells:, tableRows: y Markdown
// (con y sin alineación), en los dos dialectos de los dos lenguajes (y en una
// columna tipada de un grid en strict). La única tabla que fmt rechaza es la
// alineada con una fila sin celdas cuando escribe strict.
//
// En main falla: una fila sin celdas se relee como [""], una tabla sin
// encabezado gana un encabezado y una fila [""], una tabla vacía con caption
// se relee con TABLE004, y con caption/label una fila de otro largo se
// rellena hasta el ancho de la más larga.
func TestFormat_GeneratedTablesRoundTrip(t *testing.T) {
	shapes := generatedTableShapes()
	// Dentro de un bloque ::: basta un subconjunto: lo que se prueba ahí es la
	// sangría del cuerpo de la tabla, no cada forma de tabla otra vez.
	var calloutShapes []tableShape
	for _, sh := range shapes {
		if sh.label == "" && (sh.headers == nil || len(sh.headers) == 2 && sh.headers[0] == "A") {
			calloutShapes = append(calloutShapes, sh)
		}
	}
	specials := specialCellShapes()
	refused, total := 0, 0
	for _, d := range tableDialects {
		t.Run(d.name, func(t *testing.T) {
			r, n := roundTripGeneratedTables(t, d, append(append([]tableShape{}, shapes...), specials...))
			refused += r
			total += n
		})
	}
	for _, d := range calloutTableDialects {
		t.Run(d.name, func(t *testing.T) {
			r, n := roundTripGeneratedTables(t, d, calloutShapes)
			refused += r
			total += n
		})
	}
	t.Logf("%d generated tables round-tripped, %d refused", total, refused)
}

// isArraysQuoteRefusal dice si err es el rechazo de una tabla que solo cabe en
// headers:/rows: y tiene una celda con una comilla doble.
func isArraysQuoteRefusal(err error) bool {
	var unsupported *UnsupportedElementError
	return errors.As(err, &unsupported) && strings.Contains(err.Error(), "can only be written as TABLE with headers: and rows:") && strings.Contains(err.Error(), "double quote")
}

// roundTripGeneratedTables devuelve cuántas tablas fmt rechazó con razón y
// cuántas pasaron.
func roundTripGeneratedTables(t *testing.T, d tableDialect, shapes []tableShape) (refused, total int) {
	accepted := map[string]int{}
	exercised := map[string]bool{}
	failures := 0
	for _, form := range tableSourceForms {
		for _, shape := range shapes {
			if failures > 10 {
				return refused, total
			}
			table, wantHeaders, wantRows, ok := form.write(shape, d)
			if !ok {
				continue
			}
			src := d.wrap(table)
			doc, diags := d.parse(src)
			if e, bad := hasErrorDiag(diags); bad {
				// fmt rechaza un archivo con errores del parser. Solo
				// tableRows: puede darlos acá (una grilla que no cubre sus
				// columnas, o una fila sin celdas).
				if form.name != "TABLE tableRows:" {
					t.Errorf("%s / %s: the generated source does not parse: %s\n%s", form.name, shape, e.String(), src)
					failures++
				}
				continue
			}
			got := firstTable(doc)
			if got == nil {
				t.Errorf("%s / %s: no table was read from\n%s", form.name, shape, src)
				failures++
				continue
			}
			if jsonOf(t, got.Headers) != jsonOf(t, wantHeaders) || jsonOf(t, got.Rows) != jsonOf(t, wantRows) {
				t.Errorf("%s / %s: the source was read as headers=%s rows=%s, want headers=%s rows=%s\n%s",
					form.name, shape, jsonOf(t, got.Headers), jsonOf(t, got.Rows), jsonOf(t, wantHeaders), jsonOf(t, wantRows), src)
				failures++
				continue
			}
			// La alineación solo se escribe en una tabla pipe, y strict
			// descarta la fila "|": una tabla alineada con una fila sin
			// celdas (solo se lee así en flex) no tiene forma strict. Dentro
			// de un bloque ::: la tabla va en el texto del bloque, como la
			// escribió el autor.
			wantRefusal := len(got.ColumnAlign) > 0 && hasEmptyRow(got.Rows) && !d.flexOut && d.callout == ""
			once, err := d.format(doc)
			if wantRefusal {
				if !isAlignmentRefusal(err) {
					t.Errorf("%s / %s: want fmt to refuse the alignment, got %v\n%s", form.name, shape, err, src)
					failures++
				} else {
					exercised["an aligned table with a row without cells, refused"] = true
					refused++
				}
				continue
			}
			if err != nil && isArraysQuoteRefusal(err) && shapeHas(tableShape{headers: got.Headers, rows: got.Rows}, hasQuote) {
				// Una tabla que solo cabe en headers:/rows: (sin encabezado y
				// con una fila de otro largo o sin celdas) y tiene una comilla
				// doble no tiene forma.
				refused++
				if len(got.Headers) > 0 {
					t.Errorf("%s / %s: a table with a header was refused for its quote: %v\n%s", form.name, shape, err, src)
					failures++
				}
				continue
			}
			if err != nil {
				// Ninguna otra celda generada tiene algo que fmt no pueda
				// escribir.
				t.Errorf("%s / %s: fmt refused the table: %v\n%s", form.name, shape, err, src)
				failures++
				continue
			}
			doc2, diags2 := d.parse(once)
			if e, bad := hasErrorDiag(diags2); bad {
				t.Errorf("%s / %s: the fmt output does not parse: %s\n--- input ---\n%s\n--- fmt ---\n%s", form.name, shape, e.String(), src, once)
				failures++
				continue
			}
			// Una fila que el parser deja afuera da un aviso (TABLE008, por
			// ejemplo) sin cambiar el AST de lo que sí leyó: el aviso tampoco
			// puede aparecer por primera vez después de fmt.
			if extra := newRuleIDs(diags, diags2); len(extra) > 0 {
				t.Errorf("%s / %s: the fmt output gives diagnostics the input did not: %v\n--- input ---\n%s\n--- fmt ---\n%s", form.name, shape, extra, src, once)
				failures++
				continue
			}
			if !reflect.DeepEqual(stripPositions(doc.ContentBlocks), stripPositions(doc2.ContentBlocks)) {
				t.Errorf("%s / %s: the AST changed through fmt\n before: %s\n after:  %s\n--- input ---\n%s\n--- fmt ---\n%s",
					form.name, shape, jsonOf(t, stripPositions(firstTable(doc))), jsonOf(t, stripPositions(firstTable(doc2))), src, once)
				failures++
				continue
			}
			twice, err := d.format(doc2)
			if err != nil || twice != once {
				t.Errorf("%s / %s: fmt is not idempotent (%v)\n--- once ---\n%s\n--- twice ---\n%s", form.name, shape, err, once, twice)
				failures++
				continue
			}
			accepted[form.name]++
			if hasEmptyRow(got.Rows) {
				exercised["a row without cells"] = true
			}
			if len(got.Headers) == 0 {
				exercised["no headers"] = true
			}
			if len(got.Headers) > 0 && (got.Caption != "" || got.Label != "") {
				for _, r := range got.Rows {
					if len(r) != len(got.Headers) {
						exercised["a miscounted row with caption or label"] = true
					}
				}
			}
		}
	}
	if failures > 0 {
		return refused, total
	}
	for _, form := range tableSourceForms {
		n := accepted[form.name]
		total += n
		if n < 20 {
			t.Errorf("%s: only %d generated tables went through fmt", form.name, n)
		}
	}
	whats := []string{"a row without cells", "no headers", "a miscounted row with caption or label"}
	if !d.strict && !d.flexOut && d.callout == "" {
		whats = append(whats, "an aligned table with a row without cells, refused")
	}
	for _, what := range whats {
		if !exercised[what] {
			t.Errorf("no generated table had %s", what)
		}
	}
	t.Logf("%d generated tables round-tripped through fmt, %d refused", total, refused)
	return refused, total
}

// Cada caso que main reescribía con otro AST, con la forma que fmt escribe
// ahora.
func TestFormat_TableEmptyRowsRegressions(t *testing.T) {
	strictSlide := tableDialects[0]
	flexDoc := tableDialects[5]
	for _, tc := range []struct {
		name  string
		d     tableDialect
		table string
		want  string // la tabla que escribe fmt, en la columna 0
	}{
		{
			name:  "row without cells under headers",
			d:     strictSlide,
			table: "TABLE\n  headers: [\"A\", \"B\"]\n  rows:\n    [\"1\", \"2\"]\n    []",
			want:  "TABLE\n  | A | B |\n  |---|---|\n  | 1 | 2 |\n  |",
		},
		{
			name:  "headerless table",
			d:     strictSlide,
			table: "TABLE\n  rows:\n    [\"1\"]\n    [\"2\", \"3\"]",
			want:  "TABLE\n  rows:\n    [\"1\"]\n    [\"2\", \"3\"]",
		},
		{
			name:  "headerless table with a row without cells",
			d:     strictSlide,
			table: "TABLE\n  rows:\n    []",
			want:  "TABLE\n  rows:\n    []",
		},
		{
			name:  "headerless rectangular table keeps cells:",
			d:     strictSlide,
			table: "TABLE\n  rows:\n    [\"1\"]\n    [\"2\"]",
			want:  "TABLE\n  cells:\n    - [{content: \"1\"}]\n    - [{content: \"2\"}]",
		},
		{
			name:  "empty table",
			d:     strictSlide,
			table: "TABLE\n  rows:",
			want:  "TABLE\n  rows:",
		},
		{
			name:  "caption and a row with more cells than headers",
			d:     strictSlide,
			table: "TABLE\n  caption: \"Ventas\"\n  headers: [\"A\", \"B\"]\n  rows:\n    [\"1\", \"2\", \"3\"]\n    [\"4\", \"5\"]",
			want:  "TABLE\n  caption: \"Ventas\"\n  | A | B |\n  |---|---|\n  | 1 | 2 | 3 |\n  | 4 | 5 |",
		},
		{
			name:  "label and a row without cells",
			d:     strictSlide,
			table: "TABLE\n  label: \"tbl:a\"\n  headers: [\"A\"]\n  rows:\n    [\"1\"]\n    []",
			want:  "TABLE\n  label: \"tbl:a\"\n  | A |\n  |---|\n  | 1 |\n  |",
		},
		{
			name:  "caption and rectangular rows keep cells:",
			d:     strictSlide,
			table: "TABLE\n  caption: \"Ventas\"\n  headers: [\"A\"]\n  rows:\n    [\"1\"]",
			want:  "TABLE\n  caption: \"Ventas\"\n  cells:\n    - [{content: A, header: true, scope: col}]\n    - [{content: \"1\"}]",
		},
		{
			name:  "Markdown row without cells in flex",
			d:     flexDoc,
			table: "| A |\n|---|\n| 1 |\n|",
			want:  "| A |\n|---|\n| 1 |\n|",
		},
		{
			name:  "Markdown row of empty cells stays a pipe table",
			d:     flexDoc,
			table: "| A | B |\n|---|---|\n|  |  |",
			want:  "| A | B |\n|---|---|\n|  |  |",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc, diags := tc.d.parse(tc.d.wrap(tc.table))
			if e, bad := hasErrorDiag(diags); bad {
				t.Fatalf("fixture does not parse: %s", e.String())
			}
			out, err := tc.d.format(doc)
			if err != nil {
				t.Fatalf("fmt: %v", err)
			}
			want := tc.want
			if tc.d.strict {
				want = indent(want, 2)
			}
			if !strings.Contains(out, want+"\n") {
				t.Errorf("fmt wrote\n%s\nwant the table\n%s", out, want)
			}
			doc2, _ := tc.d.parse(out)
			if !reflect.DeepEqual(stripPositions(firstTable(doc)), stripPositions(firstTable(doc2))) {
				t.Errorf("the table changed through fmt:\n before: %s\n after:  %s", jsonOf(t, stripPositions(firstTable(doc))), jsonOf(t, stripPositions(firstTable(doc2))))
			}
		})
	}
}

// Una celda que headers:/rows: no puede escribir se rechaza en vez de
// cambiar la tabla, y el mensaje dice qué tabla, qué la obliga a esa forma y
// cómo arreglarlo. Una tabla sin encabezado con una fila de otro largo solo
// cabe ahí; con encabezado, TABLE con filas "|" lleva la comilla.
func TestFormat_TableArraysFormRefusesUnquotableCells(t *testing.T) {
	for _, d := range []tableDialect{tableDialects[0], tableDialects[5]} {
		for name, tc := range map[string]struct {
			cell string
			want []string
		}{
			"double quote": {`say "hi"`, []string{"without a header whose first row is | say \"hi\" |", "no header row", "the row | 1 | 2 | has 2 cells where the first row has 1", "double quote", "add a header row"}},
			"line break":   {"a\nb", []string{"no header row", "line break"}},
		} {
			t.Run(d.name+"/"+name, func(t *testing.T) {
				doc, diags := d.parse(d.wrap(fmt.Sprintf("TABLE\n  rows:\n%[1]s[\"x\"]\n%[1]s[\"1\", \"2\"]", strings.Repeat(" ", d.rowsIndent()))))
				if e, bad := hasErrorDiag(diags); bad {
					t.Fatalf("fixture does not parse: %s", e.String())
				}
				tbl := firstTable(doc)
				tbl.Rows[0][0] = tc.cell
				tbl.Cells = ast.DeriveCellsFromFlat(tbl.Headers, tbl.Rows)
				_, err := d.format(doc)
				var unsupported *UnsupportedElementError
				if !errors.As(err, &unsupported) {
					t.Fatalf("want UnsupportedElementError, got %v", err)
				}
				for _, w := range tc.want {
					if !strings.Contains(err.Error(), w) {
						t.Errorf("the message does not say %q:\n%v", w, err)
					}
				}
			})
		}
	}
}

// Con encabezado, una fila de otro largo y una comilla doble se escriben en
// TABLE con filas "|", como las escribió el autor (main rellenaba la fila).
func TestFormat_CaptionedRaggedTableWithQuote(t *testing.T) {
	const table = "TABLE\n  caption: \"Ventas\"\n  | Producto | Nota |\n  |---|---|\n  | Café \"premium\" |\n  | Té | ok |"
	for _, d := range tableDialects {
		t.Run(d.name, func(t *testing.T) {
			doc, diags := d.parse(d.wrap(table))
			if e, bad := hasErrorDiag(diags); bad {
				t.Fatalf("fixture does not parse: %s", e.String())
			}
			out, err := d.format(doc)
			if err != nil {
				t.Fatalf("fmt: %v", err)
			}
			doc2, _ := d.parse(out)
			if !reflect.DeepEqual(stripPositions(doc.ContentBlocks), stripPositions(doc2.ContentBlocks)) {
				t.Errorf("the AST changed through fmt\n%s", out)
			}
			if !strings.Contains(out, "| Café \"premium\" |\n") {
				t.Errorf("fmt did not keep the row as written:\n%s", out)
			}
		})
	}
}

// Una tabla Markdown alineada con una fila "|" (sin celdas), que solo se lee
// así en flex: el flex de documento la escribe igual, con la alineación y la
// fila "|". Strict no tiene forma para las dos cosas juntas (la alineación
// solo va en una tabla pipe y strict descarta la fila "|"), así que fmt la
// rechaza en vez de perder una de las dos, como main.
func TestFormat_AlignedTableWithRowWithoutCells(t *testing.T) {
	const table = "| A | B |\n|:---|---:|\n| 1 | 2 |\n|"
	for _, d := range tableDialects {
		if d.strict {
			continue
		}
		t.Run(d.name, func(t *testing.T) {
			doc, diags := d.parse(d.wrap(table))
			if e, bad := hasErrorDiag(diags); bad {
				t.Fatalf("fixture does not parse: %s", e.String())
			}
			if tbl := firstTable(doc); len(tbl.ColumnAlign) != 2 || len(tbl.Rows) != 2 || len(tbl.Rows[1]) != 0 {
				t.Fatalf("fixture: %s", jsonOf(t, stripPositions(tbl)))
			}
			out, err := d.format(doc)
			if !d.flexOut {
				if !isAlignmentRefusal(err) {
					t.Fatalf("want fmt to refuse the alignment, got %v\n%s", err, out)
				}
				return
			}
			if err != nil {
				t.Fatalf("fmt: %v", err)
			}
			if !strings.Contains(out, table+"\n") {
				t.Errorf("fmt wrote\n%s\nwant the table\n%s", out, table)
			}
			doc2, _ := d.parse(out)
			if !reflect.DeepEqual(stripPositions(firstTable(doc)), stripPositions(firstTable(doc2))) {
				t.Errorf("the table changed through fmt")
			}
		})
	}
}

func table003(doc *ast.AST) int {
	n := 0
	for _, d := range linter.New().LintUnfiltered(doc) {
		if d.RuleID == "TABLE003" {
			n++
		}
	}
	return n
}

// TABLE003 es un error del linter, y fmt no corre el linter (ni con este ni
// con ningún otro error del linter): solo rechaza los errores del parser. Lo
// que fmt no puede hacer es cambiar la fila a la que apunta. Con caption o
// label, main escribía cells:, que rellena las filas hasta el ancho de la más
// larga: el TABLE003 desaparecía y la tabla ganaba celdas vacías. Y una fila
// sin celdas bajo un encabezado de una columna volvía como [""]: un TABLE003
// que desaparecía en silencio.
func TestFormat_TableWithTABLE003KeepsTheMiscountedRow(t *testing.T) {
	for _, d := range tableDialects {
		if d.grid {
			// El linter no mira dentro de una columna de grid: ahí no hay
			// TABLE003 que conservar (la tabla la cubre la propiedad).
			continue
		}
		for name, table := range map[string]string{
			"caption":           "TABLE\n  caption: \"Ventas\"\n  headers: [\"A\", \"B\"]\n  rows:\n%[1]s[\"1\", \"2\", \"3\"]\n%[1]s[\"4\", \"5\"]",
			"label":             "TABLE\n  label: \"tbl:a\"\n  headers: [\"A\", \"B\"]\n  rows:\n%[1]s[\"4\", \"5\"]\n%[1]s[\"6\"]",
			"row without cells": "TABLE\n  headers: [\"A\"]\n  rows:\n%[1]s[\"1\"]\n%[1]s[]",
		} {
			t.Run(d.name+"/"+name, func(t *testing.T) {
				src := d.wrap(fmt.Sprintf(table, strings.Repeat(" ", d.rowsIndent())))
				doc, diags := d.parse(src)
				if e, bad := hasErrorDiag(diags); bad {
					t.Fatalf("fixture does not parse: %s", e.String())
				}
				before := stripPositions(firstTable(doc))
				if table003(doc) != 1 {
					t.Fatalf("fixture: want one TABLE003, got %d", table003(doc))
				}
				out, err := d.format(doc)
				if err != nil {
					t.Fatalf("fmt: %v", err)
				}
				doc2, _ := d.parse(out)
				if n := table003(doc2); n != 1 {
					t.Errorf("after fmt: %d TABLE003, want 1\n%s", n, out)
				}
				if after := stripPositions(firstTable(doc2)); !reflect.DeepEqual(before, after) {
					t.Errorf("the table changed through fmt:\n before: %s\n after:  %s\n%s", jsonOf(t, before), jsonOf(t, after), out)
				}
			})
		}
	}
}

// Un bloque ::: guarda sus líneas recortadas (Content), y fmt las escribía
// así: el cuerpo de un TABLE perdía la sangría y al releerse la tabla salía
// vacía, con headers:/rows: como texto. En el flex de documento, además, una
// cerca de código con sangría se escribía con {typed}, que en flex es parte
// del tipo: el bloque :::note pasaba a ser :::note{typed}.
func TestFormat_CalloutNestedIndentation(t *testing.T) {
	flexDoc := "---\ntitle: A\n---\n# A\n\n%s\n"
	flexSlides := "---\nmode: flex\ntitle: A\n---\n# A\n\n%s\n"
	for _, tc := range []struct {
		name   string
		src    string
		slides bool
		format func(*ast.AST) (string, error)
		want   string
	}{
		{
			name:   "TABLE with prose after it, doclang flex",
			src:    fmt.Sprintf(flexDoc, ":::note\nTABLE\n  caption: \"Ventas\"\n  headers: [\"A\", \"B\"]\n  rows:\n      [\"1\", \"2\"]\nDespués.\n:::"),
			format: FormatDocument,
			want:   ":::note\nTABLE\n  caption: \"Ventas\"\n  headers: [\"A\", \"B\"]\n  rows:\n      [\"1\", \"2\"]\nDespués.\n:::\n",
		},
		{
			name:   "TABLE cells:, doclang flex",
			src:    fmt.Sprintf(flexDoc, ":::tip\nTABLE\n  cells:\n    - [{content: A, header: true, colspan: 2}]\n    - [{content: \"1\"}, {content: \"2\"}]\n:::"),
			format: FormatDocument,
			want:   ":::tip\nTABLE\n  cells:\n    - [{content: A, header: true, colspan: 2}]\n    - [{content: \"1\"}, {content: \"2\"}]\n:::\n",
		},
		{
			name:   "TABLE tableRows:, doclang flex",
			src:    fmt.Sprintf(flexDoc, ":::note\nTABLE\n  tableRows:\n    - section: header\n      cells: [{content: A, header: true}]\n    - cells: [{content: \"1\"}]\n:::"),
			format: FormatDocument,
			want:   ":::note\nTABLE\n  tableRows:\n    - section: header\n      cells: [{content: A, header: true}]\n    - cells: [{content: \"1\"}]\n:::\n",
		},
		{
			name:   "fenced code with indentation, doclang flex",
			src:    fmt.Sprintf(flexDoc, ":::note\n```python\ndef f():\n    return 1\n```\n:::"),
			format: FormatDocument,
			want:   ":::note\n```python\ndef f():\n    return 1\n```\n:::\n",
		},
		{
			name:   "TABLE, doclang flex to strict",
			src:    fmt.Sprintf(flexDoc, ":::note\nTABLE\n  headers: [\"A\"]\n  rows:\n      [\"1\"]\n:::"),
			format: FormatDocumentStrict,
			want:   "  :::note{typed}\n  TABLE\n    headers: [\"A\"]\n    rows:\n        [\"1\"]\n  :::\n",
		},
		{
			name:   "TABLE, slidelang flex",
			src:    fmt.Sprintf(flexSlides, ":::note\nTABLE\n  headers: [\"A\"]\n  rows:\n      [\"1\"]\n:::"),
			slides: true,
			format: FormatStrict,
			want:   "  :::note{typed}\n  TABLE\n    headers: [\"A\"]\n    rows:\n        [\"1\"]\n  :::\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := tableDialect{slides: tc.slides}
			doc, diags := d.parse(tc.src)
			if e, bad := hasErrorDiag(diags); bad {
				t.Fatalf("fixture does not parse: %s", e.String())
			}
			out, err := tc.format(doc)
			if err != nil {
				t.Fatalf("fmt: %v", err)
			}
			if !strings.Contains(out, tc.want) {
				t.Errorf("fmt wrote\n%s\nwant the block\n%s", out, tc.want)
			}
			d.slides = tc.slides
			doc2, _ := d.parse(out)
			if !reflect.DeepEqual(stripPositions(doc.ContentBlocks), stripPositions(doc2.ContentBlocks)) {
				t.Errorf("the AST changed through fmt\n before: %s\n after:  %s", jsonOf(t, stripPositions(doc.ContentBlocks)), jsonOf(t, stripPositions(doc2.ContentBlocks)))
			}
		})
	}
}
