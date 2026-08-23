// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"path/filepath"
	"testing"

	"go.ziradocs.com/slidelang/v2/internal/generator/css/themes"
)

// TestCreateTheme_OutputPassesStrictValidation is the §2.1 regression for
// `themes create`: its four bundled templates (theme_templates.go) were
// written with unprefixed selectors (.slide) and variable usages
// (var(--primary-color)) — exactly the broken shape `themes validate
// --strict` was just taught to reject (motor-temas-v2.md §2.1). Without
// namespaceTemplate, this command scaffolded a theme its own validator
// would immediately fail, and its own success message told the user to
// run `themes validate` next.
func TestCreateTheme_OutputPassesStrictValidation(t *testing.T) {
	for _, tmpl := range []string{"business", "academic", "creative", "minimal"} {
		t.Run(tmpl, func(t *testing.T) {
			outputPath := filepath.Join(t.TempDir(), "theme-"+tmpl)

			if err := createTheme(tmpl, tmpl, outputPath, "Test Author", "", false, true, "", "", false); err != nil {
				t.Fatalf("createTheme(%q) failed: %v", tmpl, err)
			}

			theme, err := themes.LoadExternalTheme(filepath.Join(outputPath, "theme.json"))
			if err != nil {
				t.Fatalf("failed to load the just-created theme: %v", err)
			}

			if err := themes.NewStrictThemeValidator().ValidateTheme(theme); err != nil {
				t.Errorf("template %q failed --strict validation: %v", tmpl, err)
			}
		})
	}
}
