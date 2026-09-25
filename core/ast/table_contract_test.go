package ast

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v5"
	"go.ziradocs.com/core/v2/diagnostics"
)

func contractFixture() *AST {
	pos := diagnostics.NewPosition(1, 1)
	doc := NewAST(pos)
	block := NewContentBlock(pos, "content")
	block.NodeID = "SlideA"
	table := NewTableElement(pos)
	table.NodeID = "TableA"
	table.TableRows = []TableRow{
		{NodeID: "HeaderRow", Section: "header", Cells: []TableRowCell{{NodeID: "H1", Content: "Key", IsHeader: true}, {NodeID: "H2", Content: "Value", IsHeader: true}}},
		{NodeID: "RowA", Cells: []TableRowCell{{NodeID: "A1", Content: "A", RowSpan: 2}, {NodeID: "A2", Content: "1"}}},
		{NodeID: "RowB", Cells: []TableRowCell{{NodeID: "B2", Content: "2"}}},
	}
	table.SyncTableViews()
	block.Elements = append(block.Elements, table)
	doc.ContentBlocks = append(doc.ContentBlocks, *block)
	SetTableContract(doc)
	return doc
}

func TestTableRowsSpanAnchorsAndInvalidation(t *testing.T) {
	doc := contractFixture()
	if err := ValidateTableContract(doc); err != nil {
		t.Fatal(err)
	}
	table := doc.ContentBlocks[0].Elements[0].(*TableElement)
	if table.Rows[1][0] != "A" || len(table.TableRows[2].Cells) != 1 {
		t.Fatalf("covered coordinate unexpectedly authored: %+v %+v", table.Rows, table.TableRows)
	}
	if target, ok := ResolveTableIdentity(doc, "A1"); !ok || target.Kind != "cell" || target.Cell.Content != "A" {
		t.Fatalf("anchor identity unavailable: %+v %v", target, ok)
	}
	// Removing the covered row invalidates the anchor's rowspan. It is never
	// silently retargeted to a new coordinate.
	table.TableRows = table.TableRows[:2]
	table.SyncTableViews()
	if _, found := ResolveTableIdentity(doc, "RowB"); found {
		t.Fatal("deleted row still resolves")
	}
	if err := ValidateTableRows(table); err == nil || !strings.Contains(err.Error(), "outside the table grid") {
		t.Fatalf("expected invalidated span, got %v", err)
	}
	// A span that reaches a different semantic section cannot be represented
	// as valid HTML thead/tbody/tfoot row groups.
	table.TableRows = []TableRow{
		{Section: "header", Cells: []TableRowCell{{Content: "H", IsHeader: true, RowSpan: 2}}},
		{Section: "body", Cells: []TableRowCell{}},
	}
	table.SyncTableViews()
	if err := ValidateTableRows(table); err == nil || !strings.Contains(err.Error(), "section boundary") {
		t.Fatalf("cross-section span accepted: %v", err)
	}
}

func TestTableRowsRawDecodeRejectsLossAndConflict(t *testing.T) {
	doc := contractFixture()
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeAST(data); err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	table := raw["contentBlocks"].([]any)[0].(map[string]any)["elements"].([]any)[0].(map[string]any)
	table["rows"] = []any{[]any{"changed"}}
	bad, _ := json.Marshal(raw)
	if _, err := DecodeAST(bad); err == nil || !strings.Contains(err.Error(), "projections") {
		t.Fatalf("conflicting projection accepted: %v", err)
	}
	delete(table, "tableRows")
	bad, _ = json.Marshal(raw)
	if _, err := DecodeAST(bad); err == nil || !strings.Contains(err.Error(), "without tableRows") {
		t.Fatalf("lost authored rows accepted: %v", err)
	}
	raw["schemaVersion"] = LegacySchemaVersion
	delete(raw, "capabilities")
	table["tableRows"] = []any{map[string]any{"nodeId": "Row", "cells": []any{map[string]any{"content": "A"}}}}
	bad, _ = json.Marshal(raw)
	if _, err := DecodeAST(bad); err == nil || !strings.Contains(err.Error(), "requires schemaVersion") {
		t.Fatalf("tableRows under legacy version accepted: %v", err)
	}
}

func TestTableRowsDocumentWideCollisions(t *testing.T) {
	doc := contractFixture()
	table := doc.ContentBlocks[0].Elements[0].(*TableElement)
	table.TableRows[1].Cells[0].NodeID = "SlideA"
	if issues := ValidateNodeIDs(doc); len(issues) == 0 || !strings.Contains(issues[0].Message, "duplicate") {
		t.Fatalf("slide/cell collision accepted: %v", issues)
	}
	table.TableRows[1].Cells[0].NodeID = "TableA"
	if issues := ValidateNodeIDs(doc); len(issues) == 0 {
		t.Fatal("table/cell collision accepted")
	}
	table.TableRows[1].Cells[0].NodeID = "RowA"
	if issues := ValidateNodeIDs(doc); len(issues) == 0 {
		t.Fatal("row/cell collision accepted")
	}
}

func TestTableRowsSchemaVersions(t *testing.T) {
	compiler := jsonschema.NewCompiler()
	schema, err := compiler.Compile(filepath.Join("..", "..", "schema", "ast.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, doc := range []*AST{NewAST(diagnostics.NewPosition(1, 1)), contractFixture()} {
		data, err := json.Marshal(doc)
		if err != nil {
			t.Fatal(err)
		}
		var value any
		if err := json.Unmarshal(data, &value); err != nil {
			t.Fatal(err)
		}
		if err := schema.Validate(value); err != nil {
			t.Fatalf("schema rejected version %s: %v", doc.SchemaVersion, err)
		}
	}
}

func TestNestedTableRowsProjectionConflict(t *testing.T) {
	doc := contractFixture()
	block := &doc.ContentBlocks[0]
	table := block.Elements[0]
	callout := NewSpecialBlockElement(diagnostics.NewPosition(1, 1), "info", "")
	callout.Elements = []Element{table}
	block.Elements = []Element{callout}
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeAST(data); err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	nested := raw["contentBlocks"].([]any)[0].(map[string]any)["elements"].([]any)[0].(map[string]any)["elements"].([]any)[0].(map[string]any)
	nested["headers"] = []any{"Wrong"}
	bad, _ := json.Marshal(raw)
	if _, err := DecodeAST(bad); err == nil || !strings.Contains(err.Error(), "projections") {
		t.Fatalf("nested projection conflict accepted: %v", err)
	}
}
