// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package structure

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMarkdownSlideStructureRule_Apply(t *testing.T) {
	rule := NewMarkdownSlideStructureRule()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "Document with existing separators - should not change",
			input: `---
title: Test Presentation
---

# Main Title
## Subtitle One
Content here

---

# Second Slide
## Another subtitle
More content`,
			expected: `---
title: Test Presentation
---

# Main Title
## Subtitle One
Content here

---

# Second Slide
## Another subtitle
More content`,
		},
		{
			name: "Single title with one subtitle - should not change",
			input: `---
title: Test
---

# Main Title
## Single Subtitle
Some content here`,
			expected: `---
title: Test
---

# Main Title
## Single Subtitle
Some content here`,
		},
		{
			name: "Multiple substantial sections without separators - should add separators",
			input: `---
title: Test
---

# Main Title
## First Section
This is substantial content for the first section.
It has multiple lines of content.
And more detailed information.
This should be a separate slide.

## Second Section
This is also substantial content.
With multiple lines and details.
Should also be a separate slide.
More content here.

## Third Section
Another substantial section.
With enough content to warrant.
Being its own slide.
Additional information.`,
			expected: `---
title: Test
---

# Main Title
## First Section
This is substantial content for the first section.
It has multiple lines of content.
And more detailed information.
This should be a separate slide.

---

## Second Section
This is also substantial content.
With multiple lines and details.
Should also be a separate slide.
More content here.

---

## Third Section
Another substantial section.
With enough content to warrant.
Being its own slide.
Additional information.`,
		},
		{
			name: "Multiple short sections - should not change",
			input: `---
title: Test
---

# Main Title
## Section One
Short content

## Section Two
Also short

## Section Three
Brief`,
			expected: `---
title: Test
---

# Main Title
## Section One
Short content

## Section Two
Also short

## Section Three
Brief`,
		},
		{
			name: "No frontmatter, multiple substantial sections",
			input: `# Main Title
## First Section
This is substantial content for the first section.
It has multiple lines of content.
And more detailed information.
This should be a separate slide.

## Second Section
This is also substantial content.
With multiple lines and details.
Should also be a separate slide.
More content here.`,
			expected: `# Main Title
## First Section
This is substantial content for the first section.
It has multiple lines of content.
And more detailed information.
This should be a separate slide.

---

## Second Section
This is also substantial content.
With multiple lines and details.
Should also be a separate slide.
More content here.`,
		},
		{
			name: "No h1 title - should not change",
			input: `## First Section
Some content here

## Second Section  
More content here`,
			expected: `## First Section
Some content here

## Second Section  
More content here`,
		},
		{
			name: "Mixed content with existing separators - should not change",
			input: `# Main Title
## First Section
Content

---

## Second Section
More content`,
			expected: `# Main Title
## First Section
Content

---

## Second Section
More content`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := rule.Apply(tt.input)
			if err != nil {
				t.Errorf("MarkdownSlideStructureRule.Apply() error = %v", err)
				return
			}
			if result != tt.expected {
				t.Errorf("MarkdownSlideStructureRule.Apply() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestMarkdownSlideStructureRule_HasExistingSeparators(t *testing.T) {
	rule := NewMarkdownSlideStructureRule()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name: "Has triple dash separator",
			input: `---
title: Test
---

# Title
Content

---

# Another slide`,
			expected: true,
		},
		{
			name: "Has triple asterisk separator",
			input: `# Title
Content

***

# Another slide`,
			expected: true,
		},
		{
			name: "Has triple underscore separator",
			input: `# Title
Content

___

# Another slide`,
			expected: true,
		},
		{
			name: "No separators",
			input: `# Title
Content
## Subtitle
More content`,
			expected: false,
		},
		{
			name: "Only frontmatter separator",
			input: `---
title: Test
---

# Title
Content
## Subtitle`,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := strings.Split(tt.input, "\n")
			result := rule.hasExistingSeparators(lines)
			if result != tt.expected {
				t.Errorf("hasExistingSeparators() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestMarkdownSlideStructureRule_HasProblematicPattern(t *testing.T) {
	rule := NewMarkdownSlideStructureRule()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name: "Multiple substantial sections without separators",
			input: `# Main Title
## First Section
This is substantial content.
Multiple lines here.
More content.
Should be separate slide.

## Second Section
Also substantial content.
Multiple lines here too.
More detailed info.
Another separate slide.`,
			expected: true,
		},
		{
			name: "Short sections - not problematic",
			input: `# Main Title
## First Section
Short

## Second Section
Also short`,
			expected: false,
		},
		{
			name: "Only one h2 section",
			input: `# Main Title
## Only Section
Even if substantial content.
Multiple lines.
Still only one section.`,
			expected: false,
		},
		{
			name: "No h1 title",
			input: `## First Section
Content here

## Second Section
More content`,
			expected: false,
		},
		{
			name: "Already has separators between sections",
			input: `# Main Title
## First Section
Substantial content.
Multiple lines.

---

## Second Section
Also substantial.
Multiple lines too.`,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := strings.Split(tt.input, "\n")
			result := rule.hasProblematicPattern(lines)
			if result != tt.expected {
				t.Errorf("hasProblematicPattern() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestMarkdownSlideStructureRule_Metadata(t *testing.T) {
	rule := NewMarkdownSlideStructureRule()

	if rule.Priority() != 2 {
		t.Errorf("Priority() = %v, want %v", rule.Priority(), 2)
	}

	expectedDesc := "Corrige estructura markdown con un # y múltiples ## convirtiéndolos en slides independientes"
	if rule.Description() != expectedDesc {
		t.Errorf("Description() = %v, want %v", rule.Description(), expectedDesc)
	}
}

// TestMarkdownSlideStructureRule_Integration prueba la regla con archivos reales usando testdata
func TestMarkdownSlideStructureRule_Integration(t *testing.T) {
	rule := NewMarkdownSlideStructureRule()

	// Leer archivo de entrada
	inputPath := filepath.Join("testdata", "markdown_slides_input.slidelang")
	inputContent, err := os.ReadFile(inputPath)
	if err != nil {
		t.Skipf("Skipping integration test - input file not found: %v", err)
		return
	}

	// Leer archivo esperado
	expectedPath := filepath.Join("testdata", "markdown_slides_expected.slidelang")
	expectedContent, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Skipf("Skipping integration test - expected file not found: %v", err)
		return
	}

	// Aplicar la regla
	result, err := rule.Apply(string(inputContent))
	if err != nil {
		t.Fatalf("MarkdownSlideStructureRule.Apply() error = %v", err)
	}

	// Comparar resultado
	if result != string(expectedContent) {
		t.Errorf("Integration test failed. Got:\n%s\n\nExpected:\n%s", result, string(expectedContent))

		// Crear archivo de salida para debugging
		outputPath := filepath.Join("testdata", "markdown_slides_actual.slidelang")
		if writeErr := os.WriteFile(outputPath, []byte(result), 0644); writeErr == nil {
			t.Logf("Actual output written to: %s", outputPath)
		}
	}
}

// BenchmarkMarkdownSlideStructureRule_Apply benchmarks para medir performance
func BenchmarkMarkdownSlideStructureRule_Apply(b *testing.B) {
	rule := NewMarkdownSlideStructureRule()

	// Contenido de prueba con múltiples secciones
	content := `---
title: Test Presentation
---

# Main Title
## First Section
This is substantial content for the first section.
It has multiple lines of content.
And more detailed information.
This should be a separate slide.

## Second Section
This is also substantial content.
With multiple lines and details.
Should also be a separate slide.
More content here.

## Third Section
Another substantial section.
With enough content to warrant.
Being its own slide.
Additional information.`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := rule.Apply(content)
		if err != nil {
			b.Fatalf("Error in benchmark: %v", err)
		}
	}
}
