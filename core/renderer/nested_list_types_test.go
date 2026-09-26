package renderer

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

func TestNestedListTypesHTMLPreservesHierarchyAndEscaping(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	points := ast.NewPointsElement(pos)
	points.ListType = "ordered"
	parent := ast.NewPointItem(pos, "ParentA")
	parent.SubListType = "unordered"
	child := ast.NewPointItem(pos, "ChildA")
	child.SubListType = "ordered"
	grandchild := ast.NewPointItem(pos, "<script>ChildB</script>")
	child.SubPoints = append(child.SubPoints, *grandchild)
	parent.SubPoints = append(parent.SubPoints, *child)
	points.Items = append(points.Items, *parent)
	html := RenderElementToHTML(points, nil, nil)
	if !strings.Contains(html, "<ol><li>ParentA<ul><li>ChildA<ol><li>") || !strings.Contains(html, "</li></ol></li></ul></li></ol>") {
		t.Fatalf("nested order lost: %s", html)
	}
	if strings.Contains(html, "<script>") {
		t.Fatalf("unsafe markup in nested item: %s", html)
	}
}
