package formatter

import (
	"encoding/json"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
)

func parseTableRowsFixture(t *testing.T, source string) (*ast.AST, []string) {
	t.Helper()
	p := parser.New(util.NewNoop())
	p.SetNormalization(false)
	doc, diagnostics := p.Parse(source, "table.slidelang")
	var errors []string
	for _, d := range diagnostics {
		if d.IsError() {
			errors = append(errors, d.Message)
		}
	}
	return doc, errors
}

func identifiedTableSource(body string) string {
	return "---\nmode: strict\ntitle: A\n---\nSLIDE content\n  title: \"A\"\n  <!-- node-id: TableA -->\n  TABLE\n    tableRows:\n" + body
}

func TestTableRowsRoundTripAndIdentity(t *testing.T) {
	source := identifiedTableSource("      - nodeId: HeaderRow\n        section: header\n        cells: [{nodeId: KeyHeader, content: Key, header: true, scope: col}, {nodeId: ValueHeader, content: Value, header: true}]\n      - nodeId: DataA\n        cells: [{nodeId: KeyA, content: A}, {nodeId: ValueA, content: '1'}]\n      - nodeId: DataB\n        section: footer\n        cells: [{nodeId: KeyB, content: B}, {nodeId: ValueB, content: '2'}]\n")
	doc, issues := parseTableRowsFixture(t, source)
	if len(issues) != 0 {
		t.Fatalf("parse diagnostics: %v", issues)
	}
	if doc.SchemaVersion != ast.SchemaVersion || len(doc.Capabilities) != 1 || doc.Capabilities[0] != ast.TableRowsCapability {
		t.Fatalf("missing opt-in contract: %s %v", doc.SchemaVersion, doc.Capabilities)
	}
	table := doc.ContentBlocks[0].Elements[0].(*ast.TableElement)
	if table.NodeID != "TableA" || len(table.TableRows) != 3 || table.TableRows[2].Section != "footer" || table.Rows[1][0] != "B" {
		t.Fatalf("unexpected table: %+v", table)
	}
	if target, ok := ast.ResolveTableIdentity(doc, "ValueA"); !ok || target.Kind != "cell" || target.Row.NodeID != "DataA" {
		t.Fatalf("cell resolution failed: %+v %v", target, ok)
	}
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := ast.DecodeAST(data)
	if err != nil {
		t.Fatal(err)
	}
	formatted, err := FormatStrict(restored)
	if err != nil {
		t.Fatal(err)
	}
	reparsed, issues := parseTableRowsFixture(t, formatted)
	if len(issues) != 0 {
		t.Fatalf("reparse diagnostics: %v\n%s", issues, formatted)
	}
	second := reparsed.ContentBlocks[0].Elements[0].(*ast.TableElement)
	if second.TableRows[1].Cells[1].NodeID != "ValueA" || second.TableRows[2].Section != "footer" {
		t.Fatalf("roundtrip lost identity/section: %+v", second.TableRows)
	}
	formatted2, err := FormatStrict(reparsed)
	if err != nil || formatted != formatted2 {
		t.Fatalf("non-idempotent: %v\n%s\n---\n%s", err, formatted, formatted2)
	}
}

func TestTableRowsRejectInvalidSource(t *testing.T) {
	for _, tc := range []struct{ name, body, want string }{
		{"duplicate", "      - nodeId: RowA\n        cells: [{nodeId: CellA, content: A}]\n      - nodeId: RowA\n        cells: [{content: B}]\n", "duplicate nodeId"},
		{"span outside", "      - nodeId: RowA\n        cells: [{nodeId: CellA, content: A, rowspan: 2}]\n", "outside the table grid"},
		{"mixed", "      - cells: [{content: A}]\n    headers: [A]\n", "cannot be combined"},
		{"unknown", "      - cells: [{content: A, fakeId: X}]\n", "invalid tableRows YAML"},
		{"section order", "      - section: footer\n        cells: [{content: A}]\n      - section: header\n        cells: [{content: B}]\n", "follows a later section"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, issues := parseTableRowsFixture(t, identifiedTableSource(tc.body))
			if !strings.Contains(strings.Join(issues, " | "), tc.want) {
				t.Fatalf("want %q in %v", tc.want, issues)
			}
		})
	}
}
