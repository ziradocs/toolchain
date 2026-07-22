// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package enhancement

import (
	"testing"

	"go.ziradocs.com/core/v2/internal/normalize/normalizer/base"
)

func TestChartFormatterRule_Apply(t *testing.T) {
	rule := NewChartFormatterRule()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "Format chart with no indentation",
			input: `## Análisis de ventas

<<chart:bar title="Ventas por trimestre">>
title: Ventas Trimestrales 2024
labels: ["Q1", "Q2", "Q3", "Q4"]
datasets:
data: [120, 190, 300, 250]
backgroundColor: ["#FF6384", "#36A2EB", "#FFCE56", "#4BC0C0"]
borderColor: "#333"
borderWidth: 1

>>`,
			expected: `## Análisis de ventas

<<chart:bar title="Ventas por trimestre">>
  title: Ventas Trimestrales 2024
  labels: ["Q1", "Q2", "Q3", "Q4"]
  datasets:
    data: [120, 190, 300, 250]
    backgroundColor: ["#FF6384", "#36A2EB", "#FFCE56", "#4BC0C0"]
    borderColor: "#333"
    borderWidth: 1

>>`,
		},
		{
			name: "Format line chart with mixed indentation",
			input: `## Performance metrics

<<chart:line>>
title: Website Performance
labels: ["Jan", "Feb", "Mar", "Apr"]
datasets:
  data: [65, 59, 80, 81]
label: "Page Views"
  backgroundColor: "rgba(75,192,192,0.2)"
borderColor: "rgba(75,192,192,1)"
  borderWidth: 2
>>`,
			expected: `## Performance metrics

<<chart:line>>
  title: Website Performance
  labels: ["Jan", "Feb", "Mar", "Apr"]
  datasets:
    data: [65, 59, 80, 81]
    label: "Page Views"
    backgroundColor: "rgba(75,192,192,0.2)"
    borderColor: "rgba(75,192,192,1)"
    borderWidth: 2
>>`,
		},
		{
			name: "Format pie chart without closing tag",
			input: `## Market share

<<chart:pie>>
title: Market Share 2024
labels: ["Product A", "Product B", "Product C"]
datasets:
data: [300, 150, 100]
backgroundColor: ["#FF6384", "#36A2EB", "#FFCE56"]

## Next section`,
			expected: `## Market share

<<chart:pie>>
  title: Market Share 2024
  labels: ["Product A", "Product B", "Product C"]
  datasets:
    data: [300, 150, 100]
    backgroundColor: ["#FF6384", "#36A2EB", "#FFCE56"]

## Next section`,
		},
		{
			name: "Multiple datasets in chart",
			input: `<<chart:bar>>
title: Comparison Chart
labels: ["A", "B", "C"]
datasets:
data: [10, 20, 30]
label: "Series 1"
backgroundColor: "#FF6384"
datasets:
data: [15, 25, 35]
label: "Series 2"
backgroundColor: "#36A2EB"
>>`,
			expected: `<<chart:bar>>
  title: Comparison Chart
  labels: ["A", "B", "C"]
  datasets:
    data: [10, 20, 30]
    label: "Series 1"
    backgroundColor: "#FF6384"
  datasets:
    data: [15, 25, 35]
    label: "Series 2"
    backgroundColor: "#36A2EB"
>>`,
		},
		{
			name: "Chart with empty lines and comments",
			input: `<<chart:doughnut>>
title: Revenue Breakdown

labels: ["Sales", "Marketing", "Operations"]
datasets:

data: [500000, 200000, 150000]
backgroundColor: ["#FF6384", "#36A2EB", "#FFCE56"]

borderWidth: 2
>>`,
			expected: `<<chart:doughnut>>
  title: Revenue Breakdown

  labels: ["Sales", "Marketing", "Operations"]
  datasets:

    data: [500000, 200000, 150000]
    backgroundColor: ["#FF6384", "#36A2EB", "#FFCE56"]

    borderWidth: 2
>>`,
		},
		{
			name: "No chart blocks - should not modify",
			input: `## Regular content

This is just normal markdown content.

- List item 1
- List item 2

Some more text.`,
			expected: `## Regular content

This is just normal markdown content.

- List item 1
- List item 2

Some more text.`,
		},
		{
			name: "Chart block already properly indented",
			input: `<<chart:line>>
  title: Already Formatted
  labels: ["A", "B", "C"]
  datasets:
    data: [1, 2, 3]
    backgroundColor: "#FF6384"
>>`,
			expected: `<<chart:line>>
  title: Already Formatted
  labels: ["A", "B", "C"]
  datasets:
    data: [1, 2, 3]
    backgroundColor: "#FF6384"
>>`,
		},
		{
			name: "Chart with advanced properties",
			input: `<<chart:scatter>>
title: Performance vs Cost
labels: ["Low", "Medium", "High"]
datasets:
data: [{x: 10, y: 20}, {x: 15, y: 10}, {x: 20, y: 25}]
label: "Products"
backgroundColor: "rgba(255,99,132,0.2)"
borderColor: "rgba(255,99,132,1)"
pointRadius: 5
pointHoverRadius: 8
fill: false
tension: 0.1
>>`,
			expected: `<<chart:scatter>>
  title: Performance vs Cost
  labels: ["Low", "Medium", "High"]
  datasets:
    data: [{x: 10, y: 20}, {x: 15, y: 10}, {x: 20, y: 25}]
    label: "Products"
    backgroundColor: "rgba(255,99,132,0.2)"
    borderColor: "rgba(255,99,132,1)"
    pointRadius: 5
    pointHoverRadius: 8
    fill: false
    tension: 0.1
>>`,
		},
		{
			name: "Chart ending with slide separator",
			input: `<<chart:bar>>
title: Q4 Results
labels: ["Oct", "Nov", "Dec"]
datasets:
data: [100, 120, 140]
backgroundColor: "#36A2EB"

---

## Next slide content`,
			expected: `<<chart:bar>>
  title: Q4 Results
  labels: ["Oct", "Nov", "Dec"]
  datasets:
    data: [100, 120, 140]
    backgroundColor: "#36A2EB"

---

## Next slide content`,
		},
		{
			name: "Multiple chart blocks in same content",
			input: `## Charts Demo

<<chart:pie>>
title: Chart 1
labels: ["A", "B"]
datasets:
data: [50, 50]
>>

Some text between charts.

<<chart:bar>>
title: Chart 2
labels: ["X", "Y"]
datasets:
data: [30, 70]
>>`,
			expected: `## Charts Demo

<<chart:pie>>
  title: Chart 1
  labels: ["A", "B"]
  datasets:
    data: [50, 50]
>>

Some text between charts.

<<chart:bar>>
  title: Chart 2
  labels: ["X", "Y"]
  datasets:
    data: [30, 70]
>>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := rule.Apply(tt.input)
			if err != nil {
				t.Errorf("ChartFormatterRule.Apply() error = %v", err)
				return
			}
			if result != tt.expected {
				t.Errorf("ChartFormatterRule.Apply() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestChartFormatterRule_ExtractChartType(t *testing.T) {
	rule := NewChartFormatterRule()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Extract bar chart type",
			input:    `<<chart:bar title="Sales">>`,
			expected: "bar",
		},
		{
			name:     "Extract line chart type",
			input:    `<<chart:line>>`,
			expected: "line",
		},
		{
			name:     "Extract pie chart type",
			input:    `<<chart:pie title="Market Share">>`,
			expected: "pie",
		},
		{
			name:     "Extract doughnut chart type",
			input:    `<<chart:doughnut>>`,
			expected: "doughnut",
		},
		{
			name:     "Extract scatter chart type",
			input:    `<<chart:scatter title="Performance">>`,
			expected: "scatter",
		},
		{
			name:     "No chart type - should return empty",
			input:    `<<chart: >>`,
			expected: "",
		},
		{
			name:     "Invalid chart tag",
			input:    `<<mermaid>>`,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rule.extractChartType(tt.input)
			if result != tt.expected {
				t.Errorf("extractChartType() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestChartFormatterRule_IsChartDataLine(t *testing.T) {
	rule := NewChartFormatterRule()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "Title property",
			input:    "title: My Chart",
			expected: true,
		},
		{
			name:     "Labels property",
			input:    "labels: [\"A\", \"B\", \"C\"]",
			expected: true,
		},
		{
			name:     "Datasets property",
			input:    "datasets:",
			expected: true,
		},
		{
			name:     "Data property",
			input:    "data: [10, 20, 30]",
			expected: true,
		},
		{
			name:     "Background color property",
			input:    "backgroundColor: \"#FF6384\"",
			expected: true,
		},
		{
			name:     "Array start",
			input:    "[10, 20, 30]",
			expected: true,
		},
		{
			name:     "List item",
			input:    "- Item 1",
			expected: true,
		},
		{
			name:     "String value",
			input:    "\"Some value\"",
			expected: true,
		},
		{
			name:     "Regular text",
			input:    "This is just text",
			expected: false,
		},
		{
			name:     "Markdown header",
			input:    "## Section",
			expected: false,
		},
		{
			name:     "Empty line",
			input:    "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rule.isChartDataLine(tt.input)
			if result != tt.expected {
				t.Errorf("isChartDataLine() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestChartFormatterRule_IsEndOfChartBlock(t *testing.T) {
	rule := NewChartFormatterRule()

	tests := []struct {
		name        string
		trimmed     string
		lines       []string
		index       int
		expected    bool
		description string
	}{
		{
			name:        "Explicit end tag",
			trimmed:     ">>",
			lines:       []string{"<<chart:bar>>", "title: Test", ">>"},
			index:       2,
			expected:    true,
			description: "Should detect explicit >> end tag",
		},
		{
			name:        "Slide separator",
			trimmed:     "---",
			lines:       []string{"<<chart:bar>>", "title: Test", "---"},
			index:       2,
			expected:    true,
			description: "Should detect slide separator",
		},
		{
			name:        "New section header",
			trimmed:     "## New Section",
			lines:       []string{"<<chart:bar>>", "title: Test", "## New Section"},
			index:       2,
			expected:    true,
			description: "Should detect section header",
		},
		{
			name:        "Another SlideLang block",
			trimmed:     "<<mermaid>>",
			lines:       []string{"<<chart:bar>>", "title: Test", "<<mermaid>>"},
			index:       2,
			expected:    true,
			description: "Should detect another SlideLang block",
		},
		{
			name:        "Chart data line",
			trimmed:     "data: [1, 2, 3]",
			lines:       []string{"<<chart:bar>>", "title: Test", "data: [1, 2, 3]"},
			index:       2,
			expected:    false,
			description: "Should not end on chart data line",
		},
		{
			name:        "Empty line with chart data following",
			trimmed:     "",
			lines:       []string{"<<chart:bar>>", "title: Test", "", "data: [1, 2, 3]"},
			index:       2,
			expected:    false,
			description: "Empty line followed by chart data should not end block",
		},
		{
			name:        "Empty line with non-chart content following",
			trimmed:     "",
			lines:       []string{"<<chart:bar>>", "title: Test", "", "Regular text"},
			index:       2,
			expected:    true,
			description: "Empty line followed by non-chart content should end block",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rule.isEndOfChartBlock(tt.trimmed, tt.lines, tt.index)
			if result != tt.expected {
				t.Errorf("isEndOfChartBlock() = %v, want %v. %s", result, tt.expected, tt.description)
			}
		})
	}
}

// TODO: Test private methods via public API instead
/*
func TestChartFormatterRule_CalculateIndentLevel(t *testing.T) {
	rule := NewChartFormatterRule()

	tests := []struct {
		name         string
		line         string
		currentLevel int
		expected     int
	}{
		{
			name:         "Title property - main level",
			line:         "title: My Chart",
			currentLevel: 0,
			expected:     1,
		},
		{
			name:         "Labels property - main level",
			line:         "labels: [\"A\", \"B\"]",
			currentLevel: 0,
			expected:     1,
		},
		{
			name:         "Datasets property - main level",
			line:         "datasets:",
			currentLevel: 1,
			expected:     1,
		},
		{
			name:         "Data property - dataset level",
			line:         "data: [10, 20, 30]",
			currentLevel: 1,
			expected:     2,
		},
		{
			name:         "Background color - dataset level",
			line:         "backgroundColor: \"#FF6384\"",
			currentLevel: 1,
			expected:     2,
		},
		{
			name:         "Array content",
			line:         "[10, 20, 30]",
			currentLevel: 2,
			expected:     3,
		},
		{
			name:         "List item",
			line:         "- Item",
			currentLevel: 1,
			expected:     2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rule.calculateIndentLevel(tt.line, tt.currentLevel)
			if result != tt.expected {
				t.Errorf("calculateIndentLevel() = %v, want %v", result, tt.expected)
			}
		})
	}
}
*/

func TestChartFormatterRule_BuildIndentedLine(t *testing.T) {
	rule := NewChartFormatterRule()

	tests := []struct {
		name       string
		line       string
		level      int
		baseIndent string
		expected   string
	}{
		{
			name:       "Level 1 with 2-space indent",
			line:       "title: Test",
			level:      1,
			baseIndent: "  ",
			expected:   "  title: Test",
		},
		{
			name:       "Level 2 with 2-space indent",
			line:       "data: [1, 2, 3]",
			level:      2,
			baseIndent: "  ",
			expected:   "    data: [1, 2, 3]",
		},
		{
			name:       "Level 0 (no indent)",
			line:       "content",
			level:      0,
			baseIndent: "  ",
			expected:   "content",
		},
		{
			name:       "Level 3 with 4-space indent",
			line:       "nested: value",
			level:      3,
			baseIndent: "    ",
			expected:   "            nested: value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rule.buildIndentedLine(tt.line, tt.level, tt.baseIndent)
			if result != tt.expected {
				t.Errorf("buildIndentedLine() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestChartFormatterRule_Metadata(t *testing.T) {
	rule := NewChartFormatterRule()

	if rule.Description() == "" {
		t.Error("Description should not be empty")
	}

	if rule.Priority() <= 0 {
		t.Error("Priority should be positive")
	}
	if rule.Category() == base.CategoryEnhancement {
		// Expected category
	} else {
		t.Error("Category should be CategoryEnhancement")
	}
}
