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
// título no es parte de la ruta, <...> admite espacios y una palabra suelta
// después de un espacio deja de ser enlace.
func TestParseLinkDestination(t *testing.T) {
	for _, tc := range []struct {
		in, dest, title string
		consumed        int
		ok              bool
	}{
		{`img/a.png)`, "img/a.png", "", 10, true},
		{`img/a.png "t")`, "img/a.png", "t", 14, true},
		{`img/a.png 't')`, "img/a.png", "t", 14, true},
		{`img/a.png (t))`, "img/a.png", "t", 14, true},
		{`img/a.png   "t"  ) y`, "img/a.png", "t", 18, true},
		{`<mi dir/a.png>)`, "mi dir/a.png", "", 15, true},
		{`<mi dir/a.png> "t")`, "mi dir/a.png", "t", 19, true},
		{`<a)b.png>)`, "a)b.png", "", 10, true},
		{`mi%20dir/a.png)`, "mi%20dir/a.png", "", 15, true},
		{`foto_(1).png)`, "foto_(1).png", "", 13, true},
		{`a.png "di \"hola\"")`, "a.png", `di "hola"`, 20, true},
		{` a.png )`, "a.png", "", 8, true},
		{` )`, "", "", 2, true},
		{`)`, "", "", 0, false},
		{`t.png bare)`, "", "", 0, false},
		{`mi dir/a.png)`, "", "", 0, false},
		{`a.png "t" x)`, "", "", 0, false},
		{`a.png"t")`, `a.png"t"`, "", 9, true},
		{`<a.png>"t")`, "", "", 0, false},
		{`a.png (t(u)))`, "", "", 0, false},
		{`<a b`, "", "", 0, false},
		{`<a<b>)`, "", "", 0, false},
		{`foto_(1.png`, "", "", 0, false},
		{"a.png \"t\nu\")", "", "", 0, false},
	} {
		dest, title, n, ok := ParseLinkDestination(tc.in)
		if ok != tc.ok || dest != tc.dest || title != tc.title || n != tc.consumed {
			t.Errorf("ParseLinkDestination(%q) = %q %q %d %v; want %q %q %d %v", tc.in, dest, title, n, ok, tc.dest, tc.title, tc.consumed, tc.ok)
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
		{`![a](t.png bare)`, `![a](t.png bare)`},
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
