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
	for _, body := range []string{"3. First\n5. Gap", "0. Zero", "-1. Negative", "1e3. Exponential", "3.5. Fraction", "9007199254740992. Unsafe", "a. Alpha", "1. First\n0. Zero", "1. First\n  2. Child"} {
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

func TestMathSourceStrictDedentAndDollarLiteral(t *testing.T) {
	for _, tc := range []struct{ body, want string }{
		{"  <<math>>\n    x\n      y  \n    \n      z\n  <<end>>", "x\n  y  \n\n  z"},
		{"  <<math>>\n\n      x\n \n  <<end>>", "\n  x\n "},
	} {
		d := literalParse(t, "---\nmode: strict\nast_capabilities: [math-source-v1]\n---\nSLIDE content\n"+tc.body+"\n", false)
		if got := d.ContentBlocks[0].Elements[0].(*ast.MathElement).Content; got != tc.want {
			t.Fatalf("dedent %q want %q", got, tc.want)
		}
	}
	for _, tc := range []struct{ body, want string }{
		{"$$  x  $$", "  x  "},
		{"$$\n\n  x  \n\n$$", "\n  x  \n"},
	} {
		d := literalParse(t, "---\nmode: flex\nast_capabilities: [math-source-v1]\n---\n# Math\n\n"+tc.body+"\n", false)
		if got := d.ContentBlocks[0].Elements[0].(*ast.MathElement).Content; got != tc.want {
			t.Fatalf("dollar %q want %q", got, tc.want)
		}
	}
	for _, body := range []string{"  <<math>>\n   x\n  <<end>>", "  <<math>>\n    x"} {
		p := parser.New(util.NewNoop())
		p.SetNormalization(false)
		_, issues := p.Parse("---\nmode: strict\nast_capabilities: [math-source-v1]\n---\nSLIDE content\n"+body+"\n", "bad.slidelang")
		found := false
		for _, i := range issues {
			found = found || i.IsError()
		}
		if !found {
			t.Fatalf("invalid literal math accepted: %q", body)
		}
	}
}

func TestListStartKeepsLegacyLongMarkerDispatch(t *testing.T) {
	source := "---\nmode: flex\n---\n# List\n\n100. Hundred\n101. Next\n"
	legacy := literalParse(t, source, false)
	if ast.UsesListStart(legacy) {
		t.Fatal("legacy gained list-start")
	}
	if _, ok := legacy.ContentBlocks[0].Elements[0].(*ast.TextElement); !ok {
		t.Fatal("legacy long marker dispatch changed")
	}
	opted := literalParse(t, strings.Replace(source, "mode: flex", "mode: flex\nast_capabilities: [list-start-v1]", 1), false)
	if got := opted.ContentBlocks[0].Elements[0].(*ast.PointsElement).Start; got == nil || *got != 100 {
		t.Fatal("opt-in long ordinal lost")
	}
}

func TestJSONMetadataRepresentabilityRejectsLoss(t *testing.T) {
	groupDoc := literalParse(t, "---\nmode: flex\n---\n# Group\n\n:::code-group\n```go [Tab]\nx\n```\n:::\n", false)
	group := groupDoc.ContentBlocks[0].Elements[0].(*ast.CodeGroupElement)
	for _, language := range []string{"", "go other", "go\nother", "go\tother"} {
		group.CodeBlocks[0].Language = language
		raw, _ := json.Marshal(groupDoc)
		decoded, err := ast.DecodeAST(raw)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := FormatStrict(decoded); err == nil {
			t.Fatalf("ambiguous language accepted %q", language)
		}
	}
	doc := literalParse(t, "---\nmode: strict\n---\nSLIDE content\n  <<math>>\n    x=1\n  <<end>>\n", false)
	m := doc.ContentBlocks[0].Elements[0].(*ast.MathElement)
	for _, field := range []string{"caption", "label"} {
		for _, value := range []string{"first\nsecond", "first\rsecond"} {
			m.Caption = ""
			m.Label = ""
			if field == "caption" {
				m.Caption = value
			} else {
				m.Label = value
			}
			raw, _ := json.Marshal(doc)
			decoded, err := ast.DecodeAST(raw)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := FormatStrict(decoded); err == nil {
				t.Fatalf("Math %s line break accepted", field)
			}
		}
	}
}

func TestListStartInvalidOrdinalMakesProgressAndKeepsFollowingBody(t *testing.T) {
	for _, document := range []bool{false, true} {
		for _, mode := range []string{"strict", "flex"} {
			source := "---\nmode: " + mode + "\nast_capabilities: [list-start-v1]\n---\n"
			if mode == "strict" {
				if document {
					source += "SECTION \"List\"\n"
				} else {
					source += "SLIDE content\n"
				}
				source += "  POINTS\n    -1. Invalid\n  TEXT\n    Following body\n"
			} else {
				source += "# List\n\n-1. Invalid\n\nFollowing body\n"
			}
			p := parser.New(util.NewNoop())
			p.SetNormalization(false)
			var doc *ast.AST
			var rejected bool
			if document {
				d, issues := p.ParseDocument(source, "invalid.doclang")
				doc = d
				for _, i := range issues {
					rejected = rejected || i.IsError()
				}
			} else {
				d, issues := p.Parse(source, "invalid.slidelang")
				doc = d
				for _, i := range issues {
					rejected = rejected || i.IsError()
				}
			}
			if !rejected {
				t.Fatal("invalid ordinal succeeded")
			}
			kept := false
			_ = ast.Walk(doc, func(n ast.Node) error {
				if e, ok := n.(*ast.TextElement); ok && strings.Contains(e.Content, "Following body") {
					kept = true
				}
				return nil
			})
			if !kept {
				t.Fatalf("invalid ordinal swallowed following body document=%v mode=%s", document, mode)
			}
		}
	}
}
