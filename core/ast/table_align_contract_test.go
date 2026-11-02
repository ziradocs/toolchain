// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package ast

import (
	"encoding/json"
	"testing"

	"go.ziradocs.com/core/v2/diagnostics"
)

func TestTableAlignContract(t *testing.T) {
	schema := compileASTSchema(t)
	pos := diagnostics.NewPosition(1, 1)
	doc := NewAST(pos)
	doc.ContentBlocks = []ContentBlock{*NewContentBlock(pos, "content")}
	table := NewTableElement(pos)
	table.Headers = []string{"A", "B"}
	table.Rows = [][]string{{"1", "2"}}
	table.ColumnAlign = []string{"", "right"}
	doc.ContentBlocks[0].Elements = []Element{table}
	SetTableContract(doc)
	if doc.SchemaVersion != TableAlignSchemaVersion || len(doc.Capabilities) != 1 || doc.Capabilities[0] != TableAlignCapability {
		t.Fatalf("contract = %s %v", doc.SchemaVersion, doc.Capabilities)
	}
	data, _ := json.Marshal(doc)
	if _, err := DecodeAST(data); err != nil {
		t.Fatal(err)
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
	check("previous extension version", func(root map[string]any) { root["schemaVersion"] = ContentMarkdownSchemaVersion })
	check("capability without columnAlign", func(root map[string]any) { delete(el(root), "columnAlign") })
	check("unknown alignment", func(root map[string]any) { el(root)["columnAlign"] = []any{"", "justify"} })
	check("nothing declared", func(root map[string]any) { el(root)["columnAlign"] = []any{"", ""} })

	// Una entrada por columna: el schema no conoce Headers, el decoder sí.
	table.ColumnAlign = []string{"right"}
	SetTableContract(doc)
	short, _ := json.Marshal(doc)
	if _, err := DecodeAST(short); err == nil {
		t.Error("decoder accepted columnAlign shorter than headers")
	}
}
