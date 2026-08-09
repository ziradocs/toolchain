// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"strings"

	"go.yaml.in/yaml/v3"
	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// TableParser maneja elementos de tabla (YAML y Markdown)
type TableParser struct{}

// CanParse determina si una línea es el inicio de una tabla
func (p *TableParser) CanParse(line string, mode string) bool {
	trimmed := strings.TrimSpace(line)

	// TABLE keyword: recognized in both strict and flex. doclang ONLY parses
	// flex (DocumentFlexParser.parseSectionContent sets ctx.Mode = "flex"),
	// so a TABLE block limited to strict was unauthorable in doclang —
	// needed so the explicit merged-cell `cells:` syntax (issue #20) exists
	// in the CLI the enterprise/A11Y rulepack actually consumes. Before this
	// change, a TABLE block in flex fell through to the TextParser fallback
	// and rendered as literal text.
	//
	// Exact-token match (trimmed == "TABLE", not HasPrefix): widening this to
	// flex means it now runs over ordinary doclang prose too, where a line
	// can legitimately start with the word "TABLE" without being the block
	// keyword (e.g. "TABLE 3.1: Resultados" or "TABLE OF CONTENTS" as a
	// heading) — HasPrefix would swallow that line as a table start and
	// drop it as content. Every real TABLE block (tests, examples) is the
	// bare token with nothing else on the line, so exact match is safe.
	if (mode == "strict" || mode == "flex") && trimmed == "TABLE" {
		return true
	}

	// Both modes: Markdown table format. Requires a leading "|" (not just
	// 2+ pipes anywhere) — without this, TableParser runs before
	// Quote/Checklist/Points/Text in the registry (GetDefaultRegistry) and
	// steals legitimate lines with 2+ pipes that aren't tables, e.g.
	// "- Compara pandas | numpy | scipy" (issue #245). Every real markdown
	// table in the corpus starts each row with "|", and strict already
	// requires the same (parser/strict.go). CanParse only sees one line (it
	// can't check whether the next one is a separator row "|---|"), so a
	// leading "|" is the only viable discriminator here.
	if strings.HasPrefix(trimmed, "|") && strings.Count(trimmed, "|") >= 2 {
		return true
	}

	return false
}

// Parse parsea una tabla desde las líneas proporcionadas
func (p *TableParser) Parse(ctx *ParseContext, startIndex int) *ParseResult {
	if startIndex >= len(ctx.Lines) {
		return &ParseResult{
			Element:       nil,
			ConsumedLines: 0,
			Error:         nil,
		}
	}

	pos := diagnostics.NewPosition(startIndex+1, 1)
	table := ast.NewTableElement(pos)
	consumed := 0
	line := strings.TrimSpace(ctx.Lines[startIndex])
	var diags []diagnostics.Diagnostic

	// The YAML "TABLE" block is recognized the same way in strict and flex
	// (see CanParse); mode no longer gates this branch, only the line's syntax.
	if strings.HasPrefix(line, "TABLE") {
		consumed++
		startIndex++

		// Parse YAML-style table
		headers, rows, caption, label, cellsExplicit, yamlDiags, yamlConsumed := p.parseYAMLTable(ctx.Lines, startIndex, pos)
		table.Caption = caption
		table.Label = label
		consumed += yamlConsumed
		diags = append(diags, yamlDiags...)

		if len(cellsExplicit) > 0 {
			// Explicit merged cells (issue #20): Cells is the source of
			// truth; Headers/Rows are DERIVED from Cells (rectangular grid)
			// so linter.ElementStructureRule (TABLE003) doesn't report a
			// false positive of "inconsistent columns" over a table with
			// colspan/rowspan.
			table.Cells = cellsExplicit
			table.Headers, table.Rows = ast.FlattenCellsToRows(cellsExplicit)
		} else {
			table.Headers = headers
			table.Rows = rows
			table.Cells = ast.DeriveCellsFromFlat(headers, rows)
		}
	} else {
		// Parse Markdown-style table
		headers, rows, markdownConsumed := p.parseMarkdownTable(ctx.Lines, startIndex)
		table.Headers = headers
		table.Rows = rows
		table.Cells = ast.DeriveCellsFromFlat(headers, rows)
		consumed = markdownConsumed
	}

	return &ParseResult{
		Element:       table,
		ConsumedLines: consumed,
		Error:         nil,
		Diagnostics:   diags,
	}
}

