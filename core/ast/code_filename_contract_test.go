// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package ast

import (
	"encoding/json"
	"testing"

	"go.ziradocs.com/core/v2/diagnostics"
)

func TestCodeFilenameContract(t *testing.T) {
	schema := compileASTSchema(t)
	pos := diagnostics.NewPosition(1, 1)
	doc := NewAST(pos)
	doc.ContentBlocks = []ContentBlock{*NewContentBlock(pos, "content")}
	code := NewCodeElement(pos, "typescript", "const x = 1;")
	code.Filename = "renewals.ts"
	doc.ContentBlocks[0].Elements = []Element{code}
	SetTableContract(doc)
	if doc.SchemaVersion != CodeFilenameSchemaVersion || len(doc.Capabilities) != 1 || doc.Capabilities[0] != CodeFilenameCapability {
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
	check("media version", func(root map[string]any) { root["schemaVersion"] = MediaFigureSchemaVersion })
	check("capability without filename", func(root map[string]any) { delete(el(root), "filename") })
	check("filename with spaces", func(root map[string]any) { el(root)["filename"] = "a b.ts" })
	check("bracket filename", func(root map[string]any) { el(root)["filename"] = "[label]" })
}
