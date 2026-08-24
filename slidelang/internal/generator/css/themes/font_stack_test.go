// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package themes

import (
	"reflect"
	"testing"
)

func TestSplitFontStack(t *testing.T) {
	cases := []struct {
		name  string
		stack string
		want  []string
	}{
		{
			name:  "simple stack",
			stack: `'Crimson Text', 'Times New Roman', serif`,
			want:  []string{`'Crimson Text'`, `'Times New Roman'`, "serif"},
		},
		{
			name:  "comma inside quoted family name",
			stack: `"Foo, Bar", serif`,
			want:  []string{`"Foo, Bar"`, "serif"},
		},
		{
			name:  "escaped quote inside quoted family name",
			stack: `"Foo \" Bar", serif`,
			want:  []string{`"Foo \" Bar"`, "serif"},
		},
		{
			name:  "unquoted multi-word family",
			stack: `Times New Roman, serif`,
			want:  []string{"Times New Roman", "serif"},
		},
		{
			name:  "single entry, no comma",
			stack: "sans-serif",
			want:  []string{"sans-serif"},
		},
		{
			name:  "trailing comma dropped as empty",
			stack: `'Inter', `,
			want:  []string{`'Inter'`},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := SplitFontStack(c.stack)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("SplitFontStack(%q) = %#v, want %#v", c.stack, got, c.want)
			}
		})
	}
}

func TestSplitFontStack_EmptyStackReturnsNoEntries(t *testing.T) {
	if got := SplitFontStack(""); len(got) != 0 {
		t.Errorf("SplitFontStack(\"\") = %#v, want no entries", got)
	}
}

func TestUnquoteFontFamily(t *testing.T) {
	cases := map[string]string{
		`"Crimson Text"`:  "Crimson Text",
		`'Crimson Text'`:  "Crimson Text",
		"serif":           "serif",
		`"Foo \" Bar"`:    `Foo " Bar`,
		`"Foo \\ Bar"`:    `Foo \ Bar`,
		`Times New Roman`: "Times New Roman",
	}
	for in, want := range cases {
		if got := unquoteFontFamily(in); got != want {
			t.Errorf("unquoteFontFamily(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestSplitFontStack_CaseInsensitiveNameMatch is a regression for the
// motor-temas-v2.md §2.3 backing check: font-family entries are compared to
// ThemeFont.Name case-insensitively (CSS font-family matching is
// case-insensitive), so "'crimson text'" backed by a ThemeFont named
// "Crimson Text" must not warn.
func TestSplitFontStack_CaseInsensitiveNameMatch(t *testing.T) {
	entries := SplitFontStack(`'crimson text', serif`)
	if len(entries) == 0 {
		t.Fatal("expected at least one entry")
	}
	first := unquoteFontFamily(entries[0])
	if !genericFontFamilyKeywords["serif"] {
		t.Fatal("sanity: serif must be a generic keyword")
	}
	if first != "crimson text" {
		t.Errorf("got %q, want %q", first, "crimson text")
	}
}
