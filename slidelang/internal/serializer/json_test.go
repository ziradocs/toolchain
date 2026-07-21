// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package serializer

import (
	"encoding/json"
	"strings"
	"testing"

	"go.ziradocs.com/core/ast"
	"go.ziradocs.com/core/diagnostics"
)

// TestSerializeToJSON_SchemaVersionPresent cubre issue #8: --format json debe
// incluir siempre un campo "schemaVersion" semver en el nivel raíz del documento,
// para que los consumidores (p. ej. el viewer) puedan detectar breaking changes
// del contrato en tiempo de compilación/carga.
func TestSerializeToJSON_SchemaVersionPresent(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	astNode := ast.NewAST(pos)

	s := New()
	data, err := s.SerializeToJSON(astNode)
	if err != nil {
		t.Fatalf("SerializeToJSON error: %v", err)
	}

	var generic map[string]interface{}
	if err := json.Unmarshal(data, &generic); err != nil {
		t.Fatalf("failed to decode serialized AST: %v", err)
	}

	got, ok := generic["schemaVersion"].(string)
	if !ok || got == "" {
		t.Fatalf("schemaVersion missing or empty in serialized output: %v", generic["schemaVersion"])
	}
	if got != ast.SchemaVersion {
		t.Errorf("schemaVersion = %q, want %q (ast.SchemaVersion)", got, ast.SchemaVersion)
	}
}

// TestSerializeToJSON_MermaidSingleEscaped cubre issue #11: el contenido Mermaid
// (con comillas y saltos de línea, el caso real reportado por el viewer) debe
// serializarse una sola vez. Al decodificar el JSON, el contenido debe ser
// idéntico al original, sin comillas envolventes ni secuencias "\\n" literales.
func TestSerializeToJSON_MermaidSingleEscaped(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)

	mermaidSource := "flowchart TD\nA[\"texto con \\\"comillas\\\"\"] --> B"

	astNode := ast.NewAST(pos)
	block := ast.NewContentBlock(pos, "content")
	mermaid := ast.NewMermaidElement(pos, "flowchart", mermaidSource)
	block.Elements = append(block.Elements, mermaid)
	astNode.ContentBlocks = append(astNode.ContentBlocks, *block)

	s := New()
	data, err := s.SerializeToJSON(astNode)
	if err != nil {
		t.Fatalf("SerializeToJSON error: %v", err)
	}

	var decoded ast.AST
	// Element es una interfaz, así que decodificamos a un map genérico para
	// inspeccionar el campo "content" del elemento mermaid tal cual llegaría al viewer.
	var generic map[string]interface{}
	if err := json.Unmarshal(data, &generic); err != nil {
		t.Fatalf("failed to decode serialized AST: %v", err)
	}

	contentBlocks, _ := generic["contentBlocks"].([]interface{})
	if len(contentBlocks) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(contentBlocks))
	}
	elements, _ := contentBlocks[0].(map[string]interface{})["elements"].([]interface{})
	if len(elements) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elements))
	}
	elem := elements[0].(map[string]interface{})

	content, ok := elem["content"].(string)
	if !ok {
		t.Fatalf("content is not a string: %T", elem["content"])
	}

	if content != mermaidSource {
		t.Errorf("mermaid content roundtrip mismatch:\n  got:  %q\n  want: %q", content, mermaidSource)
	}
	if strings.HasPrefix(content, "\"") || strings.HasSuffix(content, "\"") {
		t.Errorf("mermaid content has wrapping quotes (double-encoded): %q", content)
	}
	if strings.Contains(content, `\n`) && !strings.Contains(content, "\n") {
		t.Errorf("mermaid content contains literal \\n instead of a real newline: %q", content)
	}

	_ = decoded // decoded no se usa directamente; se valida vía el map genérico arriba
}

// TestSerializeToJSON_ChartJSONMode_NoObjectObjectLiteral cubre issue #11: un chart
// en modo JSON directo debe anidar su config como objeto JSON real. Nunca debe
// aparecer el literal "[object Object]" ni un config re-escapado como string.
func TestSerializeToJSON_ChartJSONMode_NoObjectObjectLiteral(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)

	astNode := ast.NewAST(pos)
	block := ast.NewContentBlock(pos, "content")
	chart := ast.NewChartElement(pos, "bar")
	chart.RawJSON = json.RawMessage(`{"type":"bar","data":{"labels":["A","B"],"datasets":[{"label":"S1","data":[1,2]}]}}`)
	chart.IsJSONMode = true
	block.Elements = append(block.Elements, chart)
	astNode.ContentBlocks = append(astNode.ContentBlocks, *block)

	s := New()
	data, err := s.SerializeToJSON(astNode)
	if err != nil {
		t.Fatalf("SerializeToJSON error: %v", err)
	}

	if strings.Contains(string(data), "[object Object]") {
		t.Fatal("serialized AST contains literal \"[object Object]\"")
	}

	var generic map[string]interface{}
	if err := json.Unmarshal(data, &generic); err != nil {
		t.Fatalf("failed to decode serialized AST: %v", err)
	}
	contentBlocks := generic["contentBlocks"].([]interface{})
	elements := contentBlocks[0].(map[string]interface{})["elements"].([]interface{})
	elem := elements[0].(map[string]interface{})

	rawJSON, ok := elem["rawJSON"]
	if !ok {
		t.Fatal("expected 'rawJSON' key in serialized chart element")
	}
	if _, isString := rawJSON.(string); isString {
		t.Fatalf("rawJSON serialized as string (double-encoded), want nested object: %v", rawJSON)
	}
	rawJSONMap, ok := rawJSON.(map[string]interface{})
	if !ok {
		t.Fatalf("rawJSON is not a JSON object: %T", rawJSON)
	}
	if rawJSONMap["type"] != "bar" {
		t.Errorf("rawJSON.type = %v, want bar", rawJSONMap["type"])
	}
}
