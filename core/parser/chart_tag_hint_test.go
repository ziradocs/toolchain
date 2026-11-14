// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/util"
)

// Un `<<chart>>` sin el ":" del tipo (`<<chart type="bar">>`, `<<chart>>`) no
// es un chart, y sigue sin serlo; su FLEX001 dice cómo se escribe en vez de
// remitir a la documentación. Misma regla, misma severidad, misma línea.
func TestFlexChartWithoutColonSaysHowToWriteIt(t *testing.T) {
	slides := func(src string) []diagnostics.Diagnostic {
		_, d := New(util.NewNoop()).Parse(src, "d.slidelang")
		return d
	}
	docs := func(src string) []diagnostics.Diagnostic {
		_, d := New(util.NewNoop()).ParseDocument(src, "d.doclang")
		return d
	}
	const head = "---\nmode: flex\ntitle: T\n---\n\n# S\n\n"
	for _, tc := range []struct {
		name  string
		parse func(string) []diagnostics.Diagnostic
		src   string
		line  int
		want  string
	}{
		{"slides type attribute", slides, head + "<<chart type=\"bar\">>\nlabels: [a, b]\ndata: [1, 2]\n", 8, "as in <<chart: bar>>."},
		{"slides single-quoted line type", slides, head + "<<chart type='line' title=\"x\">>\n", 8, "as in <<chart: line>>."},
		{"slides bare tag", slides, head + "<<chart>>\nlabels: [a]\n", 8, "a chart's type goes after a colon, as in <<chart: bar>>."},
		{"documents type attribute", docs, head + "<<chart type=\"pie\">>\nlabels: [a]\n", 8, "as in <<chart: pie>>."},
		{"documents bare tag", docs, head + "<<chart>>\n", 8, "as in <<chart: bar>>."},
		{"typed column", docs, head + "::: grid\n::: column typed\n<<chart type=\"bar\">>\n:::\n:::\n", 10, "as in <<chart: bar>>."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got []diagnostics.Diagnostic
			for _, d := range tc.parse(tc.src) {
				if d.RuleID == "FLEX001" {
					got = append(got, d)
				}
			}
			if len(got) != 1 {
				t.Fatalf("want one FLEX001, got %+v", got)
			}
			d := got[0]
			if !d.IsWarning() || d.Position.Line != tc.line {
				t.Errorf("want a warning on line %d, got %s on %d", tc.line, d.Severity, d.Position.Line)
			}
			if !strings.HasPrefix(d.Message, "Unrecognized line, content was discarded: \"<<chart") ||
				!strings.Contains(d.Message, "It is not read as a chart") || !strings.HasSuffix(d.Message, tc.want) {
				t.Errorf("message should say how to write the chart (%q): %q", tc.want, d.Message)
			}
			if strings.Contains(d.Message, "Check DSL Flex syntax documentation") {
				t.Errorf("the generic sentence should be replaced: %q", d.Message)
			}
		})
	}
}

// Otra línea sin reconocer sigue con el mensaje de siempre, y `<<charts>>` no
// es un `<<chart>>` sin tipo.
func TestFlexFailsafeHintOnlyForChartTags(t *testing.T) {
	src := "---\nmode: flex\ntitle: T\n---\n\n# S\n\n<<charts>>\n\n<<gráfica>>\n"
	_, diags := New(util.NewNoop()).Parse(src, "d.slidelang")
	n := 0
	for _, d := range diags {
		if d.RuleID != "FLEX001" {
			continue
		}
		n++
		if !strings.HasSuffix(d.Message, "Check DSL Flex syntax documentation.") {
			t.Errorf("want the generic message, got %q", d.Message)
		}
	}
	if n != 2 {
		t.Errorf("want two FLEX001, got %d (%+v)", n, diags)
	}
}
