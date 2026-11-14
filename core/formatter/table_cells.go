// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"fmt"
	"reflect"
	"strings"

	"go.yaml.in/yaml/v3"
	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
)

// formatTableElement serializes a TableElement for either dialect. The
// underlying TABLE-block parser (elements.TableParser) is shared and not
// mode-gated (issue #20 widened its CanParse to flex, and its
// caption:/label:/cells: handling was never mode-gated to begin with), so
// caption/label/cells are representable in both strict and flex — only the
// choice of which form to emit depends on content, not on dialect.
//
// Using the pipe form when it isn't warranted would silently drop
// information: Label has no representation outside the TABLE block, and a
// Cells shape richer than what Headers/Rows already encode (row-scoped
// headers, colspan, rowspan) would be flattened away — precisely the
// accessibility-relevant structure issue #20 exists to carry. This was a
// real bug found in code review: doclang's formatter emitted the pipe form
// whenever Caption was empty, silently dropping Label and any non-trivial
// Cells structure on `fmt --write`.
//
// flex es el dialecto de destino: decide cómo se relee cada forma candidata
// y la sangría de las filas bajo rows: (ver strictTableRowsIndent).
//
// Una tabla sin estructura propia en Cells se escribe en la primera forma que,
// releída en el dialecto de destino, da la misma tabla: pipe (sin caption ni
// label), cells:, TABLE con caption/label y filas "|", y headers:/rows:.
// Cada forma pierde algo distinto: pipe recorta los espacios de los bordes de
// una celda y no tiene encabezado vacío; cells: rellena las filas hasta un
// rectángulo; TABLE con filas "|" no lleva alineación ni tabla sin
// encabezado; headers:/rows: no lleva una comilla doble. Por eso se elige
// releyendo y no por la forma de la tabla.
func formatTableElement(e *ast.TableElement, flex bool) (string, error) {
	if e.HasTableRows() {
		if err := ast.ValidateTableRows(e); err != nil {
			return "", err
		}
		return formatTableRowsBlock(e)
	}
	if !tableCellsAreSimple(e) {
		if len(e.ColumnAlign) > 0 {
			return "", newUnsupported("table", "column alignment is only written in the separator row of a pipe table, and this table has merged or row-scoped cells, which a pipe table cannot hold")
		}
		return formatTableCellsBlock(e)
	}
	plainPipe := e.Caption == "" && e.Label == "" && len(e.Headers) > 0
	if plainPipe {
		// formatPipeTable delimits with "|", not quotes — it doesn't go
		// through quote()/checkQuotable, so it doesn't inherit that
		// limitation (a literal quote in a header/cell is representable in
		// the pipe form unchanged). A literal "|" in a cell IS re-escaped to
		// "\|" by formatPipeTable (see escapeTableCellPipe) — needed since
		// F10 (audit 2026-09-11) made a bare "|" inside a cell reachable at
		// all (code spans, and "\|" decoded on parse).
		if text := formatPipeTable(e.Headers, e.Rows, e.ColumnAlign); tableReadsBack(e, text, flex) {
			return text, nil
		}
	}
	if len(e.ColumnAlign) > 0 {
		// La alineación (table-align-v1) solo se escribe en la fila
		// delimitadora de una tabla pipe; las formas TABLE/cells no tienen
		// dónde ponerla, y perderla en silencio cambiaría el AST.
		return "", alignmentError(e, plainPipe)
	}
	if cellsKeepFlatView(e) {
		text, err := formatTableCellsBlock(e)
		if err == nil && tableReadsBack(e, text, flex) {
			return text, nil
		}
	}
	if len(e.Headers) > 0 {
		text, err := formatTablePipeBlock(e)
		if err == nil && tableReadsBack(e, text, flex) {
			return text, nil
		}
	}
	rowsIndent := strictTableRowsIndent
	if flex {
		rowsIndent = flexTableRowsIndent
	}
	text, err := formatTableArraysBlock(e, rowsIndent)
	if err != nil {
		return "", err
	}
	if !tableReadsBack(e, text, flex) {
		return "", newUnsupported("table", fmt.Sprintf("%s does not read back unchanged in any table form (pipe, cells:, TABLE with \"|\" rows, headers:/rows:)", tableName(e)))
	}
	return text, nil
}

