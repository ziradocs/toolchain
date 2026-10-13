// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0
package transform

import (
	"encoding/json"
	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/formatter"
	"go.ziradocs.com/core/v2/parser"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func chartDocument(t *testing.T, raw string) *ast.AST {
	t.Helper()
	source := "---\nmode: strict\n---\nSLIDE content\n  title: \"First\"\n  TEXT\n    Intro\nSLIDE content\n  title: \"Chart\"\n  <!-- node-id: chart-a3 -->\n  <<chart: bar>>\n  " + raw + "\n  <</chart>>\nSLIDE content\n  title: \"Last\"\n  TEXT\n    Conclusion\n"
	doc, diags := parser.New(nil).Parse(source, "input.slidelang")
	for _, d := range diags {
		if d.IsError() {
			t.Fatal(d.String())
		}
	}
	return doc
}
func chartIn(doc *ast.AST) *ast.ChartElement {
	var c *ast.ChartElement
	_ = ast.Walk(doc, func(n ast.Node) error {
		if v, ok := n.(*ast.ChartElement); ok {
			c = v
		}
		return nil
	})
	return c
}
func TestRectangularChartsPortableSourceAndReceipt(t *testing.T) {
	raw := `{"data":[["North",120],["South",95],["West",80]],"series":["Unidades"],"unit":"unidades","showDataLabels":true}`
	original := chartDocument(t, raw)
	originalJSON, _ := json.Marshal(original)
	effective, r, err := NormalizeRectangularCharts(original)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Accepted || r.OutputProduced || len(r.Outcomes) != 1 || r.Outcomes[0].Status != "converted" {
		t.Fatalf("receipt=%+v", r)
	}
	if len(effective.ContentBlocks) != 3 {
		t.Fatal("lost slides")
	}
	c := chartIn(effective)
	if c.IsJSONMode || c.RawJSON != nil || c.Unit == nil || *c.Unit != "unidades" || c.ShowDataLabels == nil || !*c.ShowDataLabels {
		t.Fatalf("chart=%+v", c)
	}
	if r.Outcomes[0].NodeID != "chart-a3" || r.Outcomes[0].OriginalRawJSON != string(chartIn(original).RawJSON) || len(r.Outcomes[0].Mapping) != 9 {
		t.Fatalf("lineage=%+v", r.Outcomes[0])
	}
	now, _ := json.Marshal(original)
	if string(now) != string(originalJSON) {
		t.Fatal("mutated original")
	}
	out, err := formatter.FormatStrict(effective)
	if err != nil {
		t.Fatal(err)
	}
	reparsed, diags := parser.New(nil).Parse(out, "effective.slidelang")
	for _, d := range diags {
		if d.IsError() {
			t.Fatal(d.String())
		}
	}
	c2 := chartIn(reparsed)
	if c2.IsJSONMode || c2.Unit == nil || *c2.Unit != "unidades" || c2.ShowDataLabels == nil || !*c2.ShowDataLabels || len(c2.Data) != 3 {
		t.Fatalf("roundtrip=%+v\n%s", c2, out)
	}
	if reparsed.SchemaVersion != ast.ChartContentSchemaVersion {
		t.Fatalf("version=%s", reparsed.SchemaVersion)
	}
	encoded, _ := json.Marshal(reparsed)
	if _, err = ast.DecodeAST(encoded); err != nil {
		t.Fatal(err)
	}
}
func TestRectangularChartsRejectWithoutMutation(t *testing.T) {
	cases := []string{
		`{"data":[["A\" , B",1],["C",2]],"series":["s"]}`,
		`{"data":[["A\\B",1],["C",2]],"series":["s"]}`,
		`{"data":[["A\nB",1],["C",2]],"series":["s"]}`,
		`{"data":[["A",1],["C",2]],"series":["S\", literal"]}`,
		`{"data":[["A",1e-100000000],["B",2]],"series":["s"]}`,
		`{"data":[["A",1e-400],["B",2]],"series":["s"]}`,
		`{"data":[["A",9007199254740993],["B",2]],"series":["s"]}`,
		`{"data":[["A",1],["B",2]],"series":[null]}`,
		`{"data":[["A",1],["B",2]],"series":["s"],"options":{}}`,
		`{"data":[["A",1],["B",2]],"series":["s"],"series":["x"]}`,
		`{"data":[["A",1],["B"]],"series":["s"]}`,
		`{"data":[["A",1],["B",null]],"series":["s"]}`,
		`{"data":[["A",1],["B",1e999]],"series":["s"]}`,
		`{"data":[["A",1],["B",2]],"series":["s","x"]}`,
		`{"data":[["A",1],["B",2]],"series":["s"],"unit":null}`,
		`{"data":[["A",1],["B",2]],"series":["s"],"showDataLabels":"true"}`,
		`{"data":{"labels":["A","B"],"datasets":[{"data":[1,2]}]}}`,
	}
	for _, raw := range cases {
		t.Run(raw, func(t *testing.T) {
			doc := chartDocument(t, raw)
			before, _ := json.Marshal(doc)
			effective, r, err := NormalizeRectangularCharts(doc)
			if err != nil {
				t.Fatal(err)
			}
			after, _ := json.Marshal(effective)
			if r.Accepted || len(r.Outcomes) != 1 || r.Outcomes[0].Status != "rejected" || string(before) != string(after) {
				t.Fatalf("changed rejected chart: %+v", r)
			}
		})
	}
}
func TestRectangularChartsFalseEmptyAndAbsence(t *testing.T) {
	for _, extra := range []string{"", `,"unit":"","showDataLabels":false`} {
		doc := chartDocument(t, `{"data":[["A",0],["B",-1]],"series":["s"]`+extra+`}`)
		effective, r, err := NormalizeRectangularCharts(doc)
		if err != nil || !r.Accepted {
			t.Fatalf("%+v %v", r, err)
		}
		c := chartIn(effective)
		if extra == "" {
			if c.Unit != nil || c.ShowDataLabels != nil || effective.SchemaVersion != ast.LegacySchemaVersion {
				t.Fatal("invented fields/capability")
			}
		} else {
			if c.Unit == nil || *c.Unit != "" || c.ShowDataLabels == nil || *c.ShowDataLabels {
				t.Fatal("lost empty/false")
			}
		}
	}
}
func TestPublishNormalizedChartSourceNoPartialOrOverwrite(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "effective.slidelang")
	side := filepath.Join(dir, "receipt.json")
	doc := chartDocument(t, `{"data":[["A",1],["B",2]],"series":["s"],"extra":1}`)
	_, r, err := NormalizeRectangularCharts(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err = PublishNormalizedChartSource(out, side, []byte("original"), nil, r); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(out); !os.IsNotExist(err) {
		t.Fatal("rejected source published")
	}
	b, err := os.ReadFile(side)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"outputProduced": false`) {
		t.Fatal(string(b))
	}
	if err := os.Remove(side); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(out, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	r.Accepted = true
	if err = PublishNormalizedChartSource(out, side, nil, []byte("changed"), r); err == nil {
		t.Fatal("overwrite accepted")
	}
	b, _ = os.ReadFile(out)
	if string(b) != "keep" {
		t.Fatal("truncated existing output")
	}
}

func TestChartContentContractIgnoresOpaqueNestedUnit(t *testing.T) {
	doc := chartDocument(t, `{"data":[["A",1],["B",2]],"series":["s"],"unit":"unidades"}`)
	c := chartIn(doc)
	c.Options = map[string]interface{}{"scales": map[string]interface{}{"y": map[string]interface{}{"ticks": map[string]interface{}{"format": map[string]interface{}{"style": "unit", "unit": "percent"}}}}}
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ast.DecodeAST(b); err != nil {
		t.Fatal(err)
	}
}

func TestRectangularChartsAcceptConsumerStableDecimals(t *testing.T) {
	doc := chartDocument(t, `{"data":[["A",0.1],["B",1e2]],"series":[""]}`)
	effective, r, err := NormalizeRectangularCharts(doc)
	if err != nil || !r.Accepted {
		t.Fatalf("%+v %v", r, err)
	}
	if chartIn(effective).Series[0] != "" {
		t.Fatal("lost literal empty series")
	}
}
func TestLiteralChartUnitRejectsNullInSource(t *testing.T) {
	source := "---\nmode: strict\n---\nSLIDE content\n  <<chart: bar>>\n    data: [[\"A\", 1], [\"B\", 2]]\n    unit: null\n  <<end>>\n"
	_, diags := parser.New(nil).Parse(source, "invalid.slidelang")
	for _, d := range diags {
		if d.RuleID == "CHART006" && d.IsError() {
			return
		}
	}
	t.Fatal("unit null accepted as empty string")
}

func TestPublishNormalizedChartSourceBindsOriginalAndEffective(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "effective.slidelang")
	side := filepath.Join(dir, "receipt.json")
	doc := chartDocument(t, `{"data":[["A",1],["B",2]],"series":["s"],"unit":"","showDataLabels":false}`)
	effective, r, err := NormalizeRectangularCharts(doc)
	if err != nil {
		t.Fatal(err)
	}
	source, err := formatter.FormatStrict(effective)
	if err != nil {
		t.Fatal(err)
	}
	if err = PublishNormalizedChartSource(out, side, []byte("original bytes"), []byte(source), r); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(side)
	var receipt ChartNormalizationReceipt
	if err = json.Unmarshal(b, &receipt); err != nil {
		t.Fatal(err)
	}
	produced, _ := os.ReadFile(out)
	if !receipt.Accepted || !receipt.OutputProduced || receipt.OriginalSource != "original bytes" || receipt.SourceSHA256 != chartHash([]byte("original bytes")) || receipt.EffectiveSourceSHA256 != chartHash(produced) {
		t.Fatalf("inconsistent receipt %+v", receipt)
	}
}

func TestLiteralChartUnitKeepsJSONEscapesAndLegacyStrings(t *testing.T) {
	raw := `{"data":[["North",1],["South",2]],"series":["Unidades"],"unit":"quoted \" unit\n\\literal", "showDataLabels":false}`
	doc := chartDocument(t, raw)
	effective, r, err := NormalizeRectangularCharts(doc)
	if err != nil || !r.Accepted {
		t.Fatalf("%+v %v", r, err)
	}
	out, err := formatter.FormatStrict(effective)
	if err != nil {
		t.Fatal(err)
	}
	reparsed, _ := parser.New(nil).Parse(out, "effective.slidelang")
	a, b := chartIn(effective), chartIn(reparsed)
	if b.Unit == nil || *a.Unit != *b.Unit || b.ShowDataLabels == nil || *b.ShowDataLabels {
		t.Fatalf("unit lost\n%s", out)
	}
	legacy := "---\nmode: strict\n---\nSLIDE content\n  <<chart: bar>>\n    data: [[\"\\n\", 1], [\"B\", 2]]\n    series: [\"\\n\"]\n  <<end>>\n"
	native, diags := parser.New(nil).Parse(legacy, "legacy.slidelang")
	for _, d := range diags {
		if d.IsError() {
			t.Fatal(d.String())
		}
	}
	c := chartIn(native)
	if c.Series[0] != `\n` || c.Data[0][0] != `\n` {
		t.Fatalf("legacy semantics changed: %+v", c)
	}
}
