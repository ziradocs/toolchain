// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build js

package main

import (
	"strings"
	"syscall/js"

	cssthemes "go.ziradocs.com/slidelang/internal/generator/css/themes"
	wasmthemes "go.ziradocs.com/slidelang/themes"
)

// namedThemeOverrideCSS returns a <style> block with a marquee named theme's
// (modern-blue, cyberpunk-neon, ...) variables and custom CSS, or ("", false)
// if name isn't one of them. These themes normally live on disk
// (slidelang/themes/<name>/{theme.json,styles.css}), loaded via
// os.ReadFile — there's no disk to read from in a browser, so
// ThemeLoader.LoadTheme silently falls back to "default" there. Rather than
// wiring pre-registered bytes through the shared, CLI-wide theme resolution
// path (resolveTheme in html_modular.go and CSSBuilder.Build in
// builder.go each construct their own themes.NewThemeLoader() independently
// — correctly threading one caller-supplied loader through both without
// touching the CLI's disk-based theme system deserves its own change, not a
// side effect of the wasm entry point), this renders with the normal
// "default" theme and then layers this override CSS on top via the cascade:
// a :root block declared later in the same document wins over an earlier
// one for the same custom properties.
func namedThemeOverrideCSS(name string) (string, bool) {
	found := false
	for _, n := range wasmthemes.Names {
		if n == name {
			found = true
			break
		}
	}
	if !found {
		return "", false
	}

	manifestJSON, err := wasmthemes.FS.ReadFile(name + "/theme.json")
	if err != nil {
		return "", false
	}
	stylesCSS, err := wasmthemes.FS.ReadFile(name + "/styles.css")
	if err != nil {
		return "", false
	}

	et, err := cssthemes.LoadExternalThemeFromBytes(manifestJSON, stylesCSS)
	if err != nil {
		return "", false
	}

	var css strings.Builder
	css.WriteString("<style>\n")
	css.WriteString(cssthemes.GenerateThemeCSS(et.ToTheme()))
	if mainCSS, ok := et.Styles["main"]; ok {
		css.WriteString(mainCSS)
	}
	css.WriteString("\n</style>\n")
	return css.String(), true
}

// applyNamedThemeOverride injects namedThemeOverrideCSS right before </head>
// when theme names one of the marquee themes; html is returned unchanged
// otherwise (including for "default"/"dark"/"minimal", already handled by
// the normal embedded-theme path).
func applyNamedThemeOverride(html, theme string) string {
	overrideCSS, ok := namedThemeOverrideCSS(theme)
	if !ok {
		return html
	}
	return strings.Replace(html, "</head>", overrideCSS+"</head>", 1)
}

type themeListOutput struct {
	Themes []string `json:"themes"`
}

// slidelangListThemes() -> JSON string. Lists the 3 built-in embedded themes
// plus the marquee named themes made available via namedThemeOverrideCSS —
// NOT a live disk scan (there's no "./themes"/"~/.slidelang/themes" to scan
// in a browser), so this is a fixed list rather than mirroring
// list_themes/`slidelang themes list` exactly.
func slidelangListThemes(_ js.Value, _ []js.Value) any {
	names := []string{"default", "dark", "minimal"}
	names = append(names, wasmthemes.Names...)
	return mustJSON(themeListOutput{Themes: names})
}
