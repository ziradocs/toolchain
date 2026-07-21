# Advanced Elements

ZiraDocs supports sophisticated visual elements including diagrams, complex charts, maps, and interactive content to create engaging presentations.

## Overview

Advanced elements in ZiraDocs allow you to:
- Create diagrams with Mermaid syntax
- Build complex and combination charts
- Display interactive maps with markers
- Add rich media and interactive content

These elements work in both [Flex Mode](flex-mode.md) and [Strict Mode](strict-mode.md) syntax.

## Diagrams with Mermaid

ZiraDocs integrates with Mermaid to create various types of diagrams directly in your presentations.

### Flowcharts

Create decision trees and process flows:

```slidelang
## Process Flow

<<mermaid>>
  graph TD
      A[Start] --> B{Decision?}
      B -->|Yes| C[Process A]
      B -->|No| D[Process B]
      C --> E[End]
      D --> E
```

**Strict Mode equivalent:**

```slidelang
SLIDE content
  title: "Process Flow"
  <<mermaid>>
    graph TD
        A[Start] --> B{Decision?}
        B -->|Yes| C[Process A]
        B -->|No| D[Process B]
        C --> E[End]
        D --> E
```

### Sequence Diagrams

Visualize interactions between systems:

```slidelang
## API Communication

<<mermaid>>
  sequenceDiagram
      participant Client
      participant API
      participant Database
      
      Client->>API: Request
      API->>Database: Query
      Database-->>API: Results
      API-->>Client: Response
```

### Class Diagrams

Show object-oriented relationships:

```slidelang
## System Architecture

<<mermaid>>
  classDiagram
      class User {
          +String name
          +String email
          +login()
          +logout()
      }
      class Order {
          +int id
          +Date created
          +calculate()
      }
      User ||--o{ Order : places
```

### Gantt Charts

Display project timelines:

```slidelang
## Project Timeline

<<mermaid>>
  gantt
      title Development Schedule
      dateFormat  YYYY-MM-DD
      section Phase 1
      Planning    :a1, 2024-01-01, 30d
      Design      :a2, after a1, 20d
      section Phase 2
      Development :a3, 2024-02-20, 45d
      Testing     :a4, after a3, 15d
```

## Advanced Charts

Create sophisticated data visualizations with multiple chart types and combinations.

### Combination Charts

Mix different chart types in a single visualization:

```slidelang
## Sales Performance

<<chart: combo>>
  type: ["bar", "bar", "line"]
  data: [
    ["Q1", 65, 45, 85],
    ["Q2", 59, 52, 90],
    ["Q3", 80, 61, 95],
    ["Q4", 81, 73, 88]
  ]
  series: ["Sales", "Target", "Growth %"]
  options:
    responsive: true
    plugins:
      legend:
        position: top
      title:
        display: true
        text: "Quarterly Performance"
    scales:
      y1:
        type: linear
        display: true
        position: left
      y2:
        type: linear
        display: true
        position: right
        grid:
          drawOnChartArea: false
```

### Multi-axis Charts

Display different data scales on the same chart:

```slidelang
## Revenue vs Units

<<chart: line>>
  data: [
    ["Jan", 50000, 1200],
    ["Feb", 45000, 1100],
    ["Mar", 60000, 1400],
    ["Apr", 55000, 1300]
  ]
  series: ["Revenue ($)", "Units Sold"]
  options:
    responsive: true
    scales:
      y:
        type: linear
        display: true
        position: left
      y1:
        type: linear
        display: true
        position: right
        grid:
          drawOnChartArea: false
```

### Stacked Charts

Show component breakdown:

```slidelang
## Market Share by Region

<<chart: bar>>
  data: [
    ["North", 25, 15, 10],
    ["South", 30, 20, 12],
    ["East", 35, 25, 15],
    ["West", 28, 18, 14]
  ]
  series: ["Product A", "Product B", "Product C"]
  options:
    responsive: true
    scales:
      x:
        stacked: true
      y:
        stacked: true
    plugins:
      title:
        display: true
        text: "Regional Market Share"
```

