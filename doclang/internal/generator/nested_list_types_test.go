package generator

import (
	"path/filepath"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

func typedDocumentPoints() *ast.PointsElement {
	pos := diagnostics.NewPosition(1, 1)
	points := ast.NewPointsElement(pos)
	points.ListType = "ordered"
	parent := ast.NewPointItem(pos, "ParentA")
	parent.SubListType = "unordered"
	child := ast.NewPointItem(pos, "ChildA")
	child.SubListType = "ordered"
	child.SubPoints = append(child.SubPoints, *ast.NewPointItem(pos, "ChildB"))
	parent.SubPoints = append(parent.SubPoints, *child)
	points.Items = append(points.Items, *parent)
	return points
}

func TestNestedListTypesMarkdownAndDOCX(t *testing.T) {
	points := typedDocumentPoints()
	md := NewMarkdownGenerator(newTestLogger()).renderElement(points)
	if !strings.Contains(md, "1. ParentA\n  - ChildA\n    1. ChildB\n") {
		t.Fatalf("Markdown lost nested list markers: %s", md)
	}
	doc := newTestAST()
	doc.ContentBlocks[0].Elements = append(doc.ContentBlocks[0].Elements, points)
	ast.SetTableContract(doc)
	output := filepath.Join(t.TempDir(), "nested.docx")
	if err := NewDOCXGenerator(newTestLogger(), "").Generate(doc, output, GeneratorOptions{Format: "docx"}); err != nil {
		t.Fatal(err)
	}
	xml := docxDocumentXML(t, output)
	parent, child, grandchild := strings.Index(xml, "ParentA"), strings.Index(xml, "ChildA"), strings.Index(xml, "ChildB")
	if parent < 0 || child <= parent || grandchild <= child {
		t.Fatalf("DOCX lost nested item order: %s", xml)
	}
	for _, tc := range []struct{ text, marker string }{{"ParentA", "1."}, {"ChildA", "•"}, {"ChildB", "1."}} {
		at := strings.Index(xml, tc.text)
		start := strings.LastIndex(xml[:at], "<w:p>")
		if start < 0 || !strings.Contains(xml[start:at], tc.marker) {
			t.Fatalf("DOCX lost %s marker %s: %s", tc.text, tc.marker, xml)
		}
	}
}
