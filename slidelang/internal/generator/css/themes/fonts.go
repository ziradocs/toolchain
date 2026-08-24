// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package themes

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"go.ziradocs.com/core/v2/util"
)

// fontFormatByExt maps a font file's extension to the CSS format() keyword
// and MIME type used to emit it as a data: URI. Anything outside this
// allowlist is skipped rather than guessed at — an @font-face with a wrong
// format() hint fails to load in some browsers with no visible error.
var fontFormatByExt = map[string]struct{ format, mime string }{
	".woff2": {"woff2", "font/woff2"},
	".woff":  {"woff", "font/woff"},
	".ttf":   {"truetype", "font/ttf"},
	".otf":   {"opentype", "font/otf"},
}

func fontFormatFor(localPath string) (format, mime string, ok bool) {
	f, found := fontFormatByExt[strings.ToLower(filepath.Ext(localPath))]
	if !found {
		return "", "", false
	}
	return f.format, f.mime, true
}

// fontWeightNumberRe matches a single CSS font-weight number, 1-1000 (CSS
// Fonts Level 4 range, not just the traditional 100-900 multiples — some
// variable fonts declare finer-grained weights).
var fontWeightNumberRe = regexp.MustCompile(`^([1-9][0-9]{0,2}|1000)$`)

// validatedFontWeight accepts "normal", "bold", a single 1-1000 number, or a
// "lo hi" range (variable-font src descriptor syntax) with lo <= hi.
// Anything else is dropped — better to omit the descriptor (falls back to
// the CSS default of 400/normal) than to emit a font-weight the browser
// rejects.
func validatedFontWeight(w string) string {
	w = strings.TrimSpace(w)
	if w == "" {
		return ""
	}
	if w == "normal" || w == "bold" {
		return w
	}
	parts := strings.Fields(w)
	switch len(parts) {
	case 1:
		if fontWeightNumberRe.MatchString(parts[0]) {
			return parts[0]
		}
	case 2:
		if fontWeightNumberRe.MatchString(parts[0]) && fontWeightNumberRe.MatchString(parts[1]) {
			lo, _ := strconv.Atoi(parts[0])
			hi, _ := strconv.Atoi(parts[1])
			if lo <= hi {
				return parts[0] + " " + parts[1]
			}
		}
	}
	util.Warn("THEME FONTS: font-weight %q inválido, se omite el descriptor", w)
	return ""
}

// validatedFontStyle accepts only the three CSS font-style keywords.
// "oblique <angle>" (variable-font syntax) is deliberately not supported
// yet — no font in this validation set has needed it.
func validatedFontStyle(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	switch s {
	case "normal", "italic", "oblique":
		return s
	}
	util.Warn("THEME FONTS: font-style %q inválido, se omite el descriptor", s)
	return ""
}

// validatedFontDisplay accepts the five CSS font-display keywords and
// defaults to "swap" — the default this package always emits when a theme
// doesn't specify one, chosen to avoid invisible text while a self-hosted
// font loads.
func validatedFontDisplay(d string) string {
	d = strings.TrimSpace(d)
	if d == "" {
		return "swap"
	}
	switch d {
	case "auto", "block", "swap", "fallback", "optional":
		return d
	}
	util.Warn("THEME FONTS: font-display %q inválido, se usa 'swap' por defecto", d)
	return "swap"
}

// cssEscapeString renders s as a double-quoted CSS string literal, safe to
// interpolate into font-family. Without this, a theme.json font name like
// `x"; } body { display: none } @font-face { font-family: "y` would inject
// arbitrary rules into the bundle — Name is author-controlled JSON that
// reaches raw CSS output for the first time in this file (previously it
// only ever reached a ThemeAsset's Name field, used in validator messages,
// never emitted CSS). Control characters (including raw newlines, which
// terminate an unescaped CSS string) are dropped rather than escaped: a
// font-family name has no legitimate use for them.
func cssEscapeString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '"' || r == '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r < 0x20 || r == 0x7f:
			continue
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// GenerateFontFaceCSS emits one @font-face rule per self-hosted font
// declared in the theme's manifest (Manifest.Assets.Fonts), each embedded
// as a data: URI — see motor-temas-v2.md §2.3: fonts are always
// self-hosted, in browser, offline-* and --embed-assets alike, so a single
// data: URI (rather than a sibling file whose relative base shifts under
// --embed-assets) is the only path that's correct in all four
// render-mode/embed-assets combinations. A font with no matching local
// file, an out-of-theme "local" path, or an unsupported extension is
// skipped with a warning — never guessed at.
func GenerateFontFaceCSS(et *ExternalTheme) string {
	if et == nil || len(et.Manifest.Assets.Fonts) == 0 {
		return ""
	}
	themeDir := filepath.Dir(et.Path)
	var out strings.Builder
	for _, font := range et.Manifest.Assets.Fonts {
		rule, ok := buildFontFaceRule(themeDir, font)
		if !ok {
			continue
		}
		out.WriteString(rule)
	}
	return out.String()
}

func buildFontFaceRule(themeDir string, font ThemeFont) (string, bool) {
	if font.Name == "" {
		util.Warn("THEME FONTS: se omite una entrada de assets.fonts sin 'name'")
		return "", false
	}
	if font.Local == "" {
		// "Auto-hospedar siempre" (motor-temas-v2.md §2.3) is only coherent
		// if the file ships with the theme: a font declared only via "url"
		// would need a build-time download, reintroducing exactly the
		// network dependency this decision removes. themes validate
		// --strict rejects this too (see validator.go); this is the
		// defense that also runs when a caller skips validation.
		util.Warn("THEME FONTS: '%s' no declara 'local' — las fuentes se auto-hospedan siempre, 'url' no se usa para emitir CSS", font.Name)
		return "", false
	}
	resolved, err := util.ResolveConfinedPath(themeDir, font.Local)
	if err != nil {
		// Same confinement helper --format pptx already uses for local
		// image sources (pptx.go). Before this function existed, Local was
		// only ever os.Stat'd for its size — reading and embedding it here
		// turns an out-of-tree "local" (e.g. "../../../etc/passwd") into
		// real exfiltration into the bundle.
		util.Warn("THEME FONTS: '%s' declara una ruta 'local' inválida (%s): %v", font.Name, font.Local, err)
		return "", false
	}
	data, err := os.ReadFile(resolved)
	if err != nil {
		util.Warn("THEME FONTS: no se pudo leer el archivo de '%s' (%s): %v", font.Name, font.Local, err)
		return "", false
	}
	format, mime, ok := fontFormatFor(font.Local)
	if !ok {
		util.Warn("THEME FONTS: '%s' tiene una extensión no soportada (%s) — formatos válidos: woff2, woff, ttf, otf", font.Name, font.Local)
		return "", false
	}

	weight := validatedFontWeight(font.Weight)
	style := validatedFontStyle(font.Style)
	display := validatedFontDisplay(font.Display)

	var b strings.Builder
	b.WriteString("@font-face {\n")
	b.WriteString("  font-family: " + cssEscapeString(font.Name) + ";\n")
	fmt.Fprintf(&b, "  src: url(data:%s;base64,%s) format(%q);\n", mime, base64.StdEncoding.EncodeToString(data), format)
	if weight != "" {
		b.WriteString("  font-weight: " + weight + ";\n")
	}
	if style != "" {
		b.WriteString("  font-style: " + style + ";\n")
	}
	b.WriteString("  font-display: " + display + ";\n")
	b.WriteString("}\n")
	return b.String(), true
}
