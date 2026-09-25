package transform

import (
	"os"
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
	parent := ast.NewPointItem(pos, "ParentA")
	parent.NodeID = "ParentA"
	parent.SubListType = "ordered"
	child := ast.NewPointItem(pos, "ChildA text")
	child.NodeID = "ChildA"
	parent.SubPoints = append(parent.SubPoints, *child)
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
	renamed := writeFilter(t, nestedHandshake+`sed 's/"nodeId":"ChildA"/"nodeId":"ChildB"/g'`)
	if _, err := RunFilters(nestedFilterFixture(), []string{renamed}, time.Second); err == nil || !strings.Contains(err.Error(), "ownership") {
		t.Fatalf("changed child identity accepted: %v", err)
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
