// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"testing"

	"go.ziradocs.com/core/v2/renderer"
)

// coreReadsCommonMarkDestinations dice si el core enlazado lee los destinos
// como CommonMark (<...> delimita la ruta, título aparte). Los tests que
// dependen de esa regla corren con los dos cores: el del árbol (go.work) y el
// publicado (GOWORK=off).
func coreReadsCommonMarkDestinations() bool {
	_, n, ok := renderer.ScanLinkDestination("<a)b>)")
	return ok && n == 6
}

func TestMarkdownImageDestinations(t *testing.T) {
	// La ruta es la que lee core, salvo un título bien formado al final de
	// una ruta suelta con espacios, que queda fuera de <...> (ver
	// splitSpacedTitle). `(1).png` y `'e` no son títulos: van dentro.
	spaced := []struct{ in, want string }{
		{"![a](cap uno/x.png)", "![a](<cap uno/x.png>)"},
		{"![a](cap uno/x.png \"T\")", "![a](<cap uno/x.png> \"T\")"},
		{"![a](cap uno/x.png 'T')", "![a](<cap uno/x.png> 'T')"},
		{"![a](fotos/a (1).png)", "![a](<fotos/a (1).png>)"},
		{"![c](c d.png 'e)f')", "![c](<c d.png 'e>)f')"},
	}
	cases := []struct{ in, want string }{
		{"![a](<cap uno/x.png>)", "![a](<cap uno/x.png>)"},
		{"![a](<cap uno/x.png> \"T\")", "![a](<cap uno/x.png> \"T\")"},
		{"![a](x.png \"con espacio\")", "![a](x.png \"con espacio\")"},
		{"![a](a(1).png 't')", "![a](a(1).png 't')"},
		{"`![a](cap uno/x.png)`", "`![a](cap uno/x.png)`"},
		{"[l](a b.html)", "[l](a b.html)"},
	}
	cases = append(cases, spaced...)
	for _, tc := range cases {
		if got := MarkdownImageDestinations(tc.in); got != tc.want {
			t.Errorf("MarkdownImageDestinations(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestDocxLocalImageSource_Spaces(t *testing.T) {
	// La ruta es la que lee el renderer HTML (ver
	// SplitMarkdownDestination), salvo un título bien formado al final de una
	// ruta suelta con espacios; `(1).png` sí es parte de la ruta.
	cases := map[string]string{
		"cap uno/x.png":         "cap uno/x.png",
		"cap uno/x.png \"T\"":   "cap uno/x.png",
		"cap uno/x.png 'e":      "cap uno/x.png 'e",
		"<cap uno/x.png> \"T\"": "cap uno/x.png",
		"<cap uno/x.png>":       "cap uno/x.png",
		"x.png 'T'":             "x.png",
		"cap%20uno/x.png":       "cap uno/x.png",
		"fotos/a (1).png":       "fotos/a (1).png",
		"fotos/a(1).png (t)":    "fotos/a(1).png",
		"x.png \"t\\\"x\"":      "x.png",
		"<a)b.png>":             "a)b.png",
		// Una ruta ya limpia que core no lee como destino (el src de un
		// encabezado, el Destination de FindInlineLinks) se usa tal cual.
		"a)b.png": "a)b.png",
	}
	for in, want := range cases {
		if got, ok := docxLocalImageSource(in); !ok || got != want {
			t.Errorf("docxLocalImageSource(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
}

func TestMarkdownDestination(t *testing.T) {
	cases := []struct{ path, title, want string }{
		{"a.png", "", "a.png"},
		{"a.png", `di "x"`, `a.png "di \"x\""`},
		{"foto_(1).png", "", "foto_(1).png"},
	}
	if coreReadsCommonMarkDestinations() {
		cases = append(cases,
			struct{ path, title, want string }{"mi dir/a.png", "", "<mi dir/a.png>"},
			struct{ path, title, want string }{"a)b.png", "t", `<a)b.png> "t"`},
		)
	}
	for _, tc := range cases {
		if got := markdownDestination(tc.path, tc.title); got != tc.want {
			t.Errorf("markdownDestination(%q, %q) = %q, want %q", tc.path, tc.title, got, tc.want)
		}
	}
}

func TestSplitSpacedTitle(t *testing.T) {
	for _, tc := range []struct{ in, path, rest string }{
		{`mi dir/x.png "T"`, "mi dir/x.png", ` "T"`},
		{`mi dir/x.png  'T'`, "mi dir/x.png", `  'T'`},
		{`mi dir/x.png (T)`, "mi dir/x.png", ` (T)`},
		{`mi dir/x.png "t\"x"`, "mi dir/x.png", ` "t\"x"`},
		{`a b "x" "y"`, `a b "x"`, ` "y"`},
		{"fotos/a (1).png", "", ""},
		{"c d.png 'e", "", ""},
		{`mi dir/x.png"T"`, "", ""},
		{`mi dir/x.png "T\"`, "", ""},
		{`mi dir/x.png (a(b))`, "", ""},
		{` "T"`, "", ""},
	} {
		path, rest, ok := splitSpacedTitle(tc.in)
		if ok != (tc.path != "") || path != tc.path || rest != tc.rest {
			t.Errorf("splitSpacedTitle(%q) = %q %q %v; want %q %q", tc.in, path, rest, ok, tc.path, tc.rest)
		}
	}
}
