package formatter

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
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

func TestNestedListTypesHomogeneousDepthThree(t *testing.T) {
	ordered := strings.Replace(strings.Replace(typedStrictSource, "- ChildA", "1. ChildA", 1), "- ChildB", "2. ChildB", 1)
	unordered := typedStrictSource
	for _, replacement := range [][2]string{{"1. ParentA", "- ParentA"}, {"2. ParentB", "- ParentB"}, {"1. GrandchildA", "- GrandchildA"}, {"1. ChildC", "- ChildC"}} {
		unordered = strings.Replace(unordered, replacement[0], replacement[1], 1)
	}
	for _, tc := range []struct{ source, kind string }{{ordered, "ordered"}, {unordered, "unordered"}} {
		doc := parseTypedStrict(t, tc.source)
		points := doc.ContentBlocks[0].Elements[0].(*ast.PointsElement)
		if points.ListType != tc.kind || points.Items[0].SubListType != tc.kind || points.Items[0].SubPoints[0].SubListType != tc.kind {
			t.Fatalf("homogeneous %s types lost: %+v", tc.kind, points)
		}
		formatted, err := FormatStrict(doc)
		if err != nil {
			t.Fatal(err)
		}
		reparsed := parseTypedStrict(t, formatted)
		if !reflect.DeepEqual(shapes(points.Items), shapes(reparsed.ContentBlocks[0].Elements[0].(*ast.PointsElement).Items)) {
			t.Fatalf("homogeneous %s round-trip changed", tc.kind)
		}
	}
}

func TestNestedListTypesIDsSurviveReorderEditAndInsert(t *testing.T) {
	doc := parseTypedStrict(t, typedStrictSource)
	points := doc.ContentBlocks[0].Elements[0].(*ast.PointsElement)
	points.Items[0], points.Items[1] = points.Items[1], points.Items[0]
	points.Items[0].Content = "ParentB edited"
	child := ast.NewPointItem(points.GetPosition(), "ChildD")
	child.NodeID = "ChildD"
	points.Items[0].SubPoints = append(points.Items[0].SubPoints, *child)
	want := shapes(points.Items)
	formatted, err := FormatStrict(doc)
	if err != nil {
		t.Fatal(err)
	}
	reparsed := parseTypedStrict(t, formatted)
	got := shapes(reparsed.ContentBlocks[0].Elements[0].(*ast.PointsElement).Items)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("IDs followed order/text instead of owners: %v -> %v", want, got)
	}
	points.Items[0].SubPoints[1].NodeID = "ChildA"
	if _, err := FormatStrict(doc); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate ID accepted: %v", err)
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

func TestNestedListTypesRejectEmptyOrOrphanedItems(t *testing.T) {
	for _, tc := range []struct{ name, source, expected string }{
		{"empty parent and orphan child", strings.Replace(typedStrictSource, "1. ParentA", "1. ", 1), "orphan nested list item"},
		{"invalid child marker", strings.Replace(typedStrictSource, "- ChildA", "1) ChildA", 1), "invalid list marker"},
		{"empty block", "---\nmode: strict\nast_capabilities: [nested-list-types-v1]\n---\nSLIDE content\n  POINTS\n", "empty POINTS list"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := parser.New(util.NewNoop())
			p.SetNormalization(false)
			_, issues := p.Parse(tc.source, "fixture.slidelang")
			for _, issue := range issues {
				if issue.IsError() && strings.Contains(issue.Message, tc.expected) {
					return
				}
			}
			t.Fatalf("missing %s diagnostic: %v", tc.expected, issues)
		})
	}
}

func TestNestedListTypesRejectUnknownOrDuplicateSourceCapability(t *testing.T) {
	for _, declaration := range []string{"ast_capabilities: [unknown-feature]", "ast_capabilities: [nested-list-types-v1, nested-list-types-v1]", "ast_capabilities: []", "ast_capabilities: null", "ast_capabilities: true"} {
		source := strings.Replace(typedStrictSource, "ast_capabilities: [nested-list-types-v1]", declaration, 1)
		p := parser.New(util.NewNoop())
		p.SetNormalization(false)
		_, issues := p.Parse(source, "fixture.slidelang")
		found := false
		for _, issue := range issues {
			if issue.IsError() && strings.Contains(issue.Message, "ast_capabilities") {
				found = true
			}
		}
		if !found {
			t.Fatalf("capability declaration %q accepted: %v", declaration, issues)
		}
	}
}

func TestNestedListTypesAbsentOptInKeepsLegacyContract(t *testing.T) {
	source := strings.Replace(typedStrictSource, "ast_capabilities: [nested-list-types-v1]\n", "", 1)
	doc := parseTypedStrict(t, source)
	if doc.SchemaVersion != ast.LegacySchemaVersion || len(doc.Capabilities) != 0 || ast.UsesNestedListTypes(doc) {
		t.Fatalf("legacy document inflated: %s %v", doc.SchemaVersion, doc.Capabilities)
	}
}

func TestNestedListTypesOptInWithoutNestedItemsUsesLegacyJSON(t *testing.T) {
	source := "---\nmode: strict\nast_capabilities: [nested-list-types-v1]\n---\nSLIDE content\n  title: \"List\"\n  POINTS\n    - ParentA\n"
	doc := parseTypedStrict(t, source)
	if doc.SchemaVersion != ast.LegacySchemaVersion || len(doc.Capabilities) != 0 || ast.UsesNestedListTypes(doc) {
		t.Fatalf("empty opt-in inflated AST: %s %v", doc.SchemaVersion, doc.Capabilities)
	}
	formatted, err := FormatStrict(doc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(formatted, "ast_capabilities:") {
		t.Fatal("source opt-in was lost")
	}
}

func TestNestedListTypesBothDSLsAndDialects(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		document     bool
		format       func(*ast.AST) (string, error)
	}{
		{"slides flex", "---\nmode: flex\nast_capabilities: [nested-list-types-v1]\n---\n# List\n1. ParentA\n  - ChildA\n    1. ChildB\n", false, FormatStrict},
		{"document flex", "---\nmode: flex\nast_capabilities: [nested-list-types-v1]\n---\n# List\n\n1. ParentA\n  - ChildA\n    1. ChildB\n", true, FormatDocument},
		{"document strict", "---\nmode: strict\nast_capabilities: [nested-list-types-v1]\n---\nSECTION \"List\"\n  POINTS\n    1. ParentA\n      - ChildA\n        1. ChildB\n", true, FormatDocumentStrict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := parser.New(util.NewNoop())
			p.SetNormalization(false)
			parse := func(source string) *ast.AST {
				var doc *ast.AST
				var issues []diagnostics.Diagnostic
				if tc.document {
					doc, issues = p.ParseDocument(source, "fixture.doclang")
				} else {
					doc, issues = p.Parse(source, "fixture.slidelang")
				}
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
			doc := parse(tc.source)
			if doc.SchemaVersion != ast.SchemaVersion {
				t.Fatalf("version %s", doc.SchemaVersion)
			}
			var points *ast.PointsElement
			for _, element := range doc.ContentBlocks[0].Elements {
				if p, ok := element.(*ast.PointsElement); ok {
					points = p
				}
			}
			if points == nil || points.Items[0].SubListType != "unordered" || points.Items[0].SubPoints[0].SubListType != "ordered" {
				t.Fatalf("nested types missing: %+v", points)
			}
			formatted, err := tc.format(doc)
			if err != nil {
				t.Fatal(err)
			}
			reparsed := parse(formatted)
			if !ast.UsesNestedListTypes(reparsed) {
				t.Fatal("formatter lost nested list types")
			}
		})
	}
}
