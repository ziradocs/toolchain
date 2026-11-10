// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package linter

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
)

// El kicker de un slide se pinta con el mismo Markdown inline que el título
// (renderer.BlockTitleHTML), así que un enlace que el renderer descarta ahí
// también se reporta, igual que en title, heading y subtitle.
func TestLinkDestinationRuleCoversSlideHeaderFields(t *testing.T) {
	src := "---\nmode: strict\ntitle: \"Probe\"\n---\n\n" +
		"SLIDE content\n" +
		"  kicker: \"[k](javascript:alert(1))\"\n" +
		"  title: \"[t](vbscript:x)\"\n" +
		"  subtitle: \"[s](data:text/html,x)\"\n" +
		"  TEXT\n    Cuerpo.\n"
	doc, parseDiags := parser.New(util.NewNoop()).Parse(src, "probe.slidelang")
	if doc == nil {
		t.Fatalf("parse returned no AST: %v", parseDiags)
	}
	if doc.ContentBlocks[0].Kicker == "" {
		t.Fatalf("the kicker did not reach the AST: %+v", doc.ContentBlocks[0])
	}
	var msgs []string
	for _, d := range link001(New().Lint(doc)) {
		msgs = append(msgs, d.Message)
	}
	joined := strings.Join(msgs, "\n")
	for _, want := range []string{`"javascript:alert(1)"`, `"vbscript:x"`, `"data:text/html,x"`} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing LINK001 for %s; got:\n%s", want, joined)
		}
	}
}
