// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package ast

import (
	"encoding/json"
	"testing"

	"go.ziradocs.com/core/v2/diagnostics"
)

func TestImageTitleContract(t *testing.T) {
	schema := compileASTSchema(t)
	pos := diagnostics.NewPosition(1, 1)
	doc := NewAST(pos)
	doc.ContentBlocks = []ContentBlock{*NewContentBlock(pos, "content")}
	img := NewImageElement(pos, "a.png", "A")
	doc.ContentBlocks[0].Elements = []Element{img}
	SetTableContract(doc)
	if doc.SchemaVersion != LegacySchemaVersion || len(doc.Capabilities) != 0 {
		t.Fatalf("sin título: contract = %s %v", doc.SchemaVersion, doc.Capabilities)
	}
	img.Title = "Título"
	SetTableContract(doc)
	if doc.SchemaVersion != ImageTitleSchemaVersion || len(doc.Capabilities) != 1 || doc.Capabilities[0] != ImageTitleCapability {
		t.Fatalf("contract = %s %v", doc.SchemaVersion, doc.Capabilities)
	}
	if img.ImageTitle() != "Título" {
		t.Error("ImageTitle no devuelve Title")
	}
	data, _ := json.Marshal(doc)
	decoded, err := DecodeAST(data)
	if err != nil {
		t.Fatal(err)
	}
	if got := decoded.ContentBlocks[0].Elements[0].(*ImageElement).Title; got != "Título" {
		t.Errorf("title tras decodificar = %q", got)
	}
	var base map[string]any
	_ = json.Unmarshal(data, &base)
	if err := schema.Validate(base); err != nil {
		t.Fatalf("schema rejected valid document: %v", err)
	}
	el := func(root map[string]any) map[string]any {
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
	check("legacy version", func(root map[string]any) { root["schemaVersion"] = LegacySchemaVersion; delete(root, "capabilities") })
	check("previous extension version", func(root map[string]any) { root["schemaVersion"] = TableAlignSchemaVersion })
	check("capability without title", func(root map[string]any) { delete(el(root), "title") })
	check("multi-line title", func(root map[string]any) { el(root)["title"] = "a\nb" })
}
