// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package themes

import "testing"

// TestResolveTheme_PriorityOrder cubre la prioridad documentada: flag >
// frontmatter > config default > "default" — el mismo orden que
// (g *Generator) resolveTheme aplicaba antes de extraerse a esta función
// (issue #30).
func TestResolveTheme_PriorityOrder(t *testing.T) {
	tests := []struct {
		name             string
		flagTheme        string
		frontmatterTheme string
		configDefault    string
		wantThemeName    string
	}{
		{"flag wins over everything", "dark", "minimal", "minimal", "dark"},
		{"frontmatter wins over config default", "", "minimal", "dark", "minimal"},
		{"config default wins when flag and frontmatter are absent", "", "default", "dark", "dark"},
		{"falls back to \"default\" when nothing else is set", "", "default", "", "default"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			theme, err := ResolveTheme(tt.flagTheme, tt.frontmatterTheme, tt.configDefault)
			if err != nil {
				t.Fatalf("ResolveTheme(%q, %q, %q) unexpected error: %v", tt.flagTheme, tt.frontmatterTheme, tt.configDefault, err)
			}
			if theme.Name != tt.wantThemeName {
				t.Errorf("ResolveTheme(%q, %q, %q) = theme %q, want %q", tt.flagTheme, tt.frontmatterTheme, tt.configDefault, theme.Name, tt.wantThemeName)
			}
		})
	}
}

// TestResolveTheme_ExplicitFrontmatterDefaultTreatedAsAbsent pina el matiz
// heredado de config.ExtractThemeFromFrontmatter: un documento con
// `theme: default` LITERAL en su frontmatter es indistinguible de "sin
// tema declarado" (ambos casos producen el string "default"), así que cae
// al config default en vez de "ganar" como si fuera una elección explícita.
// Es exactamente el tipo de matiz que un refactor "limpia" por accidente —
// este test lo fija.
func TestResolveTheme_ExplicitFrontmatterDefaultTreatedAsAbsent(t *testing.T) {
	theme, err := ResolveTheme("", "default", "dark")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if theme.Name != "dark" {
		t.Errorf("theme = %q, want %q (frontmatter 'default' should be treated as absent, falling through to config default)", theme.Name, "dark")
	}
}

// TestResolveTheme_FrontmatterSourceIsUntrusted confirma la propiedad de
// seguridad ME-2 (docs/SECURITY_AUDIT_2026-07.md): un nombre de tema que
// viene del frontmatter debe pasar trusted=false a LoadTheme. Un nombre con
// traversal ("..") que fuera trusted=true intentaría resolverse como ruta
// externa; con trusted=false, LoadTheme lo rechaza ANTES de tocar el
// filesystem y ResolveTheme cae a "default" con un error no-nil.
func TestResolveTheme_FrontmatterSourceIsUntrusted(t *testing.T) {
	theme, err := ResolveTheme("", "../evil", "")
	if err == nil {
		t.Fatal("expected an error for a path-traversal frontmatter theme name (must be untrusted)")
	}
	if theme == nil || theme.Name != "default" {
		t.Fatalf("expected a non-nil fallback to the 'default' theme, got %+v", theme)
	}
}

// TestResolveTheme_NeverReturnsNilTheme confirma el contrato general: pase
// lo que pase (nombre inválido, tema inexistente), el *Theme devuelto nunca
// es nil — el caller siempre puede leer .Variables sin nil-check.
func TestResolveTheme_NeverReturnsNilTheme(t *testing.T) {
	theme, err := ResolveTheme("this-theme-does-not-exist", "default", "")
	if err == nil {
		t.Fatal("expected an error for a nonexistent theme")
	}
	if theme == nil {
		t.Fatal("theme must never be nil, even on a failed resolution")
	}
}
