// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"encoding/json"
	"html"
	"os"
	"regexp"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// delimiterRun es un tramo de texto con su formato, la forma común en la
// que se comparan el HTML de core y las runs del PPTX.
type delimiterRun struct {
	text               string
	bold, italic, code bool
}

type delimiterCase struct {
	Src  string `json:"src"`
	HTML string `json:"html"`
	Kind string `json:"kind"`
}

// loadDelimiterTable lee la tabla de core. slidelang compila contra el core
// publicado, así que no puede comparar contra la salida de core en vivo: la
// tabla es el contrato que comparten core (HTML), este PPTX y el DOCX.
func loadDelimiterTable(t *testing.T) []delimiterCase {
	t.Helper()
	raw, err := os.ReadFile("../../../core/renderer/testdata/inline_delimiters.json")
	if err != nil {
		t.Fatal(err)
	}
	var data struct {
		Cases []delimiterCase `json:"cases"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	return data.Cases
}

var delimiterTagRe = regexp.MustCompile(`</?(strong|em|code)>`)

// htmlRuns convierte el HTML inline de la tabla (solo strong, em y code) en
// runs. ok es false si trae otra etiqueta (<mark>, <del>), que el PPTX no
// tiene.
func htmlRuns(s string) (runs []delimiterRun, ok bool) {
	if strings.Contains(delimiterTagRe.ReplaceAllString(s, ""), "<") {
		return nil, false
	}
	var cur delimiterRun
	last := 0
	for _, loc := range delimiterTagRe.FindAllStringIndex(s, -1) {
		if loc[0] > last {
			cur.text = html.UnescapeString(s[last:loc[0]])
			runs = append(runs, cur)
		}
		tag := s[loc[0]:loc[1]]
		on := !strings.HasPrefix(tag, "</")
		switch strings.Trim(tag, "</>") {
		case "strong":
			cur.bold = on
		case "em":
			cur.italic = on
		case "code":
			cur.code = on
		}
		last = loc[1]
	}
	if last < len(s) {
		cur.text = html.UnescapeString(s[last:])
		runs = append(runs, cur)
	}
	return mergeRuns(runs), true
}

func mergeRuns(runs []delimiterRun) []delimiterRun {
	var out []delimiterRun
	for _, r := range runs {
		if r.text == "" {
			continue
		}
		if n := len(out); n > 0 && out[n-1].bold == r.bold && out[n-1].italic == r.italic && out[n-1].code == r.code {
			out[n-1].text += r.text
			continue
		}
		out = append(out, r)
	}
	return out
}

// El PPTX abre la negrita y la cursiva con la misma regla que core (un
// delimitador abre solo si no lo sigue un espacio; al cerrar no se exige
// nada) y resuelve los mismos escapes. Las filas con resaltado o tachado
// quedan fuera porque el PPTX no tiene esos formatos, y las de regresión
// (***x***, cursiva anidada al cierre) porque el PPTX nunca combinó negrita
// y cursiva en un solo delimitador.
func TestPPTXSplitInline_MatchesCoreDelimiterTable(t *testing.T) {
	for _, tc := range loadDelimiterTable(t) {
		if tc.Kind == "regression" {
			continue
		}
		want, ok := htmlRuns(tc.HTML)
		if !ok {
			continue
		}
		var got []delimiterRun
		for _, seg := range pptxSplitInline(tc.Src) {
			got = append(got, delimiterRun{text: seg.text, bold: seg.bold, italic: seg.italic, code: seg.code})
		}
		got = mergeRuns(got)
		if len(got) != len(want) {
			t.Errorf("%q: runs %+v, want %+v", tc.Src, got, want)
			continue
		}
		for i := range got {
			if got[i] != want[i] {
				t.Errorf("%q: runs %+v, want %+v", tc.Src, got, want)
				break
			}
		}
	}
}

// Un escape dentro de un code span se ve tal cual, como en el HTML.
func TestPPTXSplitInline_EscapeInsideCodeStays(t *testing.T) {
	segs := pptxSplitInline("`\\*` y \\*")
	if len(segs) != 2 || segs[0].text != `\*` || !segs[0].code || segs[1].text != " y *" {
		t.Errorf("segments %+v", segs)
	}
}

// La tabla también por la ruta de un encabezado `###`: el HTML de core se
// convierte de vuelta en fuente (pptxTextContent) y se vuelve a partir; las
// runs tienen que ser las mismas que dice el HTML.
func TestPPTXTextContent_MatchesCoreDelimiterTable(t *testing.T) {
	for _, tc := range loadDelimiterTable(t) {
		if tc.Kind == "regression" {
			continue
		}
		want, ok := htmlRuns(tc.HTML)
		if !ok {
			continue
		}
		want = mergeRuns(append([]delimiterRun{{text: "### "}}, want...))
		el := ast.NewRawHTMLTextElement(diagnostics.NewPosition(1, 1), `<h3 id="x">`+tc.HTML+`</h3>`)
		var got []delimiterRun
		for _, seg := range pptxSplitInline(pptxTextContent(el)) {
			got = append(got, delimiterRun{text: seg.text, bold: seg.bold, italic: seg.italic, code: seg.code})
		}
		got = mergeRuns(got)
		if len(got) != len(want) {
			t.Errorf("%q: runs %+v, want %+v", tc.Src, got, want)
			continue
		}
		for i := range got {
			if got[i] != want[i] {
				t.Errorf("%q: runs %+v, want %+v", tc.Src, got, want)
				break
			}
		}
	}
}
