// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"testing"

	"go.ziradocs.com/core/v2/renderer"
)

// coreReadsCommonMarkDestinations dice si el core enlazado lee los destinos
// como CommonMark (una ruta suelta con espacios no es imagen). Los tests que
// dependen de esa regla corren con los dos cores: el del árbol (go.work) y el
// publicado (GOWORK=off).
func coreReadsCommonMarkDestinations() bool {
	_, _, ok := renderer.ScanLinkDestination("a b)")
	return !ok
}

func TestMarkdownImageDestinations(t *testing.T) {
	spaced := []struct{ in, want string }{
		{"![a](cap uno/x.png)", "![a](<cap uno/x.png>)"},
		{"![a](cap uno/x.png \"T\")", "![a](<cap uno/x.png> \"T\")"},
	}
	if coreReadsCommonMarkDestinations() {
		// Con destinos CommonMark una ruta suelta con espacios ya no es
		// imagen: queda como texto, sin <...>.
		spaced = []struct{ in, want string }{
			{"![a](cap uno/x.png)", "![a](cap uno/x.png)"},
			{"![a](cap uno/x.png \"T\")", "![a](cap uno/x.png \"T\")"},
		}
	}
	cases := []struct{ in, want string }{
		{"![a](<cap uno/x.png>)", "![a](<cap uno/x.png>)"},
		{"![a](x.png \"con espacio\")", "![a](x.png \"con espacio\")"},
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
	cases := map[string]string{
		"cap uno/x.png":       "cap uno/x.png",
		"cap uno/x.png \"T\"": "cap uno/x.png",
		"<cap uno/x.png>":     "cap uno/x.png",
		"x.png 'T'":           "x.png",
		"cap%20uno/x.png":     "cap uno/x.png",
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
