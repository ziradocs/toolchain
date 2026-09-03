// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package linter

import (
	"strconv"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

func findDiagnostic(diags []diagnostics.Diagnostic, code string) *diagnostics.Diagnostic {
	for i := range diags {
		if diags[i].Code == code {
			return &diags[i]
		}
	}
	return nil
}

// Los tests llaman a través de SlideLayoutValidationRule.Check() (el
// dispatch real que usa el linter, ver rules.go) en vez de invocar
// validateContentSlideTitle directamente — eso fija la garantía de que
// LAYOUT003 solo corre para slides con BlockType "content" (el schema
// "content" es el único que lo lista en ValidationRules); llamar la
// función a secas no detectaría una futura regresión en ese cableado
// (hallazgo de code-review de PR #152).

// Issue #103 (Causa A): un slide "content" sin título es un diseño
// deliberado (p. ej. una cita de pantalla completa) — LAYOUT003 debe ser
// Warning, nunca bloquear el build. El caso "sin título Y sin elementos"
// ya lo cubre LAYOUT004 (validateContentSlideElements) como Error por
// separado, así que LAYOUT003 no necesita un caso especial para eso.
func TestSlideLayoutValidation_ContentWithoutTitle_LAYOUT003IsWarning(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	block := ast.NewContentBlock(pos, "content")
	block.Elements = append(block.Elements, ast.NewTextElement(pos, "algún contenido"))

	diags := (&SlideLayoutValidationRule{}).Check(block)

	diag := findDiagnostic(diags, "LAYOUT003")
	if diag == nil {
		t.Fatal("se esperaba un diagnóstico LAYOUT003")
	}
	if diag.Severity != diagnostics.Warning {
		t.Errorf("severity = %v, want %v (Warning)", diag.Severity, diagnostics.Warning)
	}
}

// Un slide "content" sin título NI elementos sigue fallando el build —
// pero vía LAYOUT004, no vía LAYOUT003 (que ahora siempre es Warning).
func TestSlideLayoutValidation_ContentEmptySlide_LAYOUT004IsError(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	block := ast.NewContentBlock(pos, "content")

	diags := (&SlideLayoutValidationRule{}).Check(block)

	titleDiag := findDiagnostic(diags, "LAYOUT003")
	if titleDiag == nil {
		t.Fatal("se esperaba un diagnóstico LAYOUT003")
	}
	if titleDiag.Severity != diagnostics.Warning {
		t.Errorf("LAYOUT003 severity = %v, want %v (Warning, ahora siempre)", titleDiag.Severity, diagnostics.Warning)
	}

	elementsDiag := findDiagnostic(diags, "LAYOUT004")
	if elementsDiag == nil {
		t.Fatal("se esperaba un diagnóstico LAYOUT004 (el slide no tiene ni título ni elementos)")
	}
	if elementsDiag.Severity != diagnostics.Error {
		t.Errorf("LAYOUT004 severity = %v, want %v (Error) — un slide vacío debe seguir bloqueando el build", elementsDiag.Severity, diagnostics.Error)
	}
}

// Un slide "content" con título nunca debería producir LAYOUT003, sin
// importar cuántos elementos tenga.
func TestSlideLayoutValidation_ContentWithTitle_NoLAYOUT003(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	block := ast.NewContentBlock(pos, "content")
	block.Title = "Un título"
	block.Elements = append(block.Elements, ast.NewTextElement(pos, "algún contenido"))

	diags := (&SlideLayoutValidationRule{}).Check(block)

	if diag := findDiagnostic(diags, "LAYOUT003"); diag != nil {
		t.Errorf("no se esperaba LAYOUT003 para un slide con título, se obtuvo severity=%v", diag.Severity)
	}
}

// Confirma el scoping por BlockType: un slide "title" (el schema de
// portada) no tiene "content_requires_title" en su ValidationRules, así
// que un Title vacío ahí nunca debería producir LAYOUT003 — esa regla es
// exclusiva del schema "content".
func TestSlideLayoutValidation_TitleBlockType_NeverProducesLAYOUT003(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	block := ast.NewContentBlock(pos, "title")
	block.Heading = "Portada"

	diags := (&SlideLayoutValidationRule{}).Check(block)

	if diag := findDiagnostic(diags, "LAYOUT003"); diag != nil {
		t.Errorf("LAYOUT003 no debería aplicar a slides tipo 'title', se obtuvo severity=%v", diag.Severity)
	}
}

// Issue #200: los mensajes de LAYOUT_MIN_ELEMENTS/LAYOUT_MAX_ELEMENTS
// construían el número con string(rune(schema.MinElements + '0')) —
// aritmética ASCII de un solo dígito. Para n >= 10 eso produce un rune
// basura (n=10 → rune(58) → ':') en vez del texto "10". Ningún schema
// hardcodeado en GetSlideLayoutSchemas() llega hoy a ese umbral (por eso el
// bug estaba latente), así que estos tests construyen un SlideLayoutSchema
// a mano y llaman a validateElementCountLimits directamente — el helper
// standalone extraído de SlideLayoutValidationRule.Check — para ejercer el
// código sin depender del mapa de schemas real.
func TestValidateElementCountLimits_DoubleDigitMin_MessageHasDecimalString(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	schema := SlideLayoutSchema{MinElements: 12}
	slide := ast.NewContentBlock(pos, "content")
	slide.Elements = append(slide.Elements, ast.NewTextElement(pos, "solo un elemento"))

	diags := validateElementCountLimits("content", schema, slide)

	diag := findDiagnostic(diags, "LAYOUT_MIN_ELEMENTS")
	if diag == nil {
		t.Fatalf("se esperaba un diagnóstico LAYOUT_MIN_ELEMENTS, obtenidos: %+v", diags)
	}
	if !strings.Contains(diag.Message, "12") {
		t.Errorf("el mensaje debería contener el texto decimal \"12\", obtenido: %q", diag.Message)
	}
	// Confirma explícitamente que no aparece el rune basura que producía la
	// aritmética ASCII rota (rune(12+'0') == rune(60) == '<').
	if strings.ContainsRune(diag.Message, rune(60)) {
		t.Errorf("el mensaje contiene el caracter de control corrupto '<' (rune 60), obtenido: %q", diag.Message)
	}
}

func TestValidateElementCountLimits_DoubleDigitMax_MessageHasDecimalString(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	schema := SlideLayoutSchema{MaxElements: 15}
	slide := ast.NewContentBlock(pos, "content")
	for i := 0; i < 20; i++ {
		slide.Elements = append(slide.Elements, ast.NewTextElement(pos, "elemento "+strconv.Itoa(i)))
	}

	diags := validateElementCountLimits("content", schema, slide)

	diag := findDiagnostic(diags, "LAYOUT_MAX_ELEMENTS")
	if diag == nil {
		t.Fatalf("se esperaba un diagnóstico LAYOUT_MAX_ELEMENTS, obtenidos: %+v", diags)
	}
	if !strings.Contains(diag.Message, "15") {
		t.Errorf("el mensaje debería contener el texto decimal \"15\", obtenido: %q", diag.Message)
	}
	// rune(15+'0') == rune(63) == '?' — confirma que no se filtra ese
	// caracter de control corrupto en el mensaje.
	if strings.ContainsRune(diag.Message, rune(63)) {
		t.Errorf("el mensaje contiene el caracter de control corrupto '?' (rune 63), obtenido: %q", diag.Message)
	}
}

// Issue #240: un slide de título con SOLO `title:` (sin `heading:`) no puede
// ser un error. La plantilla ya lo renderiza —cae a $slide.Title cuando no
// hay Heading— y el kit de LLMs siempre documentó "heading o title", así que
// LAYOUT001 era la única pieza que lo rechazaba: el resultado era que la
// forma documentada rompía el build, y en el PRIMER slide de cualquier deck.
func TestSlideLayoutValidation_TitleOnly_NoLAYOUT001(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	block := ast.NewContentBlock(pos, "title")
	block.Title = "Only a title, exactly as the reference allows"
	block.Subtitle = "One line"

	diags := (&SlideLayoutValidationRule{}).Check(block)

	if diag := findDiagnostic(diags, "LAYOUT001"); diag != nil {
		t.Fatalf("no se esperaba LAYOUT001 con `title` presente, se obtuvo: %+v", diag)
	}
}

// `heading` sigue siendo la propiedad canónica y sigue bastando por sí sola.
func TestSlideLayoutValidation_HeadingOnly_NoLAYOUT001(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	block := ast.NewContentBlock(pos, "title")
	block.Heading = "ZiraDocs Strict Mode"

	diags := (&SlideLayoutValidationRule{}).Check(block)

	if diag := findDiagnostic(diags, "LAYOUT001"); diag != nil {
		t.Fatalf("no se esperaba LAYOUT001 con `heading` presente, se obtuvo: %+v", diag)
	}
}

// Lo que el cambio NO relaja: un slide de título sin NADA con qué titularse
// sigue siendo Error. Es el caso que STRICT001 cubría y que, tras retirarla
// (era una regla muerta: LAYOUT001 disparaba siempre antes), tiene que
// seguir cubriendo LAYOUT001 por sí sola.
func TestSlideLayoutValidation_TitleBlockWithNeither_LAYOUT001IsError(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	block := ast.NewContentBlock(pos, "title")

	diags := (&SlideLayoutValidationRule{}).Check(block)

	diag := findDiagnostic(diags, "LAYOUT001")
	if diag == nil {
		t.Fatal("se esperaba un diagnóstico LAYOUT001 sin heading ni title")
	}
	if diag.Severity != diagnostics.Error {
		t.Errorf("severity = %v, want %v (Error)", diag.Severity, diagnostics.Error)
	}
	if !strings.Contains(diag.Message, "'title'") {
		t.Errorf("el mensaje debe nombrar ambas propiedades para que sea accionable, se obtuvo: %q", diag.Message)
	}
}

// STRICT001 se retiró en el issue #240 por duplicar a LAYOUT001. Este guard
// evita que vuelva por la puerta de atrás: si alguien la reintroduce, un
// bloque de título vacío en modo strict emitiría DOS errores para el mismo
// defecto.
func TestStrictModeValidation_TitleBlockWithNeither_OnlyLAYOUT001(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	astNode := ast.NewAST(pos)
	astNode.FrontMatter = ast.NewFrontMatterNode(pos)
	astNode.FrontMatter.Mode = "strict"
	astNode.ContentBlocks = append(astNode.ContentBlocks, *ast.NewContentBlock(pos, "title"))

	diags := (&StrictModeValidationRule{}).Check(astNode)

	for i := range diags {
		if diags[i].RuleID == "STRICT001" || diags[i].Code == "STRICT001" {
			t.Fatalf("STRICT001 se retiró (issue #240): duplica LAYOUT001, se obtuvo: %+v", diags[i])
		}
	}
}

// Issue #239: un BlockType sin schema no se podía validar, pero se ignoraba
// en SILENCIO — un typo ("comparision") o un layout inventado se veían igual
// que uno correcto. Con `layout:` en flex esa puerta se abre de par en par,
// así que el nombre desconocido tiene que reportarse.
func TestSlideLayoutValidation_UnknownLayoutWarns(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	block := ast.NewContentBlock(pos, "comparision") // typo de "comparison"
	block.Title = "Con typo"

	diags := (&SlideLayoutValidationRule{}).Check(block)

	diag := findDiagnostic(diags, "LAYOUT_UNKNOWN")
	if diag == nil {
		t.Fatal("se esperaba LAYOUT_UNKNOWN para un layout sin schema")
	}
	if diag.Severity != diagnostics.Warning {
		t.Errorf("severity = %v, want %v — no puede romper el build", diag.Severity, diagnostics.Warning)
	}
	if !strings.Contains(diag.Message, "comparision") {
		t.Errorf("el mensaje debe nombrar el layout ofensor, se obtuvo: %q", diag.Message)
	}
	// La lista de nombres sale de los schemas, no de una constante aparte:
	// agregar un layout no puede dejar este mensaje desactualizado.
	for name := range GetSlideLayoutSchemas() {
		if !strings.Contains(diag.Message, name) {
			t.Errorf("el mensaje omite el layout conocido %q: %q", name, diag.Message)
		}
	}
}

// Un layout CON schema no dispara LAYOUT_UNKNOWN, y un bloque sin tipo
// tampoco: son los dos caminos que ya existían y que este aviso no puede
// invadir.
func TestSlideLayoutValidation_KnownAndUntypedDoNotWarn(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)

	known := ast.NewContentBlock(pos, "stats")
	known.Title = "Métricas"
	known.Elements = append(known.Elements, ast.NewTextElement(pos, "algo"))
	if d := findDiagnostic((&SlideLayoutValidationRule{}).Check(known), "LAYOUT_UNKNOWN"); d != nil {
		t.Errorf("un layout con schema no puede reportar LAYOUT_UNKNOWN: %+v", d)
	}

	untyped := ast.NewContentBlock(pos, "")
	if d := findDiagnostic((&SlideLayoutValidationRule{}).Check(untyped), "LAYOUT_UNKNOWN"); d != nil {
		t.Errorf("un bloque sin tipo no puede reportar LAYOUT_UNKNOWN: %+v", d)
	}
}

// LAYOUT_UNKNOWN no puede acusar a un tipo que los generadores SÍ conocen.
// slidelang mapea "cover"/"intro" al layout de título y "chapter"/
// "with_directive" al de contenido (config.IsSlideTitle/IsSlideContent), así
// que son válidos de punta a punta aunque no tengan schema propio.
func TestSlideLayoutValidation_SchemalessButKnownTypesDoNotWarn(t *testing.T) {
	for _, slideType := range []string{"cover", "intro", "chapter", "with_directive"} {
		t.Run(slideType, func(t *testing.T) {
			pos := diagnostics.NewPosition(1, 1)
			block := ast.NewContentBlock(pos, slideType)
			block.Title = "X"

			diags := (&SlideLayoutValidationRule{}).Check(block)

			if d := findDiagnostic(diags, "LAYOUT_UNKNOWN"); d != nil {
				t.Errorf("%q lo reconoce el generador; no puede reportarse como desconocido: %+v", slideType, d)
			}
		})
	}
}