// parseYAMLTable parsea una tabla en formato YAML (bloque "TABLE"). El
// quinto valor de retorno es no-nil solo si el bloque declaró "cells:"
// explícito (celdas fusionadas, issue #20) — en ese caso el llamador debe
// tratar Cells como fuente de verdad y derivar Headers/Rows de él (ver
// ast.FlattenCellsToRows), ignorando los headers/rows acumulados acá (que
// para un bloque "cells:" quedan vacíos, ya que esa sintaxis no declara
// headers:/rows: por separado).
func (p *TableParser) parseYAMLTable(lines []string, startIndex int, pos diagnostics.Position) ([]string, [][]string, string, string, [][]ast.TableCell, []diagnostics.Diagnostic, int) {
	// Initialized as empty slices (not nil): Headers/Rows have no omitempty
	// in the AST, so a nil value would serialize as JSON null instead of []
	// (issue #8 - violates the contract's JSON Schema).
	headers := []string{}
	rows := [][]string{}
	var caption string
	var label string
	// Named differently from "cells" on purpose: the "|" fallback branch
	// below already declares a local variable "cells" ([]string, one row
	// split by pipes) unrelated to this one — same name in nested scopes
	// would still compile (shadowing), but would confuse a reader.
	var explicitCells [][]ast.TableCell
	var diags []diagnostics.Diagnostic
	consumed := 0
	expectedIndent := -1 // Auto-detect indentation level

	for i := startIndex; i < len(lines); i++ {
		line := lines[i]
		currentIndent := CalculateIndentLevel(line)
		trimmedLine := strings.TrimSpace(line)

		// Skip empty lines
		if trimmedLine == "" {
			consumed++
			continue
		}

		// Auto-detect expected indentation from first non-empty line
		if expectedIndent == -1 && currentIndent > 0 {
			expectedIndent = currentIndent
		}

		// If we haven't detected indentation yet and line has no indentation, break
		if expectedIndent == -1 && currentIndent == 0 {
			break
		}

		// Check if this line should be part of the table block
		if expectedIndent > 0 && currentIndent < expectedIndent {
			break
		}

		// Parse YAML table properties
		if strings.HasPrefix(trimmedLine, "headers:") {
			headersStr := strings.TrimPrefix(trimmedLine, "headers:")
			headersStr = strings.TrimSpace(headersStr)

			// Remove brackets and parse array
			if strings.HasPrefix(headersStr, "[") && strings.HasSuffix(headersStr, "]") {
				headersStr = trimInlineArrayBrackets(headersStr)
				headers = append(headers, splitInlineArray(headersStr)...)
			}
		} else if strings.HasPrefix(trimmedLine, "rows:") {
			// Process rows array
			consumed++
			i++
			for i < len(lines) {
				rowLine := lines[i]
				if !strings.HasPrefix(rowLine, "      [") {
					i-- // Back up one line
					break
				}

				rowTrimmed := strings.TrimSpace(rowLine)
				if strings.HasPrefix(rowTrimmed, "[") && strings.HasSuffix(rowTrimmed, "],") {
					rowTrimmed = strings.TrimSuffix(rowTrimmed, ",")
				}
				if strings.HasPrefix(rowTrimmed, "[") && strings.HasSuffix(rowTrimmed, "]") {
					rowStr := trimInlineArrayBrackets(rowTrimmed)
					rows = append(rows, splitInlineArray(rowStr))
				}
				consumed++
				i++
			}
			continue
		} else if strings.HasPrefix(trimmedLine, "cells:") {
			// Explicit merged cells (issue #20): the value is a YAML
			// sequence of cell rows, indented under "cells:". Collect the
			// lines that belong to it, then parse them as full YAML — more
			// robust than the hand-rolled string splitting headers:/rows:
			// use above, needed because here the value is a nested
			// structure (a list of lists of maps), not a flat array.
			//
			// A sequence item is still part of this value either when it's
			// indented past "cells:" itself, OR when it sits at the exact
			// same indentation and starts with "-" — the idiomatic YAML
			// style for a block sequence under a mapping key
			// (`cells:\n- [...]`). Treating same-indent-without-"-" as the
			// end of the block (the pre-fix behavior) silently produced an
			// empty cellsBlockLines for that idiomatic style, and thus a
			// silently empty table.
			cellsIndent := currentIndent
			consumed++
			i++
			var cellsBlockLines []string
			for i < len(lines) {
				rl := lines[i]
				rlTrimmed := strings.TrimSpace(rl)
				if rlTrimmed == "" {
					cellsBlockLines = append(cellsBlockLines, rl)
					consumed++
					i++
					continue
				}
				rlIndent := CalculateIndentLevel(rl)
				if rlIndent < cellsIndent || (rlIndent == cellsIndent && !strings.HasPrefix(rlTrimmed, "-")) {
					i-- // Let the outer loop reprocess this line
					break
				}
				cellsBlockLines = append(cellsBlockLines, rl)
				consumed++
				i++
			}
			if parsedCells, cellDiags, ok := parseCellsYAML(cellsBlockLines, pos); ok {
				explicitCells = parsedCells
				diags = append(diags, cellDiags...)
			} else if len(cellsBlockLines) > 0 {
				diags = append(diags, diagnostics.NewWarning(
					`The "cells:" block is not valid YAML and was ignored; the table will be empty`,
					pos, "table-parser").WithRuleID("TABLE004"))
			}
			continue
		} else if strings.HasPrefix(trimmedLine, "caption:") {
			captionStr := strings.TrimPrefix(trimmedLine, "caption:")
			caption = strings.Trim(strings.TrimSpace(captionStr), "\"")
		} else if strings.HasPrefix(trimmedLine, "label:") {
			// issue #239: identificador de referencia cruzada (p. ej. "tbl:ventas").
			labelStr := strings.TrimPrefix(trimmedLine, "label:")
			label = strings.Trim(strings.TrimSpace(labelStr), "\"")
		} else if strings.Contains(trimmedLine, "|") {
			// Fallback: Parse table row (separated by |) for compatibility
			cells := strings.Split(trimmedLine, "|")
			for j := range cells {
				cells[j] = strings.TrimSpace(cells[j])
			}

			if len(headers) == 0 {
				headers = cells
			} else {
				rows = append(rows, cells)
			}
		}

		consumed++
	}

	return headers, rows, caption, label, explicitCells, diags, consumed
}

