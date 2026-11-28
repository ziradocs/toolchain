// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package pipeline

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/util"
)

func TestNormalizeTitle(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"plain", "Guía Q3", "Guía Q3"},
		{"trimmed", "  Guía  ", "Guía"},
		{"line breaks become one space", "Uno\nmode: strict\r\ntheme: x", "Uno mode: strict theme: x"},
		{"front matter delimiters stay text", "A\n---\n# B", "A --- # B"},
		{"tabs and C0/C1 controls", "a\tb\x07c\x1bd\u0085e\u009ff", "a b c d e f"},
		{"a run of controls is one space", "a\n\n\t\x00b", "a b"},
		{"unicode line and paragraph separators", "a\u2028b\u2029c", "a b c"},
		{"invalid UTF-8 (a lone surrogate as bytes)", "x\xed\xa0\x80y", "x\uFFFDy"},
		{"bidi embeddings, overrides and isolates are removed", "\u202Eabc\u202C d\u2066e\u2069 \u202A\u202B\u202Df\u2067\u2068", "abc de f"},
		{"other format characters are kept inside a visible title", "a\u200Bb\uFEFFc", "a\u200Bb\uFEFFc"},
		{"markup and braces are kept verbatim", `</title><script>"'{{title}}&`, `</title><script>"'{{title}}&`},
		{"non-ASCII kept", "Ñandú 漢字 🚀", "Ñandú 漢字 🚀"},
		{"exactly the maximum length", strings.Repeat("é", MaxTitleRunes), strings.Repeat("é", MaxTitleRunes)},
		{"the length counts after normalizing", "  " + strings.Repeat("a", MaxTitleRunes) + "\u202E\n ", strings.Repeat("a", MaxTitleRunes)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalizeTitle(tc.in)
			if err != nil {
				t.Fatalf("NormalizeTitle(%q): %v", tc.in, err)
			}
			if got != tc.want {
				t.Errorf("NormalizeTitle(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
	blanks := []string{"", "   ", "\n\t", "\x00\x07", "\uFEFF", "\u200B\u200C\u200D\u2060", " \uFEFF\u200B ", "\u202E\u202C", "\u00A0\u3000\u200E\u200F"}
	for _, blank := range blanks {
		if _, err := NormalizeTitle(blank); !errors.Is(err, errBlankTitle) {
			t.Errorf("NormalizeTitle(%q) error = %v, want errBlankTitle", blank, err)
		}
	}
	for _, long := range []string{strings.Repeat("a", MaxTitleRunes+1), strings.Repeat("漢", MaxTitleRunes+1), strings.Repeat("a", 60000)} {
		if _, err := NormalizeTitle(long); !errors.Is(err, errTitleTooLong) {
			t.Errorf("NormalizeTitle(%d runes) error = %v, want errTitleTooLong", len([]rune(long)), err)
		}
	}
}

func TestValidateTitle(t *testing.T) {
	for _, format := range []string{"html", "pdf", "docx"} {
		if err := ValidateTitle("T", format); err != nil {
			t.Errorf("ValidateTitle(T, %s) = %v", format, err)
		}
	}
	for _, format := range []string{"markdown", "md"} {
		if err := ValidateTitle("T", format); err == nil {
			t.Errorf("ValidateTitle(T, %s) = nil, want an error", format)
		}
		if err := ValidateTitle("", format); err != nil {
			t.Errorf("ValidateTitle(\"\", %s) = %v: without --title markdown is fine", format, err)
		}
	}
}

// prepareTitleFixture parsea src como lo hace build y devuelve el AST
// serializado de Prepare, para compararlo después de Generate.
func prepareTitleFixture(t *testing.T, src string) (*Prepared, string) {
	t.Helper()
	prepared, err := Prepare(util.NewNoop(), []byte(src), PrepareOptions{MaxInputBytes: 1 << 20, IncludeRoot: t.TempDir()})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	data, err := json.Marshal(prepared.Doc)
	if err != nil {
		t.Fatal(err)
	}
	return prepared, string(data)
}

func generateTitleHTML(t *testing.T, prepared *Prepared, title string) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "doc.html")
	if err := Generate(util.NewNoop(), prepared, "doc.doclang", GenerateOptions{Format: "html", OutputFile: out, RenderMode: "browser", DiagramBackend: "chromium", Title: title}); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	page, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	return string(page)
}

