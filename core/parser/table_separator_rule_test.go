// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/util"
)

// La fila separadora de una tabla Markdown se reconoce con la misma regla en
// los dos dialectos (elements.IsMarkdownSeparatorLine): solo guiones, dos
// puntos, espacios y "|". Una fila con otro texto ("| 2019 | --- |",
// "| a --- b | c |") es de datos en flex, dentro de un ":::" y en strict. En
// origin/main la tabla de flex y de ":::" la saltaba como separador (sus
// celdas no llegaban al AST, con TABLE008 desde #526) y strict la saltaba
// justo después del encabezado.

// sepContext arma un documento con la tabla en un contexto dado.
type sepContext struct {
	name     string
	document bool
	// prefix va antes de la tabla; indent es la sangría de cada línea de
	// la tabla; suffix cierra el contexto.
	prefix string
	indent int
	suffix string
	// pipeless: el cuerpo de un SECTION o un SLIDE strict, que acepta un
	// separador sin "|" de borde. En un callout o una columna tipada esa
	// línea corta la tabla, igual que en origin/main.
	pipeless bool
	// multiColumn: solo tablas de dos columnas o más. En slidelang auto, el
	// normalizador (TablesRule) reescribe una tabla de una columna cuyas
	// filas no tienen "-" ("| w2 |"), igual que en origin/main; no es la
	// regla del separador.
	multiColumn bool
}

func sepFront(mode string) string {
	return "---\nmode: " + mode + "\ntitle: \"D\"\n---\n"
}

func sepContexts() []sepContext {
	var cs []sepContext
	for _, mode := range []string{"flex", "flex-full", "auto"} {
		cs = append(cs,
			sepContext{name: "doclang " + mode, document: true, prefix: sepFront(mode) + "# Sec\n\n"},
			sepContext{name: "doclang " + mode + " :::note", document: true, prefix: sepFront(mode) + "# Sec\n\n:::note\n", suffix: ":::\n"},
			sepContext{name: "slidelang " + mode, prefix: sepFront(mode) + "# Deck\n\n## S\n\n", multiColumn: mode == "auto"},
			sepContext{name: "slidelang " + mode + " :::note", prefix: sepFront(mode) + "# Deck\n\n## S\n\n:::note\n", suffix: ":::\n", multiColumn: mode == "auto"},
		)
	}
	doc := sepFront("strict") + "SECTION \"S\"\n"
	slide := sepFront("strict") + "SLIDE content\n  title: \"T\"\n"
	cs = append(cs,
		sepContext{name: "doclang strict", document: true, prefix: doc, indent: 2, pipeless: true},
		sepContext{name: "doclang strict :::note", document: true, prefix: doc + "  :::note\n", indent: 2, suffix: "  :::\n"},
		sepContext{name: "doclang strict typed callout", document: true, prefix: doc + "  :::info{typed}\n", indent: 2, suffix: "  :::\n"},
		sepContext{name: "doclang strict typed column", document: true, prefix: doc + "  <<grid>>\n  <<column typed>>\n", indent: 4, suffix: "  <<end>>\n"},
		sepContext{name: "slidelang strict", prefix: slide, indent: 2, pipeless: true},
		sepContext{name: "slidelang strict :::note", prefix: slide + "  :::note\n", indent: 2, suffix: "  :::\n"},
		sepContext{name: "slidelang strict typed callout", prefix: slide + "  :::info{typed}\n", indent: 2, suffix: "  :::\n"},
		sepContext{name: "slidelang strict typed column", prefix: slide + "  <<grid>>\n  <<column typed>>\n", indent: 4, suffix: "  <<end>>\n"},
	)
	return cs
}

func (c sepContext) build(lines []string) string {
	var b strings.Builder
	b.WriteString(c.prefix)
	pad := strings.Repeat(" ", c.indent)
	for _, l := range lines {
		b.WriteString(pad + l + "\n")
	}
	b.WriteString(c.suffix)
	return b.String()
}

