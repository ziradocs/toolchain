# Flex Mode Syntax

**ZiraDocs Flex Mode** uses an extended Markdown syntax that will be familiar to many users. It allows for more natural and less verbose writing by automatically inferring content types. This mode is ideal for rapid prototyping and users who prefer the simplicity and familiarity of Markdown.

## When to Use Flex Mode

✅ **Recommended for:**
- Quick content authoring and prototyping
- Markdown-familiar writers
- Simple to medium complexity presentations
- Rapid iteration and content development
- Natural, flowing content creation

❌ **Consider Strict Mode instead for:**
- Programmatic generation of presentations
- Complex layouts requiring precise control
- Team environments needing strict structure
- Maximum parser predictability

## Key Principles

- **Markdown-based**: Uses standard Markdown syntax for most common elements (headings, lists, text, code, images)
- **Content inference**: ZiraDocs automatically determines content type and slide structure based on Markdown conventions
- **Less verbosity**: Requires significantly less syntax than Strict Mode
- **Slide separators**: By default, `---` (three dashes) on its own line separates slides

## Standard Markdown Elements

### Headings (`#`, `##`, `###`, etc.)

Markdown headings are used for slide titles and section headings within slides.

```slidelang
---
mode: flex
title: "My Flex Presentation"
---

# First Slide Title
## Subtitle for this slide

Regular text content goes here.

---

## Second Slide Title (H2 level)

Another paragraph of text.

---

### Third Slide with H3 Title

Content with a smaller heading.
```

**Heading Guidelines:**
- `#` (H1) - Primary slide titles
- `##` (H2) - Secondary slide titles or main section headings
- `###` (H3) - Subsection headings within slides
- `####` and below - Content hierarchy within slides

### Paragraphs and Text

Regular text is interpreted as paragraph content. ZiraDocs supports full Markdown inline formatting.

```slidelang
This is a simple paragraph.

This is another paragraph, separated by a blank line.

Text can include **bold text**, *italic text*, `inline code`, 
==highlighted text==, ~~strikethrough text~~, and [links](https://example.com).
```

### Lists (Ordered and Unordered)

Standard Markdown list syntax is supported with automatic marker detection and styling.

**Unordered lists:**
```slidelang
- First bullet point
- Second bullet point
  - Nested sub-point
  - Another sub-point
- Third main point
```

**Ordered lists:**
```slidelang
1. First numbered item
2. Second numbered item
   a. Sub-item with letter
   b. Another lettered sub-item
3. Third numbered item
```

**Mixed lists:**
```slidelang
1. Main numbered item
   - Sub-item with bullet
   - Another bullet sub-item
2. Second main item
   a. Lettered sub-item
   b. Another lettered sub-item
```

**Checklists:**
```slidelang
## Project Status

- [x] Requirements gathering completed
- [x] Design phase finished
- [ ] Development in progress
- [ ] Testing phase
- [ ] Deployment
```

### Code Blocks

Use standard Markdown fenced code blocks with language specification for syntax highlighting.

````slidelang
```python
def greet_user(name):
    print(f"Hello, {name} from ZiraDocs Flex!")
    return f"Welcome to the presentation, {name}!"

# Usage
greet_user("Developer")
```

```javascript
// Modern JavaScript example
const createSlide = (title, content) => ({
    title,
    content,
    timestamp: new Date().toISOString()
});

const mySlide = createSlide("Welcome", "Hello World!");
```

```yaml
# Configuration example
---
mode: flex
title: "API Documentation"
variables:
  api_version: "v2.1"
  base_url: "https://api.example.com"
---
```
````

### Images

Standard Markdown image syntax with support for captions and alt text.

```slidelang
![Chart showing Q4 results](assets/q4-chart.png "Q4 Performance Chart")

![Company logo](assets/logo.svg)

![Screenshot of dashboard](assets/dashboard-screenshot.png "Main dashboard interface showing key metrics")
```

### Block Quotes

Perfect for testimonials, inspirational quotes, or important references.

