// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
)

// parseDocumentNoNormalize parses without the content normalizer, which
// restructures/re-emits subsection headers in a way that no longer exercises
// parseSubsectionHeader's own <hN>/IsRawHTML materialization — the exact
// mechanism this test targets. parseDocument (document_roundtrip_test.go),
// which DOES normalize, is the right tool for corpus-style round-trip tests,
// just not for this parser-level behavior.
func parseDocumentNoNormalize(t *testing.T, content string) *parser.DocumentFlexParser {
	t.Helper()
	return parser.NewDocumentFlexParser(content, util.NewNoop())
}

// TestFormatDocument_LangSpanInHeading_FailsLoud cubre issue #63: un
// [texto]{lang=xx} dentro de un heading (## texto) se materializa a
// <span lang="xx"> en tiempo de parseo (core/parser/document_flex.go), así
// que para cuando FormatDocument corre ya no queda ningún [texto]{lang=xx}
// que reemitir — solo el <span> ya renderizado. stripTags lo colapsaría a
// texto liso, perdiendo la marca de idioma EN SILENCIO. Debe fallar fuerte
// (*UnsupportedElementError) en vez de eso.
func TestFormatDocument_LangSpanInHeading_FailsLoud(t *testing.T) {
	src := "# Título\n\n## Bonjour [tout le monde]{lang=fr}\n\nTexto normal.\n"
	p := parseDocumentNoNormalize(t, src)
	doc, diags := p.Parse()
	for _, d := range diags {
		if d.Severity == "error" {
			t.Fatalf("parse error: %s", d.Message)
		}
	}

	_, err := FormatDocument(doc)
	if err == nil {
		t.Fatal("expected FormatDocument to fail on a lang span inside a heading, got nil error")
	}
	if _, ok := err.(*UnsupportedElementError); !ok {
		t.Fatalf("FormatDocument error type = %T, want *UnsupportedElementError: %v", err, err)
	}
	if !strings.Contains(err.Error(), "lang") {
		t.Errorf("expected error message to mention the lang span, got: %v", err)
	}
}

// TestFormatDocument_PlainHeading_StillRoundTrips confirms the guard is
// scoped to lang spans specifically — an ordinary heading with no lang span
// (including one with **bold**/*italic*, the pre-existing lossy-but-accepted
// case) must still format without error.
func TestFormatDocument_PlainHeading_StillRoundTrips(t *testing.T) {
	src := "# Título\n\n## Bonjour **tout le monde**\n\nTexto normal.\n"
	p := parseDocumentNoNormalize(t, src)
	doc, diags := p.Parse()
	for _, d := range diags {
		if d.Severity == "error" {
			t.Fatalf("parse error: %s", d.Message)
		}
	}

	out, err := FormatDocument(doc)
	if err != nil {
		t.Fatalf("FormatDocument: %v", err)
	}
	if !strings.Contains(out, "## Bonjour") {
		t.Errorf("expected formatted output to contain the heading, got: %q", out)
	}
}
