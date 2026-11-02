// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// El destino de un enlace o imagen sigue la gramática de CommonMark: el
// título no es parte de la ruta y <...> admite espacios. Una ruta suelta con
// espacios que no abren un título se sigue leyendo como antes (hasta el ")"
// que balancea), marcada Spaced para el aviso del linter.
func TestParseLinkDestination(t *testing.T) {
	for _, tc := range []struct {
		in, dest, title string
		consumed        int
		spaced, ok      bool
	}{
		{`img/a.png)`, "img/a.png", "", 10, false, true},
		{`img/a.png "t")`, "img/a.png", "t", 14, false, true},
		{`img/a.png 't')`, "img/a.png", "t", 14, false, true},
		{`img/a.png (t))`, "img/a.png", "t", 14, false, true},
		{`img/a.png   "t"  ) y`, "img/a.png", "t", 18, false, true},
		{`<mi dir/a.png>)`, "mi dir/a.png", "", 15, false, true},
		{`<mi dir/a.png> "t")`, "mi dir/a.png", "t", 19, false, true},
		{`<a)b.png>)`, "a)b.png", "", 10, false, true},
		{`mi%20dir/a.png)`, "mi%20dir/a.png", "", 15, false, true},
		{`foto_(1).png)`, "foto_(1).png", "", 13, false, true},
		{`a.png "di \"hola\"")`, "a.png", `di "hola"`, 20, false, true},
		{` a.png )`, "a.png", "", 8, false, true},
		{` )`, "", "", 2, false, true},
		{`a.png"t")`, `a.png"t"`, "", 9, false, true},
		// Compatibilidad: ruta suelta con espacios.
		{`t.png bare)`, "t.png bare", "", 11, true, true},
		{`mi dir/a.png)`, "mi dir/a.png", "", 13, true, true},
		{`a.png "t" x)`, `a.png "t" x`, "", 12, true, true},
		{`a (b c))`, "a", "b c", 8, false, true},
		{`a(b c))`, "a(b c)", "", 7, true, true},
		{`a.png (t(u)))`, "a.png (t(u))", "", 13, true, true},
		// No es enlace.
		{`)`, "", "", 0, false, false},
		{`<a.png>"t")`, "", "", 0, false, false},
		{`<a b`, "", "", 0, false, false},
		{`<a<b>)`, "", "", 0, false, false},
		{`<a b) c`, "", "", 0, false, false},
		{`foto_(1.png`, "", "", 0, false, false},
		{`b (c`, "", "", 0, false, false},
		{"a.png \"t\nu\")", "", "", 0, false, false},
	} {
		d, ok := ParseLinkDestination(tc.in)
		if ok != tc.ok || d.Dest != tc.dest || d.Title != tc.title || d.Consumed != tc.consumed || d.Spaced != tc.spaced {
			t.Errorf("ParseLinkDestination(%q) = %+v %v; want %q %q %d spaced=%v %v", tc.in, d, ok, tc.dest, tc.title, tc.consumed, tc.spaced, tc.ok)
		}
		raw, rn, rok := ScanLinkDestination(tc.in)
		if rok != tc.ok || rn != tc.consumed || (rok && raw != tc.in[:rn-1]) {
			t.Errorf("ScanLinkDestination(%q) = %q %d %v", tc.in, raw, rn, rok)
		}
	}
}

func TestInlineImageDestinations(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{`![a](img/a.png "t")`, `<img src="img/a.png" alt="a" title="t">`},
		{`![a](img/a.png 't')`, `<img src="img/a.png" alt="a" title="t">`},
		{`![a](img/a.png (t))`, `<img src="img/a.png" alt="a" title="t">`},
		{`![a](<mi dir/a.png>)`, `<img src="mi%20dir/a.png" alt="a">`},
		{`![a](mi%20dir/a.png)`, `<img src="mi%20dir/a.png" alt="a">`},
		{`![a](t.png bare)`, `<img src="t.png%20bare" alt="a">`},
		{`[l](mi página.html)`, `<a href="mi%20página.html">l</a>`},
		{`![a](a.png) ![b](b.png)`, `<img src="a.png" alt="a"> <img src="b.png" alt="b">`},
		{`![a](a.png){width=30%} ![b](b.png){fit=cover focus="50% 50%"} x{y=1}`, `<img src="a.png" alt="a"> <img src="b.png" alt="b"> x{y=1}`},
		{`[![a](x.png "t")](https://y "u")`, `<a href="https://y" title="u"><img src="x.png" alt="a" title="t"></a>`},
		{`[l](<https://e.com/a b>)`, `<a href="https://e.com/a%20b">l</a>`},
		{`![a](x.png "&amp; &lt;")`, `<img src="x.png" alt="a" title="&amp; &lt;">`},
		{"![a](x.png \"a *b* `c`\")", `<img src="x.png" alt="a" title="a b c">`},
	} {
		if got := ProcessInlineMarkdownSecure(tc.in); got != tc.want {
			t.Errorf("ProcessInlineMarkdownSecure(%q)\n got %q\nwant %q", tc.in, got, tc.want)
		}
	}
}

func TestFindInlineLinks_CleanDestination(t *testing.T) {
	links := FindInlineLinks(`![a](<mi dir/a.png> "t") [l](javascript:x "u")`)
	if len(links) != 2 || links[0].Destination != "mi dir/a.png" || links[1].Destination != "javascript:x" || !links[1].Dropped {
		t.Errorf("FindInlineLinks = %#v", links)
	}
}

func TestRenderImageElement_TitleAndSpaces(t *testing.T) {
	img := ast.NewImageElement(diagnostics.NewPosition(1, 1), "mi dir/a.png", "A")
	img.Title = `di "x"`
	if got, want := renderImageElement(img, nil, nil), `<img src="mi%20dir/a.png" alt="A" title="di &quot;x&quot;">`; got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

// Un destino local es una URL: en offline-inline "mi%20dir/a.png" se lee del
// archivo "mi dir/a.png", igual que lo resuelve el navegador.
func TestOfflineInline_PercentEncodedLocalSource(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "mi dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "mi dir", "a.png"), []byte("\x89PNG\r\n\x1a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := &RenderContext{ImageMode: "offline-inline", AssetRoot: dir}
	for _, src := range []string{"mi%20dir/a.png", "mi dir/a.png"} {
		img := ast.NewImageElement(diagnostics.NewPosition(1, 1), src, "A")
		if got := renderImageElement(img, nil, ctx); !strings.Contains(got, `src="data:image/png;base64,`) {
			t.Errorf("%s: %s", src, got)
		}
	}
	html := resolveInlineImages(ProcessInlineMarkdownSecure(`![i](mi%20dir/a.png "t")`), ctx)
	if !strings.Contains(html, `src="data:image/png;base64,`) || !strings.Contains(html, `title="t"`) {
		t.Errorf("inline: %s", html)
	}
}

func TestLocalPathFromURL(t *testing.T) {
	for in, want := range map[string]string{
		"mi%20dir/a.png": "mi dir/a.png",
		"100%.png":       "100%.png",
		"100%25.png":     "100%.png",
		"a.png":          "a.png",
	} {
		if got := LocalPathFromURL(in); got != want {
			t.Errorf("LocalPathFromURL(%q) = %q, want %q", in, got, want)
		}
	}
}
