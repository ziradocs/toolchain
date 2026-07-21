// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"testing"

	"github.com/invopop/jsonschema"
	orderedmap "github.com/pb33f/ordered-map/v2"

	"go.ziradocs.com/core/ast"
)

func newDefWithProps(props ...string) *jsonschema.Schema {
	om := orderedmap.New[string, *jsonschema.Schema]()
	for _, p := range props {
		om.Set(p, &jsonschema.Schema{})
	}
	return &jsonschema.Schema{Properties: om}
}

func TestMergeDefs(t *testing.T) {
	dst := jsonschema.Definitions{"A": &jsonschema.Schema{}}
	src := jsonschema.Definitions{"B": &jsonschema.Schema{}}

	mergeDefs(dst, src)

	if _, ok := dst["A"]; !ok {
		t.Error("mergeDefs dropped a pre-existing entry")
	}
	if _, ok := dst["B"]; !ok {
		t.Error("mergeDefs did not merge the source entry")
	}
}

func TestSetTypeConst_Success(t *testing.T) {
	defs := jsonschema.Definitions{"TextElement": newDefWithProps("type", "content")}

	if err := setTypeConst(defs, "TextElement", ast.NodeTypeText); err != nil {
		t.Fatalf("setTypeConst() error = %v", err)
	}

	typeProp, _ := defs["TextElement"].Properties.Get("type")
	if typeProp.Const != string(ast.NodeTypeText) {
		t.Errorf("type.Const = %v, want %q", typeProp.Const, ast.NodeTypeText)
	}
}

// TestSetTypeConst_UnknownDef cubre el bug encontrado al revisar esta PR:
// antes de este fix, un defName con typo (o un struct renombrado) hacía que
// setTypeConst retornara en silencio sin error, dejando el schema sin su
// discriminador sin ningún aviso - y el CI de schema-drift no lo detectaría
// (regenera con el mismo typo y no hay diff contra lo committeado).
func TestSetTypeConst_UnknownDef(t *testing.T) {
	defs := jsonschema.Definitions{"TextElement": newDefWithProps("type")}

	if err := setTypeConst(defs, "TextElemnt", ast.NodeTypeText); err == nil {
		t.Fatal("expected an error for a typo'd/unknown definition name, got nil")
	}
}

func TestSetTypeConst_MissingTypeProperty(t *testing.T) {
	defs := jsonschema.Definitions{"NoType": newDefWithProps("content")}

	if err := setTypeConst(defs, "NoType", ast.NodeTypeText); err == nil {
		t.Fatal("expected an error when the definition has no \"type\" property, got nil")
	}
}

func TestSetElementsProperty_Success(t *testing.T) {
	defs := jsonschema.Definitions{"ContentBlock": newDefWithProps("elements")}
	union := &jsonschema.Schema{OneOf: []*jsonschema.Schema{{Ref: "#/$defs/TextElement"}}}

	if err := setElementsProperty(defs, "ContentBlock", union); err != nil {
		t.Fatalf("setElementsProperty() error = %v", err)
	}

	elementsProp, _ := defs["ContentBlock"].Properties.Get("elements")
	if elementsProp.Items != union {
		t.Error("setElementsProperty did not set Items to the given union schema")
	}
	if elementsProp.Type != "array" {
		t.Errorf("elements.Type = %q, want \"array\"", elementsProp.Type)
	}
}

// TestSetElementsProperty_UnknownDef misma clase de bug que
// TestSetTypeConst_UnknownDef, para el otro helper que antes no-opeaba en
// silencio.
func TestSetElementsProperty_UnknownDef(t *testing.T) {
	defs := jsonschema.Definitions{"ContentBlock": newDefWithProps("elements")}

	if err := setElementsProperty(defs, "ContentBlok", &jsonschema.Schema{}); err == nil {
		t.Fatal("expected an error for a typo'd/unknown definition name, got nil")
	}
}

func TestSetElementsProperty_MissingElementsProperty(t *testing.T) {
	defs := jsonschema.Definitions{"NoElements": newDefWithProps("content")}

	if err := setElementsProperty(defs, "NoElements", &jsonschema.Schema{}); err == nil {
		t.Fatal("expected an error when the definition has no \"elements\" property, got nil")
	}
}

func TestOverrideProperty_Success(t *testing.T) {
	defs := jsonschema.Definitions{"ChartElement": newDefWithProps("rawJSON")}
	objSchema := &jsonschema.Schema{Type: "object"}

	if err := overrideProperty(defs, "ChartElement", "rawJSON", objSchema); err != nil {
		t.Fatalf("overrideProperty() error = %v", err)
	}

	got, _ := defs["ChartElement"].Properties.Get("rawJSON")
	if got != objSchema {
		t.Error("overrideProperty did not replace the property with the given schema")
	}
	if got.Type != "object" {
		t.Errorf("rawJSON.Type = %q, want \"object\"", got.Type)
	}
}

// TestOverrideProperty_UnknownDef y _MissingProperty cubren la misma clase de
// bug que TestSetTypeConst_UnknownDef: un defName/propName con typo debe fallar
// ruidosamente en vez de no-opear.
func TestOverrideProperty_UnknownDef(t *testing.T) {
	defs := jsonschema.Definitions{"ChartElement": newDefWithProps("rawJSON")}

	if err := overrideProperty(defs, "ChartElemnt", "rawJSON", &jsonschema.Schema{Type: "object"}); err == nil {
		t.Fatal("expected an error for a typo'd/unknown definition name, got nil")
	}
}

func TestOverrideProperty_MissingProperty(t *testing.T) {
	defs := jsonschema.Definitions{"ChartElement": newDefWithProps("data")}

	if err := overrideProperty(defs, "ChartElement", "rawJSON", &jsonschema.Schema{Type: "object"}); err == nil {
		t.Fatal("expected an error when the definition has no \"rawJSON\" property, got nil")
	}
}