```slidelang
> The best presentations tell a story that resonates with the audience 
> and drives them to take action.

> "Simplicity is the ultimate sophistication."
> — Leonardo da Vinci

> Data without context is just noise. Great presentations 
> transform data into actionable insights.
> -- Data Science Team
```

**Quote features:**
- **Multi-line support**: Quotes can span multiple lines
- **Automatic author detection**: Lines starting with `--` or `—` are styled as attributions
- **Variable support**: Use template variables within quotes

### Slide Separators

Use three dashes (`---`) on their own line to separate slides.

```slidelang
# First Slide
Content for the first slide.

---

# Second Slide
Content for the second slide.

---

## Third Slide
More content here.
```

## ZiraDocs Extensions

Flex Mode includes powerful extensions beyond standard Markdown for advanced features.

### Special Blocks

Create highlighted information blocks using the `:::` syntax:

```slidelang
:::info
💡 **Quick Tip**
This is an informational block for helpful guidance.
:::

:::warning
⚠️ **Important Notice**
Pay attention to this warning before proceeding.
:::

:::success
✅ **Great Job!**
This indicates successful completion or positive outcomes.
:::

:::danger
🚨 **Critical Alert**
This is for critical information that requires immediate attention.
:::
```

### Charts and Data Visualization

Embed interactive charts directly in your slides:

```slidelang
## Sales Performance

<<chart: bar>>
  data: [
    ["Q1 2024", 125000, 98000],
    ["Q2 2024", 145000, 112000],
    ["Q3 2024", 167000, 128000],
    ["Q4 2024", 189000, 145000]
  ]
  series: ["Revenue", "Profit"]
  options:
    responsive: true
    plugins:
      title:
        display: true
        text: "Quarterly Financial Performance"
```

### Diagrams with Mermaid

Create flowcharts, sequence diagrams, and more:

```slidelang
## User Authentication Flow

<<mermaid>>
  sequenceDiagram
    participant U as User
    participant F as Frontend
    participant A as Auth Service
    participant D as Database
    
    U->>F: Enter credentials
    F->>A: Validate user
    A->>D: Check credentials
    D-->>A: User data
    A-->>F: JWT token
    F-->>U: Login success
```

### Interactive Maps

Display geographic data with interactive maps:

```slidelang
## Global User Distribution

<<map>>
  type: world
  markers:
    - lat: 37.7749
      lng: -122.4194
      label: "San Francisco"
      value: 15420
    - lat: 51.5074
      lng: -0.1278
      label: "London"
      value: 8930
  zoom: 3
```

### Code Groups

Display multiple code examples with tabs:

```slidelang
## API Examples

:::code-group

```javascript [JavaScript]
const response = await fetch('/api/users');
const users = await response.json();
console.log(users);
```

```python [Python]
import requests

response = requests.get('/api/users')
users = response.json()
print(users)
```

```curl [cURL]
curl -X GET https://api.example.com/users \
  -H "Authorization: Bearer your-token"
```

:::
```

## Specialized Layouts

Flex Mode supports automatic layout detection and manual layout specification:

### Automatic Layout Detection

ZiraDocs automatically detects slide types based on content:

```slidelang
# Welcome to Our Product
## Revolutionizing the industry

![Hero image](assets/hero.png)

*Automatically detected as 'title' layout*

---

## Key Features

- Feature one with benefits
- Feature two with advantages  
- Feature three with value proposition

*Automatically detected as 'content' layout*

---

| Feature | Basic | Pro | Enterprise |
|---------|-------|-----|------------|
| Users | 5 | 50 | Unlimited |
| Storage | 1GB | 100GB | 1TB |
| Support | Email | Priority | Dedicated |

*Automatically detected as 'comparison' layout*
```

### Manual Layout Specification

Override automatic detection using YAML frontmatter:

```slidelang
---
layout: hero
---

# Transform Your Business
## With our innovative solutions

[Get Started Today](#cta){.btn-primary}

---
layout: testimonial
---

> "This solution increased our productivity by 300% in just three months. 
> The results speak for themselves."
> 
> **— Sarah Johnson, CEO of TechCorp**
```

## Variables and Templating

Use variables defined in your FrontMatter for dynamic content:

```slidelang
---
mode: flex
title: "Product Launch Presentation"
variables:
  product_name: "SuperWidget Pro"
  launch_date: "March 2025"
  price: "$99/month"
  company: "Innovation Corp"
---

# Introducing {{product_name}}
## Launching {{launch_date}}

Welcome to {{company}}'s latest innovation.

---

## Pricing

Get {{product_name}} for just {{price}}.

**Special launch offer:** 50% off first year!
```

## Complete Example

Here's a complete Flex Mode presentation showcasing various features:

```slidelang
---
mode: flex
title: "ZiraDocs Flex Demo"
author: "Demo Team"
theme: "modern-blue"
variables:
  app_name: "ZiraDocs"
  version: "v3.0"
---

# Welcome to {{app_name}} {{version}}
## Making presentations simple and powerful

![ZiraDocs Logo](assets/logo.png)

---

## Why Choose {{app_name}}?

Our platform offers:

- **Simple syntax** that's easy to learn
- **Powerful features** for professional presentations
- **Flexible modes** for different use cases
- **Rich ecosystem** of themes and extensions

---

## Getting Started

:::info
**Quick Start Guide**
Follow these simple steps to create your first presentation.
:::

1. **Create your file** with `.slidelang` extension
2. **Add frontmatter** with mode and configuration
3. **Write your content** using Markdown syntax
4. **Build and preview** with the CLI tool

```bash
# Install ZiraDocs CLI
go install go.ziradocs.com/slidelang/cmd/slidelang@latest

# Create your first presentation
slidelang build my-presentation.slidelang
```

---

## Performance Metrics

<<chart: bar>>
  data: [
    ["Creation Speed", 85, 45],
    ["Learning Curve", 90, 30],
    ["Feature Richness", 80, 95]
  ]
  series: ["{{app_name}}", "Traditional Tools"]
  options:
    responsive: true
    plugins:
      title:
        display: true
        text: "Comparison with Traditional Presentation Tools"

---

> "{{app_name}} has revolutionized how we create presentations. 
> The learning curve is minimal, but the possibilities are endless."
> 
> **— Alex Chen, Technical Writer**

---

## What's Next?

Ready to get started with {{app_name}}?

- 📚 [Read the documentation](docs.slidelang.com)
- 🚀 [Try the online editor](editor.slidelang.com)
- 💬 [Join our community](community.slidelang.com)

**Thank you for using {{app_name}}!**
```

## Best Practices

1. **Keep it simple** - Leverage Markdown's natural readability
2. **Use consistent heading levels** - Maintain logical hierarchy
3. **Leverage automatic detection** - Let ZiraDocs infer layouts when possible
4. **Test your content** - Preview regularly to ensure proper rendering
5. **Use variables wisely** - Create reusable content with template variables
6. **Structure with separators** - Use `---` to clearly separate slides

## Migration to Strict Mode

If you need more control, you can migrate from Flex to Strict Mode:

| Flex Mode | Strict Mode | Notes |
|-----------|-------------|-------|
| `# Title` | `SLIDE content title: "Title"` | Explicit declarations required |
| `- List item` | `POINTS - List item` | Wrapped in explicit blocks |
| Text paragraphs | `TEXT` block | All content must be explicitly typed |
| ` ```code``` ` | `CODE` block with language | More configuration options |

## Next Steps

- Learn about [Strict Mode](strict-mode.md) for more structured presentations
- Explore [FrontMatter](frontmatter.md) for advanced configuration options
- Check out [Inline Formatting](inline-formatting.md) for text styling
- Discover [Advanced Elements](advanced-elements.md) for interactive content

---

*For more examples and use cases, see the [examples directory](../../../examples/) in the ZiraDocs repository.*
