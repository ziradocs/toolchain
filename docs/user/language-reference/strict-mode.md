# Strict Mode Syntax

**ZiraDocs Strict Mode** provides a formal and structured syntax for defining presentations. It uses explicit keywords and follows rigorous validation rules, making it 100% predictable for parsers and analysis tools. This mode is ideal when you need maximum precision and control over your presentation structure.

## When to Use Strict Mode

✅ **Recommended for:**
- Programmatic generation of presentations
- Teams requiring consistent structure
- Complex presentations with multiple layouts
- Integration with automated workflows
- Maximum parser predictability

❌ **Consider Flex Mode instead for:**
- Quick content authoring
- Markdown-familiar writers
- Simple presentations
- Rapid prototyping

## Core Concepts

- **Explicit declarations**: Every presentation element is defined with a keyword
- **Clear hierarchy**: Presentation structure is defined through nesting and specific blocks
- **Strict validation**: The ZiraDocs parser verifies compliance with language rules, helping prevent errors
- **Layout awareness**: Built-in support for specialized slide types with validation

## Essential Keywords

### `SLIDE`

Defines a new slide with optional type or layout specification.

```slidelang
---
mode: strict
title: "My Presentation"
---

SLIDE title
  heading: "Welcome to ZiraDocs"
  subtitle: "Structured presentations made simple"
  logo: "assets/logo.png"

SLIDE content
  title: "Main Content Slide"
  // Content elements go here
```

**Slide Properties:**

For `title` slides:
- `heading`: Main title (alternative to `title`)
- `subtitle`: Presentation subtitle
- `logo`: Path to image file for logo

For all slides:
- `title`: Slide title

### `TEXT`

Defines a paragraph text block with Markdown inline formatting support.

```slidelang
SLIDE content
  title: "Text Example"
  TEXT
    This is a paragraph with **bold text**, *italic text*, and `inline code`.
    
    You can include [links](https://example.com) and multiple lines.
```

### `POINTS`

Defines a list of bullet points with automatic marker detection and nesting support.

```slidelang
SLIDE content
  title: "List Examples"
  POINTS
    - First point with dash
    - Second point
      - Nested sub-point
      - Another sub-point
    - Third main point
```

**Supported List Types:**

**Bullet lists:**
```slidelang
POINTS
  - Dash bullets
  * Asterisk bullets
  + Plus sign bullets
```

**Numbered lists:**
```slidelang
POINTS
  1. First item
  2. Second item
  3. Third item
```

**Alphabetical lists:**
```slidelang
POINTS
  a. First option
  b. Second option
  c. Third option
```

**Mixed and nested lists:**
```slidelang
POINTS
  1. Main process
     a. Sub-step A
     b. Sub-step B
  2. Second process
     - Important detail
     - Another detail
```

### `CODE`

Defines a code block with optional language specification for syntax highlighting.

```slidelang
SLIDE content
  title: "Code Example"
  CODE python
    def hello_world():
        print("Hello, ZiraDocs Strict!")
        return "success"
```

**Available languages:** javascript, python, typescript, go, java, c++, sql, yaml, json, html, css, bash, and more.

### `IMAGE`

Inserts an image with optional caption and alt text.

```slidelang
SLIDE content
  title: "Image Example"
  IMAGE "assets/chart.png" "Sales performance chart"
    caption: "Q4 2024 sales results showing 25% growth"
```

### `TABLE`

Creates structured tables with headers and data rows.

```slidelang
SLIDE content
  title: "Data Table"
  TABLE
    headers: ["Product", "Q3", "Q4", "Growth"]
    rows: [
      ["Widget A", "$125K", "$156K", "+25%"],
      ["Widget B", "$89K", "$112K", "+26%"],
      ["Widget C", "$203K", "$234K", "+15%"]
    ]
    caption: "Product performance by quarter"
```

## Advanced Elements

### Charts and Visualizations

Create interactive charts using Chart.js integration:

```slidelang
SLIDE content
  title: "Sales Performance"
  <<chart: bar>>
    data: [
      ["Q1", 45, 32, 28],
      ["Q2", 52, 38, 35],
      ["Q3", 61, 45, 42],
      ["Q4", 73, 51, 48]
    ]
    series: ["Product A", "Product B", "Product C"]
    options:
      responsive: true
      plugins:
        title:
          display: true
          text: "Quarterly Sales by Product"
```

**Supported chart types:** `bar`, `line`, `pie`, `doughnut`, `combo`, `radar`, `scatter`

### Diagrams with Mermaid

Create technical diagrams using Mermaid syntax:

```slidelang
SLIDE content
  title: "System Architecture"
  <<mermaid>>
    graph TD
        A[User] --> B[Frontend]
        B --> C[API Gateway]
        C --> D[Microservices]
        D --> E[Database]
```

### Interactive Maps

Display maps with markers and geographic data:

```slidelang
SLIDE content
  title: "Global Presence"
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
    zoom: 2
```

### Grid Layout

Arrange content in side-by-side columns with a `<<grid>>` block. Each
`<<column>>` marker starts a new column; the grid closes with `<<end>>`. A
column's body is written verbatim (Markdown-style content — headings, lists,
prose), and any lines before the first `<<column>>` become loose prose spanning
the whole grid.

```slidelang
SLIDE content
  title: "Two-Column Layout"
  <<grid>>
  <<column>>
  ## Left column
  - point one
  - point two
  <<column>>
  ## Right column
  Prose content on the right.
  <<end>>
```