// tableReadsBack dice si text, una tabla escrita en la columna 0, se relee en
// el dialecto de destino como e, sin ningún aviso del parser. Va seguida de
// un párrafo: una tabla que se lo tragara no se relee igual.
func tableReadsBack(e *ast.TableElement, text string, flex bool) bool {
	var doc *ast.AST
	var diags []diagnostics.Diagnostic
	if flex {
		doc, diags = parser.New(util.NewNoop()).ParseDocument("---\nmode: flex\n---\n\n# x\n\n"+text+"\n\nafter\n", "")
	} else {
		p := parser.New(util.NewNoop())
		p.SetNormalization(false)
		doc, diags = p.Parse("---\nmode: strict\n---\n\nSLIDE content\n  title: \"x\"\n"+indent(text, 2)+"\n  TEXT\n    after\n", "")
	}
	if doc == nil || len(diags) > 0 {
		return false
	}
	var got *ast.TableElement
	n := 0
	_ = ast.Walk(doc, func(node ast.Node) error {
		if t, ok := node.(*ast.TableElement); ok {
			got = t
			n++
		}
		return nil
	})
	if n != 1 {
		return false
	}
	want := *e
	if len(want.Cells) == 0 {
		// Ver tableCellsAreSimple: un Cells sin poblar es el derivado.
		want.Cells = ast.DeriveCellsFromFlat(want.Headers, want.Rows)
	}
	return reflect.DeepEqual(stripPositions(&want), stripPositions(got))
}

// tableName nombra una tabla por su contenido, para un mensaje: su
// encabezado, o su primera fila si no tiene.
func tableName(e *ast.TableElement) string {
	if len(e.Headers) > 0 {
		return fmt.Sprintf("the table with header %s", pipeRowText(e.Headers))
	}
	if len(e.Rows) > 0 {
		return fmt.Sprintf("the table without a header whose first row is %s", pipeRowText(e.Rows[0]))
	}
	return "an empty table"
}

func pipeRowText(cells []string) string {
	if len(cells) == 0 {
		return "|"
	}
	return "| " + strings.Join(cells, " | ") + " |"
}

// alignmentError explica por qué una tabla alineada no tiene forma: la única
// que lleva la alineación es la tabla pipe.
func alignmentError(e *ast.TableElement, plainPipe bool) error {
	if plainPipe {
		for _, row := range e.Rows {
			if len(row) == 0 {
				return newUnsupported("table", fmt.Sprintf("%s has column alignment and a row without cells (a \"|\" line): strict drops a \"|\" row, and only a pipe table carries the alignment, so no strict table keeps both. Give the row its cells (\"%s\"), delete it, or remove the alignment from the separator row", tableName(e), strings.TrimSpace(pipeRowText(make([]string, len(e.Headers))))))
			}
		}
		return newUnsupported("table", fmt.Sprintf("%s has column alignment, and the pipe table that would carry it does not read back unchanged (a cell with spaces at its edges, for instance). Remove the alignment from the separator row, or fix the cell", tableName(e)))
	}
	var has []string
	if e.Caption != "" {
		has = append(has, "a caption")
	}
	if e.Label != "" {
		has = append(has, "a label")
	}
	if len(e.Headers) == 0 {
		has = append(has, "no header row")
	}
	return newUnsupported("table", fmt.Sprintf("%s has column alignment, which only the separator row of a plain pipe table carries, and it has %s, which a plain pipe table cannot hold. Remove the alignment, or what keeps it from being a plain pipe table", tableName(e), strings.Join(has, " and ")))
}

// formatTablePipeBlock escribe la forma TABLE con caption/label y las filas
// "|" dentro del bloque, la que escribe el autor. Dentro del bloque una fila
// "|" se lee como una fila sin celdas en los dos dialectos, y una fila de
// otro largo que el encabezado queda tal cual. No lleva alineación (el bloque
// salta la fila delimitadora sin leerla) ni una tabla sin encabezado (la
// primera fila "|" es siempre el encabezado).
func formatTablePipeBlock(e *ast.TableElement) (string, error) {
	if err := checkQuotable("table", "caption", e.Caption); err != nil {
		return "", err
	}
	if err := checkQuotable("table", "label", e.Label); err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("TABLE\n")
	if e.Caption != "" {
		fmt.Fprintf(&b, "  caption: %s\n", quote(e.Caption))
	}
	if e.Label != "" {
		fmt.Fprintf(&b, "  label: %s\n", quote(e.Label))
	}
	b.WriteString(indent(formatPipeTable(e.Headers, e.Rows, nil), 2))
	return b.String(), nil
}

