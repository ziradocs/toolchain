package formatter

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
)

const typedStrictSource = `---
mode: strict
ast_capabilities: [nested-list-types-v1]
---
SLIDE content
  title: "List"
  <!-- node-id: PointsA -->
  POINTS
    <!-- node-id: ParentA -->
    1. ParentA
      <!-- node-id: ChildA -->
      - ChildA
        <!-- node-id: GrandchildA -->
        1. GrandchildA
      <!-- node-id: ChildB -->
      - ChildB
    <!-- node-id: ParentB -->
    2. ParentB
      <!-- node-id: ChildC -->
      1. ChildC
`

type pointShape struct {
	ID, Content, ChildType string
	Children               []pointShape
}

func shapes(items []ast.PointItem) []pointShape {
	out := make([]pointShape, len(items))
	for i, item := range items {
		out[i] = pointShape{ID: item.NodeID, Content: item.Content, ChildType: item.SubListType, Children: shapes(item.SubPoints)}
	}
	return out
}

func parseTypedStrict(t *testing.T, source string) *ast.AST {
	t.Helper()
	p := parser.New(util.NewNoop())
	p.SetNormalization(false)
	doc, issues := p.Parse(source, "fixture.slidelang")
	for _, issue := range issues {
		if issue.IsError() {
			t.Fatalf("parse: %s", issue.Message)
		}
	}
	if doc == nil {
		t.Fatal("nil AST")
	}
	return doc
}

func TestNestedListTypesStrictRoundTrip(t *testing.T) {
	doc := parseTypedStrict(t, typedStrictSource)
	if doc.SchemaVersion != ast.SchemaVersion || !reflect.DeepEqual(doc.Capabilities, []string{ast.NestedListTypesCapability}) {
		t.Fatalf("contract: %s %v", doc.SchemaVersion, doc.Capabilities)
	}
	points := doc.ContentBlocks[0].Elements[0].(*ast.PointsElement)
	if points.NodeID != "PointsA" || points.ListType != "ordered" || points.Items[0].SubListType != "unordered" || points.Items[0].SubPoints[0].SubListType != "ordered" || points.Items[1].SubListType != "ordered" {
		t.Fatalf("types/identities lost: %+v", points)
	}
	encoded, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := ast.DecodeAST(encoded)
	if err != nil {
		t.Fatal(err)
	}
	formatted, err := FormatStrict(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(formatted, "ast_capabilities:") {
		t.Fatalf("opt-in lost: %s", formatted)
	}
	reparsed := parseTypedStrict(t, formatted)
	again := reparsed.ContentBlocks[0].Elements[0].(*ast.PointsElement)
	if !reflect.DeepEqual(shapes(points.Items), shapes(again.Items)) || again.ListType != points.ListType || again.NodeID != points.NodeID {
		t.Fatalf("round-trip changed list: %v -> %v", shapes(points.Items), shapes(again.Items))
	}
}

func TestNestedListTypesRejectMixedSiblingMarkers(t *testing.T) {
	source := strings.Replace(typedStrictSource, "      - ChildB", "      2. ChildB", 1)
	p := parser.New(util.NewNoop())
	p.SetNormalization(false)
	_, issues := p.Parse(source, "fixture.slidelang")
	for _, issue := range issues {
		if issue.IsError() && strings.Contains(issue.Message, "mixed markers") {
			return
		}
	}
	t.Fatalf("missing marker diagnostic: %v", issues)
}

func TestNestedListTypesAbsentOptInKeepsLegacyContract(t *testing.T) {
	source := strings.Replace(typedStrictSource, "ast_capabilities: [nested-list-types-v1]\n", "", 1)
	doc := parseTypedStrict(t, source)
	if doc.SchemaVersion != ast.LegacySchemaVersion || len(doc.Capabilities) != 0 || ast.UsesNestedListTypes(doc) {
		t.Fatalf("legacy document inflated: %s %v", doc.SchemaVersion, doc.Capabilities)
	}
}