// trimInlineArrayBrackets quita UN "[" inicial y UN "]" final del valor de
// headers:/rows:, en vez del strings.Trim(s, "[]") anterior, que quitaba
// TODOS los corchetes de los extremos y por lo tanto se comía el corchete
// que fuera parte del contenido de la primera/última celda (p. ej.
// `["[borrador]", "B"]` perdía el "[" de la celda). El llamador ya verificó
// que el valor empieza en "[" y termina en "]".
func trimInlineArrayBrackets(s string) string {
	return strings.TrimSuffix(strings.TrimPrefix(s, "["), "]")
}

// splitInlineArray parte el contenido (ya sin corchetes) de un array inline
// del bloque TABLE — headers: o una fila de rows: — en sus celdas,
// respetando las comillas dobles: una coma DENTRO de una celda entrecomillada
// ("Manual, dual-approval", "1,000", "Berlin, Germany") ya no parte la celda
// en dos. Antes se hacía strings.Split(s, ",") a secas, así que ese contenido
// —muy común— generaba una celda de más y disparaba el diagnóstico del linter
// sobre el número de columnas (TABLE003, "Table row has incorrect number of
// columns"), que culpaba a la fila cuando el problema real era el contenido.
//
// Reglas, en orden:
//
//   - Una `\"` dentro de una celda entrecomillada NO cierra las comillas y se
//     desescapa a `"` literal al desenvolver la celda. Es el ÚNICO escape
//     reconocido: `\\` y cualquier otro backslash pasan verbatim, a propósito,
//     porque el dialecto strict no escapa backslashes en ningún lado (ver el
//     comentario de quote() en formatter/util.go, que documenta por qué
//     escaparlos corrompía el round-trip de valores con `\` literal). Este
//     escape es solo de LECTURA: el formatter nunca emite `\"` — para una
//     celda con comilla literal usa la forma "cells:" (YAML real, que se
//     entrecomilla sola), y checkQuotable() rechaza el resto de los campos
//     entrecomillados del dialecto. O sea, acepta más de lo que emite; no
//     introduce un escape nuevo en la salida.
//   - Si las comillas quedan desbalanceadas —conteo IMPAR de comillas sin
//     escapar, p. ej. `A", B` o una celda terminada en `\"`— se cae al split
//     ingenuo por comas de siempre, así que ese caso conserva exactamente el
//     comportamiento previo. Ojo: es una red de seguridad por conteo, no una
//     validación; entrada malformada con un número PAR de comillas mal puestas
//     (`A", B", C`) sí puede diferir del split ingenuo. Entrada bien formada
//     —con o sin comillas— es el caso que este splitter garantiza.
//   - Un array vacío (`headers: []`) produce cero celdas, no una celda vacía
//     como antes (strings.Split("", ",") devuelve un elemento vacío). Ese
//     header fantasma falseaba el conteo de columnas; ahora un `headers: []`
//     reporta el TABLE001 que corresponde ("should have headers defined").
//
// Comillas simples ('A, B') NO se reconocen como delimitador: el dialecto
// strict entrecomilla con comillas dobles en todas partes (quote()), así que
// una comilla simple es contenido literal, no sintaxis.
func splitInlineArray(s string) []string {
	items := []string{}
	start := 0
	inQuotes := false
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\':
			// Solo dentro de comillas: fuera de ellas no hay nada que
			// escapar, y consumir el siguiente carácter podría saltarse una
			// coma delimitadora legítima.
			if inQuotes && i+1 < len(s) {
				i++
			}
		case '"':
			inQuotes = !inQuotes
		case ',':
			if !inQuotes {
				items = append(items, unquoteInlineArrayItem(s[start:i]))
				start = i + 1
			}
		}
	}
	if inQuotes {
		// Comillas desbalanceadas: comportamiento legacy (ver doc arriba).
		legacy := []string{}
		for _, part := range strings.Split(s, ",") {
			legacy = append(legacy, strings.Trim(strings.TrimSpace(part), "\""))
		}
		return legacy
	}
	if last := strings.TrimSpace(s[start:]); last != "" || len(items) > 0 {
		items = append(items, unquoteInlineArrayItem(s[start:]))
	}
	return items
}