// Sangría de las filas de rows: respecto del cuerpo que devuelve
// formatTableElement. El parser lee una fila de rows: solo si empieza
// exactamente a seis espacios del comienzo de la línea (TABLE008 si no): en
// strict el cuerpo de un elemento se escribe con dos espacios (y una columna
// tipada de un grid se lee ya sin su sangría extra), en el flex de documento
// sin sangría.
const (
	strictTableRowsIndent = 4
	flexTableRowsIndent   = 6
)

// cellsKeepFlatView dice si la forma cells: relee los mismos Headers/Rows:
// el parser los deriva de Cells con ast.FlattenCellsToRows, que rellena
// cada fila (y el encabezado) hasta el ancho de la más larga, y deja en nil
// (null en el JSON) una fila sin celdas de una tabla de ancho cero. Una tabla
// vacía tampoco: "cells: []" se relee vacía, pero con el aviso TABLE004 de un
// bloque cells: inválido.
func cellsKeepFlatView(e *ast.TableElement) bool {
	if len(e.Headers) == 0 && len(e.Rows) == 0 {
		return false
	}
	headers, rows := ast.FlattenCellsToRows(ast.DeriveCellsFromFlat(e.Headers, e.Rows))
	if !sameStrings(headers, e.Headers) || len(rows) != len(e.Rows) {
		return false
	}
	for i := range rows {
		if !sameStrings(rows[i], e.Rows[i]) {
			return false
		}
	}
	return true
}

// sameStrings compara como el JSON del AST: un slice nil (null) no es uno
// vacío ([]).
func sameStrings(a, b []string) bool {
	if len(a) != len(b) || (a == nil) != (b == nil) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// formatTableArraysBlock escribe la forma TABLE con headers: (si hay
// encabezado) y una fila de rows: por línea. Es la única forma que relee cada
// fila tal cual: una fila sin celdas ([]), una tabla sin encabezado y una fila
// con otro número de celdas que el encabezado (TABLE003, que así sigue
// apuntando a la misma fila después de fmt). Cada celda va entre comillas
// dobles, así que una celda con una comilla doble o un salto de línea no se
// puede escribir y se rechaza.
func formatTableArraysBlock(e *ast.TableElement, rowsIndent int) (string, error) {
	if err := checkQuotable("table", "caption", e.Caption); err != nil {
		return "", err
	}
	if err := checkQuotable("table", "label", e.Label); err != nil {
		return "", err
	}
	array := func(field string, cells []string) (string, error) {
		for _, c := range cells {
			if strings.ContainsAny(c, "\"\r\n") {
				return "", arraysCellError(e, c)
			}
		}
		return formatInlineArray("table", field, cells)
	}
	var b strings.Builder
	b.WriteString("TABLE\n")
	if e.Caption != "" {
		fmt.Fprintf(&b, "  caption: %s\n", quote(e.Caption))
	}
	if e.Label != "" {
		fmt.Fprintf(&b, "  label: %s\n", quote(e.Label))
	}
	if len(e.Headers) > 0 {
		headers, err := array("headers", e.Headers)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "  headers: %s\n", headers)
	}
	b.WriteString("  rows:")
	pad := strings.Repeat(" ", rowsIndent)
	for _, row := range e.Rows {
		line, err := array("rows", row)
		if err != nil {
			return "", err
		}
		b.WriteString("\n" + pad + line)
	}
	return b.String(), nil
}

// arraysCellError explica el rechazo de una celda que headers:/rows: no puede
// escribir: qué fila obligó a esa forma y cómo arreglarlo.
func arraysCellError(e *ast.TableElement, cell string) error {
	what := "double quote"
	if strings.ContainsAny(cell, "\r\n") {
		what = "line break"
	}
	why := "no other table form reads it back unchanged"
	fix := "remove the " + what + " from the cell"
	if len(e.Headers) == 0 {
		why = "it has no header row"
		fix = "add a header row (with one, the table can be written with \"|\" rows), or " + fix
	}
	for _, row := range e.Rows {
		width := len(e.Headers)
		if width == 0 && len(e.Rows) > 0 {
			width = len(e.Rows[0])
		}
		if len(row) == 0 {
			why += ", and it has a row without cells"
			break
		}
		if len(row) != width {
			why += fmt.Sprintf(", and the row %s has %d cells where the %s has %d", pipeRowText(row), len(row), map[bool]string{true: "header", false: "first row"}[len(e.Headers) > 0], width)
			fix = "give every row the same number of cells, " + fix
			break
		}
	}
	return newUnsupported("table", fmt.Sprintf(
		"%s can only be written as TABLE with headers: and rows:, because %s; each cell there goes on one line in double quotes with no escape, and the cell \"%s\" has a %s. To format it, %s",
		tableName(e), why, cell, what, fix))
}

