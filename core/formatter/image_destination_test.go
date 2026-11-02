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

func TestFormatStrict_ImageTitleUnsupported(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	img := ast.NewImageElement(pos, "a.png", "A")
	img.Title = "t"
	if _, err := formatStrictImage(img); err == nil {
		t.Error("strict fmt perdió el título en silencio")
	}
	if _, err := markdownImageDestination("a <b.png", ""); err == nil {
		t.Error("una ruta con espacios y < no se puede escribir y debía fallar")
	}
}
