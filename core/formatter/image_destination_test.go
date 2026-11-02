// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// fmt escribe de vuelta el título de una imagen y envuelve en <...> la ruta
// con espacios, que suelta ya no se leería como imagen.
func TestFormatDocument_ImageTitleAndBrackets(t *testing.T) {
	src := "# T\n\n![a](img/a.png \"Un \\\"título\\\"\")\n\n![b](<mi dir/b.png>)\n\n![c](mi%20dir/c.png 'x')\n"
	out, err := FormatDocument(parseDocumentOrFail(t, src))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`![a](img/a.png "Un \"título\"")`, `![b](<mi dir/b.png>)`, `![c](mi%20dir/c.png "x")`} {
		if !strings.Contains(out, want) {
			t.Errorf("fmt no escribe %s:\n%s", want, out)
		}
	}
	again, err := FormatDocument(parseDocumentOrFail(t, out))
	if err != nil {
		t.Fatal(err)
	}
	if again != out {
		t.Errorf("fmt no es idempotente:\n%s\n---\n%s", out, again)
	}
}

func TestMarkdownImageDestination_Unrepresentable(t *testing.T) {
	if _, err := markdownImageDestination("a <b.png", ""); err == nil {
		t.Error("una ruta con espacios y < no se puede escribir y debía fallar")
	}
}

// fmt strict (el default de slidelang fmt) escribe el título como propiedad
// `title:` del IMAGE y la vuelve a leer igual, también desde flex con
// comillas simples o paréntesis.
func TestFormatStrict_ImageTitleRoundTrip(t *testing.T) {
	for _, src := range []string{
		"---\ntitle: D\nmode: flex\n---\n\n# Uno\n\n![a](img/a.png 'Un título')\n",
		"---\ntitle: D\nmode: flex\n---\n\n# Uno\n\n![a](<mi dir/a.png> (con \"comillas\" dentro))\n",
	} {
		doc := mustParse(t, src, false)
		want := collectImages(doc)[0]
		out, err := FormatStrict(doc)
		if err != nil {
			t.Fatalf("FormatStrict: %v", err)
		}
		if !strings.Contains(out, "  title: ") {
			t.Fatalf("sin title:\n%s", out)
		}
		again := collectImages(mustParse(t, out, false))
		if len(again) != 1 || again[0].Title != want.Title || again[0].Source != want.Source {
			t.Errorf("round trip: %q %q -> %#v\n%s", want.Source, want.Title, again, out)
		}
		out2, err := FormatStrict(mustParse(t, out, false))
		if err != nil || out2 != out {
			t.Errorf("fmt strict no es idempotente (%v):\n%s\n---\n%s", err, out, out2)
		}
	}
}

func TestFormatStrict_ImageTitleEdgeQuote(t *testing.T) {
	img := ast.NewImageElement(diagnostics.NewPosition(1, 1), "a.png", "A")
	img.Title = `"citado"`
	if _, err := formatStrictImage(img); err == nil {
		t.Error("un título con comillas en los extremos no se puede leer de vuelta y debía fallar")
	}
}