### Strict Mode Chart Examples

```slidelang
SLIDE content
  title: "Advanced Analytics Dashboard"
  <<chart: combo>>
    type: ["bar", "line", "line"]
    data: [
      ["Jan", 65, 28, 45],
      ["Feb", 59, 48, 52],
      ["Mar", 80, 40, 61],
      ["Apr", 81, 19, 73]
    ]
    series: ["Revenue", "Costs", "Profit Margin"]
    options:
      responsive: true
      plugins:
        legend:
          position: top
        title:
          display: true
          text: "Financial Performance"

SLIDE content
  title: "Trend Analysis"
  <<chart: line>>
    data: [
      ["Week 1", 120, 95, 87],
      ["Week 2", 135, 102, 94],
      ["Week 3", 142, 108, 98],
      ["Week 4", 158, 115, 105]
    ]
    series: ["Target", "Actual", "Forecast"]
    options:
      responsive: true
      tension: 0.4
```

## Maps and Geolocation

Display geographic data with interactive maps, markers, and heatmaps.

### World Maps

Show global presence or data:

```slidelang
## Global Operations

<<map>>
  type: world
  markers:
    - lat: 40.7128
      lng: -74.0060
      label: "New York HQ"
      value: 450
      color: "blue"
    - lat: 51.5074
      lng: -0.1278
      label: "London Office"
      value: 380
      color: "green"
    - lat: 35.6762
      lng: 139.6503
      label: "Tokyo Branch"
      value: 520
      color: "red"
  heatmap: true
  zoom: 2
  options:
    showScale: true
    projection: "mercator"
```

### Regional Maps

Focus on specific geographic areas:

```slidelang
## US Operations

<<map>>
  type: region
  center: [39.8283, -98.5795]
  markers:
    - lat: 37.7749
      lng: -122.4194
      label: "San Francisco"
      value: 125
      color: "blue"
    - lat: 41.8781
      lng: -87.6298
      label: "Chicago"
      value: 98
      color: "red"
    - lat: 25.7617
      lng: -80.1918
      label: "Miami"
      value: 87
      color: "green"
  zoom: 4
  options:
    showTooltips: true
    clustering: true
```

### Choropleth Maps

Display data by geographic regions:

```slidelang
## Sales by State

<<map>>
  type: choropleth
  region: "usa"
  data:
    CA: 450000
    TX: 380000
    NY: 520000
    FL: 280000
  colorScale: ["#f7fbff", "#08519c"]
  options:
    showLegend: true
    tooltip: true
```

### Strict Mode Map Examples

```slidelang
SLIDE content
  title: "Global Market Presence"
  <<map>>
    type: world
    markers:
      - lat: 40.7128
        lng: -74.0060
        label: "New York"
        value: 450
        color: "blue"
      - lat: 51.5074
        lng: -0.1278
        label: "London"
        value: 380
        color: "green"
      - lat: 35.6762
        lng: 139.6503
        label: "Tokyo"
        value: 520
        color: "red"
      - lat: -33.8688
        lng: 151.2093
        label: "Sydney"
        value: 290
        color: "orange"
    heatmap: true
    zoom: 2

SLIDE content
  title: "Regional Distribution Centers"
  <<map>>
    type: region
    center: [39.8283, -98.5795]
    markers:
      - lat: 47.6062
        lng: -122.3321
        label: "Seattle DC"
        status: "active"
      - lat: 32.7767
        lng: -96.7970
        label: "Dallas DC"
        status: "active"
      - lat: 33.7490
        lng: -84.3880
        label: "Atlanta DC"
        status: "planned"
    zoom: 4
```

## Grid Layouts

