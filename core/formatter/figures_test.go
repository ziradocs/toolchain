// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"reflect"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
)

func firstOfType[T ast.Element](t *testing.T, doc *ast.AST) T {
	t.Helper()
	var zero T
	var found T
	ok := false
	_ = ast.Walk(doc, func(n ast.Node) error {
		if e, is := n.(T); is && !ok {
			found, ok = e, true
		}
		return nil
	})
	if !ok {
		t.Fatalf("no %T in document", zero)
	}
	return found
}

// Un null en una fila por categoría se queda en su columna (no se corre a la
// serie de al lado) y sobrevive a fmt en ambos sentidos.
func TestChartNullKeepsItsColumnThroughFmt(t *testing.T) {
	src := "---\nmode: strict\n---\nSLIDE content\n  title: \"S\"\n  <<chart: bar>>\n    series: [\"S1\", \"S2\"]\n    data: [[\"A\", 1, 10], [\"B\", null, 20], [\"C\", 3, null]]\n  <<end>>\n"
	doc := mustParse(t, src, false)
	chart := firstOfType[*ast.ChartElement](t, doc)
	want := [][]interface{}{{"A", 1, 10}, {"B", nil, 20}, {"C", 3, nil}}
	if !reflect.DeepEqual(chart.Data, want) {
		t.Fatalf("data = %#v, want %#v", chart.Data, want)
	}
	out, err := FormatStrict(doc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `["B", null, 20]`) {
		t.Fatalf("null lost in fmt:\n%s", out)
	}
	if got := firstOfType[*ast.ChartElement](t, mustParse(t, out, false)).Data; !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip data = %#v, want %#v", got, want)
	}
}

// El pie de un diagrama llena el Title existente en flex y strict, y fmt lo
// conserva, eligiendo comillas simples si el pie trae dobles.
func TestDiagramTitlesRoundTrip(t *testing.T) {
	for _, tc := range []struct{ name, src, title string }{
		{"mermaid strict", "---\nmode: strict\n---\nSLIDE content\n  title: \"S\"\n  <<mermaid title=\"Approval flow\">>\n    graph TD\n      A --> B\n", "Approval flow"},
		{"mermaid flex", "---\nmode: flex\n---\n# Deck\n\n## S\n\n<<mermaid title='The \"final\" flow'>>\n  graph TD\n    A --> B\n<<end>>\n", `The "final" flow`},
		{"plantuml strict", "---\nmode: strict\n---\nSLIDE content\n  title: \"S\"\n  <<plantuml title=\"Sequence\">>\n    Alice -> Bob: hi\n  <<end>>\n", "Sequence"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc := mustParse(t, tc.src, false)
			title := func(d *ast.AST) string {
				var got string
				_ = ast.Walk(d, func(n ast.Node) error {
					switch e := n.(type) {
					case *ast.MermaidElement:
						got = e.Title
					case *ast.PlantUMLElement:
						got = e.Title
					}
					return nil
				})
				return got
			}
			if got := title(doc); got != tc.title {
				t.Fatalf("title = %q, want %q", got, tc.title)
			}
			out, err := FormatStrict(doc)
			if err != nil {
				t.Fatal(err)
			}
			if got := title(mustParse(t, out, false)); got != tc.title {
				t.Fatalf("title after fmt = %q, want %q\n%s", got, tc.title, out)
			}
		})
	}
}

func TestDiagramTitleRejectsInvalidAttributes(t *testing.T) {
	for _, tag := range []string{
		`<<mermaid caption="x">>`,
		`<<mermaid title="">>`,
		`<<mermaid title="a" title="b">>`,
		`<<mermaid title="unclosed>>`,
		`<<plantuml size="big">>`,
	} {
		src := "---\nmode: strict\n---\nSLIDE content\n  title: \"S\"\n  " + tag + "\n    graph TD\n  <<end>>\n"
		_, diags := parseAny(src)
		if !anyError(diags) {
			t.Errorf("%s accepted without error", tag)
		}
	}
	doc := mustParse(t, "---\nmode: strict\n---\nSLIDE content\n  title: \"S\"\n  <<mermaid>>\n    graph TD\n", false)
	firstOfType[*ast.MermaidElement](t, doc).Title = "a \"b\" 'c'"
	if _, err := FormatStrict(doc); err == nil {
		t.Fatal("unrepresentable title accepted")
	}
}

// Un node-id después de un diagrama cerrado por indentación (sin <<end>>) se
// liga al elemento siguiente en vez de quedar huérfano.
func TestNodeIDAfterDiagramWithoutEnd(t *testing.T) {
	src := "---\nmode: strict\n---\nSLIDE content\n  title: \"S\"\n  <!-- node-id: DiagramA -->\n  <<mermaid>>\n    graph TD\n      A --> B\n  <!-- node-id: TextA -->\n  TEXT\n    After.\n"
	doc := mustParse(t, src, false)
	ids := map[string]ast.NodeType{}
	_ = ast.Walk(doc, func(n ast.Node) error {
		if in, ok := n.(ast.IdentityNode); ok && in.GetNodeID() != "" {
			ids[in.GetNodeID()] = n.GetType()
		}
		return nil
	})
	if ids["DiagramA"] != ast.NodeTypeMermaid || ids["TextA"] != ast.NodeTypeText {
		t.Fatalf("node ids = %v", ids)
	}
}

func parseAny(src string) (*ast.AST, []diagnostics.Diagnostic) {
	return parser.New(util.NewNoop()).Parse(src, "")
}

func anyError(diags []diagnostics.Diagnostic) bool {
	for _, d := range diags {
		if d.IsError() {
			return true
		}
	}
	return false
}
