// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/util"
)

func TestQuoteParser_CanParse(t *testing.T) {
	parser := &QuoteParser{}

	tests := []struct {
		name     string
		line     string
		mode     string
		expected bool
	}{
		// Modo strict
		{
			name:     "strict mode with QUOTE keyword",
			line:     "QUOTE",
			mode:     "strict",
			expected: true,
		},
		{
			name:     "strict mode without QUOTE keyword",
			line:     "This is normal text",
			mode:     "strict",
			expected: false,
		},
		// Modo flex
		{
			name:     "flex mode with markdown quote",
			line:     "> This is a quote",
			mode:     "flex",
			expected: true,
		},
		{
			name:     "flex mode with indented quote",
			line:     "  > This is also a quote",
			mode:     "flex",
			expected: true,
		},
		{
			name:     "flex mode without quote marker",
			line:     "This is normal text",
			mode:     "flex",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.CanParse(tt.line, tt.mode)
			if result != tt.expected {
				t.Errorf("CanParse() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestQuoteParser_ParseFlex(t *testing.T) {
	parser := &QuoteParser{}

	tests := []struct {
		name             string
		lines            []string
		expectedContent  string
		expectedAuthor   string
		expectedConsumed int
	}{
		{
			name: "simple quote",
			lines: []string{
				"> This is a simple quote",
			},
			expectedContent:  "This is a simple quote",
			expectedAuthor:   "",
			expectedConsumed: 1,
		},
		{
			name: "multiline quote",
			lines: []string{
				"> This is a quote",
				"> that spans multiple lines",
			},
			expectedContent:  "This is a quote\nthat spans multiple lines",
			expectedAuthor:   "",
			expectedConsumed: 2,
		},
		{
			name: "quote with author",
			lines: []string{
				"> This is a quote with author",
				"> -- John Doe",
			},
			expectedContent:  "This is a quote with author",
			expectedAuthor:   "John Doe",
			expectedConsumed: 2,
		},
		{
			name: "quote with em dash author",
			lines: []string{
				"> Life is what happens",
				"> — Albert Einstein",
			},
			expectedContent:  "Life is what happens",
			expectedAuthor:   "Albert Einstein",
			expectedConsumed: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &ParseContext{
				Mode:   "flex",
				Lines:  tt.lines,
				Logger: util.NewNoop(),
			}

			result := parser.Parse(ctx, 0)

			if result.Error != nil {
				t.Errorf("Parse() error = %v", result.Error)
				return
			}

			if result.ConsumedLines != tt.expectedConsumed {
				t.Errorf("ConsumedLines = %v, expected %v", result.ConsumedLines, tt.expectedConsumed)
			}

			quote, ok := result.Element.(*ast.QuoteElement)
			if !ok {
				t.Errorf("Element is not a QuoteElement, got %T", result.Element)
				return
			}

			if quote.Content != tt.expectedContent {
				t.Errorf("Content = %q, expected %q", quote.Content, tt.expectedContent)
			}

			if quote.Author != tt.expectedAuthor {
				t.Errorf("Author = %q, expected %q", quote.Author, tt.expectedAuthor)
			}
		})
	}
}

// TestQuoteParser_ParseStrict_DoesNotSwallowSiblingElement cubre la misma
// clase de bug que el fix de IMAGE (issue: "strict IMAGE deja de tragarse
// elementos hermanos"), pero para QUOTE: parseStrict (internal/elements/
// quote.go) termina su loop solo en línea vacía, "---", o
// IsNewElement(line, "strict") — y esa función (internal/elements/common.go)
// solo reconoce keywords en mayúsculas en modo strict, no los marcadores
// simbólicos @ (directiva), ::: (special block), << (math/mermaid/etc.) ni |
// (tabla Markdown). A diferencia de CHECKLIST, QUOTE no tiene NINGÚN guard de
// indentación — así que un elemento hermano inmediatamente después (sin línea
// en blanco) se traga siempre como texto de la cita, sin importar su sangría.
func TestQuoteParser_ParseStrict_DoesNotSwallowSiblingElement(t *testing.T) {
	parser := &QuoteParser{}
	logger := util.NewNoop()

	cases := []struct {
		name         string
		siblingLines []string
	}{
		{
			name:         "math block sibling",
			siblingLines: []string{"<<math>>", "x^2", "<<end>>"},
		},
		{
			name:         "special block sibling",
			siblingLines: []string{":::info", "Nota importante.", ":::"},
		},
		{
			name:         "directive sibling",
			siblingLines: []string{"@center"},
		},
		{
			name:         "markdown table sibling",
			siblingLines: []string{"| a | b |", "|---|---|", "| 1 | 2 |"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lines := []string{
				"QUOTE",
				"This is a strict mode quote",
			}
			lines = append(lines, tc.siblingLines...)

			ctx := &ParseContext{
				Mode:   "strict",
				Lines:  lines,
				Logger: logger,
			}

			result := parser.Parse(ctx, 0)

			if result.Error != nil {
				t.Fatalf("Parse() error = %v", result.Error)
			}

			const quoteLines = 2 // "QUOTE" + content line
			if result.ConsumedLines != quoteLines {
				t.Errorf("ConsumedLines = %d, want %d (sibling element must not be consumed)", result.ConsumedLines, quoteLines)
			}

			quote, ok := result.Element.(*ast.QuoteElement)
			if !ok {
				t.Fatalf("Parse() returned wrong element type: %T", result.Element)
			}
			if quote.Content != "This is a strict mode quote" {
				t.Errorf("Content = %q, want %q", quote.Content, "This is a strict mode quote")
			}
			for _, sib := range tc.siblingLines {
				if strings.Contains(quote.Content, strings.TrimSpace(sib)) {
					t.Errorf("quote content %q leaked sibling element text %q", quote.Content, sib)
				}
			}
		})
	}
}

func TestQuoteParser_ParseStrict(t *testing.T) {
	parser := &QuoteParser{}

	tests := []struct {
		name             string
		lines            []string
		expectedContent  string
		expectedAuthor   string
		expectedSource   string
		expectedConsumed int
	}{
		{
			name: "strict quote with metadata",
			lines: []string{
				"QUOTE",
				"This is a strict mode quote",
				"AUTHOR: John Doe",
				"SOURCE: Famous Speeches",
			},
			expectedContent:  "This is a strict mode quote",
			expectedAuthor:   "John Doe",
			expectedSource:   "Famous Speeches",
			expectedConsumed: 4,
		},
		{
			name: "strict quote without metadata",
			lines: []string{
				"QUOTE",
				"Simple quote without author",
			},
			expectedContent:  "Simple quote without author",
			expectedAuthor:   "",
			expectedSource:   "",
			expectedConsumed: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &ParseContext{
				Mode:   "strict",
				Lines:  tt.lines,
				Logger: util.NewNoop(),
			}

			result := parser.Parse(ctx, 0)

			if result.Error != nil {
				t.Errorf("Parse() error = %v", result.Error)
				return
			}

			if result.ConsumedLines != tt.expectedConsumed {
				t.Errorf("ConsumedLines = %v, expected %v", result.ConsumedLines, tt.expectedConsumed)
			}

			quote, ok := result.Element.(*ast.QuoteElement)
			if !ok {
				t.Errorf("Element is not a QuoteElement, got %T", result.Element)
				return
			}

			if quote.Content != tt.expectedContent {
				t.Errorf("Content = %q, expected %q", quote.Content, tt.expectedContent)
			}

			if quote.Author != tt.expectedAuthor {
				t.Errorf("Author = %q, expected %q", quote.Author, tt.expectedAuthor)
			}

			if quote.Source != tt.expectedSource {
				t.Errorf("Source = %q, expected %q", quote.Source, tt.expectedSource)
			}
		})
	}
}
