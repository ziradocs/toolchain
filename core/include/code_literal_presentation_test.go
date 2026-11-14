// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package include

import (
	"strings"
	"testing"
)

// expandPresentation expande con KeepIncludesInCode y Presentation, la
// combinación para una fuente .slidelang.
func expandPresentation(content, baseDir string, read ReadFunc) (Expansion, error) {
	return ExpandWithOptions(content, baseDir, read, ExpandOptions{KeepIncludesInCode: true, Presentation: true})
}

// Un `@include` dentro del cuerpo de un CODE de un SLIDE es código. Sin
// Presentation, el AST que decide qué es código era el de ParseDocument, que
// no lee un SLIDE: el `@include` se expandía, y un archivo sin la sangría del
// cuerpo cortaba el CODE ("unexpected content").
func TestExpand_PresentationCodeBodiesAreLiteral(t *testing.T) {
	files := map[string]string{
		"/base/part.txt":       "included line\n",
		"/base/frag.slidelang": "  CODE text\n    @include part.txt\n",
	}
	strictDeck := "---\nmode: strict\ntitle: \"Deck\"\n---\n\nSLIDE content\n  title: \"A\"\n"
	for _, tc := range []struct {
		name string
		src  string
	}{
		{"strict CODE", strictDeck + "  CODE text\n    first\n    @include part.txt\n    last\n"},
		{"strict CODE{verbatim}", strictDeck + "  CODE{verbatim} txt\n    @include part.txt\n"},
		{"strict code-group", strictDeck + "  :::code-group\n  ```txt [a]\n  @include part.txt\n  ```\n  :::\n"},
		{"strict CODE in a typed column", strictDeck + "  <<grid>>\n  <<column typed>>\n    CODE text\n      @include part.txt\n  <<end>>\n"},
		{"flex fence", "---\nmode: flex\ntitle: \"Deck\"\n---\n\n## A\n\n```txt\n@include part.txt\n```\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res, err := expandPresentation(tc.src, "/base", memFS(files))
			if err != nil {
				t.Fatalf("Expand: %v", err)
			}
			if res.Text != tc.src {
				t.Errorf("a code body must not be expanded:\n%s", res.Text)
			}
			if len(res.KeptInCode) != 1 {
				t.Errorf("KeptInCode = %v, want one line", res.KeptInCode)
			}
			code := codeContents(t, tc.src, false)
			if !strings.Contains(strings.Join(code, "\n"), "@include part.txt") {
				t.Fatalf("fixture: the presentation parser should keep the directive as code, got %q", code)
			}
		})
	}

	// Un archivo incluido que no empieza con SLIDE se lee como el cuerpo de
	// un SLIDE: su `@include` en código también queda.
	src := strictDeck + "@include frag.slidelang\n"
	res, err := expandPresentation(src, "/base", memFS(files))
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	want := strictDeck + "  CODE text\n    @include part.txt\n\n"
	if res.Text != want {
		t.Errorf("included fragment:\n got %q\nwant %q", res.Text, want)
	}
}

// Fuera del código, Presentation expande igual que la expansión de siempre
// (ExpandWithSourceMap): mismo texto y mismos orígenes, y un archivo incluido
// no se cierra (OpenAtEnd vacío).
func TestExpand_PresentationOutsideCodeExpandsAsBefore(t *testing.T) {
	files := map[string]string{
		"/base/body.slidelang":  "  TEXT\n    Incluido.\n",
		"/base/flow.mmd":        "    A-->B\n",
		"/base/slide.slidelang": "SLIDE content\n  title: \"B\"\n  TEXT\n    Otra.\n",
		"/base/open.md":         "```python\nx = 1\n",
		"/base/col.slidelang":   "    TEXT\n      En columna.\n",
	}
	strictDeck := "---\nmode: strict\ntitle: \"Deck\"\n---\n\nSLIDE content\n  title: \"A\"\n"
	flexDeck := "---\nmode: flex\ntitle: \"Deck\"\n---\n\n## A\n\n"
	for _, tc := range []struct {
		name, src string
	}{
		{"slide body", strictDeck + "@include body.slidelang\n"},
		{"after a CODE body, at its indentation", strictDeck + "  CODE\n    x = 1\n  @include body.slidelang\n"},
		{"inside a diagram", strictDeck + "  <<mermaid>>\n    graph TD\n    @include flow.mmd\n  <<end>>\n"},
		{"a whole slide", strictDeck + "  TEXT\n    Uno.\n\n@include slide.slidelang\n"},
		{"typed column", strictDeck + "  <<grid>>\n  <<column typed>>\n@include col.slidelang\n  <<end>>\n"},
		{"flex, a file that leaves a fence open", flexDeck + "@include open.md\n\nDespués.\n"},
		{"flex, inside a callout", flexDeck + ":::note\n@include body.slidelang\n:::\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res, err := expandPresentation(tc.src, "/base", memFS(files))
			if err != nil {
				t.Fatalf("Expand: %v", err)
			}
			want, sources, err := ExpandWithSourceMap(tc.src, "/base", memFS(files))
			if err != nil {
				t.Fatalf("ExpandWithSourceMap: %v", err)
			}
			if res.Text != want {
				t.Errorf("text:\n got %q\nwant %q", res.Text, want)
			}
			if len(res.Sources.Lines) != len(sources.Lines) {
				t.Fatalf("sources: %d lines, want %d", len(res.Sources.Lines), len(sources.Lines))
			}
			for i := range sources.Lines {
				if res.Sources.Lines[i] != sources.Lines[i] {
					t.Errorf("source of line %d: %+v, want %+v", i+1, res.Sources.Lines[i], sources.Lines[i])
				}
			}
			if len(res.OpenAtEnd) != 0 || len(res.KeptInCode) != 0 {
				t.Errorf("OpenAtEnd = %+v, KeptInCode = %v, want none", res.OpenAtEnd, res.KeptInCode)
			}
		})
	}
}

// Sin KeepIncludesInCode, Presentation no cambia nada: se expande en todas
// partes, también en código.
func TestExpand_PresentationNeedsKeepIncludesInCode(t *testing.T) {
	files := map[string]string{"/base/part.txt": "    included line\n"}
	src := "---\nmode: strict\ntitle: \"Deck\"\n---\n\nSLIDE content\n  title: \"A\"\n  CODE text\n    @include part.txt\n"
	res, err := ExpandWithOptions(src, "/base", memFS(files), ExpandOptions{Presentation: true})
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	want, err := Expand(src, "/base", memFS(files))
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if res.Text != want || strings.Contains(res.Text, "@include") {
		t.Errorf("got %q, want %q", res.Text, want)
	}
}
