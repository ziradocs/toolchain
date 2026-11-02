// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package linter

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
)

// Una ruta suelta con espacios se sigue leyendo como antes, con un LINK001
// que sugiere <...> o %20; los atributos de una imagen inline se descartan
// con un IMG004. Una ruta entre <...> o con %20 no avisa.
func TestLinkDestinationRuleSpacedAndInlineAttrs(t *testing.T) {
	src := "---\ntitle: \"Probe\"\n---\n\n# Probe\n\n" +
		"Ver [la página](mi página.html) y ![foto](mi foto.png).\n\n" +
		"Bien [a](<mi página.html>) y ![b](mi%20foto.png).\n\n" +
		"![uno](a.png){width=30%} ![dos](b.png)\n"
	doc, parseDiags := parser.New(util.NewNoop()).ParseDocument(src, "test.doclang")
	if doc == nil {
		t.Fatalf("parse returned no AST: %v", parseDiags)
	}
	var link, img []string
	for _, d := range New().WithDialect(DialectDocuments).Lint(doc) {
		switch d.RuleID {
		case "LINK001":
			link = append(link, d.Message)
		case "IMG004":
			img = append(img, d.Message)
		}
	}
	joined := strings.Join(link, "\n")
	if len(link) != 2 || !strings.Contains(joined, `"mi página.html" has spaces`) || !strings.Contains(joined, `"mi foto.png" has spaces`) {
		t.Errorf("LINK001 = %q", link)
	}
	if len(img) != 1 || !strings.Contains(img[0], `"a.png"`) {
		t.Errorf("IMG004 = %q", img)
	}
}
