package formatter

import (
	"encoding/json"
	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
	"reflect"
	"strings"
	"testing"
)

func literalParse(t *testing.T, source string, document bool) *ast.AST {
	t.Helper()
	p := parser.New(util.NewNoop())
	p.SetNormalization(false)
	var doc *ast.AST
	if document {
		d, issues := p.ParseDocument(source, "fixture.doclang")
		doc = d
		for _, i := range issues {
			if i.IsError() {
				t.Fatal(i.Message)
			}
		}
	} else {
		d, issues := p.Parse(source, "fixture.slidelang")
		doc = d
		for _, i := range issues {
			if i.IsError() {
				t.Fatal(i.Message)
			}
		}
	}
	if doc == nil {
		t.Fatal("nil AST")
	}
	return doc
}

func TestJSONLiteralPayloadSourceToAST(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		want       string
	}{
		{"code", "```go\n\n  x := 1\n\n```", "\n  x := 1\n"},
		{"code_group", ":::code-group\n```go [Tab]\n\n  x := 1\n\n```\n:::", "\n  x := 1\n"},
		{"math", "<<math>>\n\n  x = 1\n \n  y = 2  \n\n<<end>>", "\n  x = 1\n \n  y = 2  \n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			caps := ""
			if tc.name == "math" {
				caps = "ast_capabilities: [math-source-v1]\n"
			}
			source := "---\nmode: flex\n" + caps + "---\n# Literal\n\n" + tc.body + "\n"
			for _, document := range []bool{false, true} {
				doc := literalParse(t, source, document)
				wantPayload := func(d *ast.AST) string {
					t.Helper()
					for _, b := range d.ContentBlocks {
						for _, e := range b.Elements {
							switch x := e.(type) {
							case *ast.CodeElement:
								return x.Content
							case *ast.CodeGroupElement:
								return x.CodeBlocks[0].Content
							case *ast.MathElement:
								return x.Content
							}
						}
					}
					t.Fatal("payload missing")
					return ""
				}
				if got := wantPayload(doc); got != tc.want {
					t.Fatalf("source payload %q want %q", got, tc.want)
				}
				data, _ := json.Marshal(doc)
				decoded, err := ast.DecodeAST(data)
				if err != nil {
					t.Fatal(err)
				}
				formats := []func(*ast.AST) (string, error){FormatStrict}
				if document {
					formats = []func(*ast.AST) (string, error){FormatDocument, FormatDocumentStrict}
				}
				for _, format := range formats {
					out, err := format(decoded)
					if err != nil {
						t.Fatal(err)
					}
					again := literalParse(t, out, document)
					if got := wantPayload(again); got != tc.want {
						t.Fatalf("roundtrip %q want %q\n%s", got, tc.want, out)
					}
				}
			}
		})
	}
}

func TestListStartSourceAndJSONRoundTrips(t *testing.T) {
	source := "---\nmode: strict\nast_capabilities: [nested-list-types-v1, list-start-v1]\n---\nSLIDE content\n  <!-- node-id: ListA -->\n  POINTS\n    <!-- node-id: ParentA -->\n    3. Three\n      <!-- node-id: ChildA -->\n      17. Seventeen\n      <!-- node-id: ChildB -->\n      18. Eighteen\n    <!-- node-id: ParentB -->\n    4. Four\n"
	doc := literalParse(t, source, false)
	p := doc.ContentBlocks[0].Elements[0].(*ast.PointsElement)
	if p.Start == nil || *p.Start != 3 || p.Items[0].SubListStart == nil || *p.Items[0].SubListStart != 17 {
		t.Fatalf("source ordinals lost: %+v", p)
	}
	raw, _ := json.Marshal(doc)
	decoded, err := ast.DecodeAST(raw)
	if err != nil {
		t.Fatal(err)
	}
	out, err := FormatStrict(decoded)
	if err != nil {
		t.Fatal(err)
	}
	again := literalParse(t, out, false).ContentBlocks[0].Elements[0].(*ast.PointsElement)
	if !reflect.DeepEqual(p.Start, again.Start) || !reflect.DeepEqual(p.Items[0].SubListStart, again.Items[0].SubListStart) || p.NodeID != again.NodeID {
		t.Fatalf("list starts/identity lost\n%s", out)
	}
	for _, document := range []bool{false, true} {
		for _, body := range []string{"1. One\n2. Two", "100. Hundred\n101. Next", "9007199254740991. Max"} {
			d := literalParse(t, "---\nmode: flex\nast_capabilities: [list-start-v1]\n---\n# List\n\n"+body+"\n", document)
			if !ast.UsesListStart(d) || d.SchemaVersion != ast.ListStartSchemaVersion {
				t.Fatalf("opt-in ordinal missing for %s", body)
			}
		}
	}
	legacy := literalParse(t, strings.Replace(source, ", list-start-v1", "", 1), false)
	if ast.UsesListStart(legacy) {
		t.Fatal("legacy gained ordinal contract")
	}
}

