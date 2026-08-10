// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
)

// TestPlantUMLParser_StrictModeStopsAtStrictBlockBoundaryWithoutEnduml cubre
// issue #107: en modo strict, un <<plantuml>> sin @enduml se detiene en el
// siguiente límite de bloque strict en vez de tragárselo hasta EOF.
func TestPlantUMLParser_StrictModeStopsAtStrictBlockBoundaryWithoutEnduml(t *testing.T) {
	parser := &PlantUMLParser{}
	ctx := &ParseContext{
		Mode: "strict",
		Lines: []string{
			"<<plantuml>>",
			"@startuml",
			"Alice -> Bob: hello",
			"SLIDE next",
		},
	}

	result := parser.Parse(ctx, 0)
	if result.Error != nil {
		t.Fatalf("Parse() error = %v", result.Error)
	}
	if result.ConsumedLines != 3 {
		t.Fatalf("ConsumedLines = %d, want 3 (must not consume the SLIDE line)", result.ConsumedLines)
	}
	diagram := result.Element.(*ast.PlantUMLElement)
	if strings.Contains(diagram.Content, "SLIDE") {
		t.Errorf("Content = %q, must not have absorbed the SLIDE line", diagram.Content)
	}
}

// TestPlantUMLParser_FlexModeDoesNotStopOnContentStartingWithSLIDE es la
// regresión encontrada en code review de #107/PR #111: el chequeo de límite
// strict añadido al loop de <<plantuml>> corría sin condicionar el modo, así
// que en FLEX (donde el contenido va a columna 0 sin indentación) un
// diagrama con un participante/actor literalmente llamado "SLIDE" (p. ej.
// "SLIDE -> API: render") cortaba el bloque antes de tiempo. El chequeo debe
// estar gateado a ctx.Mode=="strict".
func TestPlantUMLParser_FlexModeDoesNotStopOnContentStartingWithSLIDE(t *testing.T) {
	parser := &PlantUMLParser{}
	ctx := &ParseContext{
		Mode: "flex",
		Lines: []string{
			"<<plantuml>>",
			"@startuml",
			"SLIDE -> API: render",
			"@enduml",
		},
	}

	result := parser.Parse(ctx, 0)
	if result.Error != nil {
		t.Fatalf("Parse() error = %v", result.Error)
	}
	diagram := result.Element.(*ast.PlantUMLElement)
	if !strings.Contains(diagram.Content, "SLIDE -> API: render") {
		t.Errorf("Content = %q, want it to contain %q — a flex-mode content line starting with \"SLIDE \" must not be treated as a strict boundary", diagram.Content, "SLIDE -> API: render")
	}
	if !strings.Contains(diagram.Content, "@enduml") {
		t.Errorf("Content = %q, want it to contain the closing @enduml", diagram.Content)
	}
}
