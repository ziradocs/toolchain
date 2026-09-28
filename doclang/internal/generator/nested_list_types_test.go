package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuin/goldmark"
	goldast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
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
	child.SubPoints = append(child.SubPoints, *ast.NewPointItem(pos, "GrandchildA"))
	parent.SubPoints = append(parent.SubPoints, *child, *ast.NewPointItem(pos, "ChildB"))
	points.Items = append(points.Items, *parent)
	second := ast.NewPointItem(pos, "ParentB")
	second.SubListType = "ordered"
	second.SubPoints = append(second.SubPoints, *ast.NewPointItem(pos, "ChildC"))
	points.Items = append(points.Items, *second)
	return points
}

// Goldmark builds a CommonMark tree here; checking marker text alone would
// miss a child list parsed as a sibling of its numbered parent.
func commonMarkListShape(t *testing.T, markdown string) string {
	t.Helper()
	root := goldmark.New().Parser().Parse(text.NewReader([]byte(markdown)))
	list, ok := root.FirstChild().(*goldast.List)
	if !ok || list.NextSibling() != nil {
		t.Fatalf("expected one root list, got:\n%s", markdown)
	}
	var shape func(*goldast.List) string
	shape = func(list *goldast.List) string {
		kind := "unordered"
		if list.IsOrdered() {
			kind = "ordered"
		}
		var items []string
		for node := list.FirstChild(); node != nil; node = node.NextSibling() {
			if node.Kind() != goldast.KindListItem {
				t.Fatalf("unexpected list child %s", node.Kind())
			}
			var nested []string
			for child := node.FirstChild(); child != nil; child = child.NextSibling() {
				if sub, ok := child.(*goldast.List); ok {
					nested = append(nested, shape(sub))
				}
			}
			items = append(items, fmt.Sprintf("%d[%s]", len(items)+1, strings.Join(nested, ",")))
		}
		return kind + "(" + strings.Join(items, ",") + ")"
	}
	return shape(list)
}

func TestNestedListTypesMarkdownCommonMarkTree(t *testing.T) {
	points := typedDocumentPoints()
	md := NewMarkdownGenerator(newTestLogger()).renderElement(points)
	want := "ordered(1[unordered(1[ordered(1[])],2[])],2[ordered(1[])])"
	if got := commonMarkListShape(t, md); got != want {
		t.Fatalf("CommonMark tree = %s, want %s:\n%s", got, want, md)
	}
	if !strings.Contains(md, "1. ParentA\n   - ChildA\n     1. GrandchildA\n   - ChildB\n2. ParentB\n   1. ChildC\n") {
		t.Fatalf("Markdown marker/content columns changed:\n%s", md)
	}
}

func TestNestedListTypesMarkdownMarkerWidthAtTen(t *testing.T) {
	points := ast.NewPointsElement(diagnostics.NewPosition(1, 1))
	points.ListType = "ordered"
	for i := 1; i <= 10; i++ {
		item := ast.NewPointItem(points.GetPosition(), fmt.Sprintf("Parent%d", i))
		if i == 9 || i == 10 {
			item.SubListType = "unordered"
			item.SubPoints = append(item.SubPoints, *ast.NewPointItem(points.GetPosition(), fmt.Sprintf("Child%d", i)))
		}
		points.Items = append(points.Items, *item)
	}
	md := NewMarkdownGenerator(newTestLogger()).renderElement(points)
	if !strings.Contains(md, "9. Parent9\n   - Child9\n10. Parent10\n    - Child10\n") {
		t.Fatalf("child indentation did not follow marker width:\n%s", md)
	}
	want := "ordered(1[],2[],3[],4[],5[],6[],7[],8[],9[unordered(1[])],10[unordered(1[])])"
	if got := commonMarkListShape(t, md); got != want {
		t.Fatalf("CommonMark tree = %s, want %s:\n%s", got, want, md)
	}
}

func TestNestedListTypesDOCXRejectsBeforeOutput(t *testing.T) {
	doc := newTestAST()
	doc.ContentBlocks[0].Elements = append(doc.ContentBlocks[0].Elements, typedDocumentPoints())
	ast.SetTableContract(doc)
	output := filepath.Join(t.TempDir(), "nested.docx")
	err := NewDOCXGenerator(newTestLogger(), "").Generate(doc, output, GeneratorOptions{Format: "docx"})
	if err == nil || !strings.Contains(err.Error(), "nested-list-types-v1") {
		t.Fatalf("DOCX accepted typed nested lists: %v", err)
	}
	if _, statErr := os.Stat(output); !os.IsNotExist(statErr) {
		t.Fatalf("DOCX wrote output before rejecting capability: %v", statErr)
	}
}
