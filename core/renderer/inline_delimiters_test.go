// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"encoding/json"
	"os"
	"testing"
)

type inlineDelimiterCase struct {
	Src  string `json:"src"`
	HTML string `json:"html"`
	Kind string `json:"kind"`
}

// loadInlineDelimiterCases lee la tabla compartida con slidelang (PPTX) y
// doclang (DOCX): los tres comparan contra ella para que sus copias de la
// regla no vuelvan a divergir.
func loadInlineDelimiterCases(t *testing.T) []inlineDelimiterCase {
	t.Helper()
	raw, err := os.ReadFile("testdata/inline_delimiters.json")
	if err != nil {
		t.Fatal(err)
	}
	var data struct {
		Cases []inlineDelimiterCase `json:"cases"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	return data.Cases
}

// Un delimitador de formato abre solo si no lo sigue un espacio (NBSP
// incluido) ni el fin de la línea; al cerrar no se exige nada. La misma
// regla en una línea suelta, en el cuerpo y en el título de un slide.
func TestInlineFormats_DelimiterOpeningRule(t *testing.T) {
	for _, tc := range loadInlineDelimiterCases(t) {
		t.Run(tc.Src, func(t *testing.T) {
			if got := ProcessInlineMarkdownSecureLine(tc.Src); got != tc.HTML {
				t.Errorf("line: %q, want %q", got, tc.HTML)
			}
			if got := ProcessTextWithVariablesAndMarkdownSecure(tc.Src, nil); got != tc.HTML {
				t.Errorf("body: %q, want %q", got, tc.HTML)
			}
			if got := BlockTitleHTML(tc.Src, nil); got != tc.HTML {
				t.Errorf("title: %q, want %q", got, tc.HTML)
			}
		})
	}
}
