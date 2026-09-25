//go:build !js

package generator

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/util"
)

func typedSlidePoints() *ast.PointsElement {
	points := ast.NewPointsElement(pos())
	points.ListType = "ordered"
	parent := ast.NewPointItem(pos(), "ParentA")
	parent.SubListType = "unordered"
	child := ast.NewPointItem(pos(), "ChildA")
	child.SubListType = "ordered"
	child.SubPoints = append(child.SubPoints, *ast.NewPointItem(pos(), "ChildB"))
	parent.SubPoints = append(parent.SubPoints, *child)
	points.Items = append(points.Items, *parent)
	return points
}

func TestNestedListTypesSlideHTMLAndPPTX(t *testing.T) {
	points := typedSlidePoints()
	doc := ast.NewAST(pos())
	doc.FrontMatter = ast.NewFrontMatterNode(pos())
	block := ast.NewContentBlock(pos(), "content")
	block.Title = "List"
	block.Elements = append(block.Elements, points)
	doc.ContentBlocks = append(doc.ContentBlocks, *block)
	ast.SetTableContract(doc)
	g := New(util.NewNoop())
	html, err := g.RenderHTMLPreview(doc, GeneratorOptions{}, renderer.NewDefaultRenderContext())
	if err != nil {
		t.Fatal(err)
	}
	parent := strings.Index(html, "ParentA")
	child := strings.Index(html, "ChildA")
	grandchild := strings.Index(html, "ChildB")
	if parent < 0 || child <= parent || grandchild <= child || !strings.Contains(html[parent:child], "<ul>") || !strings.Contains(html[child:grandchild], "<ol>") {
		t.Fatalf("slide HTML lost nested types: %s", html)
	}
	xml := buildPPTXWithElements(t, "typed-list", points)
	for _, tc := range []struct{ text, level, bullet string }{
		{"ParentA", `lvl="0"`, "buAutoNum"},
		{"ChildA", `lvl="1"`, "buChar"},
		{"ChildB", `lvl="2"`, "buAutoNum"},
	} {
		at := strings.Index(xml, tc.text)
		if at < 0 {
			t.Fatalf("PPTX lost %s", tc.text)
		}
		start := strings.LastIndex(xml[:at], "<a:p>")
		if start < 0 {
			t.Fatalf("PPTX paragraph missing for %s", tc.text)
		}
		if !strings.Contains(xml[start:at], tc.level) || !strings.Contains(xml[start:at], tc.bullet) {
			t.Fatalf("PPTX lost %s level/type: %s", tc.text, xml[start:at])
		}
	}
}
