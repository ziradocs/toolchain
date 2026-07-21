// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"testing"
)

// TestIsJustASeparator cubre issue #57: la disambiguación "separador entre
// sub-bloques vs. cierre del bloque padre" extraída de GridParser.Parse a un
// helper reusable, para que un futuro elemento con sub-bloques repetidos
// (tabs, steps, accordion) no reimplemente el mismo lookahead a mano y
// reintroduzca el bug de doble-avance de issue #9.
func TestIsJustASeparator(t *testing.T) {
	tests := []struct {
		name     string
		lines    []string
		i        int
		expected bool
	}{
		{
			name: "another sub-block follows immediately: it's just a separator",
			lines: []string{
				"::: column",
				"content",
				":::", // i=2
				"::: column",
				"more content",
				":::",
			},
			i:        2,
			expected: true,
		},
		{
			name: "blank lines between separator and next sub-block don't confuse the lookahead",
			lines: []string{
				"::: column",
				"content",
				":::", // i=2
				"",
				"",
				"::: column",
				"more content",
				":::",
			},
			i:        2,
			expected: true,
		},
		{
			name: "closing marker follows: this was the real parent closing",
			lines: []string{
				"::: column",
				"content",
				":::", // i=2
				":::", // real grid closing
			},
			i:        2,
			expected: false,
		},
		{
			name: "block terminator (SLIDE) follows: this was the real parent closing",
			lines: []string{
				"::: column",
				"content",
				":::", // i=2
				"SLIDE next",
			},
			i:        2,
			expected: false,
		},
		{
			name: "end of file with nothing following: this was the real parent closing",
			lines: []string{
				"::: column",
				"content",
				":::", // i=2, last line
			},
			i:        2,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsJustASeparator(tt.lines, tt.i, "::: column", ":::", "SLIDE ")
			if got != tt.expected {
				t.Errorf("IsJustASeparator() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestIsNewElement(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		mode     string
		expected bool
	}{
		// Strict mode tests
		{
			name:     "strict mode - TEXT keyword",
			line:     "TEXT",
			mode:     "strict",
			expected: true,
		},
		{
			name:     "strict mode - CHECKLIST keyword",
			line:     "CHECKLIST",
			mode:     "strict",
			expected: true,
		},
		{
			name:     "strict mode - QUOTE keyword with whitespace",
			line:     "  QUOTE  ",
			mode:     "strict",
			expected: true,
		},
		{
			name:     "strict mode - non-keyword",
			line:     "Regular content",
			mode:     "strict",
			expected: false,
		},
		{
			name:     "strict mode - partial keyword",
			line:     "QUOTING something",
			mode:     "strict",
			expected: false,
		},

		// Flex mode tests
		{
			name:     "flex mode - header",
			line:     "# Header",
			mode:     "flex",
			expected: true,
		},
		{
			name:     "flex mode - list with dash",
			line:     "- List item",
			mode:     "flex",
			expected: true,
		},
		{
			name:     "flex mode - list with asterisk",
			line:     "* List item",
			mode:     "flex",
			expected: true,
		},
		{
			name:     "flex mode - list with plus",
			line:     "+ List item",
			mode:     "flex",
			expected: true,
		},
		{
			name:     "flex mode - code block",
			line:     "```python",
			mode:     "flex",
			expected: true,
		},
		{
			name:     "flex mode - image",
			line:     "![Alt text](image.png)",
			mode:     "flex",
			expected: true,
		},
		{
			name:     "flex mode - table",
			line:     "| Column 1 | Column 2 |",
			mode:     "flex",
			expected: true,
		},
		{
			name:     "flex mode - quote",
			line:     "> This is a quote",
			mode:     "flex",
			expected: true,
		},
		{
			name:     "flex mode - special block",
			line:     ":::info",
			mode:     "flex",
			expected: true,
		},
		{
			name:     "flex mode - numbered list",
			line:     "1. First item",
			mode:     "flex",
			expected: true,
		},
		{
			name:     "flex mode - numbered list with whitespace",
			line:     "  2. Second item  ",
			mode:     "flex",
			expected: true,
		},
		{
			name:     "flex mode - checklist unchecked",
			line:     "- [ ] Unchecked item",
			mode:     "flex",
			expected: true,
		},
		{
			name:     "flex mode - checklist checked",
			line:     "- [x] Checked item",
			mode:     "flex",
			expected: true,
		},
		{
			name:     "flex mode - checklist with asterisk",
			line:     "* [X] Checked with asterisk",
			mode:     "flex",
			expected: true,
		},
		{
			name:     "flex mode - checklist with plus",
			line:     "+ [ ] Unchecked with plus",
			mode:     "flex",
			expected: true,
		},
		{
			name:     "flex mode - regular text",
			line:     "Regular paragraph text",
			mode:     "flex",
			expected: false,
		},
		{
			name:     "flex mode - continuation text",
			line:     "More content for the current element",
			mode:     "flex",
			expected: false,
		},
		{
			name:     "flex mode - invalid checklist",
			line:     "- [invalid] Not a checklist",
			mode:     "flex",
			expected: false,
		},

		// Edge cases
		{
			name:     "empty line",
			line:     "",
			mode:     "flex",
			expected: false,
		},
		{
			name:     "whitespace only",
			line:     "   \t   ",
			mode:     "strict",
			expected: false,
		},
		{
			name:     "unknown mode",
			line:     "TEXT",
			mode:     "unknown",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsNewElement(tt.line, tt.mode)
			if result != tt.expected {
				t.Errorf("IsNewElement(%q, %q) = %v, expected %v", tt.line, tt.mode, result, tt.expected)
			}
		})
	}
}

// TestIsNewElement_StrictSymbolicMarkers cubre el mismo gap que motivó el fix
// de IMAGE ("strict IMAGE deja de tragarse elementos hermanos"): la rama
// strict de IsNewElement solo reconocía keywords en mayúsculas, no los
// marcadores simbólicos @ (directiva), ::: (special block/grid/code-group),
// << (math/mermaid/plantuml/chart/map) ni | (tabla Markdown) — con lo cual un
// CHECKLIST o QUOTE strict inmediatamente seguido de uno de estos elementos
// (sin línea en blanco de por medio) se lo tragaba como contenido propio en
// vez de cortar el loop de continuación.
func TestIsNewElement_StrictSymbolicMarkers(t *testing.T) {
	tests := []struct {
		name string
		line string
	}{
		{"directive", "@center"},
		{"special block", ":::info"},
		{"grid block", "<<grid>>"},
		{"math block", "<<math>>"},
		{"mermaid block", "<<mermaid>>"},
		{"chart block", "<<chart: bar>>"},
		{"markdown table", "| a | b |"},
		{"indented directive", "  @notes"},
		{"indented special block", "  ::: info"},
		{"indented math block", "  <<math>>"},
		{"indented markdown table", "  | a | b |"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !IsNewElement(tt.line, "strict") {
				t.Errorf("IsNewElement(%q, \"strict\") = false, expected true", tt.line)
			}
		})
	}
}

func TestIsNewElement_StrictKeywords(t *testing.T) {
	keywords := []string{
		"TEXT", "POINTS", "CODE", "IMAGE", "TABLE",
		"QUOTE", "CHECKLIST", "MERMAID", "CHART", "MAP",
		"DIRECTIVE", "SPECIAL_BLOCK", "CODE_GROUP",
	}

	for _, keyword := range keywords {
		t.Run(keyword, func(t *testing.T) {
			result := IsNewElement(keyword, "strict")
			if !result {
				t.Errorf("IsNewElement(%q, \"strict\") = false, expected true", keyword)
			}
		})
	}
}

func TestIsNewElement_FlexPatterns(t *testing.T) {
	patterns := []struct {
		pattern     string
		description string
	}{
		{"# ", "header"},
		{"- ", "list dash"},
		{"* ", "list asterisk"},
		{"+ ", "list plus"},
		{"```", "code block"},
		{"![", "image"},
		{"|", "table"},
		{"> ", "quote"},
		{":::", "special block"},
		{"- [x]", "checklist checked"},
		{"- [ ]", "checklist unchecked"},
		{"* [X]", "checklist asterisk"},
		{"+ [ ]", "checklist plus"},
	}

	for _, pattern := range patterns {
		t.Run(pattern.description, func(t *testing.T) {
			line := pattern.pattern + " content"
			result := IsNewElement(line, "flex")
			if !result {
				t.Errorf("IsNewElement(%q, \"flex\") = false, expected true for %s", line, pattern.description)
			}
		})
	}
}
