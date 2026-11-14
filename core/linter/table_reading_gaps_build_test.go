// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package linter

import (
	"bufio"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
)

// genTableGapDocs genera documentos con las tablas que tocan los huecos de
// lectura de tablas de v2.59.0: tablas pipe con separadores sin pipes de
// borde ("--- | ---"), de guiones cortos ("|--|--|") y de un guion, filas de
// solo guiones a media tabla; bloques TABLE con rows: a cualquier sangría,
// rows: [ de varias líneas (con "]" propio, con "]]" en la última fila y con
// la primera fila en la línea de rows:), cells: con "- []" y tablas en
// columnas tipadas de un grid. Van en strict (SECTION, SLIDE, columna
// tipada, :::note, callout tipado) y en flex (con y sin :::), con prosa y
// celdas con enlaces entre tablas. La semilla es fija.
func genTableGapDocs(seed int64, n int) []genDoc {
	heads := []string{"| H1 | H2 |", "| H |", "| A | B | C |", "| H1 | [x](javascript:alert(1)) |", "| 2019 | --- |"}
	seps := []string{"|---|---|", "| --- | --- |", "--- | ---", "--- | --- | ---", "---|---", ":-- | --:", "--- |", "|--|--|", "| -- | -- |", "|--|--|--|", "| -- |", "|:--|--:|", "|:-|-:|", "| - | - |", "| - |", "---", "--|--", "| ---- |", "|:--:|:--:|"}
	rows := []string{"| a | b |", "| 1 | 2 | 3 |", "| a |", "|", "| --- |", "| --- | --- |", "| :-- | --: |", "| --- | --- | --- |", "|--|--|", "| -- | -- |", "| - | - |", "| 2019 | --- |", "| [l](javascript:x) | b |", "| a --- b |", "--- | ---", "a | b", "|---|---|"}
	ctxs := tableGapContexts()
	rng := rand.New(rand.NewSource(seed))
	pick := func(xs []string) string { return xs[rng.Intn(len(xs))] }
	sp := func(k int) string { return strings.Repeat(" ", k) }
	cell := func() string {
		return pick([]string{`"a"`, `"b"`, `"1"`, `"--"`, `"[l](javascript:x)"`, `'q'`, `c`})
	}
	arrayRow := func(cols int) string {
		cs := make([]string, cols)
		for i := range cs {
			cs[i] = cell()
		}
		return "[" + strings.Join(cs, ", ") + "]"
	}
	// pipeTable es una tabla pipe; yamlTable un bloque TABLE con headers: y
	// rows:; cellsTable un bloque TABLE con cells:.
	pipeTable := func() []string {
		out := []string{pick(heads)}
		if rng.Float64() < 0.85 {
			out = append(out, pick(seps))
		}
		for r := rng.Intn(5); r > 0; r-- {
			out = append(out, pick(rows))
		}
		return out
	}
	yamlTable := func() []string {
		prop := []int{2, 4}[rng.Intn(2)]
		out := []string{"TABLE"}
		cols := 1 + rng.Intn(3)
		if rng.Float64() < 0.8 {
			out = append(out, sp(prop)+"headers: "+arrayRow(cols))
		}
		multi := rng.Float64() < 0.35
		firstOnLine := multi && rng.Float64() < 0.3
		rowIndent := prop + 1 + rng.Intn(7)
		if rng.Float64() < 0.3 {
			rowIndent = 6
		}
		nrows := rng.Intn(4)
		width := func() int {
			if rng.Float64() < 0.15 {
				return 1 + rng.Intn(3)
			}
			return cols
		}
		switch {
		case firstOnLine:
			out = append(out, sp(prop)+"rows: ["+arrayRow(width())+",")
		case multi:
			out = append(out, sp(prop)+"rows: [")
		default:
			out = append(out, sp(prop)+"rows:")
		}
		closeOnRow := multi && rng.Float64() < 0.4
		for r := 0; r < nrows; r++ {
			ind := rowIndent
			if rng.Float64() < 0.12 {
				ind = prop + 1 + rng.Intn(7)
			}
			row := arrayRow(width())
			if rng.Float64() < 0.08 {
				row = "[]"
			}
			if rng.Float64() < 0.05 {
				row = `["a", "b"`
			}
			last := r == nrows-1
			switch {
			case multi && last && closeOnRow:
				row += "]"
			case multi && !last:
				row += ","
			}
			if rng.Float64() < 0.05 {
				out = append(out, "")
			}
			out = append(out, sp(ind)+row)
		}
		if multi && (!closeOnRow || nrows == 0) {
			out = append(out, sp([]int{0, prop, rowIndent}[rng.Intn(3)])+"]")
		}
		if rng.Float64() < 0.2 {
			out = append(out, sp(prop)+`caption: "C"`)
		}
		return out
	}
	cellsTable := func() []string {
		out := []string{"TABLE", "  cells:"}
		for r := 1 + rng.Intn(3); r > 0; r-- {
			out = append(out, "    - "+pick([]string{"[]", "[{content: a}]", "[{content: H, header: true}]", "[{content: a}, {content: b}]"}))
		}
		return out
	}
	out := make([]genDoc, n)
	for k := range out {
		c := ctxs[rng.Intn(len(ctxs))]
		mode := "strict"
		if c.flex {
			mode = "flex"
		}
		var body []string
		for tables := 1 + rng.Intn(3); tables > 0; tables-- {
			switch x := rng.Float64(); {
			case x < 0.5:
				body = append(body, pipeTable()...)
			case x < 0.85:
				body = append(body, yamlTable()...)
			default:
				body = append(body, cellsTable()...)
			}
			if rng.Float64() < 0.3 {
				after := "after [t](javascript:x)"
				if !c.flex {
					after = "TEXT\n  after"
				}
				body = append(body, pick([]string{"", "---", after}))
			}
			body = append(body, "")
		}
		var b strings.Builder
		b.WriteString("---\nmode: " + mode + "\ntitle: \"G\"\n---\n\n" + c.pre)
		for _, block := range body {
			for _, l := range strings.Split(block, "\n") {
				if strings.TrimSpace(l) == "" {
					b.WriteString("\n")
				} else {
					b.WriteString(sp(c.indent) + l + "\n")
				}
			}
		}
		b.WriteString(c.suf)
		out[k].src, out[k].document = b.String(), c.document
	}
	return out
}

