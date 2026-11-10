// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// El Text de un LangRun dice lo mismo que el <span lang> del HTML: un escape
// de delimitador se resuelve (`[x\*y]{lang=en}` -> "x*y", como en
// contentHTML), salvo dentro de un code span, donde el HTML también muestra
// la barra. Un `&#42;` escrito a mano no es un escape y se queda.
func TestPopulateLangRuns_ResolvesDelimiterEscapes(t *testing.T) {
	cases := []struct{ src, want string }{
		{`Dice [x\*y]{lang=en} aquí`, "x*y"},
		{`[a \= b \~ c \\* d]{lang=fr}`, `a = b ~ c \\* d`},
		{"[con `\\*` dentro]{lang=de}", "con `\\*` dentro"},
		{`[x &#42; y]{lang=it}`, "x &#42; y"},
	}
	for _, tc := range cases {
		el := ast.NewTextElement(diagnostics.NewPosition(1, 1), tc.src)
		PopulateLangRuns(newTestDoc(el), nil)
		if len(el.LangRuns) != 1 || el.LangRuns[0].Text != tc.want {
			t.Errorf("%q: LangRuns %+v, want Text %q", tc.src, el.LangRuns, tc.want)
		}
	}
	html := ProcessTextWithVariablesAndMarkdownSecure(`Dice [x\*y]{lang=en} aquí`, nil)
	if html != `Dice <span lang="en">x*y</span> aquí` {
		t.Errorf("contentHTML %q", html)
	}
}
