// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
)

// headingAt devuelve el elemento i del bloque b como heading de subsección
// (nivel + texto interno), o falla si no lo es.
func headingAt(t *testing.T, block *ast.ContentBlock, i int) (int, string) {
	t.Helper()
	if i >= len(block.Elements) {
		t.Fatalf("el bloque tiene %d elementos, se pidió el %d", len(block.Elements), i)
	}
	el, ok := block.Elements[i].(*ast.TextElement)
	if !ok {
		t.Fatalf("elemento %d: se esperaba *ast.TextElement, se obtuvo %T", i, block.Elements[i])
	}
	if !el.IsRawHTML {
		t.Fatalf("elemento %d: se esperaba un heading (IsRawHTML), Content=%q", i, el.Content)
	}
	return el.Level, el.Content
}

// Issue #194: en flex, `###`..`######` dentro de un slide era texto plano y
// salía literal ("<p>### Foo</p>"). Ahora es un encabezado de subsección de
// verdad, el mismo <hN id> que ya produce el dialecto flex de DocLang.
func TestFlexParser_SubsectionHeadingsBecomeHeadingElements(t *testing.T) {
	astNode, _ := parseFlexBody(t,
		"## Slide", "",
		"### Nivel tres", "",
		"Texto.", "",
		"#### Nivel cuatro", "",
		"##### Nivel cinco", "",
		"###### Nivel seis",
	)

	if len(astNode.ContentBlocks) != 1 {
		t.Fatalf("se esperaba 1 bloque, hay %d", len(astNode.ContentBlocks))
	}
	block := &astNode.ContentBlocks[0]

	for _, tc := range []struct {
		index int
		level int
		text  string
	}{
		{0, 3, "Nivel tres"},
		{2, 4, "Nivel cuatro"},
		{3, 5, "Nivel cinco"},
		{4, 6, "Nivel seis"},
	} {
		level, content := headingAt(t, block, tc.index)
		if level != tc.level {
			t.Errorf("elemento %d: nivel %d, se esperaba %d", tc.index, level, tc.level)
		}
		if !strings.Contains(content, ">"+tc.text+"<") {
			t.Errorf("elemento %d: Content %q no contiene el texto %q", tc.index, content, tc.text)
		}
	}

	// El párrafo intercalado sigue siendo texto normal, no un heading.
	if el, ok := block.Elements[1].(*ast.TextElement); !ok || el.IsRawHTML {
		t.Errorf("elemento 1: se esperaba texto plano, se obtuvo %#v", block.Elements[1])
	}
}

// El anchor sale del texto, igual que en DocLang: es lo que permite
// enlazar una subsección desde fuera del slide.
func TestFlexParser_SubsectionHeadingDerivesAnchor(t *testing.T) {
	astNode, _ := parseFlexBody(t, "## Slide", "", "### Resultados del Trimestre")

	_, content := headingAt(t, &astNode.ContentBlocks[0], 0)
	if !strings.Contains(content, `id="resultados-del-trimestre"`) {
		t.Errorf("Content %q no trae el anchor derivado del texto", content)
	}
}

// La razón por la que el predicado empieza en 3 y no copia el
// isSubsectionHeader de DocLang: en un slide, `# ` y `## ` son estructura.
// Si el heading se los tragara, el deck entero colapsaría a un solo slide.
func TestFlexParser_SubsectionHeadingDoesNotStealSlideBoundaries(t *testing.T) {
	astNode, _ := parseFlexBody(t,
		"# Deck", "## Subtítulo del deck", "",
		"## Slide dos", "", "### Sub A", "", "Texto A.", "",
		"## Slide tres", "", "Texto B.",
	)

	if len(astNode.ContentBlocks) != 3 {
		t.Fatalf("se esperaban 3 bloques, hay %d", len(astNode.ContentBlocks))
	}
	if got := astNode.ContentBlocks[0].Subtitle; got != "Subtítulo del deck" {
		t.Errorf("el `## ` adyacente al `# ` dejó de ser subtítulo: %q", got)
	}
	if got := astNode.ContentBlocks[1].Title; got != "Slide dos" {
		t.Errorf("bloque 1: título %q", got)
	}
	if got := astNode.ContentBlocks[2].Title; got != "Slide tres" {
		t.Errorf("bloque 2: título %q", got)
	}
	if level, _ := headingAt(t, &astNode.ContentBlocks[1], 0); level != 3 {
		t.Errorf("el `### ` del slide dos no quedó como heading nivel 3")
	}
}

