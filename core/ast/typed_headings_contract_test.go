// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package ast

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v5"
	"go.ziradocs.com/core/v2/diagnostics"
)

func typedHeadingFixture(withTable, withLists bool) *AST {
	pos := diagnostics.NewPosition(1, 1)
	var doc *AST
	switch {
	case withLists:
		doc = nestedListFixture(withTable)
	case withTable:
		doc = contractFixture()
	default:
		doc = NewAST(pos)
		doc.ContentBlocks = []ContentBlock{*NewContentBlock(pos, "content")}
	}
	heading := NewHeadingElement(pos, 3, "Parent **heading**", "heading-parent-heading")
	heading.NodeID = "HeadingA"
	doc.ContentBlocks[0].Elements = append([]Element{heading}, doc.ContentBlocks[0].Elements...)
	SetTableContract(doc)
	return doc
}

func compileASTSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	schema, err := jsonschema.NewCompiler().Compile(filepath.Join("..", "..", "schema", "ast.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	return schema
}

// La versión es la de la extensión más nueva usada y las capabilities son
// exactamente las usadas, en orden canónico, para toda combinación.
func TestTypedHeadingContractVersionMatrix(t *testing.T) {
	schema := compileASTSchema(t)
	for _, tc := range []struct {
		name         string
		table, lists bool
		want         []string
	}{
		{"headings", false, false, []string{TypedHeadingsCapability}},
		{"headings+table", true, false, []string{TableRowsCapability, TypedHeadingsCapability}},
		{"headings+lists", false, true, []string{NestedListTypesCapability, TypedHeadingsCapability}},
		{"all", true, true, []string{TableRowsCapability, NestedListTypesCapability, TypedHeadingsCapability}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc := typedHeadingFixture(tc.table, tc.lists)
			if doc.SchemaVersion != TypedHeadingsSchemaVersion || !reflect.DeepEqual(doc.Capabilities, tc.want) {
				t.Fatalf("contract = %s %v, want %s %v", doc.SchemaVersion, doc.Capabilities, TypedHeadingsSchemaVersion, tc.want)
			}
			data, _ := json.Marshal(doc)
			decoded, err := DecodeAST(data)
			if err != nil {
				t.Fatal(err)
			}
			h, ok := decoded.ContentBlocks[0].Elements[0].(*HeadingElement)
			if !ok || h.Text != "Parent **heading**" || h.Anchor != "heading-parent-heading" || h.Level != 3 || h.NodeID != "HeadingA" {
				t.Fatalf("decoded heading = %#v", decoded.ContentBlocks[0].Elements[0])
			}
			var value any
			_ = json.Unmarshal(data, &value)
			if err := schema.Validate(value); err != nil {
				t.Fatalf("schema rejected valid document: %v", err)
			}
		})
	}
}

// Un opt-in de fuente sin encabezados no infla la versión.
func TestTypedHeadingOptInWithoutHeadingsKeepsLegacyVersion(t *testing.T) {
	doc := NewAST(diagnostics.NewPosition(1, 1))
	doc.FrontMatter = NewFrontMatterNode(diagnostics.NewPosition(1, 1))
	doc.FrontMatter.ASTCapabilities = []string{TypedHeadingsCapability}
	PromoteTypedHeadings(doc)
	SetTableContract(doc)
	if doc.SchemaVersion != LegacySchemaVersion || len(doc.Capabilities) != 0 {
		t.Fatalf("contract = %s %v", doc.SchemaVersion, doc.Capabilities)
	}
}

