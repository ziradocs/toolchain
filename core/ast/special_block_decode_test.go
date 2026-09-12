// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package ast

import (
	"encoding/json"
	"testing"

	"go.ziradocs.com/core/v2/diagnostics"
)

// TestSpecialBlockElement_DecodeRoundTrip cubre el tercer (y por ahora
// último) sitio []Element del AST — SpecialBlockElement.Elements, mismo
// mecanismo que ColumnElement.Elements (issue #9, audit 2026-09-11).
// encoding/json no puede deserializar una interfaz sellada sin ayuda
// explícita (Element.element() no exportado), así que sin
// SpecialBlockElement.UnmarshalJSON un ":::bloque" con contenido anidado
// perdería ese contenido al pasar por --format json → --filter → reparse.
func TestSpecialBlockElement_DecodeRoundTrip(t *testing.T) {
	pos := diagnostics.Position{Line: 1, Column: 1}

	block := NewSpecialBlockElement(pos, "info", "prosa suelta")
	block.Title = "Nota"
	block.Icon = "💡"
	nestedTable := NewTableElement(pos)
	nestedTable.Headers = []string{"A"}
	nestedTable.Rows = [][]string{{"1"}}
	block.Elements = append(block.Elements, NewTextElement(pos, "texto anidado"), nestedTable)

	raw, err := json.Marshal(block)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	decoded, err := DecodeElement(raw)
	if err != nil {
		t.Fatalf("DecodeElement: %v", err)
	}

	got, ok := decoded.(*SpecialBlockElement)
	if !ok {
		t.Fatalf("decoded es %T, want *SpecialBlockElement", decoded)
	}
	if got.BlockType != "info" || got.Title != "Nota" || got.Icon != "💡" || got.Content != "prosa suelta" {
		t.Errorf("campos escalares no sobrevivieron: %+v", got)
	}
	if len(got.Elements) != 2 {
		t.Fatalf("len(Elements) = %d, want 2: %+v", len(got.Elements), got.Elements)
	}
	gotText, ok := got.Elements[0].(*TextElement)
	if !ok || gotText.Content != "texto anidado" {
		t.Errorf("Elements[0] = %+v, want *TextElement con Content=\"texto anidado\"", got.Elements[0])
	}
	gotTable, ok := got.Elements[1].(*TableElement)
	if !ok || len(gotTable.Rows) != 1 || gotTable.Rows[0][0] != "1" {
		t.Errorf("Elements[1] = %+v, want *TableElement con Rows=[[\"1\"]]", got.Elements[1])
	}
}