// unquoteInlineArrayItem normaliza una celda ya separada: recorta espacios y,
// si queda envuelta en un par de comillas dobles, las quita y desescapa `\"`.
// Si no forma un par (`A"`, `"`, sin comillas), se conserva el
// strings.Trim(item, `"`) histórico para no cambiar el comportamiento de
// entrada malformada.
func unquoteInlineArrayItem(item string) string {
	item = strings.TrimSpace(item)
	if len(item) >= 2 && item[0] == '"' && item[len(item)-1] == '"' {
		return strings.ReplaceAll(item[1:len(item)-1], `\"`, `"`)
	}
	return strings.Trim(item, "\"")
}

// yamlTableCellEntry es la forma YAML de una celda dentro del bloque
// "cells:" (issue #20) — nombres de campo en minúscula sin prefijo, para que
// la sintaxis de autoría sea la aprobada:
//
//	cells:
//	  - [{content: A, header: true, colspan: 2}, {content: B, header: true}]
//	  - [{content: 1}, {content: 2}, {content: 3}]
type yamlTableCellEntry struct {
	Content string `yaml:"content"`
	Header  bool   `yaml:"header"`
	Scope   string `yaml:"scope"`
	Colspan int    `yaml:"colspan"`
	Rowspan int    `yaml:"rowspan"`
}