var (
	titleTagRe = regexp.MustCompile(`<title>[^<]*</title>`)
	nonceRe    = regexp.MustCompile(`nonce(-|=")[A-Za-z0-9+/=_-]+`)
)

// withoutTitleTag es la página sin su <title> y sin el nonce de CSP, que
// cambia en cada build: lo que tiene que quedar igual con y sin --title.
func withoutTitleTag(page string) string {
	return nonceRe.ReplaceAllString(titleTagRe.ReplaceAllString(page, "<title/>"), "nonce")
}

// La paridad que pide --title: una fuente sin frontmatter con y sin la
// bandera da el mismo AST (el de Prepare, que no la recibe, y que Generate
// no toca), el mismo cuerpo y los mismos diagnósticos. El gantt es el caso
// que separa --title de agregarle un frontmatter a la fuente: sin
// frontmatter el normalizador completa sus fechas (2026-01 → 2026-01-01), y
// un frontmatter en la fuente lo apagaría.
func TestGenerateTitle_NoFrontMatterParity(t *testing.T) {
	src := "# Setup\n\nIntro.\n\n## Install\n\nRun it.\n\n```mermaid\ngantt\n    title Plan\n    Fase 1 :a1, 2026-01, 2026-02\n```\n\n# Usage\n\nUse it.\n"
	prepared, before := prepareTitleFixture(t, src)
	diagsBefore, _ := json.Marshal(prepared.Diagnostics())
	if !strings.Contains(before, "2026-01-01") {
		t.Fatalf("fixture premise: the normalizer should have completed the gantt dates of a file without front matter:\n%s", before)
	}

	plain := generateTitleHTML(t, prepared, "")
	titled := generateTitleHTML(t, prepared, "Guía Q3")

	after, _ := json.Marshal(prepared.Doc)
	if string(after) != before {
		t.Errorf("Generate with a title changed the prepared AST")
	}
	if prepared.Doc.FrontMatter != nil {
		t.Errorf("Generate with a title left a front matter on the prepared AST")
	}
	if diagsAfter, _ := json.Marshal(prepared.Diagnostics()); string(diagsAfter) != string(diagsBefore) {
		t.Errorf("diagnostics changed: %s\nwant %s", diagsAfter, diagsBefore)
	}
	if !strings.Contains(plain, "<title>Setup</title>") {
		t.Errorf("without --title the title is the first #: %s", titleTagRe.FindString(plain))
	}
	if !strings.Contains(titled, "<title>Guía Q3</title>") {
		t.Errorf("with --title: %s", titleTagRe.FindString(titled))
	}
	if withoutTitleTag(plain) != withoutTitleTag(titled) {
		t.Errorf("the page changed beyond its <title>")
	}
	for _, turnedOn := range []string{"doclang-viewer", "sidebar-toc", ">1. "} {
		if strings.Contains(titled, turnedOn) {
			t.Errorf("--title on a file without front matter turned on %q (TOC, viewer or numbering)", turnedOn)
		}
	}
}

