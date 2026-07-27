// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package template

import (
	htmltemplate "html/template"
	"strings"
	"testing"

	"go.ziradocs.com/slidelang/v2/internal/generator/config"
	"go.ziradocs.com/slidelang/v2/internal/generator/data"
)

// TestElementTemplate_KnownTypesRenderNonEmpty renderiza de verdad el
// template "element" (GetElementTemplate) para cada .Type que un branch
// `{{else if eq .Type "..."}}` declara soportar, y exige salida no vacía.
//
// Esto es deliberadamente más fuerte que un `strings.Contains(src, "eq .Type
// \"media\"")` sobre el string del template: ese check pasaría igual si el
// branch existe pero no produce nada, o si hay un typo en el discriminador
// (`"medial"` sigue conteniendo el substring buscado). Ejecutar el template
// con datos reales es la única forma de probar que el branch efectivamente
// renderiza.
func TestElementTemplate_KnownTypesRenderNonEmpty(t *testing.T) {
	types := []string{
		"text", "points", "code", "image", "table", "code_group",
		"special_block", "mermaid", "chart", "map", "quote", "checklist",
		"directive", "grid", "column", "media", "plantuml", "math",
	}

	tmpl := mustParseElementTemplate(t)

	for _, typ := range types {
		t.Run(typ, func(t *testing.T) {
			got := executeElement(t, tmpl, data.ElementData{Type: typ})
			if strings.TrimSpace(got) == "" {
				t.Errorf("el branch %q del template \"element\" no produjo salida", typ)
			}
			if strings.Contains(got, "Unsupported element type") {
				t.Errorf("el tipo conocido %q cayó en el fallback {{else}} — el branch no matcheó", typ)
			}
		})
	}
}

// TestElementTemplate_UnknownTypeFallsBackVisibly cubre issue #35: un .Type
// que ningún branch reconoce no debe desaparecer en silencio — debe quedar
// un rastro visible en el HTML generado, ya que un template no puede
// loggear como sí puede converter.go. "plantuml" era el canario histórico
// acá (PlantUMLElement/MathElement no tenían branch — issue #38); ahora que
// sí lo tienen, hay que usar un .Type genuinamente inventado en su lugar,
// que ningún branch real vaya a reclamar nunca.
func TestElementTemplate_UnknownTypeFallsBackVisibly(t *testing.T) {
	tmpl := mustParseElementTemplate(t)

	got := executeElement(t, tmpl, data.ElementData{Type: "totally-unknown-element-type"})
	if !strings.Contains(got, "Unsupported element type: totally-unknown-element-type") {
		t.Errorf("esperaba el fallback visible para un tipo no reconocido, got: %q", got)
	}
}

func mustParseElementTemplate(t *testing.T) *htmltemplate.Template {
	t.Helper()
	tb := NewTemplateBuilder()
	tmpl, err := htmltemplate.New("element").Funcs(config.HTMLTemplateFuncs()).Parse(tb.GetElementTemplate())
	if err != nil {
		t.Fatalf("parseando GetElementTemplate(): %v", err)
	}
	return tmpl
}

func executeElement(t *testing.T, tmpl *htmltemplate.Template, ed data.ElementData) string {
	t.Helper()
	var buf strings.Builder
	if err := tmpl.ExecuteTemplate(&buf, "element", ed); err != nil {
		t.Fatalf("ExecuteTemplate(%q): %v", ed.Type, err)
	}
	return buf.String()
}
