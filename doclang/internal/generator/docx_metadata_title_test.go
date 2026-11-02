// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"path/filepath"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/parser"
)

// TestDOCXGenerator_PlainMarkdownTitle usa el AST que de verdad arma el
// parser para un Markdown sin front matter: el primer `#` llega como
// Heading de un bloque "title", no como Title. docxMetadataTitle solo leía
// Title, así que el dc:title quedaba vacío (el <title> del HTML y el del PDF
// ya salían bien, por DocumentTitle).
func TestDOCXGenerator_PlainMarkdownTitle(t *testing.T) {
	for _, tc := range []struct {
		name, src, fallback, want string
	}{
		{"primer H1", "# Informe offline\n\nTexto.\n", "", "Informe offline"},
		{"dos H1: el primero", "# Uno\n\nA\n\n# Dos\n\nB\n", "", "Uno"},
		{"texto antes del H1", "Intro.\n\n# Título\n\nTexto.\n", "", "Título"},
		{"sin H1: el respaldo del caller", "## Solo dos\n\nTexto.\n", "notas", "notas"},
		{"front matter gana", "---\ntitle: Del FM\n---\n\n# Otro\n\nTexto.\n", "notas", "Del FM"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc, _ := parser.New(newTestLogger()).ParseDocument(tc.src, "x.md")
			if got := docxMetadataTitle(doc, tc.fallback); got != tc.want {
				t.Fatalf("docxMetadataTitle = %q, want %q", got, tc.want)
			}
			output := filepath.Join(t.TempDir(), "out.docx")
			if err := New(newTestLogger()).Generate(doc, output, GeneratorOptions{Format: "docx", Title: tc.fallback}); err != nil {
				t.Fatalf("Generate() error = %v", err)
			}
			if got := docxCoreTitle(t, output); got != tc.want {
				t.Errorf("dc:title = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestDOCXGenerator_MathWithoutChromium_LeavesPlaceholder: sin Chromium la
// ecuación ya no desaparece en silencio; queda un marcador y un aviso con
// la línea.
func TestDOCXGenerator_MathWithoutChromium_LeavesPlaceholder(t *testing.T) {
	logger := newTestLogger()
	gen := NewDOCXGenerator(logger, t.TempDir())
	doc := astWithElements(ast.NewMathElement(diagnostics.NewPosition(7, 1), "E = mc^2"))
	output := filepath.Join(t.TempDir(), "out.docx")
	// ChromiumPath inexistente y sin autoinstalar: chromiumRenderer queda nil.
	if err := gen.Generate(doc, output, GeneratorOptions{Format: "docx", ChromiumPath: filepath.Join(t.TempDir(), "no-chromium")}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if gen.chromiumRenderer != nil {
		t.Skip("a Chromium was found anyway; this test needs it missing")
	}
	if xml := docxDocumentXML(t, output); !strings.Contains(xml, "Equation (not rendered: Chromium not available)") {
		t.Errorf("missing placeholder in document.xml:\n%s", xml)
	}
	found := false
	for _, w := range logger.warns {
		if strings.Contains(w, "equation at line 7 was not rendered") {
			found = true
		}
	}
	if !found {
		t.Errorf("missing warning naming the line; warns = %v", logger.warns)
	}
}