// tableGapContext es dónde va la tabla: el documento, el dialecto, lo que va
// antes y después y la sangría de cada línea de la tabla.
type tableGapContext struct {
	document, flex bool
	pre            string
	indent         int
	suf            string
}

func tableGapContexts() []tableGapContext {
	return []tableGapContext{
		{true, false, "SECTION \"S\"\n", 2, ""},
		{false, false, "SLIDE content\n  title: \"T\"\n", 2, ""},
		{false, false, "SLIDE content\n  title: \"T\"\n  <<grid>>\n  <<column typed>>\n", 4, "  <<end>>\n"},
		{true, false, "SECTION \"S\"\n  <<grid>>\n  <<column typed>>\n", 4, "  <<end>>\n"},
		{true, false, "SECTION \"S\"\n  :::note\n", 2, "  :::\n"},
		{false, false, "SLIDE content\n  title: \"T\"\n  :::note\n", 2, "  :::\n"},
		{true, false, "SECTION \"S\"\n  :::info{typed}\n", 2, "  :::\n"},
		{false, false, "SLIDE content\n  title: \"T\"\n  :::info{typed}\n", 2, "  :::\n"},
		{true, true, "# Doc\n\n", 0, ""},
		{false, true, "# Slide\n\n", 0, ""},
		{true, true, "# Doc\n\n:::note\n", 0, ":::\n"},
		{false, true, "# Slide\n\n:::tip\n", 0, ":::\n"},
	}
}

const tableGapSeed, tableGapDocs = 559, 4000

type genDoc = struct {
	src      string
	document bool
}

// Diferencial de build contra origin/integration/v2.59.0 (84e227e, la base
// de este cambio): testdata/table_reading_gaps_base_failures.txt tiene los
// índices de los documentos generados que no compilaban allá (errores del
// parser o del linter, con la política aplicada). Todo documento que
// compilaba tiene que seguir compilando; uno que no compilaba puede pasar a
// compilar. El archivo se generó una vez con este mismo generador en la base
// (TABLE_GAPS_WRITE_BASE=1) y no se regenera con el código nuevo.
func TestTableReadingGapsBuildDifferential(t *testing.T) {
	buildDifferential(t, genTableGapDocs(tableGapSeed, tableGapDocs), "testdata/table_reading_gaps_base_failures.txt", "genTableGapDocs(559, 4000)")
}