// El rescate de subtítulo de parseContentBlock mira len(block.Elements)==0.
// Un heading ES un elemento, así que después de uno el `## ` vuelve a abrir
// slide — que es justo lo que debe pasar: el bloque ya tiene contenido.
func TestFlexParser_SubsectionHeadingCountsAsElementForSubtitleRescue(t *testing.T) {
	astNode, _ := parseFlexBody(t, "## Slide", "", "### Sub", "", "## Otro slide")

	if len(astNode.ContentBlocks) != 2 {
		t.Fatalf("se esperaban 2 bloques, hay %d", len(astNode.ContentBlocks))
	}
	if astNode.ContentBlocks[0].Subtitle != "" {
		t.Errorf("el `## ` se absorbió como subtítulo aunque el bloque ya tenía un heading: %q",
			astNode.ContentBlocks[0].Subtitle)
	}
	if got := astNode.ContentBlocks[1].Title; got != "Otro slide" {
		t.Errorf("bloque 1: título %q", got)
	}
}

// Un `###` como PRIMERA línea del bloque: el bloque no tiene título (no hay
// `# `/`## ` que lo dé) y el heading es su primer elemento. Importa porque
// el gate de emisión de bloques (issue #239) exige elementos, título o
// heading para no tirar el bloque.
func TestFlexParser_SubsectionHeadingAsFirstLineOfBlock(t *testing.T) {
	astNode, _ := parseFlexBody(t, "### Arranca con subsección", "", "Texto.")

	if len(astNode.ContentBlocks) != 1 {
		t.Fatalf("se esperaba 1 bloque, hay %d", len(astNode.ContentBlocks))
	}
	block := &astNode.ContentBlocks[0]
	if block.Title != "" {
		t.Errorf("el heading no debe llenar el título del slide: %q", block.Title)
	}
	if level, _ := headingAt(t, block, 0); level != 3 {
		t.Errorf("el primer elemento no es un heading nivel 3")
	}
}

// Formas que NO son encabezado y deben seguir su camino al registry.
func TestFlexParser_NotSubsectionHeadings(t *testing.T) {
	for _, tc := range []struct {
		name string
		line string
	}{
		{"sin espacio", "###foo"},
		{"solo almohadillas", "###"},
		{"espacio pero sin texto", "###   "},
		{"siete niveles", "####### Demasiados"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			astNode, _ := parseFlexBody(t, "## Slide", "", tc.line)
			if len(astNode.ContentBlocks) == 0 {
				return // la línea no produjo nada; tampoco es un heading
			}
			for i, el := range astNode.ContentBlocks[0].Elements {
				if te, ok := el.(*ast.TextElement); ok && te.IsRawHTML {
					t.Errorf("elemento %d: %q se convirtió en heading (%q)", i, tc.line, te.Content)
				}
			}
		})
	}
}

// flexSubsectionLevel es el predicado; se prueba directo para fijar los
// límites sin depender del resto del loop.
func TestFlexSubsectionLevel(t *testing.T) {
	for _, tc := range []struct {
		line string
		want int
	}{
		{"### Foo", 3},
		{"#### Foo", 4},
		{"##### Foo", 5},
		{"###### Foo", 6},
		{"###\tFoo", 3},
		{"# Foo", 0},
		{"## Foo", 0},
		{"####### Foo", 0},
		{"###Foo", 0},
		{"###", 0},
		{"###  ", 0},
		{"", 0},
		{"texto normal", 0},
	} {
		if got := flexSubsectionLevel(tc.line); got != tc.want {
			t.Errorf("flexSubsectionLevel(%q) = %d, se esperaba %d", tc.line, got, tc.want)
		}
	}
}

// Un `###` dentro de un fence o de un bloque `:::` no llega al loop:
// CodeParser y SpecialBlockParser consumen el bloque entero de una vez. Es
// una propiedad de la que depende el diseño, así que se fija con un test.
func TestFlexParser_SubsectionHeadingNotStolenFromCodeOrSpecialBlock(t *testing.T) {
	astNode, _ := parseFlexBody(t,
		"## Slide", "",
		"```bash", "### no es un heading", "echo hola", "```", "",
		":::note", "### tampoco acá", ":::",
	)

	for i, el := range astNode.ContentBlocks[0].Elements {
		if te, ok := el.(*ast.TextElement); ok && te.IsRawHTML {
			t.Errorf("elemento %d: se creó un heading desde dentro de un bloque (%q)", i, te.Content)
		}
	}
}