func formatTableRowsBlock(e *ast.TableElement) (string, error) {
	if err := checkQuotable("table", "caption", e.Caption); err != nil && e.Caption != "" {
		return "", err
	}
	if err := checkQuotable("table", "label", e.Label); err != nil && e.Label != "" {
		return "", err
	}
	outer := &yaml.Node{Kind: yaml.SequenceNode}
	for _, row := range e.TableRows {
		r := &yaml.Node{Kind: yaml.MappingNode}
		if row.NodeID != "" {
			appendYAMLKV(r, "nodeId", row.NodeID)
		}
		if row.Section != "" {
			appendYAMLKV(r, "section", row.Section)
		}
		cells := &yaml.Node{Kind: yaml.SequenceNode, Style: yaml.FlowStyle}
		for _, cell := range row.Cells {
			c := &yaml.Node{Kind: yaml.MappingNode, Style: yaml.FlowStyle}
			if cell.NodeID != "" {
				appendYAMLKV(c, "nodeId", cell.NodeID)
			}
			appendYAMLKV(c, "content", cell.Content)
			if cell.IsHeader {
				appendYAMLKV(c, "header", true)
			}
			if cell.Scope != "" {
				appendYAMLKV(c, "scope", cell.Scope)
			}
			if cell.ColSpan > 1 {
				appendYAMLKV(c, "colspan", cell.ColSpan)
			}
			if cell.RowSpan > 1 {
				appendYAMLKV(c, "rowspan", cell.RowSpan)
			}
			cells.Content = append(cells.Content, c)
		}
		key := &yaml.Node{}
		_ = key.Encode("cells")
		r.Content = append(r.Content, key, cells)
		outer.Content = append(outer.Content, r)
	}
	encoded, err := yaml.Marshal(outer)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("TABLE\n")
	if e.Caption != "" {
		fmt.Fprintf(&b, "  caption: %s\n", quote(e.Caption))
	}
	if e.Label != "" {
		fmt.Fprintf(&b, "  label: %s\n", quote(e.Label))
	}
	b.WriteString("  tableRows:\n")
	b.WriteString(indent(strings.TrimRight(string(encoded), "\n"), 4))
	return strings.TrimRight(b.String(), "\n"), nil
}

// tableCellsAreSimple reports whether e.Cells is exactly the shape
// ast.DeriveCellsFromFlat(e.Headers, e.Rows) would produce — i.e. the
// table carries no explicit cell structure beyond what the flat
// Headers/Rows view already encodes. ColSpan/RowSpan are compared via
// normalizeSpan since 0 and 1 both mean "no span".
//
// An empty e.Cells is treated as simple, not as "no structure could be
// derived": Cells is additive/omitempty, so nothing guarantees a caller
// that builds a TableElement directly (rather than through
// elements.TableParser, which always populates it) keeps it in sync with
// Headers/Rows — treating empty Cells as "not simple" would route such a
// table into formatTableCellsBlock, which would then have nothing to
// serialize and silently drop every row (worse than the bug this file
// fixes). formatTableCellsBlock has the matching fallback: it derives
// Cells from Headers/Rows itself when e.Cells is empty.
func tableCellsAreSimple(e *ast.TableElement) bool {
	if len(e.Cells) == 0 {
		return true
	}
	derived := ast.DeriveCellsFromFlat(e.Headers, e.Rows)
	if len(e.Cells) != len(derived) {
		return false
	}
	for i := range e.Cells {
		if len(e.Cells[i]) != len(derived[i]) {
			return false
		}
		for j := range e.Cells[i] {
			a, b := e.Cells[i][j], derived[i][j]
			if a.Content != b.Content || a.IsHeader != b.IsHeader || a.Scope != b.Scope ||
				normalizeSpan(a.ColSpan) != normalizeSpan(b.ColSpan) ||
				normalizeSpan(a.RowSpan) != normalizeSpan(b.RowSpan) {
				return false
			}
		}
	}
	return true
}