// parseTables devuelve las tablas del documento y sus diagnósticos.
func (c sepContext) parseTables(src string) ([]*ast.TableElement, []diagnostics.Diagnostic) {
	p := New(util.NewNoop())
	var doc *ast.AST
	var diags []diagnostics.Diagnostic
	if c.document {
		doc, diags = p.ParseDocument(src, "t.doclang")
	} else {
		doc, diags = p.Parse(src, "t.slidelang")
	}
	var tables []*ast.TableElement
	_ = ast.Walk(doc, func(n ast.Node) error {
		if t, ok := n.(*ast.TableElement); ok {
			tables = append(tables, t)
		}
		return nil
	})
	return tables, diags
}

type wantTable struct {
	headers []string
	rows    [][]string
	align   []string
}

func (c sepContext) check(t *testing.T, lines []string, want wantTable) {
	t.Helper()
	src := c.build(lines)
	tables, diags := c.parseTables(src)
	if len(tables) != 1 {
		t.Fatalf("%s: %d tables\n%s", c.name, len(tables), src)
	}
	got := tables[0]
	rows := got.Rows
	if len(rows) == 0 {
		rows = nil
	}
	if !reflect.DeepEqual(got.Headers, want.headers) || !reflect.DeepEqual(rows, want.rows) || !reflect.DeepEqual(got.ColumnAlign, want.align) {
		t.Fatalf("%s:\n got headers=%q rows=%q align=%q\nwant headers=%q rows=%q align=%q\n%s",
			c.name, got.Headers, rows, got.ColumnAlign, want.headers, want.rows, want.align, src)
	}
	for _, d := range diags {
		if d.RuleID == "TABLE008" || d.IsError() {
			t.Fatalf("%s: unexpected diagnostic %v\n%s", c.name, d, src)
		}
	}
}

// Las filas del reporte, en cada posición de la tabla y en cada contexto.
func TestMarkdownTableDashCellIsData(t *testing.T) {
	cases := []struct {
		name  string
		lines []string
		want  wantTable
	}{
		{
			name:  "after the separator",
			lines: []string{"| Año | Valor |", "| --- | --- |", "| 2019 | --- |", "| a --- b | c |", "| 2020 | 5 |"},
			want:  wantTable{headers: []string{"Año", "Valor"}, rows: [][]string{{"2019", "---"}, {"a --- b", "c"}, {"2020", "5"}}},
		},
		{
			// En strict, la línea que sigue al encabezado se tomaba por
			// separador solo por tener "---".
			name:  "right after the header, no separator",
			lines: []string{"| Año | Valor |", "| 2019 | --- |", "| 2020 | 5 |"},
			want:  wantTable{headers: []string{"Año", "Valor"}, rows: [][]string{{"2019", "---"}, {"2020", "5"}}},
		},
		{
			// En flex, la primera línea se saltaba y la siguiente quedaba
			// de encabezado.
			name:  "as the first line",
			lines: []string{"| 2019 | --- |", "| --- | --- |", "| 2020 | 5 |"},
			want:  wantTable{headers: []string{"2019", "---"}, rows: [][]string{{"2020", "5"}}},
		},
		{
			name:  "aligned separator, colon text cell",
			lines: []string{"| A | B |", "|:--|--:|", "| :-- | x |", "| y | --: |"},
			want:  wantTable{headers: []string{"A", "B"}, rows: [][]string{{":--", "x"}, {"y", "--:"}}, align: []string{"left", "right"}},
		},
		{
			// Lo que no cambia: un guion corto marca una celda vacía y es
			// una fila de datos en los dos dialectos, como antes.
			name:  "short dashes stay data",
			lines: []string{"| A | B |", "| --- | --- |", "| - | -- |", "| 1 | 2 |"},
			want:  wantTable{headers: []string{"A", "B"}, rows: [][]string{{"-", "--"}, {"1", "2"}}},
		},
	}
	for _, c := range sepContexts() {
		for _, tc := range cases {
			t.Run(c.name+"/"+tc.name, func(t *testing.T) {
				c.check(t, tc.lines, tc.want)
			})
		}
	}
}

