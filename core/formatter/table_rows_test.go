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
		{"missing cells", "      - nodeId: RowA\n", "must declare cells"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, issues := parseTableRowsFixture(t, identifiedTableSource(tc.body))
			if !strings.Contains(strings.Join(issues, " | "), tc.want) {
				t.Fatalf("want %q in %v", tc.want, issues)
			}
		})
	}
}

func TestTableRowsAuthoredEditsKeepIDs(t *testing.T) {
	source := identifiedTableSource("      - section: header\n        cells: [{content: Key, header: true}]\n      - nodeId: RowA\n        cells: [{nodeId: CellA, content: A}]\n      - nodeId: RowB\n        cells: [{nodeId: CellB, content: B}]\n")
	doc, issues := parseTableRowsFixture(t, source)
	if len(issues) != 0 {
		t.Fatal(issues)
	}
	table := doc.ContentBlocks[0].Elements[0].(*ast.TableElement)
	// Identity travels with the authored record through reorder, insertion,
	// and content edits, rather than being re-created from text or position.
	table.TableRows[1], table.TableRows[2] = table.TableRows[2], table.TableRows[1]
	table.TableRows = append(table.TableRows[:2], append([]ast.TableRow{{Cells: []ast.TableRowCell{{Content: "X"}}}}, table.TableRows[2:]...)...)
	table.TableRows[3].Cells[0].Content = "A edited"
	table.SyncTableViews()
	out, err := FormatStrict(doc)
	if err != nil {
		t.Fatal(err)
	}
	reparsed, issues := parseTableRowsFixture(t, out)
	if len(issues) != 0 {
		t.Fatal(issues)
	}
	got := reparsed.ContentBlocks[0].Elements[0].(*ast.TableElement).TableRows
	if got[1].NodeID != "RowB" || got[1].Cells[0].NodeID != "CellB" || got[2].NodeID != "" || got[3].NodeID != "RowA" || got[3].Cells[0].Content != "A edited" {
		t.Fatalf("identity drift after edits: %+v", got)
	}
	// Deleting an identified source row is allowed and does not recycle its ID.
	table.TableRows = append(table.TableRows[:1], table.TableRows[2:]...)
	table.SyncTableViews()
	if _, err := FormatStrict(doc); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	// Copying an identified row without new IDs must fail.
	table.TableRows = append(table.TableRows, table.TableRows[2])
	table.SyncTableViews()
	if _, err := FormatStrict(doc); err == nil || !strings.Contains(err.Error(), "duplicate nodeId") {
		t.Fatalf("duplicate row accepted: %v", err)
	}
}

func TestTableRowsMergedRoundTrip(t *testing.T) {
	source := identifiedTableSource("      - section: header\n        cells: [{content: Key, header: true}, {content: Value, header: true}]\n      - nodeId: RowA\n        cells: [{nodeId: AnchorA, content: A, rowspan: 2}, {content: '1'}]\n      - nodeId: RowB\n        cells: [{content: '2'}]\n")
	doc, issues := parseTableRowsFixture(t, source)
	if len(issues) != 0 {
		t.Fatal(issues)
	}
	table := doc.ContentBlocks[0].Elements[0].(*ast.TableElement)
	if table.Rows[1][0] != "A" {
		t.Fatalf("flat span projection: %+v", table.Rows)
	}
	out, err := FormatStrict(doc)
	if err != nil {
		t.Fatal(err)
	}
	reparsed, issues := parseTableRowsFixture(t, out)
	if len(issues) != 0 {
		t.Fatal(issues)
	}
	got := reparsed.ContentBlocks[0].Elements[0].(*ast.TableElement)
	if got.TableRows[1].Cells[0].NodeID != "AnchorA" || got.TableRows[1].Cells[0].RowSpan != 2 || len(got.TableRows[2].Cells) != 1 {
		t.Fatalf("merged anchor changed: %+v", got.TableRows)
	}
}

func TestTableRowsFullyCoveredRow(t *testing.T) {
	source := identifiedTableSource("      - section: header\n        cells: [{content: A, header: true}, {content: B, header: true}]\n      - nodeId: RowA\n        cells: [{nodeId: AnchorA, content: A, colspan: 2, rowspan: 2}]\n      - nodeId: CoveredRow\n        cells: []\n")
	doc, issues := parseTableRowsFixture(t, source)
	if len(issues) != 0 {
		t.Fatal(issues)
	}
	out, err := FormatStrict(doc)
	if err != nil {
		t.Fatal(err)
	}
	reparsed, issues := parseTableRowsFixture(t, out)
	if len(issues) != 0 {
		t.Fatal(issues)
	}
	got := reparsed.ContentBlocks[0].Elements[0].(*ast.TableElement)
	if got.TableRows[2].NodeID != "CoveredRow" || len(got.TableRows[2].Cells) != 0 || got.Rows[1][1] != "A" {
		t.Fatalf("covered row lost: %+v", got)
	}
}

func TestTableRowsFormatterRejectsUnrepresentableContract(t *testing.T) {
	doc, issues := parseTableRowsFixture(t, identifiedTableSource("      - cells: [{content: A}]\n"))
	if len(issues) != 0 {
		t.Fatal(issues)
	}
	doc.SchemaVersion = ast.LegacySchemaVersion
	if _, err := FormatStrict(doc); err == nil || !strings.Contains(err.Error(), "requires schemaVersion") {
		t.Fatalf("formatter downgraded tableRows: %v", err)
	}
}

func TestTableRowsRepeatedContentUsesExplicitIDs(t *testing.T) {
	source := identifiedTableSource("      - section: header\n        cells: [{content: Key, header: true}]\n      - nodeId: First\n        cells: [{nodeId: FirstCell, content: A}]\n      - nodeId: Second\n        cells: [{nodeId: SecondCell, content: A}]\n")
	doc, issues := parseTableRowsFixture(t, source)
	if len(issues) != 0 {
		t.Fatal(issues)
	}
	table := doc.ContentBlocks[0].Elements[0].(*ast.TableElement)
	if table.Rows[0][0] != table.Rows[1][0] || table.TableRows[1].Cells[0].NodeID == table.TableRows[2].Cells[0].NodeID {
		t.Fatal("probe did not create duplicate content with distinct identities")
	}
	out, err := FormatStrict(doc)
	if err != nil {
		t.Fatal(err)
	}
	reparsed, issues := parseTableRowsFixture(t, out)
	if len(issues) != 0 {
		t.Fatal(issues)
	}
	got := reparsed.ContentBlocks[0].Elements[0].(*ast.TableElement).TableRows
	if got[1].NodeID != "First" || got[2].NodeID != "Second" || got[1].Cells[0].NodeID != "FirstCell" || got[2].Cells[0].NodeID != "SecondCell" {
		t.Fatalf("duplicate content identity drift: %+v", got)
	}
}