This is the strict-mode counterpart of the flex `::: grid` / `::: column`
form — both produce the same grid structure. `<<end>>` terminates the grid, so
any element after it (a `TEXT`, another block) is parsed independently.

## Special Blocks

Create highlighted information blocks for different types of content:

```slidelang
SLIDE content
  title: "Important Information"
  
  :::info
  💡 **Information**
  This is an informational block for helpful tips.
  :::

  :::warning
  ⚠️ **Warning**
  This is a warning block for important notices.
  :::

  :::danger
  🚨 **Danger**
  This is a critical alert block.
  :::

  :::success
  ✅ **Success**
  This indicates successful completion.
  :::

  :::tip
  💡 **Pro Tip**
  Helpful advice for better workflow.
  :::
```

## Specialized Layouts

ZiraDocs Strict includes built-in support for specialized slide layouts with automatic validation:

### `title` - Title Slide Layout
```slidelang
SLIDE title
  heading: "Main Presentation Title"
  subtitle: "Descriptive subtitle"
  logo: "assets/logo.png"
```

### `content` - General Content Layout
```slidelang
SLIDE content
  title: "Standard Content"
  TEXT
    Regular content with mixed elements.
  POINTS
    - First point
    - Second point
```

### `comparison` - Side-by-Side Comparison
```slidelang
SLIDE comparison
  title: "Feature Comparison"
  :::info
    title: "Option A"
    content: "Features and benefits of first option"
  :::success
    title: "Option B" 
    content: "Features and benefits of second option"
```

### `stats` - Data and Statistics
```slidelang
SLIDE stats
  title: "Performance Metrics"
  TABLE
    headers: ["Metric", "Q3", "Q4", "Change"]
    rows: [
      ["Revenue", "$1.2M", "$1.8M", "+50%"],
      ["Users", "10K", "15K", "+50%"]
    ]
```

### `code_example` - Technical Documentation
```slidelang
SLIDE code_example
  title: "API Implementation"
  TEXT
    Basic usage example:
  CODE javascript
    const api = new ZiraDocs.API();
    const result = await api.build('presentation.slidelang');
```

## Layout Validation

ZiraDocs automatically validates that slide content matches its declared layout:

- **`comparison` slides** must have at least 2 comparable elements
- **`stats` slides** must include data tables, charts, or metrics
- **`code_example` slides** must contain at least one CODE block
- **`title` slides** must have heading or title defined

If validation fails, the linter will show specific warnings to help you fix the issues.

## Variables and Templating

Use variables defined in your FrontMatter for dynamic content:

```slidelang
---
mode: strict
title: "Product Presentation"
variables:
  product_name: "SuperWidget"
  version: "v2.0"
  release_date: "Q2 2025"
---

SLIDE title
  heading: "Introducing {{product_name}} {{version}}"
  subtitle: "Available {{release_date}}"

SLIDE content
  title: "{{product_name}} Features"
  TEXT
    {{product_name}} includes powerful new capabilities in {{version}}.
```

## Slide Structure Example

Here's a complete example showing proper slide structure:

```slidelang
---
mode: strict
title: "Complete Presentation Example"
author: "ZiraDocs Team"
variables:
  app_name: "ZiraDocs"
---

SLIDE title
  heading: "Welcome to {{app_name}}"
  subtitle: "Structured presentations made simple"
  logo: "assets/logo.png"

SLIDE content
  title: "Key Features"
  TEXT
    {{app_name}} provides powerful features for creating professional presentations.
  POINTS
    - Explicit syntax for maximum control
    - Built-in validation and error checking
    - Support for complex layouts and elements
    - Interactive charts and diagrams

SLIDE code_example
  title: "Getting Started"
  TEXT
    Create your first slide with this simple syntax:
  CODE slidelang
    SLIDE content
      title: "My First Slide"
      TEXT
        Hello, world!

SLIDE stats
  title: "Usage Statistics"
  TABLE
    headers: ["Feature", "Adoption", "Satisfaction"]
    rows: [
      ["Basic slides", "100%", "95%"],
      ["Advanced layouts", "78%", "92%"],
      ["Interactive elements", "45%", "98%"]
    ]
```

## Best Practices

1. **Use descriptive slide titles** - Every slide should have a clear, descriptive title
2. **Choose appropriate layouts** - Match your slide layout to your content type
3. **Validate regularly** - Run the linter frequently to catch issues early
4. **Structure your content** - Use logical hierarchy with headings and bullet points
5. **Leverage variables** - Use FrontMatter variables for repeated content
6. **Test your presentations** - Verify that advanced elements render correctly

## Migration from Flex Mode

If you're migrating from Flex Mode, here are the key differences:

| Flex Mode | Strict Mode | Notes |
|-----------|-------------|-------|
| `# Title` | `SLIDE content title: "Title"` | Explicit slide declaration |
| `- List item` | `POINTS - List item` | Explicit points block |
| Text paragraphs | `TEXT` block | Explicit text declaration |
| ` ```code``` ` | `CODE` block | Explicit code block |
| `::: grid` / `::: column` | `<<grid>>` / `<<column>>` / `<<end>>` | Delimited grid block (see [Grid Layout](#grid-layout)) |

## Next Steps

- Learn about [Flex Mode](flex-mode.md) for rapid content creation
- Explore [FrontMatter](frontmatter.md) for advanced configuration
- Check out [Specialized Layouts](specialized-layouts.md) for layout-specific features
- See [Advanced Elements](advanced-elements.md) for charts, diagrams, and interactive content

---

*For more examples and use cases, see the [examples directory](../../../examples/) in the ZiraDocs repository.*