// Una fila "|" sola es una fila sin celdas, no un separador (no tiene
// guiones). Flex y ":::" ya la dejaban en el AST como fila vacía; strict la
// descartaba sin aviso, en contra de la regla de #525.
func TestMarkdownTableBarePipeRowIsEmptyRow(t *testing.T) {
	for _, tc := range []struct {
		name  string
		lines []string
	}{
		{"after the separator", []string{"| a | b |", "| --- | --- |", "|", "| 1 | 2 |"}},
		{"right after the header", []string{"| a | b |", "|", "| 1 | 2 |"}},
		{"last row", []string{"| a | b |", "| --- | --- |", "| 1 | 2 |", "|"}},
	} {
		want := wantTable{headers: []string{"a", "b"}, rows: [][]string{{}, {"1", "2"}}}
		if tc.name == "last row" {
			want.rows = [][]string{{"1", "2"}, {}}
		}
		for _, c := range sepContexts() {
			t.Run(c.name+"/"+tc.name, func(t *testing.T) { c.check(t, tc.lines, want) })
		}
	}
}

// En strict, un separador escrito solo con ":--", "--:" o ":-:" (sin "---")
// es separador, como en flex. En origin/main era la primera fila de datos,
// con celdas ":--".
func TestStrictColonOnlySeparator(t *testing.T) {
	lines := []string{"| A | B | C |", "|:--|--:|:-:|", "| 1 | 2 | 3 |"}
	want := wantTable{headers: []string{"A", "B", "C"}, rows: [][]string{{"1", "2", "3"}}, align: []string{"left", "right", "center"}}
	for _, c := range sepContexts() {
		t.Run(c.name, func(t *testing.T) { c.check(t, lines, want) })
	}
}

// Propiedad sobre tablas generadas, en todos los contextos: un separador de
// verdad (cualquier mezcla de "---", ":--", "--:", ":-:", con y sin "|"
// final, con espacios, también U+00A0) justo después del encabezado se lee
// como separador y da su alineación; toda otra fila, aunque tenga guiones en
// sus celdas, es una fila de datos con sus celdas tal cual. Y todos los
// contextos leen la misma tabla.
func TestMarkdownTableSeparatorRuleProperty(t *testing.T) {
	sepCells := []string{"---", "----", ":--", ":---", "--:", "---:", ":-:", ":---:"}
	dataCells := []string{"alfa", "2019", "---", "a --- b", ":--", "--:", ":-:", "x---y", "-", "--", "n/a", "----", ":---:", "b - c", "3:--4"}
	contexts := sepContexts()
	rng := rand.New(rand.NewSource(20261114))
	sp := func() string { return strings.Repeat(" ", rng.Intn(3)) }
	// En el separador, también espacios Unicode (U+00A0), que el recorte de
	// cada celda quita igual que un espacio.
	sepSp := func() string {
		if rng.Intn(5) == 0 {
			return "\u00a0"
		}
		return sp()
	}
	for n := 0; n < 300; n++ {
		cols := 1 + rng.Intn(4)
		var lines []string
		var want wantTable
		var header []string
		for i := 0; i < cols; i++ {
			header = append(header, fmt.Sprintf("h%d", i))
		}
		lines = append(lines, "| "+strings.Join(header, " | ")+" |")
		want.headers = header

		// Separador, en la línea que sigue al encabezado.
		hasSep := rng.Intn(5) != 0
		noLeadingPipe := false
		if hasSep {
			var cells, align []string
			declared := false
			for i := 0; i < cols; i++ {
				c := sepCells[rng.Intn(len(sepCells))]
				cells = append(cells, sepSp()+c+sepSp())
				a := ""
				switch l, r := strings.HasPrefix(c, ":"), strings.HasSuffix(c, ":"); {
				case l && r:
					a = "center"
				case r:
					a = "right"
				case l:
					a = "left"
				}
				declared = declared || a != ""
				align = append(align, a)
			}
			line := "|" + strings.Join(cells, "|")
			if rng.Intn(4) != 0 {
				line += "|"
			}
			// Sin pipes de borde, solo el cuerpo strict la lee (flex corta
			// la tabla en una línea que no empieza con "|"), y solo con
			// "---".
			if rng.Intn(6) == 0 && strings.Contains(line, "---") {
				line = strings.Trim(line, "|")
				noLeadingPipe = true
			}
			lines = append(lines, line)
			if declared {
				want.align = align
			}
		}

		for r := rng.Intn(5); r > 0; r-- {
			var row []string
			text := false
			for i := 0; i < cols; i++ {
				c := dataCells[rng.Intn(len(dataCells))]
				row = append(row, c)
				text = text || strings.Trim(c, "-: ") != ""
			}
			// Una fila de puros guiones y dos puntos es separador (en flex,
			// en cualquier posición): no es lo que se prueba aquí, salvo si
			// son guiones cortos, que no lo son en ningún dialecto.
			short := true
			for _, c := range row {
				short = short && (c == "-" || c == "--")
			}
			if !text && !short {
				row[rng.Intn(cols)] = fmt.Sprintf("w%d", n)
			}
			var cells []string
			for _, c := range row {
				cells = append(cells, sp()+c+sp())
			}
			lines = append(lines, "| "+strings.Join(cells, " | ")+" |")
			want.rows = append(want.rows, row)
		}

		for _, c := range contexts {
			if (noLeadingPipe && !c.pipeless) || (cols == 1 && c.multiColumn) {
				continue
			}
			c.check(t, lines, want)
		}
	}
}