// Dos encabezados con el mismo texto derivaban el mismo `id`. Dos ids
// iguales en una página son un error de html-validate (`no-dup-id`) y, peor,
// rompen la navegación: `#details` resuelve siempre al primero.
func TestFlexParser_SubsectionHeadingAnchorsAreUnique(t *testing.T) {
	astNode, _ := parseFlexBody(t,
		"## Slide uno", "", "### Details", "",
		"## Slide dos", "", "### Details", "",
		"## Slide tres", "", "### Details",
	)

	var ids []string
	for i := range astNode.ContentBlocks {
		for _, el := range astNode.ContentBlocks[i].Elements {
			if te, ok := el.(*ast.TextElement); ok && te.IsRawHTML {
				ids = append(ids, te.Content)
			}
		}
	}
	if len(ids) != 3 {
		t.Fatalf("se esperaban 3 encabezados, hay %d", len(ids))
	}
	for i, want := range []string{`id="details"`, `id="details-2"`, `id="details-3"`} {
		if !strings.Contains(ids[i], want) {
			t.Errorf("encabezado %d: %q no contiene %q", i, ids[i], want)
		}
	}
}

// El sufijo del deduplicador no puede pisar un anchor real: un deck con
// "Details", "Details 2" y otro "Details" no debe producir dos `details-2`.
func TestFlexParser_SubsectionHeadingDedupDoesNotCollideWithRealAnchor(t *testing.T) {
	astNode, _ := parseFlexBody(t,
		"## Slide", "", "### Details", "", "### Details 2", "", "### Details",
	)

	seen := map[string]bool{}
	for _, el := range astNode.ContentBlocks[0].Elements {
		te, ok := el.(*ast.TextElement)
		if !ok || !te.IsRawHTML {
			continue
		}
		start := strings.Index(te.Content, `id="`) + 4
		id := te.Content[start : start+strings.Index(te.Content[start:], `"`)]
		if seen[id] {
			t.Errorf("id duplicado %q en %q", id, te.Content)
		}
		seen[id] = true
	}
	if len(seen) != 3 {
		t.Errorf("se esperaban 3 ids distintos, hay %d: %v", len(seen), seen)
	}
}

// Un encabezado que no deja un solo carácter utilizable no puede quedar con
// `id=""`, que es igual de inválido que uno que empieza por dígito.
func TestFlexParser_SubsectionHeadingWithNoUsableCharactersFallsBack(t *testing.T) {
	astNode, _ := parseFlexBody(t, "## Slide", "", "### 🚀")

	_, content := headingAt(t, &astNode.ContentBlocks[0], 0)
	if strings.Contains(content, `id=""`) {
		t.Errorf("el encabezado quedó con id vacío: %q", content)
	}
	if !strings.Contains(content, `id="h"`) {
		t.Errorf("se esperaba el anchor de fallback en %q", content)
	}
}

// Un encabezado con dígito inicial produce un anchor direccionable.
func TestFlexParser_SubsectionHeadingAnchorStartsWithLetter(t *testing.T) {
	astNode, _ := parseFlexBody(t, "## Slide", "", "### 1. Primer paso")

	_, content := headingAt(t, &astNode.ContentBlocks[0], 0)
	if !strings.Contains(content, `id="h-1-primer-paso"`) {
		t.Errorf("anchor inesperado en %q", content)
	}
}

// El loop solo mira la línea donde arranca cada iteración, así que un
// encabezado pegado al párrafo anterior lo tenía que soltar TextParser.
func TestFlexParser_SubsectionHeadingGluedToPreviousParagraph(t *testing.T) {
	astNode, _ := parseFlexBody(t,
		"## Slide", "",
		"Paragraph before.",
		"### Nested heading",
		"Paragraph after.",
	)

	block := &astNode.ContentBlocks[0]
	if len(block.Elements) != 3 {
		t.Fatalf("se esperaban 3 elementos (texto, encabezado, texto), hay %d", len(block.Elements))
	}
	if level, content := headingAt(t, block, 1); level != 3 || !strings.Contains(content, "Nested heading") {
		t.Errorf("elemento 1: nivel %d, content %q", level, content)
	}
	for _, i := range []int{0, 2} {
		te, ok := block.Elements[i].(*ast.TextElement)
		if !ok || te.IsRawHTML {
			t.Fatalf("elemento %d: se esperaba texto plano", i)
		}
		if strings.Contains(te.Content, "###") {
			t.Errorf("elemento %d se tragó el encabezado: %q", i, te.Content)
		}
	}
}
