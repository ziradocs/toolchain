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

// El cuerpo de un PlantUML conserva la sangría relativa (quita solo la que
// comparten todas sus líneas), en los tres formatos y con espacios o tabs;
// @startuml y @enduml van sin sangría. Antes cada línea se recortaba.
func TestPlantUMLParser_PreservesRelativeIndentation(t *testing.T) {
	want := "@startuml\nclass A {\n  +x\n}\n@enduml"
	for _, tt := range []struct {
		name, mode string
		lines      []string
		want       string
	}{
		{"fence", "flex", []string{"```plantuml", "@startuml", "class A {", "  +x", "}", "@enduml", "```"}, want},
		{"native indented body", "flex", []string{"@startuml", "    class A {", "      +x", "    }", "@enduml"}, want},
		{"tag", "strict", []string{"<<plantuml>>", "    @startuml", "    class A {", "      +x", "    }", "    @enduml", "<<end>>"}, want},
		{"tag with tabs", "flex", []string{"<<plantuml>>", "\t@startuml", "\tclass A {", "\t\t+x", "\t}", "\t@enduml"}, "@startuml\nclass A {\n\t+x\n}\n@enduml"},
		{"fence without markers", "flex", []string{"```plantuml", "  class A {", "    +x", "  }", "```"}, want},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := (&PlantUMLParser{}).Parse(&ParseContext{Mode: tt.mode, Lines: tt.lines}, 0)
			elem, ok := got.Element.(*ast.PlantUMLElement)
			if !ok {
				t.Fatalf("element = %T, want *ast.PlantUMLElement", got.Element)
			}
			if elem.Content != tt.want {
				t.Errorf("content = %q, want %q", elem.Content, tt.want)
			}
			if got.ConsumedLines != len(tt.lines) {
				t.Errorf("consumed %d lines, want %d", got.ConsumedLines, len(tt.lines))
			}
			if elem.DiagramType != "class" {
				t.Errorf("diagramType = %q, want class", elem.DiagramType)
			}
		})
	}
}

// Conservar la sangría no cambia el tipo detectado: los patrones se corren
// sobre las líneas recortadas, como antes.
func TestPlantUMLParser_IndentedBodyKeepsDetectedType(t *testing.T) {
	got := (&PlantUMLParser{}).Parse(&ParseContext{Mode: "flex", Lines: []string{"@startuml", "  participant A", "    A -> B: hola", "@enduml"}}, 0)
	if elem := got.Element.(*ast.PlantUMLElement); elem.DiagramType != "sequence" {
		t.Errorf("diagramType = %q, want sequence", elem.DiagramType)
	}
}

// Lo que la sangría no toca sigue como antes: las líneas en blanco de adentro,
// también junto a los marcadores, se conservan; los espacios y el "\r" del
// final de cada línea se quitan; un cuerpo vacío sin marcadores es una línea
// vacía. Y el formato nativo ya no pega el @enduml a la última línea.
func TestPlantUMLParser_BodyEdges(t *testing.T) {
	for _, tt := range []struct {
		name  string
		lines []string
		want  string
	}{
		{"blank lines next to the markers", []string{"```plantuml", "@startuml", "", "    class A", "", "@enduml", "```"}, "@startuml\n\nclass A\n\n@enduml"},
		{"crlf and trailing spaces", []string{"```plantuml", "@startuml\r", "  class A {  \r", "    +x\r", "  }\r", "@enduml\r", "```"}, "@startuml\nclass A {\n  +x\n}\n@enduml"},
		{"empty fence", []string{"```plantuml", "```"}, "@startuml\n\n@enduml"},
		{"only markers", []string{"```plantuml", "@startuml", "@enduml", "```"}, "@startuml\n@enduml"},
		{"no markers keeps trailing blank", []string{"```plantuml", "", "class A", "", "```"}, "@startuml\nclass A\n\n@enduml"},
		{"native", []string{"@startuml", "class A", "@enduml"}, "@startuml\nclass A\n@enduml"},
		{"blank after enduml", []string{"```plantuml", "@startuml", "class A", "@enduml", "", "```"}, "@startuml\nclass A\n@enduml"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := (&PlantUMLParser{}).Parse(&ParseContext{Mode: "flex", Lines: tt.lines}, 0)
			if c := got.Element.(*ast.PlantUMLElement).Content; c != tt.want {
				t.Errorf("content = %q, want %q", c, tt.want)
			}
		})
	}
}

// El tipo se detecta sobre las líneas recortadas: con una línea en la
// columna 0 (skinparam) el cuerpo no tiene sangría común y "  class A {"
// conserva la suya, pero sigue siendo un diagrama de clases.
func TestPlantUMLParser_TypeIgnoresKeptIndentation(t *testing.T) {
	got := (&PlantUMLParser{}).Parse(&ParseContext{Mode: "flex", Lines: []string{"@startuml", "skinparam monochrome true", "  class A {", "    +x", "  }", "@enduml"}}, 0)
	elem := got.Element.(*ast.PlantUMLElement)
	if elem.Content != "@startuml\nskinparam monochrome true\n  class A {\n    +x\n  }\n@enduml" {
		t.Errorf("content = %q", elem.Content)
	}
	if elem.DiagramType != "class" {
		t.Errorf("diagramType = %q, want class", elem.DiagramType)
	}
}

// Los espacios Unicode (U+00A0, U+3000) se quitan del final de una línea y de
// su sangría, como cuando cada línea se recortaba entera; los espacios y tabs
// de la sangría se conservan.
func TestPlantUMLParser_UnicodeSpaces(t *testing.T) {
	got := (&PlantUMLParser{}).Parse(&ParseContext{Mode: "flex", Lines: []string{"<<plantuml>>", "  @startuml", "  A -> B   ", "     B -> C　", "  @enduml"}}, 0)
	if c := got.Element.(*ast.PlantUMLElement).Content; c != "@startuml\nA -> B\n  B -> C\n@enduml" {
		t.Errorf("content = %q", c)
	}
}