func TestListStartRejectsInvalidSource(t *testing.T) {
	for _, body := range []string{"3. First\n5. Gap", "0. Zero", "9007199254740992. Unsafe", "a. Alpha", "1. First\n0. Zero", "1. First\n  2. Child"} {
		for _, mode := range []string{"strict", "flex"} {
			source := "---\nmode: " + mode + "\nast_capabilities: [list-start-v1]\n---\n"
			if mode == "strict" {
				source += "SLIDE content\n  POINTS\n" + indent(body, 4)
			} else {
				source += "# List\n\n" + body
			}
			p := parser.New(util.NewNoop())
			p.SetNormalization(false)
			_, issues := p.Parse(source, "bad.slidelang")
			found := false
			for _, i := range issues {
				found = found || i.IsError()
			}
			if !found {
				t.Fatalf("invalid ordinal accepted: %s", source)
			}
		}
	}
}

func TestLiteralRepresentationGuardsAndDocumentAuthority(t *testing.T) {
	doc := literalParse(t, "---\nmode: strict\n---\nSLIDE content\n  <<math>>\n    x=1\n  <<end>>\n", false)
	m := doc.ContentBlocks[0].Elements[0].(*ast.MathElement)
	m.Content = "  x=1\n\n  y=2  "
	if _, err := FormatStrict(doc); err == nil {
		t.Fatal("legacy formatter silently lost whitespace")
	}
	// Document capability alone is sufficient, even without parsing/DecodeAST.
	doc.Capabilities = []string{ast.MathSourceCapability}
	doc.SchemaVersion = ast.MathSourceSchemaVersion
	out, err := FormatStrict(doc)
	if err != nil {
		t.Fatal(err)
	}
	got := literalParse(t, out, false).ContentBlocks[0].Elements[0].(*ast.MathElement).Content
	if got != m.Content {
		t.Fatalf("programmatic document policy lost %q", got)
	}
	for _, content := range []string{"x\n<<end>>\ny", "caption: literal", "label: literal", "x\n---\ny"} {
		m.Content = content
		if _, err := FormatStrict(doc); err == nil {
			t.Fatalf("delimiter collision accepted %q", content)
		}
	}
	groupDoc := literalParse(t, "---\nmode: flex\n---\n# Group\n\n:::code-group\n```go [Tab]\nx\n```\n:::\n", false)
	group := groupDoc.ContentBlocks[0].Elements[0].(*ast.CodeGroupElement)
	group.CodeBlocks[0].Label = " Tab "
	if _, err := FormatStrict(groupDoc); err == nil {
		t.Fatal("tab label trimming silently accepted")
	}
}

func TestImageFollowedByHeadingPreservesSiblingIdentity(t *testing.T) {
	source := "---\nmode: strict\nast_capabilities: [typed-headings-v1]\n---\nSLIDE content\n  <!-- node-id: ImageA -->\n  IMAGE \"asset.png\" \"Alt\"\n  <!-- node-id: Context -->\n  SECTION \"After image\"\n    level: 3\n"
	doc := literalParse(t, source, false)
	if len(doc.ContentBlocks[0].Elements) != 2 {
		t.Fatal("image swallowed heading")
	}
	if doc.ContentBlocks[0].Elements[1].(*ast.HeadingElement).NodeID != "Context" {
		t.Fatal("heading identity lost")
	}
	out, err := FormatStrict(doc)
	if err != nil {
		t.Fatal(err)
	}
	if len(literalParse(t, out, false).ContentBlocks[0].Elements) != 2 {
		t.Fatal("roundtrip swallowed heading")
	}
}
