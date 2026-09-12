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

// TestDetectPlantUMLType_AmbiguousBlockIsDeterministic reproduce un bug
// encontrado corriendo la verificación de corpus de issue #245 (no
// relacionado con ese issue): detectPlantUMLType recorría un
// map[string]*regexp.Regexp, y Go aleatoriza el orden de iteración de un
// map por proceso — así que un bloque PlantUML AMBIGUO (uno cuyo contenido
// matchea más de un patrón de categoría a la vez) podía resolver a un
// DiagramType distinto entre corridas del mismo binario sobre el mismo
// archivo. Reproducido con examples/advanced_elements_test.doclang: 5
// corridas de un mismo binario devolvieron "component" x3 y "sequence" x2
// para el mismo bloque.
//
// El fixture de abajo es ambiguo a propósito: "actor "/"database " matchean
// las categorías sequence, component, usecase Y deployment a la vez (ver
// los patrones en plantUMLTypePatterns). Con la lista ordenada (en vez de
// un map), "sequence" — la primera de la lista — tiene que ganar siempre,
// sin importar cuántas veces se llame ni en qué proceso.
func TestDetectPlantUMLType_AmbiguousBlockIsDeterministic(t *testing.T) {
	content := "@startuml\n" +
		"actor Customer\n" +
		"database \"Database\" as DB\n" +
		"Customer -> DB : query\n" +
		"@enduml\n"

	const want = "sequence"
	for i := 0; i < 50; i++ {
		if got := detectPlantUMLType(content); got != want {
			t.Fatalf("detectPlantUMLType() call #%d = %q, want %q (debe ser el mismo en cada llamada, no solo en la primera)", i, got, want)
		}
	}
}
