// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package enhancement

import (
	"testing"
)

func TestMapFormatterRule_Apply(t *testing.T) {
	rule := NewMapFormatterRule()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "Format map without indentation",
			input: `## Target Market

<<map>>
type: region
center: [10, -75]
markers:
- lat: 19.4326
lng: -99.1332
label: "Mexico"
value: 60
- lat: 4.7110
lng: -74.0721
label: "Colombia"
value: 30
zoom: 3

---`,
			expected: `## Target Market

<<map>>
  type: region
  center: [10, -75]
  markers:
  - lat: 19.4326
    lng: -99.1332
    label: "Mexico"
    value: 60
  - lat: 4.7110
    lng: -74.0721
    label: "Colombia"
    value: 30
  zoom: 3

---`,
		},
		{
			name: "Format world map with heatmap",
			input: `## Global Presence

<<map>>
type: world
markers:
- lat: 40.7128
lng: -74.0060
label: "New York"
value: 45
- lat: 51.5074
lng: -0.1278
label: "London"
value: 38
heatmap: true
zoom: 2

Next content`,
			expected: `## Global Presence

<<map>>
  type: world
  markers:
  - lat: 40.7128
    lng: -74.0060
    label: "New York"
    value: 45
  - lat: 51.5074
    lng: -0.1278
    label: "London"
    value: 38
  heatmap: true
  zoom: 2

Next content`,
		},
		{
			name: "Already formatted content (no changes)",
			input: `## Regional Offices

<<map>>
  type: region
  center: [39.8283, -98.5795]
  markers:
    - lat: 37.7749
      lng: -122.4194
      label: "San Francisco"
      value: 100
  zoom: 4

Content after`,
			expected: `## Regional Offices

<<map>>
  type: region
  center: [39.8283, -98.5795]
  markers:
    - lat: 37.7749
      lng: -122.4194
      label: "San Francisco"
      value: 100
  zoom: 4

Content after`,
		},
		{
			name: "Multiple map blocks",
			input: `## Map 1

<<map>>
type: world
markers:
- lat: 40.7128
lng: -74.0060
label: "New York"
zoom: 3

## Map 2

<<map>>
type: region
center: [0, 0]
markers:
- lat: 51.5074
lng: -0.1278
label: "London"
value: 50
zoom: 5

Final content`,
			expected: `## Map 1

<<map>>
  type: world
  markers:
  - lat: 40.7128
    lng: -74.0060
    label: "New York"
  zoom: 3

## Map 2

<<map>>
  type: region
  center: [0, 0]
  markers:
  - lat: 51.5074
    lng: -0.1278
    label: "London"
    value: 50
  zoom: 5

Final content`,
		},
		{
			name: "Empty map block",
			input: `## Empty Map

<<map>>

Content after`,
			expected: `## Empty Map

<<map>>

Content after`,
		},
		{
			name: "Map with only type and zoom",
			input: `## Simple Map

<<map>>
type: world
zoom: 2

Next section`,
			expected: `## Simple Map

<<map>>
  type: world
  zoom: 2

Next section`,
		},
		{
			name: "Map with special characters and quotes",
			input: `## International Offices

<<map>>
type: region
center: [20, 0]
markers:
- lat: 48.8566
lng: 2.3522
label: "Paris, France"
value: 75
- lat: 35.6762
lng: 139.6503
label: "Tokyo (東京)"
value: 80
zoom: 4

Text after`,
			expected: `## International Offices

<<map>>
  type: region
  center: [20, 0]
  markers:
  - lat: 48.8566
    lng: 2.3522
    label: "Paris, France"
    value: 75
  - lat: 35.6762
    lng: 139.6503
    label: "Tokyo (東京)"
    value: 80
  zoom: 4

Text after`,
		},
		{
			name: "No map blocks",
			input: `## Regular content

This is just regular content without map blocks.

### Another section

Some more content here.`,
			expected: `## Regular content

This is just regular content without map blocks.

### Another section

Some more content here.`,
		},
		{
			name: "Map with mixed proper and improper indentation",
			input: `## Mixed indentation

<<map>>
  type: region
center: [10, -20]
  markers:
- lat: 19.4326
  lng: -99.1332
    label: "Mexico"
  value: 60
zoom: 3

Content after`,
			expected: `## Mixed indentation

<<map>>
  type: region
  center: [10, -20]
  markers:
  - lat: 19.4326
    lng: -99.1332
    label: "Mexico"
    value: 60
  zoom: 3

Content after`,
		},
		{
			name: "Map block ending with >>",
			input: `## Map with explicit end

<<map>>
type: world
markers:
- lat: 0
lng: 0
label: "Origin"
>>

Next content`,
			expected: `## Map with explicit end

<<map>>
  type: world
  markers:
  - lat: 0
    lng: 0
    label: "Origin"
>>

Next content`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := rule.Apply(tt.input)
			if err != nil {
				t.Errorf("MapFormatterRule.Apply() error = %v", err)
				return
			}
			if result != tt.expected {
				t.Errorf("MapFormatterRule.Apply() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestMapFormatterRule_FormatMapData(t *testing.T) {
	rule := NewMapFormatterRule()

	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name: "Simple map with markers",
			input: []string{
				"type: world",
				"markers:",
				"- lat: 40.7128",
				"lng: -74.0060",
				"label: \"New York\"",
				"zoom: 3",
			},
			expected: []string{
				"  type: world",
				"  markers:",
				"  - lat: 40.7128",
				"    lng: -74.0060",
				"    label: \"New York\"",
				"  zoom: 3",
			},
		},
		{
			name: "Map with multiple markers",
			input: []string{
				"type: region",
				"center: [10, -75]",
				"markers:",
				"- lat: 19.4326",
				"lng: -99.1332",
				"label: \"Mexico\"",
				"value: 60",
				"- lat: 4.7110",
				"lng: -74.0721",
				"label: \"Colombia\"",
				"value: 30",
				"zoom: 3",
			},
			expected: []string{
				"  type: region",
				"  center: [10, -75]",
				"  markers:",
				"  - lat: 19.4326",
				"    lng: -99.1332",
				"    label: \"Mexico\"",
				"    value: 60",
				"  - lat: 4.7110",
				"    lng: -74.0721",
				"    label: \"Colombia\"",
				"    value: 30",
				"  zoom: 3",
			},
		},
		{
			name: "Map with heatmap",
			input: []string{
				"type: world",
				"heatmap: true",
				"markers:",
				"- lat: 51.5074",
				"lng: -0.1278",
				"label: \"London\"",
				"zoom: 2",
			},
			expected: []string{
				"  type: world",
				"  heatmap: true",
				"  markers:",
				"  - lat: 51.5074",
				"    lng: -0.1278",
				"    label: \"London\"",
				"  zoom: 2",
			},
		},
		{
			name: "Empty lines handling",
			input: []string{
				"type: world",
				"",
				"markers:",
				"- lat: 0",
				"lng: 0",
				"",
				"zoom: 1",
			},
			expected: []string{
				"  type: world",
				"",
				"  markers:",
				"  - lat: 0",
				"    lng: 0",
				"",
				"  zoom: 1",
			},
		},
		{
			name: "Marker without dash prefix",
			input: []string{
				"type: region",
				"markers:",
				"lat: 40.7128",
				"lng: -74.0060",
				"label: \"New York\"",
			},
			expected: []string{
				"  type: region",
				"  markers:",
				"  - lat: 40.7128",
				"    lng: -74.0060",
				"    label: \"New York\"",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rule.formatMapData(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("formatMapData() length = %v, want %v", len(result), len(tt.expected))
				return
			}
			for i, line := range result {
				if line != tt.expected[i] {
					t.Errorf("formatMapData() line %d = %q, want %q", i, line, tt.expected[i])
				}
			}
		})
	}
}