Arrange content in side-by-side columns. In **strict mode**, a grid is a
delimited `<<grid>>` … `<<end>>` block with `<<column>>` separators:

```slidelang
SLIDE content
  title: "Before vs After"
  <<grid>>
  <<column>>
  ## Before
  - manual steps
  - slow feedback
  <<column>>
  ## After
  - automated pipeline
  - instant feedback
  <<end>>
```

Lines before the first `<<column>>` become loose prose spanning the grid, and
each column body is regular Markdown-style content. The equivalent **flex**
form uses `::: grid` / `::: column`; both produce the same grid structure.

## Interactive Content

Add dynamic elements to enhance user engagement.

### Embedded Content

Include external content and media:

```slidelang
## Video Overview

<<embed>>
  type: video
  url: "https://example.com/video.mp4"
  autoplay: false
  controls: true
  width: 800
  height: 450

## Interactive Dashboard

<<embed>>
  type: iframe
  url: "https://dashboard.example.com"
  width: 100%
  height: 600
  sandbox: "allow-scripts allow-same-origin"
```

### Data Tables

Display structured data with sorting and filtering:

```slidelang
## Performance Metrics

<<table>>
  headers: ["Region", "Sales", "Growth", "Target"]
  data: [
    ["North", "$125K", "+12%", "$130K"],
    ["South", "$98K", "+8%", "$105K"],
    ["East", "$156K", "+15%", "$160K"],
    ["West", "$134K", "+10%", "$140K"]
  ]
  sortable: true
  filterable: true
  pagination: false
```

### Code Snippets with Syntax Highlighting

Display formatted code examples:

```slidelang
## API Example

<<code: javascript>>
  async function fetchData() {
    try {
      const response = await fetch('/api/data');
      const data = await response.json();
      return data;
    } catch (error) {
      console.error('Error:', error);
    }
  }

<<code: python>>
  def process_data(data):
      """Process incoming data"""
      processed = []
      for item in data:
          if item.get('valid'):
              processed.append(transform(item))
      return processed
```

## Best Practices

### Performance Considerations

- **Optimize data size**: Large datasets in charts and maps can slow rendering
- **Use appropriate chart types**: Choose the right visualization for your data
- **Limit markers**: Too many map markers can impact performance
- **Cache external content**: Embedded content should be optimized for loading

### Accessibility

- **Provide alt text**: Always include descriptive text for visual elements
- **Use high contrast**: Ensure colors are distinguishable
- **Support keyboard navigation**: Interactive elements should be accessible
- **Include data tables**: Provide alternative data representations

### Design Guidelines

- **Consistent styling**: Use theme colors and fonts
- **Clear labels**: Make chart axes and map markers easily readable
- **Appropriate sizing**: Scale elements for presentation viewing
- **Progressive disclosure**: Break complex visualizations into steps

## Common Patterns

### Dashboard Slide

Combine multiple advanced elements:

```slidelang
## Executive Dashboard

### Key Metrics
<<chart: bar>>
  data: [["Revenue", 125], ["Growth", 15], ["Satisfaction", 92]]
  series: ["Current"]

### Geographic Distribution
<<map>>
  type: world
  markers:
    - lat: 40.7128, lng: -74.0060, label: "NY", value: 45
    - lat: 51.5074, lng: -0.1278, label: "London", value: 38

### Process Flow
<<mermaid>>
  graph LR
      A[Input] --> B[Process]
      B --> C[Output]
      C --> D[Analysis]
```

## Related Topics

- [Chart Variables](charts-and-variables.md) - Dynamic chart data
- [Themes](../themes/overview.md) - Styling advanced elements
- [Special Blocks](special-blocks.md) - Basic content blocks
- [Syntax Overview](syntax-overview.md) - Language fundamentals

## Examples

See the `/examples/` directory for complete examples:
- `examples/02_diagrams_and_charts/` - Chart and diagram examples
- `examples/10_advanced_elements/` - Advanced element demonstrations