// Una fila con guiones que no cabe en la tabla se sigue saltando como
// separador, como en origin/main, con un TABLE008 que la nombra: si entrara,
// el lint (TABLE003, un error) haría fallar un documento que compilaba. Lo
// mismo con una primera línea con guiones que no va seguida de un separador
// con su número de celdas, o cuyas filas de abajo no tienen ese número.
func TestMarkdownTableDashRowThatDoesNotFit(t *testing.T) {
	for _, tc := range []struct {
		name    string
		lines   []string
		want    wantTable
		skipped string
		// sharedOnly: solo la tabla de flex, de ":::" y del callout tipado.
		// En el cuerpo strict (SECTION, SLIDE, columna tipada) una fila
		// después del separador siempre fue de datos, y la primera línea
		// siempre es el encabezado: eso no cambia.
		sharedOnly bool
	}{
		{
			name:       "fewer cells after the separator",
			lines:      []string{"| Year | Q1 | Q2 |", "|---|---|---|", "| 2019 | --- |", "| 2020 | 1 | 2 |"},
			want:       wantTable{headers: []string{"Year", "Q1", "Q2"}, rows: [][]string{{"2020", "1", "2"}}},
			skipped:    "| 2019 | --- |",
			sharedOnly: true,
		},
		{
			name:    "fewer cells right after the header",
			lines:   []string{"| Year | Q1 | Q2 |", "| 2019 | --- |", "| 2020 | 1 | 2 |"},
			want:    wantTable{headers: []string{"Year", "Q1", "Q2"}, rows: [][]string{{"2020", "1", "2"}}},
			skipped: "| 2019 | --- |",
		},
		{
			name:       "first line without a separator",
			lines:      []string{"| x --- y |", "| a | b |", "| c | d |"},
			want:       wantTable{headers: []string{"a", "b"}, rows: [][]string{{"c", "d"}}},
			skipped:    "| x --- y |",
			sharedOnly: true,
		},
		{
			name:       "first line with wider rows below",
			lines:      []string{"| 2019 | --- |", "|---|---|", "| a | b | c |", "| d | e | f |"},
			want:       wantTable{headers: []string{"a", "b", "c"}, rows: [][]string{{"d", "e", "f"}}},
			skipped:    "| 2019 | --- |",
			sharedOnly: true,
		},
	} {
		for _, c := range sepContexts() {
			if tc.sharedOnly && (c.pipeless || strings.Contains(c.name, "typed column")) {
				continue
			}
			t.Run(c.name+"/"+tc.name, func(t *testing.T) {
				src := c.build(tc.lines)
				tables, diags := c.parseTables(src)
				if len(tables) != 1 {
					t.Fatalf("%d tables\n%s", len(tables), src)
				}
				got := tables[0]
				if !reflect.DeepEqual(got.Headers, tc.want.headers) || !reflect.DeepEqual(got.Rows, tc.want.rows) {
					t.Fatalf("got %q %q, want %q %q\n%s", got.Headers, got.Rows, tc.want.headers, tc.want.rows, src)
				}
				for _, d := range diags {
					if d.RuleID == "TABLE008" && strings.Contains(d.Message, fmt.Sprintf("%q", tc.skipped)) && d.IsWarning() {
						return
					}
				}
				t.Fatalf("no TABLE008 naming %q: %v\n%s", tc.skipped, diags, src)
			})
		}
	}
}
