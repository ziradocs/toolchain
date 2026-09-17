// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
)

func TestGenerateDocumentHTML_ThemeMode(t *testing.T) {
	for _, tt := range []struct {
		name string
		mode string
		want string
	}{
		{"dark is explicit", "dark", `<html lang="es" data-theme="dark" data-theme-mode="dark">`},
		{"light is explicit", "light", `<html lang="es" data-theme="light" data-theme-mode="light">`},
		{"absent leaves system choice unset", "", `<html lang="es" data-theme="light">`},
		{"invalid does not reach markup", "sepia", `<html lang="es" data-theme="light">`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			doc := &ast.AST{FrontMatter: &ast.FrontMatterNode{ThemeMode: tt.mode}}
			html := GenerateDocumentHTML(doc, DocumentHTMLOptions{InteractiveViewer: true}, nil)
			if !strings.Contains(html, tt.want) {
				t.Errorf("expected %q in generated HTML, got %.300s", tt.want, html)
			}
			if tt.mode == "" || tt.mode == "sepia" {
				if strings.Contains(html, "data-theme-mode=") {
					t.Errorf("unexpected declared theme mode in generated HTML: %.300s", html)
				}
			}
		})
	}
}

func TestGenerateDocumentHTML_DeclaredThemeModeIgnoresSavedPreference(t *testing.T) {
	doc := &ast.AST{FrontMatter: &ast.FrontMatterNode{ThemeMode: "light"}}
	html := GenerateDocumentHTML(doc, DocumentHTMLOptions{InteractiveViewer: true}, nil)

	if !strings.Contains(html, `if (!declaredTheme && savedTheme === 'dark')`) {
		t.Error("interactive viewer still restores localStorage over an explicit theme_mode")
	}
}
