// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"bytes"
	htmltemplate "html/template"
	"strings"
	"testing"

	"go.ziradocs.com/slidelang/v2/internal/generator/data"
)

// TestBuildHTMLHead_Lang cubre el prerrequisito de los issues #62/#63:
// buildHTMLHead emitía `<html lang="es">` hardcodeado sin importar lo que el
// frontmatter declarara. Ahora emite `{{.Lang}}` con fallback a "es", igual
// que core/renderer/document_html.go. Este test ejecuta el fragmento de
// template REAL (el mismo que produce Build()) con html/template, tal como
// lo hace la pipeline de producción, para confirmar que .Lang se resuelve y
// se escapa correctamente — no solo que el string fuente lo menciona.
func TestBuildHTMLHead_Lang(t *testing.T) {
	tests := []struct {
		name     string
		lang     string
		wantLang string
	}{
		{"declared lang is honored", "fr", "fr"},
		{"declared region variant is honored", "pt-BR", "pt-BR"},
		{"no lang falls back to es", "", "es"},
	}

	tb := NewTemplateBuilder()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			head := tb.buildHTMLHead("")
			// buildHTMLHead solo emite <head>, sin cerrar el documento —
			// se completa lo mínimo para que sea un template ejecutable.
			source := head + "</head><body></body></html>"

			tmpl, err := htmltemplate.New("head").Parse(source)
			if err != nil {
				t.Fatalf("failed to parse head template: %v", err)
			}

			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, data.PresentationData{Lang: tt.lang}); err != nil {
				t.Fatalf("failed to execute head template: %v", err)
			}

			want := `<html lang="` + tt.wantLang + `"`
			if !strings.Contains(buf.String(), want) {
				t.Errorf("expected output to contain %q, got: %.200s", want, buf.String())
			}
		})
	}
}

// TestBuildHTMLHead_Lang_EscapesAttribute confirma que html/template escapa
// automáticamente un valor de lang hostil interpolado en el atributo.
func TestBuildHTMLHead_Lang_EscapesAttribute(t *testing.T) {
	tb := NewTemplateBuilder()
	source := tb.buildHTMLHead("") + "</head><body></body></html>"

	tmpl, err := htmltemplate.New("head").Parse(source)
	if err != nil {
		t.Fatalf("failed to parse head template: %v", err)
	}

	var buf bytes.Buffer
	hostile := `"><script>alert(1)</script>`
	if err := tmpl.Execute(&buf, data.PresentationData{Lang: hostile}); err != nil {
		t.Fatalf("failed to execute head template: %v", err)
	}

	if strings.Contains(buf.String(), "<script>alert(1)</script>") {
		t.Errorf("lang attribute was not escaped, output contains raw <script>: %.300s", buf.String())
	}
}
