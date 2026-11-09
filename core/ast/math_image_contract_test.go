// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package ast

import (
	"encoding/json"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/diagnostics"
)

func TestMathImageContract(t *testing.T) {
	schema := compileASTSchema(t)
	pos := diagnostics.NewPosition(1, 1)
	doc := NewAST(pos)
	doc.ContentBlocks = []ContentBlock{*NewContentBlock(pos, "content")}
	m := NewMathElement(pos, "E = mc^2")
	m.Label, m.Number = "eq:a", 1
	doc.ContentBlocks[0].Elements = []Element{m}
	SetTableContract(doc)
	if doc.SchemaVersion != LegacySchemaVersion || len(doc.Capabilities) != 0 {
		t.Fatalf("sin imagen: contract = %s %v", doc.SchemaVersion, doc.Capabilities)
	}
	m.Image = "eq-a.png"
	SetTableContract(doc)
	if doc.SchemaVersion != MathImageSchemaVersion || len(doc.Capabilities) != 1 || doc.Capabilities[0] != MathImageCapability {
		t.Fatalf("contract = %s %v", doc.SchemaVersion, doc.Capabilities)
	}
	if m.MathImage() != "eq-a.png" {
		t.Error("MathImage no devuelve Image")
	}
	data, _ := json.Marshal(doc)
	decoded, err := DecodeAST(data)
	if err != nil {
		t.Fatal(err)
	}
	got := decoded.ContentBlocks[0].Elements[0].(*MathElement)
	if got.Image != "eq-a.png" || got.Label != "eq:a" || got.Number != 1 || got.Content != "E = mc^2" {
		t.Errorf("fórmula tras decodificar = %+v", got)
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
	check("previous extension version", func(root map[string]any) { root["schemaVersion"] = ImageTitleSchemaVersion })
	check("capability without image", func(root map[string]any) { delete(el(root), "image") })
	check("empty image", func(root map[string]any) { el(root)["image"] = "" })
	check("multi-line image", func(root map[string]any) { el(root)["image"] = "a\nb.png" })
	check("image on a non-math element", func(root map[string]any) {
		el(root)["type"] = string(NodeTypeImage)
		el(root)["source"] = "a.png"
	})

	// Junto con math-source-v1 la versión es la de la más nueva.
	doc.Capabilities = []string{MathSourceCapability}
	SetTableContract(doc)
	if doc.SchemaVersion != MathImageSchemaVersion || len(doc.Capabilities) != 2 || doc.Capabilities[0] != MathSourceCapability || doc.Capabilities[1] != MathImageCapability {
		t.Fatalf("con math-source: contract = %s %v", doc.SchemaVersion, doc.Capabilities)
	}
}

// La regla "image en un elemento que no es math" se sostiene sola: sin
// math-image-v1 declarada, la de presencia no tiene nada que rechazar.
func TestMathImageOnNonMathElementRejected(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	doc := NewAST(pos)
	doc.ContentBlocks = []ContentBlock{*NewContentBlock(pos, "content")}
	doc.ContentBlocks[0].Elements = []Element{NewImageElement(pos, "a.png", "A")}
	data, _ := json.Marshal(doc)
	var root map[string]any
	_ = json.Unmarshal(data, &root)
	root["contentBlocks"].([]any)[0].(map[string]any)["elements"].([]any)[0].(map[string]any)["image"] = "b.png"
	candidate, _ := json.Marshal(root)
	_, err := DecodeAST(candidate)
	if err == nil || !strings.Contains(err.Error(), "image on a non-math element") {
		t.Fatalf("err = %v", err)
	}
}