// Paridad con una fuente sin frontmatter que usa {{title}}: sin la bandera
// queda literal (no hay title: que lo resuelva) y con ella toma el título,
// leído como el de un title: (Markdown en el cuerpo). Fuera de eso, y del
// <title>, la página es la misma.
func TestGenerateTitle_NoFrontMatterParityWithTitleVariable(t *testing.T) {
	src := "# Setup\n\nEste documento se llama {{title}}.\n\n## Detalle\n\nOtra vez {{title}}.\n"
	prepared, before := prepareTitleFixture(t, src)

	plain := generateTitleHTML(t, prepared, "")
	titled := generateTitleHTML(t, prepared, "Guía **Q3**")

	if strings.Count(plain, "{{title}}") != 2 {
		t.Fatalf("fixture premise: without front matter {{title}} stays literal")
	}
	want := strings.ReplaceAll(withoutTitleTag(plain), "{{title}}", "Guía <strong>Q3</strong>")
	if got := withoutTitleTag(titled); got != want {
		t.Errorf("the page changed beyond its <title> and {{title}}")
	}
	if after, _ := json.Marshal(prepared.Doc); string(after) != before {
		t.Errorf("Generate with a title changed the prepared AST")
	}
}

func TestGenerateTitle_WinsOverFrontMatterTitle(t *testing.T) {
	src := "---\ntitle: \"Fuente\"\nauthor: Equipo\n---\n\n# Setup\n\nEste documento se llama {{title}} y es de {{author}}.\n"
	prepared, before := prepareTitleFixture(t, src)

	plain := generateTitleHTML(t, prepared, "")
	titled := generateTitleHTML(t, prepared, "Guía Q3")

	if !strings.Contains(plain, "<title>Fuente</title>") || !strings.Contains(plain, "se llama Fuente y es de Equipo") {
		t.Fatalf("fixture premise: the front matter title is the title and {{title}}")
	}
	if !strings.Contains(titled, "<title>Guía Q3</title>") {
		t.Errorf("--title does not win over title: in front matter: %s", titleTagRe.FindString(titled))
	}
	if !strings.Contains(titled, "se llama Guía Q3 y es de Equipo") {
		t.Errorf("{{title}} does not take --title (the other front matter keys stay)")
	}
	if prepared.Doc.FrontMatter.Title != "Fuente" {
		t.Errorf("Generate changed the prepared front matter title to %q", prepared.Doc.FrontMatter.Title)
	}
	if after, _ := json.Marshal(prepared.Doc); string(after) != before {
		t.Errorf("Generate with a title changed the prepared AST")
	}
}

func TestGenerateTitle_EscapedInTitleTag(t *testing.T) {
	prepared, _ := prepareTitleFixture(t, "# Setup\n\nTexto.\n")
	page := generateTitleHTML(t, prepared, `</title><script>alert(1)</script> "q" & 'a'`)
	if strings.Contains(page, "<script>alert(1)</script>") {
		t.Fatalf("--title reached the page unescaped")
	}
	if !strings.Contains(page, "<title>&lt;/title&gt;&lt;script&gt;alert(1)&lt;/script&gt; &quot;q&quot; &amp; &#39;a&#39;</title>") {
		t.Errorf("unexpected <title>: %s", titleTagRe.FindString(page))
	}
}

func TestGenerateTitle_RefusedForMarkdownAndBlank(t *testing.T) {
	prepared, _ := prepareTitleFixture(t, "# Setup\n\nTexto.\n")
	out := filepath.Join(t.TempDir(), "doc.md")
	if err := Generate(util.NewNoop(), prepared, "doc.doclang", GenerateOptions{Format: "markdown", OutputFile: out, RenderMode: "browser", DiagramBackend: "chromium", Title: "T"}); err == nil {
		t.Errorf("Generate(markdown, Title) = nil, want an error")
	}
	if _, err := os.Stat(out); err == nil {
		t.Errorf("a refused markdown build wrote its output")
	}
	if err := Generate(util.NewNoop(), prepared, "doc.doclang", GenerateOptions{Format: "html", OutputFile: filepath.Join(t.TempDir(), "doc.html"), RenderMode: "browser", DiagramBackend: "chromium", Title: " \n "}); !errors.Is(err, errBlankTitle) {
		t.Errorf("Generate with a blank Title = %v, want errBlankTitle", err)
	}
}
