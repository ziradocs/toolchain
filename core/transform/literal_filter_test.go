package transform

import (
	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"strings"
	"testing"
	"time"
)

const literalHandshake = `if [ "$1" = "--ziradocs-capabilities" ]; then
echo '{"astSchemaVersions":["2.22.0"],"features":["nested-list-types-v1","list-start-v1","math-source-v1"]}'
exit 0
fi
`

func literalFilterFixture() *ast.AST {
	doc := nestedFilterFixture()
	points := doc.ContentBlocks[0].Elements[0].(*ast.PointsElement)
	a, b := int64(3), int64(7)
	points.Start = &a
	points.Items[0].SubListStart = &b
	doc.ContentBlocks[0].Elements = append(doc.ContentBlocks[0].Elements, ast.NewMathElement(diagnostics.NewPosition(1, 1), "  x\n\ny"))
	doc.Capabilities = append(doc.Capabilities, ast.MathSourceCapability)
	ast.SetTableContract(doc)
	return doc
}
func TestLiteralFilterNegotiationOwnershipAndPolicy(t *testing.T) {
	for _, script := range []string{"cat", `sed 's/ChildA text/Edited text/g'`} {
		out, err := RunFilters(literalFilterFixture(), []string{writeFilter(t, literalHandshake+script)}, time.Second)
		if err != nil {
			t.Fatal(err)
		}
		if !ast.UsesListStart(out) || !ast.UsesMathSource(out) {
			t.Fatal("literal capabilities lost")
		}
	}
	for _, script := range []string{
		`sed 's/"start":3/"start":4/g'`,
		`sed 's/"subListStart":7/"subListStart":8/g'`,
		`sed 's/"nodeId":"ChildA"/"nodeId":"Changed"/g'`,
		`sed 's/,"start":3//g;s/,"subListStart":7//g;s/,"list-start-v1"//g'`,
		`sed 's/,"math-source-v1"//g;s/2.22.0/2.21.0/g'`,
	} {
		if _, err := RunFilters(literalFilterFixture(), []string{writeFilter(t, literalHandshake+script)}, time.Second); err == nil {
			t.Fatalf("loss accepted: %s", script)
		}
	}
	old := writeFilter(t, nestedHandshake+"cat")
	if _, err := RunFilters(literalFilterFixture(), []string{old}, time.Second); err == nil || !strings.Contains(err.Error(), "does not support") {
		t.Fatalf("old filter accepted: %v", err)
	}
	_, err := RunBuiltins(literalFilterFixture(), []Transform{func(d *ast.AST) (*ast.AST, error) {
		for i, c := range d.Capabilities {
			if c == ast.MathSourceCapability {
				d.Capabilities = append(d.Capabilities[:i], d.Capabilities[i+1:]...)
				break
			}
		}
		ast.SetTableContract(d)
		return d, nil
	}})
	if err == nil {
		t.Fatal("builtin silently removed literal math policy")
	}
}
