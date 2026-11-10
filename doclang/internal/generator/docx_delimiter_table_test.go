// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"encoding/json"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

type docxDelimiterRun struct {
	text         string
	bold, italic bool
}

type docxDelimiterCase struct {
	Src  string `json:"src"`
	HTML string `json:"html"`
	Kind string `json:"kind"`
}

// La tabla es de core. doclang compila contra el core publicado, así que no
// compara contra la salida de core en vivo sino contra este contrato, el
// mismo que leen core (HTML) y slidelang (PPTX).
func loadDocxDelimiterTable(t *testing.T) []docxDelimiterCase {
	t.Helper()
	raw, err := os.ReadFile("../../../core/renderer/testdata/inline_delimiters.json")
	if err != nil {
		t.Fatal(err)
	}
	var data struct {
		Cases []docxDelimiterCase `json:"cases"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	return data.Cases
}

var docxDelimiterTagRe = regexp.MustCompile(`</?(strong|em)>`)

// docxExpectedRuns convierte el HTML de la tabla en runs; ok es false si trae
// una etiqueta que el DOCX no tiene (<mark>, <del>, <code>).
func docxExpectedRuns(s string) ([]docxDelimiterRun, bool) {
	if strings.Contains(docxDelimiterTagRe.ReplaceAllString(s, ""), "<") {
		return nil, false
	}
	var runs []docxDelimiterRun
	var cur docxDelimiterRun
	last := 0
	for _, loc := range docxDelimiterTagRe.FindAllStringIndex(s, -1) {
		if loc[0] > last {
			cur.text = html.UnescapeString(s[last:loc[0]])
			runs = append(runs, cur)
		}
		tag := s[loc[0]:loc[1]]
		on := !strings.HasPrefix(tag, "</")
		if strings.Contains(tag, "strong") {
			cur.bold = on
		} else {
			cur.italic = on
		}
		last = loc[1]
	}
	if last < len(s) {
		cur.text = html.UnescapeString(s[last:])
		runs = append(runs, cur)
	}
	return mergeDocxRuns(runs), true
}

func mergeDocxRuns(runs []docxDelimiterRun) []docxDelimiterRun {
	var out []docxDelimiterRun
	for _, r := range runs {
		if r.text == "" {
			continue
		}
		if n := len(out); n > 0 && out[n-1].bold == r.bold && out[n-1].italic == r.italic {
			out[n-1].text += r.text
			continue
		}
		out = append(out, r)
	}
	return out
}

var (
	docxParagraphRe = regexp.MustCompile(`(?s)<w:p\b.*?</w:p>`)
	docxRunRe       = regexp.MustCompile(`(?s)<w:r>.*?</w:r>|<w:r [^>]*>.*?</w:r>`)
	docxRunTextRe   = regexp.MustCompile(`<w:t[^>]*>([^<]*)</w:t>`)
	docxBoldRe      = regexp.MustCompile(`<w:b[\s/>]`)
	docxItalicRe    = regexp.MustCompile(`<w:i[\s/>]`)
)

// docxParagraphRuns lee las runs del párrafo de document.xml cuyo texto es
// plain.
func docxParagraphRuns(t *testing.T, xml, plain string) []docxDelimiterRun {
	t.Helper()
	for _, p := range docxParagraphRe.FindAllString(xml, -1) {
		var runs []docxDelimiterRun
		var text strings.Builder
		for _, r := range docxRunRe.FindAllString(p, -1) {
			var rt strings.Builder
			for _, m := range docxRunTextRe.FindAllStringSubmatch(r, -1) {
				rt.WriteString(html.UnescapeString(m[1]))
			}
			text.WriteString(rt.String())
			runs = append(runs, docxDelimiterRun{text: rt.String(), bold: docxBoldRe.MatchString(r), italic: docxItalicRe.MatchString(r)})
		}
		if text.String() == plain {
			return mergeDocxRuns(runs)
		}
	}
	return nil
}

// El DOCX abre la negrita y la cursiva con la misma regla que core y
// resuelve los mismos escapes. Quedan fuera las filas con resaltado o
// tachado (el DOCX no los tiene) y las de negrita con cursiva combinadas,
// que el DOCX nunca resolvió.
func TestDOCXInline_MatchesCoreDelimiterTable(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	for _, tc := range loadDocxDelimiterTable(t) {
		if tc.Kind == "regression" {
			continue
		}
		want, ok := docxExpectedRuns(tc.HTML)
		if !ok {
			continue
		}
		var plain strings.Builder
		for _, r := range want {
			plain.WriteString(r.text)
		}
		output := filepath.Join(t.TempDir(), "d.docx")
		if err := New(newTestLogger()).Generate(astWithElements(ast.NewTextElement(pos, tc.Src)), output, GeneratorOptions{Format: "docx"}); err != nil {
			t.Fatalf("%q: Generate: %v", tc.Src, err)
		}
		got := docxParagraphRuns(t, docxDocumentXML(t, output), plain.String())
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