// parseCellsYAML parses the block of lines collected under "cells:" (a YAML
// sequence of sequences of maps, flow style) into [][]ast.TableCell.
// Returns ok=false if the block is empty or not valid YAML — in that case
// the caller keeps the previous behavior (Headers/Rows derived from other
// keys in the block, typically empty if only a malformed "cells:" was
// declared) and is responsible for surfacing a diagnostic, since only the
// caller has the table's position.
//
// A declared colspan/rowspan above ast.MaxCellSpan is clamped rather than
// rejected outright, and reported via a single aggregated warning — see
// ast.MaxCellSpan's doc comment for why the cap exists. An invalid scope
// (anything other than "", "row", "col") is cleared the same way — see the
// TABLE006 comment below.
func parseCellsYAML(blockLines []string, pos diagnostics.Position) ([][]ast.TableCell, []diagnostics.Diagnostic, bool) {
	if len(blockLines) == 0 {
		return nil, nil, false
	}

	raw := strings.Join(blockLines, "\n")
	var parsed [][]yamlTableCellEntry
	if err := yaml.Unmarshal([]byte(raw), &parsed); err != nil || len(parsed) == 0 {
		return nil, nil, false
	}

	clamped := false
	invalidScope := false
	cells := make([][]ast.TableCell, len(parsed))
	for i, row := range parsed {
		cells[i] = make([]ast.TableCell, len(row))
		for j, c := range row {
			colspan := c.Colspan
			if colspan > ast.MaxCellSpan {
				colspan = ast.MaxCellSpan
				clamped = true
			}
			rowspan := c.Rowspan
			if rowspan > ast.MaxCellSpan {
				rowspan = ast.MaxCellSpan
				clamped = true
			}
			scope := c.Scope
			if scope != "" && scope != "row" && scope != "col" {
				// Cleared, not just left as-is: renderTableCells'
				// writeTableCellRow only ever emits scope="row"/"col" (a
				// fixed allowlist, defensive against interpolating an
				// arbitrary author string into an HTML attribute), so an
				// invalid value like "column" would otherwise round-trip
				// into the JSON contract (elem.Cells) while silently never
				// appearing in the rendered HTML — no error, no diagnostic,
				// just a value that quietly does nothing. Precisely the
				// kind of silent failure this a11y feature can't afford:
				// a typo'd scope looks "saved" in the JSON but the
				// accessibility structure it was meant to declare never
				// reaches the DOM.
				invalidScope = true
				scope = ""
			}
			cells[i][j] = ast.TableCell{
				Content:  c.Content,
				IsHeader: c.Header,
				Scope:    scope,
				ColSpan:  colspan,
				RowSpan:  rowspan,
			}
		}
	}

	var diags []diagnostics.Diagnostic
	if clamped {
		diags = append(diags, diagnostics.NewWarning(
			"A cell declared colspan/rowspan above the supported maximum; it was clamped",
			pos, "table-parser").WithRuleID("TABLE005"))
	}
	if invalidScope {
		diags = append(diags, diagnostics.NewWarning(
			`A cell declared a scope other than "row"/"col"; it was cleared`,
			pos, "table-parser").WithRuleID("TABLE006"))
	}
	return cells, diags, true
}

// parseMarkdownTable parsea una tabla en formato Markdown
func (p *TableParser) parseMarkdownTable(lines []string, startIndex int) ([]string, [][]string, int) {
	// Inicializados como slices vacíos (no nil), ver comentario en parseYAMLTable.
	headers := []string{}
	rows := [][]string{}
	consumed := 0

	for i := startIndex; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])

		// Stop at empty lines or lines without |
		if line == "" || !strings.Contains(line, "|") {
			break
		}

		// Skip separator lines (e.g., |---|---|)
		if strings.Contains(line, "---") || strings.Contains(line, ":-:") || strings.Contains(line, ":--") || strings.Contains(line, "--:") {
			consumed++
			continue
		}

		// Parse table row
		cells := strings.Split(line, "|")

		// Clean up cells - remove empty first/last if they exist due to leading/trailing |
		if len(cells) > 0 && strings.TrimSpace(cells[0]) == "" {
			cells = cells[1:]
		}
		if len(cells) > 0 && strings.TrimSpace(cells[len(cells)-1]) == "" {
			cells = cells[:len(cells)-1]
		}

		// Trim whitespace from each cell
		for j := range cells {
			cells[j] = strings.TrimSpace(cells[j])
		}

		// First row becomes headers
		if len(headers) == 0 {
			headers = cells
		} else {
			rows = append(rows, cells)
		}

		consumed++
	}

	return headers, rows, consumed
}