// genSeparatorMatrix recorre, sin azar, cada contexto, ancho de encabezado y
// forma de separador, con filas de cada ancho abajo: el caso de un separador
// nuevo que uniría filas que no caben es raro en el generador al azar.
func genSeparatorMatrix() []genDoc {
	seps := []string{"--|--", "-- | --", "--:|:--", ":-- | --:", "--- | ---", "--- |", "---|---|---", "|--|--|", "|--|", "| -- | -- | -- |", "|:-|-:|", "| - | - |", "---", ":---:", "|---|", "|---|---|", "| :--: | --: |", "--", "|--|--|--|", "-- | -- | --"}
	var out []genDoc
	for _, c := range tableGapContexts() {
		mode := "strict"
		if c.flex {
			mode = "flex"
		}
		for hw := 1; hw <= 3; hw++ {
			for _, sep := range seps {
				for rw := 0; rw <= 3; rw++ {
					row := func(w int) string {
						cells := make([]string, w)
						for i := range cells {
							cells[i] = fmt.Sprintf("c%d", i)
						}
						return "| " + strings.Join(cells, " | ") + " |"
					}
					lines := []string{row(hw), sep}
					if rw > 0 {
						lines = append(lines, row(rw), row(rw))
					}
					var b strings.Builder
					b.WriteString("---\nmode: " + mode + "\ntitle: \"M\"\n---\n\n" + c.pre)
					for _, l := range lines {
						b.WriteString(strings.Repeat(" ", c.indent) + l + "\n")
					}
					b.WriteString(c.suf)
					out = append(out, genDoc{b.String(), c.document})
				}
			}
		}
	}
	return out
}

// El mismo diferencial sobre genSeparatorMatrix, con su propio archivo de la
// base.
func TestTableSeparatorMatrixBuildDifferential(t *testing.T) {
	buildDifferential(t, genSeparatorMatrix(), "testdata/table_separator_matrix_base_failures.txt", "genSeparatorMatrix()")
}

// buildDifferential compara el build de docs con el de la base: path tiene
// los índices de los que no compilaban allá. Con TABLE_GAPS_WRITE_BASE=1 lo
// escribe (solo se corre así en la base).
func buildDifferential(t *testing.T, docs []genDoc, path, name string) {
	t.Helper()
	if os.Getenv("TABLE_GAPS_WRITE_BASE") == "1" {
		var b strings.Builder
		b.WriteString("# " + name + ": documentos que no compilan en 84e227e\n")
		for i, d := range docs {
			if len(buildErrors(d.src, d.document)) > 0 {
				fmt.Fprintln(&b, i)
			}
		}
		if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	failedOnBase := map[int]bool{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		i, err := strconv.Atoi(line)
		if err != nil {
			t.Fatal(err)
		}
		failedOnBase[i] = true
	}
	broke, fixed := 0, 0
	for i, d := range docs {
		errs := buildErrors(d.src, d.document)
		switch {
		case len(errs) > 0 && !failedOnBase[i]:
			broke++
			if broke <= 5 {
				t.Errorf("document %d built on the base and now fails: %v\n%s", i, errs, d.src)
			}
		case len(errs) == 0 && failedOnBase[i]:
			fixed++
		}
	}
	if broke > 0 {
		t.Errorf("%d documents that built on the base fail now", broke)
	}
	t.Logf("%d of %d documents failed on the base; %d of them build now", len(failedOnBase), len(docs), fixed)
}

// Una tabla con una fila de otro número de celdas dentro de una columna
// tipada de un grid da TABLE009, un aviso: TABLE003 no revisaba esas tablas,
// y darles ese error haría fallar documentos que compilaban. Fuera de la
// columna, la misma tabla sigue siendo TABLE003. Un TABLE003 subido o apagado
// por la política no toca TABLE009.
func TestTable009InTypedGridColumn(t *testing.T) {
	table := "| A | B |\n|---|---|\n| 1 |\n"
	for _, tc := range []struct {
		name     string
		src      string
		document bool
	}{
		{"doclang strict", "---\nmode: strict\ntitle: \"T\"\n---\n\nSECTION \"S\"\n  <<grid>>\n  <<column typed>>\n" + indentLines(table, 4) + "  <<end>>\n", true},
		{"slidelang strict", "---\nmode: strict\ntitle: \"T\"\n---\n\nSLIDE content\n  title: \"T\"\n  <<grid>>\n  <<column typed>>\n" + indentLines(table, 4) + "  <<end>>\n", false},
		{"TABLE block", "---\nmode: strict\ntitle: \"T\"\n---\n\nSECTION \"S\"\n  <<grid>>\n  <<column typed>>\n    TABLE\n      headers: [\"A\", \"B\"]\n      rows:\n        [\"1\"]\n  <<end>>\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if errs := buildErrors(tc.src, tc.document); len(errs) > 0 {
				t.Fatalf("build errors: %v\n%s", errs, tc.src)
			}
			policy := func(rule string) string {
				return strings.Replace(tc.src, "title: \"T\"\n", "title: \"T\"\nlint_policy:\n  rules:\n"+rule, 1)
			}
			if errs := buildErrors(policy("    TABLE003:\n      severity: error\n"), tc.document); len(errs) > 0 {
				t.Fatalf("build errors with TABLE003 raised to error: %v", errs)
			}
			if errs := buildErrors(policy("    TABLE009:\n      severity: error\n"), tc.document); len(errs) != 1 || !strings.HasPrefix(errs[0], "TABLE009: ") {
				t.Fatalf("with TABLE009 raised to error: %v", errs)
			}
			if errs := buildErrors(policy("    TABLE009:\n      enabled: false\n      severity: error\n"), tc.document); len(errs) > 0 {
				t.Fatalf("with TABLE009 turned off: %v", errs)
			}
			found := false
			for _, d := range lintDiags(t, tc.src, tc.document) {
				if d.RuleID == "TABLE003" {
					t.Fatalf("TABLE003 inside a typed column: %v", d)
				}
				if d.RuleID == "TABLE009" {
					if !d.IsWarning() || d.Position.Line == 0 {
						t.Fatalf("TABLE009 is not a positioned warning: %v", d)
					}
					found = true
				}
			}
			if !found {
				t.Fatalf("no TABLE009\n%s", tc.src)
			}
		})
	}
	outside := "---\nmode: strict\ntitle: \"T\"\n---\n\nSECTION \"S\"\n" + indentLines(table, 2)
	if errs := buildErrors(outside, true); len(errs) != 1 || !strings.HasPrefix(errs[0], "TABLE003") {
		t.Fatalf("outside a grid column: %v", errs)
	}
}

