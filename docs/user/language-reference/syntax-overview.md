# ZiraDocs Syntax Overview

ZiraDocs is an innovative language designed for efficient and powerful presentation creation, with a strong focus on AI optimization for generation and assistance. It allows users to focus on their message while the tool handles structuring and design.

## Operating Modes

ZiraDocs offers two syntax modes to adapt to different needs and preferences:

### **Strict Mode** 
- Formal and structured syntax
- Explicit keywords (`SLIDE`, `TEXT`, `POINTS`)
- Strict validation to ensure consistency  
- 100% predictable for parsers and analysis tools
- Ideal for automated generation and complex presentations

**When to use:** Large presentations, team collaboration, automated workflows, when you need precise control

**Learn more:** [Strict Mode Syntax](strict-mode.md)

### **Flex Mode**
- Extended Markdown syntax, familiar to most users
- Automatic content type inference
- More error-tolerant, facilitating rapid writing
- Approximately 60% less verbose than Strict Mode
- Great for quick prototyping and familiar workflow

**When to use:** Rapid prototyping, simple presentations, when you prefer Markdown syntax

**Learn more:** [Flex Mode Syntax](flex-mode.md)

## Mode Selection

Choose your operating mode in the [FrontMatter configuration](frontmatter.md):

```yaml
---
mode: flex  # or "strict"
---
```

## Core Concepts

### Slides as Building Blocks
Every presentation is composed of slides. Each slide has:
- A **type/layout** that determines its structure
- **Content** that fills the layout
- Optional **metadata** for behavior and styling

### Layout System
ZiraDocs provides specialized layouts for different use cases:

**Basic Layouts:**
- `title` - Title slides with optional subtitles
- `content` - Standard content slides with text and media
- `section` - Section dividers and transitions

**Specialized Layouts:**
- `hero` - Impact slides with large visuals
- `stats` - Data and statistics displays  
- `comparison` - Side-by-side comparisons
- `timeline` - Sequential processes and timelines
- `team` - Team member introductions
- `pricing` - Product/service pricing tables

**Learn more:** [Specialized Layouts](specialized-layouts.md)

### Content Elements

ZiraDocs supports rich content elements:

**Text Elements:**
- Headers, paragraphs, lists
- Inline formatting (bold, italic, links)
- Code snippets and blocks

**Media Elements:**
- Images with automatic sizing
- Videos and audio embeds
- Interactive charts and diagrams

**Interactive Elements:**
- Clickable elements and navigation
- Live polls and Q&A
- Real-time reactions

**Learn more:** [Advanced Elements](advanced-elements.md)

### Variables and Templates

Make your presentations dynamic with variables:

```yaml
---
variables:
  company: "Acme Corp"
  quarter: "Q3 2024"
---
```

Use in content: `Welcome to {{company}}'s {{quarter}} review`

**Learn more:** [Variables & Templates](../features/variables-templates.md)

## Quick Syntax Comparison

| Feature | Strict Mode | Flex Mode |
|---------|-------------|-----------|
| **Slide Declaration** | `SLIDE: title` | `# Title Here` |
| **Text Content** | `TEXT: "Hello world"` | `Hello world` |
| **Bullet Points** | `POINTS: ["Item 1", "Item 2"]` | `- Item 1`<br>`- Item 2` |
| **Layout Specification** | `LAYOUT: hero` | `<!-- layout: hero -->` |
| **Media Embedding** | `IMAGE: "./photo.jpg"` | `![alt](./photo.jpg)` |

## Professional Presentation Capabilities

ZiraDocs goes beyond traditional presentation tools by offering:

### Specialized Layouts
17+ predefined slide types that automatically apply specific CSS styles and intelligent validation:

**Impact Layouts:** `hero`, `testimonial`, `call_to_action`  
**Business Layouts:** `stats`, `dashboard`, `pricing`, `comparison`  
**Technical Layouts:** `code_example`, `feature_showcase`, `process`  
**Corporate Layouts:** `team`, `timeline`, `before_after`  

Each layout includes automatic validation and optimized styles to maximize visual impact.

### Complete Infographics
13 complete presentation templates that combine multiple slides into cohesive visual narratives:

**Business Reports:** `statistical_report`, `market_analysis`, `research_findings`  
**Marketing & Sales:** `product_launch`, `sales_funnel`, `competitive_battlecard`  
**Operations:** `onboarding_journey`, `crisis_communication`, `event_promotion`  
**Strategic:** `company_profile`, `investment_pitch`, `educational_guide`  

Each infographic includes coordinated visual themes, specific validations, and multi-format export.

## Key Benefits

1. **Unprecedented Flexibility:** Two syntax modes for different needs
2. **Specialized Layouts:** 17+ slide types optimized for specific use cases
3. **Complete Infographics:** 13 templates for critical business scenarios
4. **AI Optimization:** Designed from scratch for automatic generation and intelligent assistance
5. **Professional Power:** From simple slides to complex interactive experiences
6. **Open Ecosystem:** Extensible and adaptable to any workflow

The combination of Markdown simplicity with the power of specialized layouts, professional infographics, and interactive components makes it the ideal tool for the modern presentation era, where collaboration, automation, and engagement are fundamental.

With ZiraDocs, creating presentations transforms from a tedious task into a creative and efficient experience that rivals the best professional tools on the market.

## Getting Started

Ready to create your first presentation? Here are your next steps:

1. **[Install ZiraDocs](../getting-started/installation.md)** - Set up the CLI tool
2. **[Follow the Quickstart](../getting-started/quickstart.md)** - Create your first presentation in 5 minutes
3. **Choose your syntax mode:**
   - [Strict Mode](strict-mode.md) for structured, precise control
   - [Flex Mode](flex-mode.md) for rapid, Markdown-like authoring
4. **[Configure with FrontMatter](frontmatter.md)** - Set up metadata and global settings

## Related Documentation

- [Strict Mode Syntax](strict-mode.md) - Formal keyword-based syntax
- [Flex Mode Syntax](flex-mode.md) - Extended Markdown syntax  
- [FrontMatter Configuration](frontmatter.md) - YAML metadata and settings
- [Specialized Layouts](specialized-layouts.md) - Advanced slide types
- [Variables & Templates](../features/variables-templates.md) - Dynamic content
- [Best Practices](../guides/best-practices.md) - Write effective presentations

## Philosophy

ZiraDocs represents a natural evolution in presentation creation, designed for the era where:
- **Content matters more than formatting** - Focus on your message, let the tool handle the design
- **Collaboration is essential** - Version control friendly, works with existing workflows  
- **AI assistance is standard** - Optimized for automated generation and intelligent help
- **Flexibility is key** - Adapt to any use case, from simple slides to complex infographics
