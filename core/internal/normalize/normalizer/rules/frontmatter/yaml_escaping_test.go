// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package frontmatter

import (
	"strings"
	"testing"
)

func TestYamlEscapingRule_Apply(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "Escape title with colon",
			input: `---
title: InventIA: Gestión Inteligente de Inventarios para PyMEs
author: Equipo InventIA
mode: flex
---

# Content here`,
			expected: `---
title: "InventIA: Gestión Inteligente de Inventarios para PyMEs"
author: Equipo InventIA
mode: flex
---

# Content here`,
		},
		{
			name: "Multiple values with colons",
			input: `---
title: App: Sistema de Gestión
subtitle: Subtitle: Con más información
author: Team Lead: Development
mode: flex
---

# Content`,
			expected: `---
title: "App: Sistema de Gestión"
subtitle: "Subtitle: Con más información"
author: "Team Lead: Development"
mode: flex
---

# Content`,
		},
		{
			name: "Already quoted values should not change",
			input: `---
title: "Already: Quoted"
author: 'Single: Quoted'
mode: flex
---

# Content`,
			expected: `---
title: "Already: Quoted"
author: 'Single: Quoted'
mode: flex
---

# Content`,
		},
		{
			name: "Values with brackets and special chars",
			input: `---
title: System [Core]: Management Tool
description: Tool for {advanced} functionality
tags: ["tag1", "tag2"]
mode: flex
---

# Content`,
			expected: `---
title: "System [Core]: Management Tool"
description: "Tool for {advanced} functionality"
tags: "["tag1", "tag2"]"
mode: flex
---

# Content`,
		},
		{
			name: "No frontmatter should return unchanged",
			input: `# Just a regular markdown file

No frontmatter here.`,
			expected: `# Just a regular markdown file

No frontmatter here.`,
		},
		{
			name: "No problematic values should return unchanged",
			input: `---
title: Simple Title
author: John Doe
mode: flex
---

# Content`,
			expected: `---
title: Simple Title
author: John Doe
mode: flex
---

# Content`,
		},
		{
			name: "Complex value with multiple special chars",
			input: `---
title: API & Database: Complete Guide [2024]
description: Learn about API development & database design
tags: React.js, Node.js & MongoDB
mode: flex
---

# Content`,
			expected: `---
title: "API & Database: Complete Guide [2024]"
description: "Learn about API development & database design"
tags: "React.js, Node.js & MongoDB"
mode: flex
---

# Content`,
		},
		{
			name: "Indented frontmatter values",
			input: `---
  title: InventIA: Sistema Avanzado
  nested:
    subtitle: Sub: System
    description: Tool [Advanced]: Features
mode: flex
---

# Content`,
			expected: `---
  title: "InventIA: Sistema Avanzado"
  nested:
    subtitle: "Sub: System"
    description: "Tool [Advanced]: Features"
mode: flex
---

# Content`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := NewYamlEscapingRule()
			result, err := rule.Apply(tt.input)
			if err != nil {
				t.Errorf("YamlEscapingRule.Apply() error = %v", err)
				return
			}
			if result != tt.expected {
				t.Errorf("YamlEscapingRule.Apply() = %v, want %v", result, tt.expected)
				// Debug output
				t.Logf("Input lines:")
				for i, line := range strings.Split(tt.input, "\n") {
					t.Logf("  %d: %q", i, line)
				}
				t.Logf("Expected lines:")
				for i, line := range strings.Split(tt.expected, "\n") {
					t.Logf("  %d: %q", i, line)
				}
				t.Logf("Got lines:")
				for i, line := range strings.Split(result, "\n") {
					t.Logf("  %d: %q", i, line)
				}
			}
		})
	}
}

func TestYamlEscapingRule_EscapeYamlValue(t *testing.T) {
	rule := NewYamlEscapingRule()

	tests := []struct {
		name         string
		input        string
		expectedText string
		expectedMod  bool
	}{
		{
			name:         "Simple key value with colon",
			input:        "title: InventIA: System",
			expectedText: `title: "InventIA: System"`,
			expectedMod:  true,
		},
		{
			name:         "Already quoted",
			input:        `title: "Already: Quoted"`,
			expectedText: `title: "Already: Quoted"`,
			expectedMod:  false,
		},
		{
			name:         "No special characters",
			input:        "title: Simple Title",
			expectedText: "title: Simple Title",
			expectedMod:  false,
		},
		{
			name:         "Indented key value",
			input:        "  title: System: Core",
			expectedText: `  title: "System: Core"`,
			expectedMod:  true,
		},
		{
			name:         "Not a key value line",
			input:        "Just some text: not yaml",
			expectedText: "Just some text: not yaml",
			expectedMod:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resultText, resultMod := rule.escapeYamlValue(tt.input)
			if resultText != tt.expectedText {
				t.Errorf("escapeYamlValue() text = %v, want %v", resultText, tt.expectedText)
			}
			if resultMod != tt.expectedMod {
				t.Errorf("escapeYamlValue() modified = %v, want %v", resultMod, tt.expectedMod)
			}
		})
	}
}

func TestYamlEscapingRule_NeedsEscaping(t *testing.T) {
	rule := NewYamlEscapingRule()

	tests := []struct {
		name     string
		value    string
		expected bool
	}{
		{"Value with colon", "InventIA: System", true},
		{"Value with brackets", "System [Core]", true},
		{"Value with braces", "Tool {advanced}", true},
		{"Value with hash", "Version #1", true},
		{"Value with ampersand", "API & Database", true},
		{"Value with asterisk", "Config * Settings", true},
		{"Value with pipe", "Option | Choice", true},
		{"Value with backticks", "Code `example`", true},
		{"Simple value", "Simple Title", false},
		{"Numbers only", "123", false},
		{"Already quoted", `"Quoted: Value"`, false}, // This won't reach needsEscaping if isAlreadyQuoted works
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rule.needsEscaping(tt.value)
			if result != tt.expected {
				t.Errorf("needsEscaping(%q) = %v, want %v", tt.value, result, tt.expected)
			}
		})
	}
}

func TestYamlEscapingRule_IsAlreadyQuoted(t *testing.T) {
	rule := NewYamlEscapingRule()

	tests := []struct {
		name     string
		value    string
		expected bool
	}{
		{"Double quoted", `"Quoted Value"`, true},
		{"Single quoted", `'Quoted Value'`, true},
		{"Not quoted", "Unquoted Value", false},
		{"Partial quote start", `"Incomplete`, false},
		{"Partial quote end", `Incomplete"`, false},
		{"Mixed quotes", `"Mixed'`, false},
		{"Empty quotes", `""`, true},
		{"Empty single quotes", `''`, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rule.isAlreadyQuoted(tt.value)
			if result != tt.expected {
				t.Errorf("isAlreadyQuoted(%q) = %v, want %v", tt.value, result, tt.expected)
			}
		})
	}
}

func TestYamlEscapingRule_Metadata(t *testing.T) {
	rule := NewYamlEscapingRule()

	if rule.Priority() != 2 {
		t.Errorf("Priority() = %v, want %v", rule.Priority(), 2)
	}

	expectedDesc := "Escapa valores YAML que contienen caracteres especiales (dos puntos, corchetes, etc.) para evitar errores de parsing"
	if rule.Description() != expectedDesc {
		t.Errorf("Description() = %v, want %v", rule.Description(), expectedDesc)
	}
}