func indentLines(s string, n int) string {
	var b strings.Builder
	for _, l := range strings.SplitAfter(s, "\n") {
		if strings.TrimSpace(l) != "" {
			b.WriteString(strings.Repeat(" ", n))
		}
		b.WriteString(l)
	}
	return b.String()
}

func lintDiags(t *testing.T, src string, document bool) []diagnostics.Diagnostic {
	t.Helper()
	p := parser.New(util.NewNoop())
	var doc *ast.AST
	if document {
		doc, _ = p.ParseDocument(src, "t.doclang")
	} else {
		doc, _ = p.Parse(src, "t.slidelang")
	}
	l := New()
	if document {
		l = l.WithDialect(DialectDocuments)
	}
	return l.LintUnfiltered(doc)
}

// Casos sueltos de este cambio que compilan en la base y tienen que seguir
// compilando.
func TestTableReadingGapsKeepBuilding(t *testing.T) {
	flexDoc := "---\nmode: flex\ntitle: \"T\"\n---\n\n# Doc\n\n"
	strictDoc := "---\nmode: strict\ntitle: \"T\"\n---\n\nSECTION \"S\"\n"
	for _, tc := range []struct {
		name     string
		src      string
		document bool
	}{
		{"bare separator with wider rows below", flexDoc + "| A | B |\n--- | ---\n| 1 | 2 | 3 |\n| 4 | 5 | 6 |\n", true},
		{"bare separator in a typed callout with wider rows below", strictDoc + "  :::info{typed}\n  | A | B |\n  --- | ---\n  | 1 | 2 | 3 |\n  :::\n", true},
		{"flex dash-only row that does not fit", flexDoc + "| A | B |\n|---|---|\n| 1 | 2 |\n| --- |\n", true},
		{"short-dash first row", flexDoc + "| A | B |\n|--|--|\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if errs := buildErrors(tc.src, tc.document); len(errs) > 0 {
				t.Fatalf("build errors: %v\n%s", errs, tc.src)
			}
		})
	}
}