// Cada forma de perder o falsear el encabezado tipado se rechaza tanto en el
// decoder como en el schema publicado.
func TestTypedHeadingContractRejectsSilentLoss(t *testing.T) {
	schema := compileASTSchema(t)
	data, _ := json.Marshal(typedHeadingFixture(false, false))
	var base map[string]any
	_ = json.Unmarshal(data, &base)
	heading := func(root map[string]any) map[string]any {
		return root["contentBlocks"].([]any)[0].(map[string]any)["elements"].([]any)[0].(map[string]any)
	}
	check := func(name string, mutate func(map[string]any)) {
		t.Helper()
		var root map[string]any
		copyData, _ := json.Marshal(base)
		_ = json.Unmarshal(copyData, &root)
		mutate(root)
		candidate, _ := json.Marshal(root)
		if _, err := DecodeAST(candidate); err == nil {
			t.Errorf("decoder accepted %s", name)
		}
		if err := schema.Validate(root); err == nil {
			t.Errorf("schema accepted %s", name)
		}
	}
	check("missing capability", func(root map[string]any) { delete(root, "capabilities") })
	check("unknown capability", func(root map[string]any) { root["capabilities"] = []any{"typed-headings-v2"} })
	check("duplicate capability", func(root map[string]any) {
		root["capabilities"] = []any{TypedHeadingsCapability, TypedHeadingsCapability}
	})
	check("legacy version", func(root map[string]any) { root["schemaVersion"] = LegacySchemaVersion; delete(root, "capabilities") })
	check("nested-list version", func(root map[string]any) { root["schemaVersion"] = NestedListSchemaVersion })
	check("heading under nested-list contract", func(root map[string]any) {
		root["schemaVersion"] = NestedListSchemaVersion
		root["capabilities"] = []any{NestedListTypesCapability}
	})
	check("capability without heading", func(root map[string]any) {
		block := root["contentBlocks"].([]any)[0].(map[string]any)
		block["elements"] = []any{}
	})
	check("unknown heading field", func(root map[string]any) { heading(root)["html"] = "<h3>x</h3>" })
	check("level zero", func(root map[string]any) { heading(root)["level"] = 0 })
	check("level seven", func(root map[string]any) { heading(root)["level"] = 7 })
	check("empty text", func(root map[string]any) { heading(root)["text"] = "  " })
	check("multiline text", func(root map[string]any) { heading(root)["text"] = "a\nb" })
	check("unsafe anchor", func(root map[string]any) { heading(root)["anchor"] = `x"><script>` })
	check("uppercase anchor", func(root map[string]any) { heading(root)["anchor"] = "Heading" })
}

// PromoteTypedHeadings solo actúa con opt-in y conserva identidad y posición.
func TestPromoteTypedHeadingsRequiresOptInAndKeepsIdentity(t *testing.T) {
	pos := diagnostics.NewPosition(4, 1)
	legacy := NewRawHTMLTextElement(pos, `<h3 id="heading-a">A</h3>`)
	legacy.Level, legacy.HeadingSource, legacy.HeadingAnchor, legacy.NodeID = 3, "A", "heading-a", "HeadingA"
	nested := NewRawHTMLTextElement(pos, `<h4 id="heading-b">B</h4>`)
	nested.Level, nested.HeadingSource, nested.HeadingAnchor = 4, "B", "heading-b"
	block := NewSpecialBlockElement(pos, "note", "")
	block.Elements = []Element{nested}
	build := func(caps []string) *AST {
		doc := NewAST(pos)
		doc.FrontMatter = NewFrontMatterNode(pos)
		doc.FrontMatter.ASTCapabilities = caps
		c := *legacy
		n := *nested
		b := *block
		b.Elements = []Element{&n}
		cb := NewContentBlock(pos, "content")
		cb.Elements = []Element{&c, &b}
		doc.ContentBlocks = []ContentBlock{*cb}
		return doc
	}
	withoutOptIn := build(nil)
	PromoteTypedHeadings(withoutOptIn)
	if _, ok := withoutOptIn.ContentBlocks[0].Elements[0].(*TextElement); !ok {
		t.Fatal("promoted without opt-in")
	}
	doc := build([]string{TypedHeadingsCapability})
	PromoteTypedHeadings(doc)
	h, ok := doc.ContentBlocks[0].Elements[0].(*HeadingElement)
	if !ok || h.NodeID != "HeadingA" || h.Position != pos || h.Text != "A" || h.Anchor != "heading-a" {
		t.Fatalf("top-level heading = %#v", doc.ContentBlocks[0].Elements[0])
	}
	if _, ok := doc.ContentBlocks[0].Elements[1].(*SpecialBlockElement).Elements[0].(*HeadingElement); !ok {
		t.Fatal("nested heading was not promoted")
	}
}