func TestMapFormatterRule_NeedsFormatting(t *testing.T) {
	rule := NewMapFormatterRule()

	tests := []struct {
		name     string
		input    []string
		expected bool
	}{
		{
			name: "Needs formatting - no indentation",
			input: []string{
				"type: world",
				"markers:",
				"- lat: 40.7128",
				"lng: -74.0060",
			},
			expected: true,
		},
		{
			name: "Needs formatting - mixed indentation",
			input: []string{
				"  type: world",
				"markers:",
				"  - lat: 40.7128",
				"lng: -74.0060",
			},
			expected: true,
		},
		{
			name: "Needs formatting - incorrect marker indentation",
			input: []string{
				"  type: world",
				"  markers:",
				"- lat: 40.7128",
				"  lng: -74.0060",
			},
			expected: true,
		},
		{
			name: "Needs formatting - incorrect property indentation",
			input: []string{
				"  type: world",
				"  markers:",
				"  - lat: 40.7128",
				"lng: -74.0060",
			},
			expected: true,
		},
		{
			name: "No formatting needed - correct indentation",
			input: []string{
				"  type: world",
				"  markers:",
				"  - lat: 40.7128",
				"    lng: -74.0060",
				"    label: \"New York\"",
				"  zoom: 3",
			},
			expected: false,
		},
		{
			name:     "No formatting needed - empty content",
			input:    []string{},
			expected: false,
		},
		{
			name: "No formatting needed - only whitespace",
			input: []string{
				"   ",
				"",
				"   ",
			},
			expected: false,
		},
		{
			name: "No formatting needed - only non-map content",
			input: []string{
				"some random content",
				"not a map property",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rule.needsFormatting(tt.input)
			if result != tt.expected {
				t.Errorf("needsFormatting() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestMapFormatterRule_IsMapDataLine(t *testing.T) {
	rule := NewMapFormatterRule()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		// Map properties
		{"type property", "type: world", true},
		{"center property", "center: [10, -75]", true},
		{"zoom property", "zoom: 3", true},
		{"markers property", "markers:", true},
		{"heatmap property", "heatmap: true", true},

		// Marker properties
		{"lat with dash", "- lat: 40.7128", true},
		{"lat without dash", "lat: 40.7128", true},
		{"lng property", "lng: -74.0060", true},
		{"label property", "label: \"New York\"", true},
		{"value property", "value: 60", true},
		{"icon property", "icon: custom-icon.png", true},

		// Indented lines
		{"indented content", "  some indented content", true},
		{"tab indented", "\tsome tab indented", true},

		// List items
		{"dash list item", "- some list item", true},

		// Non-map lines
		{"regular text", "This is regular text", false},
		{"markdown header", "## Header", false},
		{"separator", "---", false},
		{"empty line", "", false},
		{"other slidelang block", "<<chart>>", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rule.isMapDataLine(tt.input)
			if result != tt.expected {
				t.Errorf("isMapDataLine(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestMapFormatterRule_IsEndOfMapBlock(t *testing.T) {
	rule := NewMapFormatterRule()

	tests := []struct {
		name     string
		trimmed  string
		lines    []string
		index    int
		expected bool
	}{
		{
			name:     "Explicit end with >>",
			trimmed:  ">>",
			lines:    []string{"<<map>>", "type: world", ">>"},
			index:    2,
			expected: true,
		},
		{
			name:     "Markdown separator",
			trimmed:  "---",
			lines:    []string{"<<map>>", "type: world", "---"},
			index:    2,
			expected: true,
		},
		{
			name:     "Markdown header",
			trimmed:  "## New Section",
			lines:    []string{"<<map>>", "type: world", "## New Section"},
			index:    2,
			expected: true,
		},
		{
			name:     "Another SlideLang block",
			trimmed:  "<<chart>>",
			lines:    []string{"<<map>>", "type: world", "<<chart>>"},
			index:    2,
			expected: true,
		},
		{
			name:     "Empty line followed by non-map content",
			trimmed:  "",
			lines:    []string{"<<map>>", "type: world", "", "Regular content"},
			index:    2,
			expected: true,
		},
		{
			name:     "Empty line followed by map content",
			trimmed:  "",
			lines:    []string{"<<map>>", "type: world", "", "zoom: 3"},
			index:    2,
			expected: false,
		},
		{
			name:     "Map property line",
			trimmed:  "zoom: 3",
			lines:    []string{"<<map>>", "type: world", "zoom: 3"},
			index:    2,
			expected: false,
		},
		{
			name:     "Empty line at end of file",
			trimmed:  "",
			lines:    []string{"<<map>>", "type: world", ""},
			index:    2,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rule.isEndOfMapBlock(tt.trimmed, tt.lines, tt.index)
			if result != tt.expected {
				t.Errorf("isEndOfMapBlock(%q) = %v, want %v", tt.trimmed, result, tt.expected)
			}
		})
	}
}

func TestMapFormatterRule_Metadata(t *testing.T) {
	rule := NewMapFormatterRule()

	if rule.Priority() != 5 {
		t.Errorf("Priority() = %v, want %v", rule.Priority(), 5)
	}

	expectedDesc := "Formats map blocks with proper YAML indentation"
	if rule.Description() != expectedDesc {
		t.Errorf("Description() = %v, want %v", rule.Description(), expectedDesc)
	}

	expectedName := "MapFormatter"
	if rule.GetName() != expectedName {
		t.Errorf("GetName() = %v, want %v", rule.GetName(), expectedName)
	}
}

// TestMapFormatterRule_Integration prueba la regla con el archivo real e.slidelang
func TestMapFormatterRule_Integration(t *testing.T) {
	rule := NewMapFormatterRule()

	// Contenido simulando el archivo e.slidelang problemático original
	eSlidelangContent := `## Target Market
- 25M SMBs in LATAM
- TAM: $50B USD (global)
- SAM: $3B (digitally accessible LATAM)
- SOM: $150M within 5 years

<<map>>
type: region
center: [10, -75]
markers:

lat: 19.4326
lng: -99.1332
label: "Mexico"
value: 60
lat: 4.7110
lng: -74.0721
label: "Colombia"
value: 30
lat: -33.4489
lng: -70.6693
label: "Chile"
value: 10
zoom: 3

---

## Business Model`

	expected := `## Target Market
- 25M SMBs in LATAM
- TAM: $50B USD (global)
- SAM: $3B (digitally accessible LATAM)
- SOM: $150M within 5 years

<<map>>
  type: region
  center: [10, -75]
  markers:
  - lat: 19.4326
    lng: -99.1332
    label: "Mexico"
    value: 60
  - lat: 4.7110
    lng: -74.0721
    label: "Colombia"
    value: 30
  - lat: -33.4489
    lng: -70.6693
    label: "Chile"
    value: 10
  zoom: 3

---

## Business Model`

	// Aplicar la regla
	result, err := rule.Apply(eSlidelangContent)
	if err != nil {
		t.Fatalf("MapFormatterRule.Apply() error = %v", err)
	}

	// Comparar resultado
	if result != expected {
		t.Errorf("Integration test failed. Got:\n%s\n\nExpected:\n%s", result, expected)
	}
}

// BenchmarkMapFormatterRule_Apply benchmarks para medir performance
func BenchmarkMapFormatterRule_Apply(b *testing.B) {
	rule := NewMapFormatterRule()

	// Contenido de prueba con múltiples bloques map sin formatear
	content := `## Map 1

<<map>>
type: world
markers:
- lat: 40.7128
lng: -74.0060
label: "New York"
value: 45
zoom: 2

## Map 2

<<map>>
type: region
center: [20, 0]
markers:
- lat: 51.5074
lng: -0.1278
label: "London"
value: 38
- lat: 48.8566
lng: 2.3522
label: "Paris"
value: 42
heatmap: true
zoom: 4

## Map 3

<<map>>
type: region
center: [10, -75]
markers:
- lat: 19.4326
lng: -99.1332
label: "Mexico"
- lat: 4.7110
lng: -74.0721
label: "Colombia"
zoom: 3

Regular content here.`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := rule.Apply(content)
		if err != nil {
			b.Fatalf("Error in benchmark: %v", err)
		}
	}
}

// TestMapFormatterRule_EdgeCases prueba casos edge específicos
func TestMapFormatterRule_EdgeCases(t *testing.T) {
	rule := NewMapFormatterRule()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "Map block at end of file without newline",
			input: `<<map>>
type: world
zoom: 2`,
			expected: `<<map>>
  type: world
  zoom: 2`,
		},
		{
			name: "Map with only empty markers section",
			input: `<<map>>
type: world
markers:
zoom: 2`,
			expected: `<<map>>
  type: world
  markers:
  zoom: 2`,
		},
		{
			name: "Map with malformed marker missing properties",
			input: `<<map>>
type: region
markers:
- lat: 40.7128
- lat: 51.5074
lng: -0.1278
zoom: 3`,
			expected: `<<map>>
  type: region
  markers:
  - lat: 40.7128
  - lat: 51.5074
    lng: -0.1278
  zoom: 3`,
		},
		{
			name: "Map with unrecognized properties (should pass through)",
			input: `<<map>>
type: custom
customProperty: value
markers:
- lat: 0
lng: 0
unknownProp: test`,
			expected: `<<map>>
  type: custom
  customProperty: value
  markers:
  - lat: 0
    lng: 0
  unknownProp: test`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := rule.Apply(tt.input)
			if err != nil {
				t.Errorf("MapFormatterRule.Apply() error = %v", err)
				return
			}
			if result != tt.expected {
				t.Errorf("MapFormatterRule.Apply() = %q, want %q", result, tt.expected)
			}
		})
	}
}