// normalizeSpan treats an unset span (0) and an explicit "no merge" span
// (1) as equivalent, matching ast.FlattenCellsToRows/clampSpan's own
// normalization.
func normalizeSpan(n int) int {
	if n < 1 {
		return 1
	}
	return n
}

// formatTableCellsBlock emits the TABLE form with an explicit "cells:" key
// — the source of truth for structure (see ast.FlattenCellsToRows) — plus
// optional caption:/label:. Headers/Rows are never emitted here: the
// parser (elements.TableParser) derives them back from Cells on reparse.
func formatTableCellsBlock(e *ast.TableElement) (string, error) {
	if e.Caption != "" {
		if err := checkQuotable("table", "caption", e.Caption); err != nil {
			return "", err
		}
	}
	if e.Label != "" {
		if err := checkQuotable("table", "label", e.Label); err != nil {
			return "", err
		}
	}

	cells := e.Cells
	if len(cells) == 0 {
		// See tableCellsAreSimple's doc comment: an unpopulated Cells is
		// derived on the fly rather than serialized as-is (which would
		// silently drop every row).
		cells = ast.DeriveCellsFromFlat(e.Headers, e.Rows)
	}
	cellsYAML, err := formatTableCellsYAML(cells)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	b.WriteString("TABLE\n")
	if e.Caption != "" {
		fmt.Fprintf(&b, "  caption: %s\n", quote(e.Caption))
	}
	if e.Label != "" {
		fmt.Fprintf(&b, "  label: %s\n", quote(e.Label))
	}
	b.WriteString("  cells:\n")
	b.WriteString(indent(cellsYAML, 4))
	return strings.TrimRight(b.String(), "\n"), nil
}

// formatTableCellsYAML serializes cells as the flow-style YAML sequence the
// "cells:" authoring syntax expects (see parseCellsYAML in
// internal/elements/table.go): one block-sequence item per row, each row a
// flow sequence of flow mappings, e.g.
// `- [{content: A, header: true, colspan: 2}, {content: B, header: true}]`.
//
// Built via yaml.Node rather than hand-formatted strings (unlike quote()
// elsewhere in this package) because cell content is free-form prose that
// can contain quotes, colons, or commas — exactly what YAML's own quoting
// rules exist to handle safely, and hand-rolling that would re-implement a
// YAML encoder. Content only needs escaping *at all* because parseCellsYAML
// on the other end is a real yaml.Unmarshal, not a hand-rolled reader (like
// the rest of the legacy strict dialect), so round-tripping through the
// same library guarantees whatever is emitted here parses back correctly.
func formatTableCellsYAML(cells [][]ast.TableCell) (string, error) {
	outer := &yaml.Node{Kind: yaml.SequenceNode}
	for _, row := range cells {
		rowNode := &yaml.Node{Kind: yaml.SequenceNode, Style: yaml.FlowStyle}
		for _, c := range row {
			cellNode := &yaml.Node{Kind: yaml.MappingNode, Style: yaml.FlowStyle}
			appendYAMLKV(cellNode, "content", c.Content)
			if c.IsHeader {
				appendYAMLKV(cellNode, "header", true)
			}
			if c.Scope != "" {
				appendYAMLKV(cellNode, "scope", c.Scope)
			}
			if c.ColSpan > 1 {
				appendYAMLKV(cellNode, "colspan", c.ColSpan)
			}
			if c.RowSpan > 1 {
				appendYAMLKV(cellNode, "rowspan", c.RowSpan)
			}
			rowNode.Content = append(rowNode.Content, cellNode)
		}
		outer.Content = append(outer.Content, rowNode)
	}

	out, err := yaml.Marshal(outer)
	if err != nil {
		return "", fmt.Errorf("marshaling table cells: %w", err)
	}
	return strings.TrimRight(string(out), "\n"), nil
}

// appendYAMLKV appends a key/value pair of scalar nodes to a MappingNode's
// Content (yaml.Node stores mapping entries as a flat alternating
// key/value slice, not key-value pairs). Node.Encode picks the correct
// Kind/Tag/quoting style for the Go value automatically.
func appendYAMLKV(node *yaml.Node, key string, value interface{}) {
	keyNode := &yaml.Node{}
	_ = keyNode.Encode(key)
	valNode := &yaml.Node{}
	_ = valNode.Encode(value)
	node.Content = append(node.Content, keyNode, valNode)
}
