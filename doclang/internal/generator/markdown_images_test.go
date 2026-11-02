// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import "testing"

func TestMarkdownImageDestinations(t *testing.T) {
	cases := []struct{ in, want string }{
		{"![a](cap uno/x.png)", "![a](<cap uno/x.png>)"},
		{"![a](cap uno/x.png \"T\")", "![a](<cap uno/x.png> \"T\")"},
		{"![a](<cap uno/x.png>)", "![a](<cap uno/x.png>)"},
		{"![a](x.png \"con espacio\")", "![a](x.png \"con espacio\")"},
		{"`![a](cap uno/x.png)`", "`![a](cap uno/x.png)`"},
		{"[l](a b.html)", "[l](a b.html)"},
	}
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
