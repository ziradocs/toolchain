// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package normalizer

import (
	"reflect"
	"testing"

	"go.ziradocs.com/core/v2/util"
)

// TestGetTransformRules_ReturnsExpectedRuleSet cubre el gap señalado en
// issue #197: ninguna regla individual (cada una probada de forma aislada,
// instanciada directamente, sin pasar por el registry) hubiera detectado
// que AdvancedElementsCleanupRule nunca llegó a registrarse — un comentario
// sin salto de línea se tragó la llamada al constructor en
// registry.go, y ningún test existente ejercitaba el CONTENIDO real que
// GetTransformRules() devuelve. Este test fija ese contenido explícitamente
// por tipo concreto: si una futura edición vuelve a dejar una regla
// implementada-pero-no-registrada (o duplica una, o cambia el orden sin
// querer), falla acá en vez de necesitar otra corrida de corpus empírica
// para descubrirlo.
//
// Es una lista de tipos hardcodeada a propósito (no auto-derivada del
// código fuente vía go/ast) — más simple y suficientemente robusta para
// este propósito: cualquiera que agregue/quite/reordene una regla real en
// registry.go debe actualizar esta lista a mano, lo cual documenta la
// intención en el mismo PR que cambia el registro.
func TestGetTransformRules_ReturnsExpectedRuleSet(t *testing.T) {
	rules := GetTransformRules(util.NewNoop())

	wantTypes := []string{
		"*frontmatter.YamlEscapingRule",
		"*frontmatter.BackticksCleanupRule",
		"*frontmatter.InjectionRule",
		"*enhancement.ElementClosingTagsRule",
		"*structure.SeparatorsRule",
		"*structure.MarkdownSlideStructureRule",
		"*content.TitleSubtitleRule",
		"*content.HeadersRule",
		"*enhancement.CodeGroupFormatterRule",
		"*enhancement.GraphicsRule",
		"*enhancement.MermaidRule",
		"*enhancement.ChartJSONRule",
		"*enhancement.MermaidFormatterRule",
		"*enhancement.MermaidSyntaxFixerRule",
		"*enhancement.ChartFormatterRule",
		"*enhancement.MapFormatterRule",
		"*enhancement.ImagesRule",
		"*enhancement.TablesRule",
	}

	var gotTypes []string
	for _, r := range rules {
		gotTypes = append(gotTypes, reflect.TypeOf(r).String())
	}

	if !reflect.DeepEqual(gotTypes, wantTypes) {
		t.Fatalf("GetTransformRules() returned an unexpected rule set.\ngot  (%d rules): %v\nwant (%d rules): %v",
			len(gotTypes), gotTypes, len(wantTypes), wantTypes)
	}
}
