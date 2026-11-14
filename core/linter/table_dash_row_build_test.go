// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package linter

import (
	"bufio"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
)

// Una fila con guiones en una celda que antes se saltaba como separador
// ("| 2019 | --- |") ahora es de datos solo cuando cabe en la tabla. Si no,
// un documento que compilaba dejaría de compilar por TABLE003, que es un
// error. Estas pruebas corren el build como lo corre el CLI: parser, linter
// por defecto y la política del frontmatter.

// buildErrors devuelve los errores del parser y del linter para src, como
// `doclang build` (document) o `slidelang build`: un error del parser corta
// el build; los avisos del parser y los diagnósticos del linter pasan por la
// política del frontmatter (lint_policy:), que puede subirlos a error, como
// hace el CLI (EvaluatePolicy en doclang/internal/pipeline y en
// slidelang/internal/findings).
func buildErrors(src string, document bool) []string {
	p := parser.New(util.NewNoop())
	var doc *ast.AST
	var parseDiags []diagnostics.Diagnostic
	if document {
		doc, parseDiags = p.ParseDocument(src, "t.doclang")
	} else {
		doc, parseDiags = p.Parse(src, "t.slidelang")
	}
	var errs []string
	var findings []diagnostics.Diagnostic
	for _, x := range parseDiags {
		switch {
		case x.IsError():
			errs = append(errs, x.RuleID+": "+x.Message)
		case x.Severity == diagnostics.Info && x.RuleID == "" && x.Code == "":
			// ReportableParserDiagnostics: un info sin regla no pasa por la
			// política.
		default:
			findings = append(findings, x)
		}
	}
	if len(errs) > 0 || doc == nil {
		return append(errs, "parse failed")
	}
	policy, err := ResolvePolicyConfig("", doc.FrontMatter)
	if err != nil {
		return []string{err.Error()}
	}
	l := New()
	if document {
		l = l.WithDialect(DialectDocuments)
	}
	now := time.Now()
	parserActive, _ := policy.Evaluate(findings, doc.FilePath, now)
	lintActive, _ := policy.Evaluate(l.LintUnfiltered(doc), doc.FilePath, now)
	for _, x := range append(parserActive, lintActive...) {
		if x.IsError() {
			errs = append(errs, x.RuleID+": "+x.Message)
		}
	}
	return errs
}

// Una lint_policy que sube TABLE008 a error hace fallar el build por una
// fila con guiones que no cabe en la tabla y se salta como separador: la
// política se aplica también a los avisos del parser, como en el CLI. Sin
// la política, el mismo documento compila.
func TestTableDashRowBuildAppliesPolicyToParserFindings(t *testing.T) {
	const body = "# Doc\n\n| Year | Q1 | Q2 |\n|---|---|---|\n| 2019 | --- |\n| 2020 | 1 | 2 |\n"
	plain := "---\nmode: flex\ntitle: \"T\"\n---\n\n" + body
	if errs := buildErrors(plain, true); len(errs) > 0 {
		t.Fatalf("without a policy: %v", errs)
	}
	strictPolicy := "---\nmode: flex\ntitle: \"T\"\nlint_policy:\n  rules:\n    TABLE008:\n      severity: error\n---\n\n" + body
	errs := buildErrors(strictPolicy, true)
	if len(errs) != 1 || !strings.HasPrefix(errs[0], "TABLE008: ") || !strings.Contains(errs[0], `"| 2019 | --- |"`) {
		t.Fatalf("with TABLE008 raised to error: %v", errs)
	}
	off := "---\nmode: flex\ntitle: \"T\"\nlint_policy:\n  rules:\n    TABLE003:\n      enabled: false\n---\n\n# Doc\n\n| A | B |\n|---|---|\n| 1 |\n"
	if errs := buildErrors(off, true); len(errs) > 0 {
		t.Fatalf("with TABLE003 turned off: %v", errs)
	}
}

// Los casos del reporte de revisión de #531: compilan en origin/main y
// tienen que seguir compilando.
func TestTableDashRowsKeepBuilding(t *testing.T) {
	for _, tc := range []struct {
		name     string
		src      string
		document bool
	}{
		{"strict bare pipe row", "---\nmode: strict\ntitle: \"T\"\n---\n\nSECTION \"S\"\n  | Name | Role |\n  |---|---|\n  | Ana | Dev |\n  |\n  | Bob | QA |\n", true},
		{"flex dash row with fewer cells", "---\nmode: flex\ntitle: \"T\"\n---\n\n# Doc\n\n| Year | Q1 | Q2 |\n|------|----|----|\n| 2019 | --- |\n| 2020 | 1 | 2 |\n", true},
		{"strict dash row with fewer cells after the header", "---\nmode: strict\ntitle: \"T\"\n---\n\nSLIDE content\n  title: \"A\"\n  | Year | Q1 | Q2 |\n  | 2019 | --- |\n  | 2020 | 1 | 2 |\n", false},
		{"flex first line with dashes and no separator", "---\nmode: flex\ntitle: \"T\"\n---\n\n# Doc\n\n| x --- y |\n| a | b |\n| c | d |\n", true},
		{"flex first line with dashes, separator, wider rows", "---\nmode: flex\ntitle: \"T\"\n---\n\n# Doc\n\n| 2019 | --- |\n|---|---|\n| a | b | c |\n| d | e | f |\n", true},
		{"flex first line with dashes, narrower separator", "---\nmode: flex\ntitle: \"T\"\n---\n\n# Doc\n\n| 2019 | --- |\n|---|\n| a |\n", true},
		{"slidelang flex callout dash row", "---\nmode: flex\ntitle: \"T\"\n---\n\n# Deck\n\n## S\n\n:::note\n| A | B |\n| --- | --- |\n| x --- y |\n:::\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if errs := buildErrors(tc.src, tc.document); len(errs) > 0 {
				t.Fatalf("build errors: %v\n%s", errs, tc.src)
			}
		})
	}
}

