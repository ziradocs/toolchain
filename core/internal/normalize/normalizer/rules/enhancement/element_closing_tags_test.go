// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package enhancement

import (
	"testing"
)

func TestElementClosingTagsRule_Apply(t *testing.T) {
	rule := NewElementClosingTagsRule()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "chart with >> closing",
			input: `<<chart
type: bar
title: "Test"
data: [1, 2, 3]
>>`,
			expected: `<<chart
type: bar
title: "Test"
data: [1, 2, 3]
<</chart>>`,
		},
		{
			name: "map with >> closing",
			input: `<<map
type: world
center: [0, 0]
>>`,
			expected: `<<map
type: world
center: [0, 0]
<</map>>`,
		},
		{
			name: "plantuml with >> closing",
			input: `<<plantuml
@startuml
A -> B
@enduml
>>`,
			expected: `<<plantuml
@startuml
A -> B
@enduml
<</plantuml>>`,
		},
		{
			name:     "chart inline (no changes)",
			input:    `<<chart: bar>>`,
			expected: `<<chart: bar>>`,
		},
		{
			name: "chart with correct closing (no changes)",
			input: `<<chart
type: bar
<</chart>>`,
			expected: `<<chart
type: bar
<</chart>>`,
		},
		{
			name: "multiple elements",
			input: `<<chart
type: bar
>>

Some text

<<map
type: world
>>`,
			expected: `<<chart
type: bar
<</chart>>

Some text

<<map
type: world
<</map>>`,
		},
		{
			name: "mixed correct and incorrect",
			input: `<<chart
type: bar
<</chart>>

<<map
type: world
>>`,
			expected: `<<chart
type: bar
<</chart>>

<<map
type: world
<</map>>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := rule.Apply(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("expected:\n%s\n\ngot:\n%s", tt.expected, result)
			}
		})
	}
}

func TestElementClosingTagsRule_Metadata(t *testing.T) {
	rule := NewElementClosingTagsRule()

	if rule.Name() != "ElementClosingTagsRule" {
		t.Errorf("expected name 'ElementClosingTagsRule', got '%s'", rule.Name())
	}

	if rule.Priority() != 1 {
		t.Errorf("expected priority 1, got %d", rule.Priority())
	}

	desc := rule.Description()
	if desc == "" {
		t.Error("description should not be empty")
	}
}
