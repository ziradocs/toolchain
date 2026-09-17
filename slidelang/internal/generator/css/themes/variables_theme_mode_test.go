// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package themes

import (
	"strings"
	"testing"
)

func TestGenerateThemeCSS_ExplicitThemeModeOverridesSystemPreference(t *testing.T) {
	theme := Theme{
		Variables: ThemeVariables{"--text-color": "#111111"},
		ColorSchemes: map[string]ThemeVariables{
			"dark": {"--text-color": "#eeeeee"},
		},
	}

	css := GenerateThemeCSS(theme)
	for _, want := range []string{
		`@media (prefers-color-scheme: dark)`,
		`:root[data-theme-mode="light"]`,
		`:root[data-theme-mode="dark"]`,
	} {
		if !strings.Contains(css, want) {
			t.Errorf("generated CSS missing %q:\n%s", want, css)
		}
	}
	if strings.LastIndex(css, `:root[data-theme-mode="light"]`) < strings.Index(css, `@media (prefers-color-scheme: dark)`) {
		t.Error("explicit light selector must appear after the system dark media query")
	}
}