// Una fila sin celdas no cuenta para TABLE003: no tiene celdas que no
// cuadren. Flex ya la dejaba en el AST, y en origin/main daba TABLE003.
func TestTable003IgnoresEmptyRow(t *testing.T) {
	src := "---\nmode: flex\ntitle: \"T\"\n---\n\n# Doc\n\n| A | B |\n| --- | --- |\n| 1 | 2 |\n|\n| 3 | 4 |\n"
	if errs := buildErrors(src, true); len(errs) > 0 {
		t.Fatalf("build errors: %v", errs)
	}
	src = "---\nmode: flex\ntitle: \"T\"\n---\n\n# Doc\n\n| A | B |\n| --- | --- |\n|\n| 3 |\n"
	if errs := buildErrors(src, true); len(errs) != 1 || !strings.HasPrefix(errs[0], "TABLE003") {
		t.Fatalf("a row with a wrong number of cells after an empty row: %v", errs)
	}
}

// genDashRowDocs genera documentos con tablas Markdown de filas con guiones,
// filas "|", separadores de todo tipo y encabezados con guiones, en strict
// (SECTION, SLIDE, columna tipada, :::note, callout tipado) y en flex (con y
// sin :::note), con prosa entre tablas. Es el generador de la revisión de
// #531, con semilla fija.
func genDashRowDocs(seed int64, n int) []struct {
	src      string
	document bool
} {
	seps := []string{"|---|---|", "| --- | --- |", "|:--|--:|", "|:-:|:-:|", "| :-- | --: |", "|:---|---:|", "---", "--- | ---", "| --- | ---", "|--|--|", "| - | - |", ":--|--:", "| ---- |", "|:---:|"}
	rows := []string{"| a | b |", "| 2019 | --- |", "| --- | n/a |", "| x --- y | z |", "|", "||", "| a |", "| --- | --- |", "| :-- | --: |", "| 1 | 2 | 3 |", "| a --- b |", "|a|b|", "| --- |", "| ` | ` |"}
	heads := []string{"| H1 | H2 |", "| H |", "| 2019 | --- |", "| x --- y |", "| A | B | C |", "|"}
	type ctx struct {
		document, flex bool
		pre            string
		indent         int
		suf            string
	}
	ctxs := []ctx{
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
	}
	rng := rand.New(rand.NewSource(seed))
	pick := func(xs []string) string { return xs[rng.Intn(len(xs))] }
	out := make([]struct {
		src      string
		document bool
	}, n)
	for k := range out {
		c := ctxs[rng.Intn(len(ctxs))]
		mode := "strict"
		if c.flex {
			mode = "flex"
		}
		var body []string
		for tables := 1 + rng.Intn(3); tables > 0; tables-- {
			body = append(body, pick(heads))
			if rng.Float64() < 0.85 {
				body = append(body, pick(seps))
			}
			for r := rng.Intn(5); r > 0; r-- {
				body = append(body, pick(rows))
			}
			if rng.Float64() < 0.3 {
				after := "after text"
				if !c.flex {
					after = "  TEXT\n    after"
				}
				body = append(body, pick([]string{"", "---", "TEXT", after}))
			}
			if rng.Float64() < 0.5 {
				body = append(body, "")
			}
			if !c.flex && rng.Float64() < 0.3 {
				body = append(body, "TEXT", "  between")
			}
		}
		var b strings.Builder
		b.WriteString("---\nmode: " + mode + "\ntitle: \"G\"\n---\n\n" + c.pre)
		for _, block := range body {
			for _, l := range strings.Split(block, "\n") {
				if l == "" {
					b.WriteString("\n")
				} else {
					b.WriteString(strings.Repeat(" ", c.indent) + l + "\n")
				}
			}
		}
		b.WriteString(c.suf)
		out[k].src, out[k].document = b.String(), c.document
	}
	return out
}

// Diferencial de build contra origin/main (34437c8, la base de este cambio;
// 8ba2a79 da el mismo conjunto): testdata/table_dash_row_main_failures.txt
// tiene los índices de los documentos generados que no compilaban allá
// (errores del parser o del linter, con la política aplicada). Todo documento que compilaba tiene que seguir compilando. Uno que
// no compilaba puede pasar a compilar. El archivo se generó una vez con este
// mismo generador en origin/main y no se regenera con el código nuevo.
func TestTableDashRowBuildDifferential(t *testing.T) {
	f, err := os.Open("testdata/table_dash_row_main_failures.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	failedOnMain := map[int]bool{}
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
		failedOnMain[i] = true
	}
	docs := genDashRowDocs(531, 3000)
	broke, fixed := 0, 0
	for i, d := range docs {
		errs := buildErrors(d.src, d.document)
		switch {
		case len(errs) > 0 && !failedOnMain[i]:
			broke++
			if broke <= 5 {
				t.Errorf("document %d built on main and now fails: %v\n%s", i, errs, d.src)
			}
		case len(errs) == 0 && failedOnMain[i]:
			fixed++
		}
	}
	if broke > 0 {
		t.Errorf("%d documents that built on main fail now", broke)
	}
	t.Logf("%d of %d documents failed on main; %d of them build now", len(failedOnMain), len(docs), fixed)
}
