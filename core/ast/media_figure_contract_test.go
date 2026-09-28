// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package ast

import (
	"encoding/json"
	"reflect"
	"testing"

	"go.ziradocs.com/core/v2/diagnostics"
)

func mediaFigureFixture(withHeading bool) *AST {
	pos := diagnostics.NewPosition(1, 1)
	doc := NewAST(pos)
	doc.ContentBlocks = []ContentBlock{*NewContentBlock(pos, "content")}
	media := NewMediaElement(pos, "video", "demo.mp4")
	media.Poster = "poster.jpg"
	media.Caption = "Product demo"
	doc.ContentBlocks[0].Elements = []Element{media}
	if withHeading {
		doc.ContentBlocks[0].Elements = append([]Element{NewHeadingElement(pos, 3, "Demo", "heading-demo")}, doc.ContentBlocks[0].Elements...)
	}
	SetTableContract(doc)
	return doc
}

func TestMediaFigureContract(t *testing.T) {
	schema := compileASTSchema(t)
	for _, tc := range []struct {
		name    string
		heading bool
		want    []string
	}{
		{"media only", false, []string{MediaFigureCapability}},
		{"media and headings", true, []string{TypedHeadingsCapability, MediaFigureCapability}},
	} {
		doc := mediaFigureFixture(tc.heading)
		if doc.SchemaVersion != MediaFigureSchemaVersion || !reflect.DeepEqual(doc.Capabilities, tc.want) {
			t.Fatalf("%s: contract = %s %v", tc.name, doc.SchemaVersion, doc.Capabilities)
		}
		data, _ := json.Marshal(doc)
		decoded, err := DecodeAST(data)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		m := decoded.ContentBlocks[0].Elements[len(decoded.ContentBlocks[0].Elements)-1].(*MediaElement)
		if m.Poster != "poster.jpg" || m.Caption != "Product demo" {
			t.Fatalf("%s: decoded media = %#v", tc.name, m)
		}
		var value any
		_ = json.Unmarshal(data, &value)
		if err := schema.Validate(value); err != nil {
			t.Fatalf("%s: schema rejected valid document: %v", tc.name, err)
		}
	}
	// Un media sin poster ni caption sigue siendo legado.
	plain := NewAST(diagnostics.NewPosition(1, 1))
	plain.ContentBlocks = []ContentBlock{*NewContentBlock(diagnostics.NewPosition(1, 1), "content")}
	plain.ContentBlocks[0].Elements = []Element{NewMediaElement(diagnostics.NewPosition(1, 1), "video", "a.mp4")}
	SetTableContract(plain)
	if plain.SchemaVersion != LegacySchemaVersion || len(plain.Capabilities) != 0 {
		t.Fatalf("plain media contract = %s %v", plain.SchemaVersion, plain.Capabilities)
	}

	data, _ := json.Marshal(mediaFigureFixture(false))
	var base map[string]any
	_ = json.Unmarshal(data, &base)
	media := func(root map[string]any) map[string]any {
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
	check("legacy version with poster", func(root map[string]any) { root["schemaVersion"] = LegacySchemaVersion; delete(root, "capabilities") })
	check("typed-headings version", func(root map[string]any) { root["schemaVersion"] = TypedHeadingsSchemaVersion })
	check("capability without poster or caption", func(root map[string]any) {
		m := media(root)
		delete(m, "poster")
		delete(m, "caption")
	})
	check("missing capability", func(root map[string]any) { delete(root, "capabilities") })
}
