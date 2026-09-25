package transform

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

func nestedFilterFixture() *ast.AST {
	pos := diagnostics.NewPosition(1, 1)
	doc := ast.NewAST(pos)
	block := ast.NewContentBlock(pos, "content")
	points := ast.NewPointsElement(pos)
	points.NodeID = "PointsA"
	points.ListType = "ordered"
	parent := ast.NewPointItem(pos, "ParentA")
	parent.NodeID = "ParentA"
	parent.SubListType = "ordered"
	child := ast.NewPointItem(pos, "ChildA text")
	child.NodeID = "ChildA"
	parent.SubPoints = append(parent.SubPoints, *child)
	second := ast.NewPointItem(pos, "ChildB text")
	second.NodeID = "ChildB"
	parent.SubPoints = append(parent.SubPoints, *second)
	points.Items = append(points.Items, *parent)
	block.Elements = append(block.Elements, points)
	doc.ContentBlocks = append(doc.ContentBlocks, *block)
	ast.SetTableContract(doc)
	return doc
}

const nestedHandshake = `if [ "$1" = "--ziradocs-capabilities" ]; then
echo '{"astSchemaVersions":["2.16.0"],"features":["nested-list-types-v1"]}'
exit 0
fi
`

func TestNestedListFilterNegotiationAndPreservation(t *testing.T) {
	good := writeFilter(t, nestedHandshake+"cat")
	if _, err := RunFilters(nestedFilterFixture(), []string{good}, time.Second); err != nil {
		t.Fatal(err)
	}
	changedContent := writeFilter(t, nestedHandshake+`sed 's/ChildA text/ChildB text/g'`)
	changedDoc, err := RunFilters(nestedFilterFixture(), []string{changedContent}, time.Second)
	if err != nil {
		t.Fatalf("content edit rejected: %v", err)
	}
	changedPoints := changedDoc.ContentBlocks[0].Elements[0].(*ast.PointsElement)
	if changedPoints.Items[0].SubPoints[0].Content != "ChildB text" {
		t.Fatal("filter content edit was not applied")
	}
	marker := filepath.Join(t.TempDir(), "ran")
	old := writeFilter(t, `if [ "$1" = "--ziradocs-capabilities" ]; then echo '{"astSchemaVersions":["2.15.0"],"features":["table-rows-v1"]}'; exit 0; fi
echo ran > `+marker+`
cat`)
	if _, err := RunFilters(nestedFilterFixture(), []string{old}, time.Second); err == nil || !strings.Contains(err.Error(), "does not support") {
		t.Fatalf("legacy filter accepted: %v", err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("legacy filter received AST: %v", err)
	}
	removed := writeFilter(t, nestedHandshake+`sed 's/,"subListType":"ordered"//g'`)
	if _, err := RunFilters(nestedFilterFixture(), []string{removed}, time.Second); err == nil {
		t.Fatal("removed subListType accepted")
	}
	renamed := writeFilter(t, nestedHandshake+`sed 's/"nodeId":"ChildA"/"nodeId":"ChildC"/g'`)
	if _, err := RunFilters(nestedFilterFixture(), []string{renamed}, time.Second); err == nil || !strings.Contains(err.Error(), "ownership") {
		t.Fatalf("changed child identity accepted: %v", err)
	}
	typeOnly := writeFilter(t, nestedHandshake+`sed 's/"subListType":"ordered"/"subListType":"unordered"/g'`)
	if _, err := RunFilters(nestedFilterFixture(), []string{typeOnly}, time.Second); err == nil || !strings.Contains(err.Error(), "list types changed") {
		t.Fatalf("changed nested list type accepted: %v", err)
	}
	outerTypeOnly := writeFilter(t, nestedHandshake+`sed 's/"listType":"ordered"/"listType":"unordered"/g'`)
	if _, err := RunFilters(nestedFilterFixture(), []string{outerTypeOnly}, time.Second); err == nil || !strings.Contains(err.Error(), "list types changed") {
		t.Fatalf("changed outer list type accepted: %v", err)
	}
}

func TestNestedListFilterAllowsSiblingReorder(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 unavailable")
	}
	filter := writeFilter(t, nestedHandshake+`python3 -c 'import json,sys; d=json.load(sys.stdin); d["contentBlocks"][0]["elements"][0]["items"][0]["subPoints"].reverse(); json.dump(d,sys.stdout)'`)
	out, err := RunFilters(nestedFilterFixture(), []string{filter}, time.Second)
	if err != nil {
		t.Fatalf("sibling reorder rejected: %v", err)
	}
	children := out.ContentBlocks[0].Elements[0].(*ast.PointsElement).Items[0].SubPoints
	if children[0].NodeID != "ChildB" || children[1].NodeID != "ChildA" {
		t.Fatalf("filter reorder not applied: %v", children)
	}
}

func TestNestedListFilterCombinedCapabilities(t *testing.T) {
	doc := nestedFilterFixture()
	pos := diagnostics.NewPosition(1, 1)
	table := ast.NewTableElement(pos)
	table.NodeID = "TableA"
	table.TableRows = []ast.TableRow{{NodeID: "RowA", Cells: []ast.TableRowCell{{NodeID: "CellA", Content: "A"}}}}
	table.SyncTableViews()
	doc.ContentBlocks[0].Elements = append(doc.ContentBlocks[0].Elements, table)
	ast.SetTableContract(doc)
	missing := writeFilter(t, nestedHandshake+"cat")
	if _, err := RunFilters(doc, []string{missing}, time.Second); err == nil || !strings.Contains(err.Error(), ast.TableRowsCapability) {
		t.Fatalf("combined document accepted list-only filter: %v", err)
	}
	combined := writeFilter(t, `if [ "$1" = "--ziradocs-capabilities" ]; then
echo '{"astSchemaVersions":["2.16.0"],"features":["nested-list-types-v1","table-rows-v1"]}'
exit 0
fi
cat`)
	if _, err := RunFilters(doc, []string{combined}, time.Second); err != nil {
		t.Fatal(err)
	}
}
