# Specialized Layouts

ZiraDocs provides specialized slide layouts that automatically apply optimized styling and validation for different content types. Each layout includes specific CSS styling and validation rules to ensure content is appropriate for its intended purpose.

## Overview

Specialized layouts help you create professional presentations by:

- **Automatic styling** - Each layout applies appropriate CSS styles
- **Content validation** - Built-in rules ensure appropriate content for each layout type
- **Visual optimization** - Layouts are designed for specific use cases and content types
- **Cross-mode support** - Available in both [Strict Mode](strict-mode.md) and [Flex Mode](flex-mode.md)

## Layout Types

### Basic Layouts

#### `title` - Title Slide Layout

**Purpose:** Primary presentation cover slide

**Characteristics:**
- Centered design with large typography
- Support for company logo
- Clean, impactful visual presentation

**Strict Mode:**
```slidelang
SLIDE title
  heading: "My Presentation"
  subtitle: "Descriptive subtitle"
  logo: "assets/logo.png"
```

**Flex Mode:**
```slidelang
---
layout: title
---

# My Presentation
## Descriptive subtitle

![Logo](assets/logo.png)
```

**Validation Rules:**
- Must have `heading` or title
- Should avoid complex content elements
- Maximum 1 decorative element allowed

#### `content` - General Content Layout

**Purpose:** Standard content slides for main presentation material

**Characteristics:**
- Flexible layout for mixed content types
- Support for all content elements
- General-purpose design

**Strict Mode:**
```slidelang
SLIDE content
  title: "Main Content"
  TEXT
    Regular content with mixed elements.
  POINTS
    - First point
    - Second point
```

**Flex Mode:**
```slidelang
---
layout: content
---

# Main Content

Regular content with mixed elements.

- First point
- Second point
```

**Validation Rules:**
- Must have a title
- Must have at least one content element
- No restrictions on element types

#### `section` - Section Introduction Layout

**Purpose:** Introduction slides for new presentation sections

**Characteristics:**
- Optimized for introductory text
- Clean design marking new topic sections
- Simple, readable layout

**Strict Mode:**
```slidelang
SLIDE section
  title: "Development Methodology"
  subtitle: "Processes and best practices"
  TEXT
    In this section we'll explore agile methodologies
    and how to implement them in our projects.
```

**Flex Mode:**
```slidelang
---
layout: section
---

# Development Methodology
## Processes and best practices

In this section we'll explore agile methodologies
and how to implement them in our projects.
```

**Validation Rules:**
- Must have title or subtitle
- Should contain simple, introductory content
- Avoids complex elements (code, charts, tables)

### Comparison and Analysis Layouts

#### `comparison` - Side-by-Side Comparison Layout

**Purpose:** Compare two or more options side by side

**Characteristics:**
- Column-based layout for visual comparison
- Balanced content distribution
- Clear visual differentiation

**Strict Mode:**
```slidelang
SLIDE comparison
  title: "Framework Comparison"
  :::info
    title: "React"
    content: "Component-based, virtual DOM, large ecosystem"
  :::success
    title: "Vue"
    content: "Progressive, template-based, gentle learning curve"
```

**Flex Mode:**
```slidelang
---
layout: comparison
---

# Framework Comparison

:::info React
Component-based, virtual DOM, large ecosystem
:::

:::success Vue
Progressive, template-based, gentle learning curve
:::
```

**Validation Rules:**
- Must have balanced content sections
- Minimum 2 comparable elements
- Supports tables, points, and special blocks

#### `stats` - Statistics and Data Layout

**Purpose:** Display statistical information and data visualizations

**Characteristics:**
- Optimized for charts and tables
- Clean presentation of numerical data
- Focus on data clarity

**Strict Mode:**
```slidelang
SLIDE stats
  title: "Performance Metrics"
  TABLE
    headers: ["Metric", "Q3", "Q4", "Growth"]
    rows: [
      ["Revenue", "$1.2M", "$1.8M", "+50%"],
      ["Users", "15K", "23K", "+53%"]
    ]
```

**Flex Mode:**
```slidelang
---
layout: stats
---

# Performance Metrics

| Metric | Q3 | Q4 | Growth |
|--------|----|----|--------|
| Revenue | $1.2M | $1.8M | +50% |
| Users | 15K | 23K | +53% |
```

**Validation Rules:**
- Must include data elements (charts, tables, or metrics)
- Minimum 1 data visualization element
- Avoids code blocks and diagrams

### Technical and Content Layouts

#### `code_example` - Code Example Layout

**Purpose:** Technical documentation and code demonstrations

**Characteristics:**
- Optimized for code readability
- Syntax highlighting support
- Clear technical presentation

**Strict Mode:**
```slidelang
SLIDE code_example
  title: "API Implementation"
  TEXT
    Basic usage example:
  CODE javascript
    const api = new DataProcessor({
      endpoint: 'https://api.example.com',
      apiKey: process.env.API_KEY
    });
    
    const result = await api.process(inputData);
```

**Flex Mode:**
```slidelang
---
layout: code_example
---

# API Implementation

Basic usage example:

```javascript
const api = new DataProcessor({
  endpoint: 'https://api.example.com',
  apiKey: process.env.API_KEY
});

const result = await api.process(inputData);
```
```

**Validation Rules:**
- Must contain at least one code block
- Should include explanatory text
- Optimized for technical content

### Marketing and Communication Layouts

#### `hero` - Hero/Impact Visual Layout

**Purpose:** Maximum visual impact with hero elements

**Characteristics:**
- Full-screen background image support
- Overlay text with visual effects
- Prominent call-to-action
- Cinematic design

**Strict Mode:**
```slidelang
SLIDE hero
  background: "images/hero-bg.jpg"
  title: "Revolutionizing the Future"
  subtitle: "The next generation is here"
  cta: "Discover More"
```

**Flex Mode:**
```slidelang
---
layout: hero
background: "images/hero-bg.jpg"
---

# Revolutionizing the Future
## The next generation is here

[Discover More](#cta){.btn-primary}
```

**Validation Rules:**
- Must have a title
- Maximum 3 content elements
- Avoids complex elements (code, tables, charts)

#### `testimonial` - Testimonial Layout

**Purpose:** Display customer testimonials and social proof

**Characteristics:**
- Quote-focused design
- Author attribution with photo support
- Credibility-focused styling

**Strict Mode:**
```slidelang
SLIDE testimonial
  quote: "This solution increased our productivity by 300% in just three months."
  author: "Sarah Johnson"
  position: "CEO"
  company: "TechCorp"
  avatar: "images/sarah.jpg"
```

**Flex Mode:**
```slidelang
---
layout: testimonial
---

> "This solution increased our productivity by 300% in just three months."
> 
> **— Sarah Johnson, CEO of TechCorp**

![Sarah Johnson](images/sarah.jpg)
```

**Validation Rules:**
- Should include quote and author information
- Maximum 3 content elements
- Focus on testimonial content

#### `call_to_action` - Call to Action Layout

**Purpose:** Drive specific user actions

**Characteristics:**
- Action-oriented design
- Prominent buttons and links
- Urgency and motivation focus

**Strict Mode:**
```slidelang
SLIDE call_to_action
  title: "Ready to Get Started?"
  subtitle: "Join thousands of satisfied customers"
  primary_cta: "Start Free Trial"
  secondary_cta: "Learn More"
```

**Flex Mode:**
```slidelang
---
layout: call_to_action
---

# Ready to Get Started?
## Join thousands of satisfied customers

[Start Free Trial](#primary){.btn-primary}
[Learn More](#secondary){.btn-secondary}
```

**Validation Rules:**
- Must include action-oriented content
- Should contain call-to-action elements
- Maximum 3 elements for focus

### Organization and Process Layouts

#### `timeline` - Timeline Layout

**Purpose:** Display chronological information and timelines

**Characteristics:**
- Chronological visual layout
- Clear temporal progression
- Event-focused design

**Strict Mode:**
```slidelang
SLIDE timeline
  title: "Project Roadmap"
  events: [
    { date: "Q1 2025", title: "Planning Phase", status: "completed" },
    { date: "Q2 2025", title: "Development", status: "in-progress" },
    { date: "Q3 2025", title: "Testing", status: "planned" }
  ]
```

**Flex Mode:**
```slidelang
---
layout: timeline
---

# Project Roadmap

- **Q1 2025** - Planning Phase ✅
- **Q2 2025** - Development 🔄
- **Q3 2025** - Testing 📅
```

**Validation Rules:**
- Minimum 2 temporal events
- Maximum 6 timeline elements
- Focus on chronological content

#### `process` - Process/Methodology Layout

**Purpose:** Explain step-by-step processes and methodologies

**Characteristics:**
- Sequential step layout
- Process flow visualization
- Clear progression indicators

**Strict Mode:**
```slidelang
SLIDE process
  title: "Development Process"
  steps: [
    "Requirements Analysis",
    "Design & Architecture", 
    "Implementation",
    "Testing & QA",
    "Deployment"
  ]
```

**Flex Mode:**
```slidelang
---
layout: process
---

# Development Process

1. **Requirements Analysis** - Understanding project needs
2. **Design & Architecture** - Planning the solution
3. **Implementation** - Building the product
4. **Testing & QA** - Ensuring quality
5. **Deployment** - Releasing to production
```

**Validation Rules:**
- Minimum 2 sequential steps
- Maximum 6 process steps
- Focus on procedural content

### Business and Marketing Layouts

#### `pricing` - Pricing/Plans Layout

**Purpose:** Display pricing tiers and plan comparisons

**Characteristics:**
- Pricing table optimization
- Plan comparison features
- Clear value proposition

**Strict Mode:**
```slidelang
SLIDE pricing
  title: "Choose Your Plan"
  plans: [
    { name: "Basic", price: "$29/month", features: ["5 users", "10GB storage"] },
    { name: "Pro", price: "$79/month", features: ["50 users", "100GB storage"] }
  ]
```

**Flex Mode:**
```slidelang
---
layout: pricing
---

# Choose Your Plan

| Feature | Basic | Pro |
|---------|-------|-----|
| Price | $29/month | $79/month |
| Users | 5 | 50 |
| Storage | 10GB | 100GB |
```

**Validation Rules:**
- Must include pricing information
- Should contain plan comparison
- Maximum 4 pricing tiers

#### `team` - Team Presentation Layout

**Purpose:** Introduce team members and organizational structure

**Characteristics:**
- Member profile layout
- Role and department display
- Professional presentation

**Strict Mode:**
```slidelang
SLIDE team
  title: "Meet Our Team"
  members: [
    { name: "Alice Johnson", role: "Lead Developer", email: "alice@company.com" },
    { name: "Bob Smith", role: "Product Manager", email: "bob@company.com" }
  ]
```

**Flex Mode:**
```slidelang
---
layout: team
---

# Meet Our Team

## Alice Johnson
**Lead Developer**  
alice@company.com

## Bob Smith  
**Product Manager**  
bob@company.com
```

**Validation Rules:**
- Should include member information with roles
- Maximum 8 team members
- Focus on person-related content

#### `dashboard` - Dashboard/Metrics Layout

**Purpose:** Display real-time interfaces and metrics

**Characteristics:**
- Interface simulation styling
- Integrated charts and metrics
- Real-time data presentation

**Strict Mode:**
```slidelang
SLIDE dashboard
  title: "Control Panel"
  widgets: [
    { type: "metric", title: "Total Sales", value: "$2.4M", change: "+15%" },
    { type: "chart", title: "Monthly Growth", data: "chart_data.json" }
  ]
```

**Flex Mode:**
```slidelang
---
layout: dashboard
---

# Control Panel

## Key Metrics
- **Total Sales:** $2.4M (+15%)
- **Active Users:** 15,847 (+8%)
- **Conversion Rate:** 3.2% (+0.5%)

<<chart: line>>
  title: "Monthly Growth"
  data: [125, 145, 167, 189]
```

**Validation Rules:**
- Should include metrics or charts
- Maximum 6 dashboard elements
- Focus on data visualization

#### `before_after` - Before/After Layout

**Purpose:** Show transformations and improvements

**Characteristics:**
- Split-screen comparison design
- Clear before/after sections
- Transformation focus

**Strict Mode:**
```slidelang
SLIDE before_after
  title: "Transformation Results"
  before: "Manual process taking 8 hours daily"
  after: "Automated process completed in 15 minutes"
```

**Flex Mode:**
```slidelang
---
layout: before_after
---

# Transformation Results

## Before
Manual process taking 8 hours daily
- Time-consuming
- Error-prone
- Resource intensive

## After  
Automated process completed in 15 minutes
- Efficient
- Accurate
- Cost-effective
```

**Validation Rules:**
- Must have both "before" and "after" sections
- Minimum 2 comparison elements
- Maximum 4 total elements

### Special Purpose Layouts

#### `feature_showcase` - Feature Showcase Layout

**Purpose:** Highlight product features and benefits

**Characteristics:**
- Feature-focused layout with icons
- Benefit highlighting
- Multi-feature support

**Strict Mode:**
```slidelang
SLIDE feature_showcase
  title: "Key Features"
  features: [
    { icon: "🚀", title: "Ultra Fast", description: "Processing in milliseconds" },
    { icon: "🔒", title: "Secure", description: "End-to-end encryption" }
  ]
```

**Flex Mode:**
```slidelang
---
layout: feature_showcase
---

# Key Features

## 🚀 Ultra Fast
Processing in milliseconds

## 🔒 Secure  
End-to-end encryption

## 📊 Analytics
Real-time insights
```

**Validation Rules:**
- Should highlight at least 2 features
- Maximum 6 feature highlights
- Focus on feature-benefit content

#### `closing` - Closing Slide Layout

**Purpose:** Presentation conclusion and final thoughts

**Characteristics:**
- Clean, conclusive design
- Contact information support
- Thank you messaging

**Strict Mode:**
```slidelang
SLIDE closing
  heading: "Thank You"
  subtitle: "Questions & Discussion"
  contact: "john@company.com"
```

**Flex Mode:**
```slidelang
---
layout: closing
---

# Thank You
## Questions & Discussion

**Contact:** john@company.com  
**LinkedIn:** /in/johnsmith
```

**Validation Rules:**
- Should contain simple, conclusive content
- Maximum 3 elements
- Avoids complex elements

## Layout Validation

ZiraDocs automatically validates slide content against layout requirements:

### Validation Rules

Each layout includes specific validation rules:

- **Element restrictions** - Some layouts forbid certain element types
- **Minimum/maximum elements** - Controls content complexity
- **Required properties** - Ensures essential information is present
- **Content appropriateness** - Validates content matches layout purpose

### Validation Messages

The linter provides specific guidance when validation fails:

```bash
# Example validation messages
LAYOUT001: Title slides must have a 'heading' property
LAYOUT003: Content slides must have a 'title' property  
LAYOUT011: Timeline slides should have at least 2 temporal events
LAYOUT015: Feature showcase slides should highlight at least 2 features
```

## Mode Differences

### Layout Specification

**Strict Mode:**
- Layout specified in slide declaration: `SLIDE layout_name`
- Example: `SLIDE title`, `SLIDE comparison`

**Flex Mode:**
- Layout specified in frontmatter: `layout: layout_name`
- Can be auto-detected based on content patterns

### Syntax Comparison

**Code Example in Strict Mode:**
```slidelang
SLIDE code_example
  title: "My Code"
  CODE python
    print("Hello world")
  POINTS
    - Important point
```

**Same Content in Flex Mode:**
```slidelang
---
layout: code_example
---

# My Code

```python
print("Hello world")
```

- Important point
```

### Automatic Detection (Flex Mode Only)

Flex Mode can automatically detect layouts based on content:

```slidelang
# Welcome to Our Product
## Revolutionizing the industry

![Hero image](assets/hero.png)
```
*Automatically detected as 'hero' layout*

```slidelang
| Feature | Basic | Pro |
|---------|-------|-----|
| Users | 5 | 50 |
| Storage | 1GB | 100GB |
```
*Automatically detected as 'comparison' layout*

## CSS Styling

Each layout includes specialized CSS styling:

- **Typography adjustments** for content type
- **Layout grids** optimized for content structure  
- **Color schemes** appropriate for layout purpose
- **Responsive design** for all screen sizes
- **Animation effects** for visual enhancement

## Best Practices

1. **Choose appropriate layouts** - Match layout to content type
2. **Follow validation rules** - Address linter warnings promptly
3. **Keep content focused** - Respect element limits for clarity
4. **Test responsiveness** - Verify layouts work on all devices
5. **Use consistent styling** - Leverage layout-specific CSS
6. **Validate regularly** - Run linter during development

## CLI Integration

Layout validation works seamlessly with ZiraDocs CLI:

```bash
# Validate layouts during build
slidelang build presentation.slidelang

# Lint-only mode for layout checking
slidelang build --lint-only presentation.slidelang

# Debug layout detection
slidelang build presentation.slidelang --log-level debug
```

## Common Patterns

### Progressive Presentation Structure

```slidelang
SLIDE title          # Presentation introduction
SLIDE section        # Section introductions  
SLIDE content        # Main content slides
SLIDE comparison     # Feature/option comparisons
SLIDE stats          # Supporting data
SLIDE call_to_action # Drive user action
SLIDE closing        # Conclusion and thanks
```

### Technical Documentation Flow

```slidelang
SLIDE title          # API/Product introduction
SLIDE section        # Getting started section
SLIDE code_example   # Implementation examples
SLIDE dashboard      # Usage metrics/monitoring
SLIDE feature_showcase # Key capabilities
SLIDE pricing        # Plans and pricing
SLIDE closing        # Contact and next steps
```

### Marketing Presentation Flow

```slidelang
SLIDE hero           # Eye-catching introduction
SLIDE feature_showcase # Product highlights
SLIDE testimonial    # Social proof
SLIDE before_after   # Transformation stories
SLIDE pricing        # Plans and offers
SLIDE call_to_action # Drive conversion
SLIDE closing        # Thank you and contact
```

## Related Documentation

- [Strict Mode Syntax](strict-mode.md) - Using layouts in Strict Mode
- [Flex Mode Syntax](flex-mode.md) - Using layouts in Flex Mode
- [FrontMatter Configuration](frontmatter.md) - Layout configuration options
- [Advanced Elements](advanced-elements.md) - Complex content elements
- [Themes & Styling](../features/themes-styling.md) - Visual customization

## Examples

See the [Specialized Layouts Examples](../../examples/18_specialized_layouts/) directory for complete working examples of all layout types in both Strict and Flex modes.

---

**💡 Pro Tip:** Start with basic layouts (`title`, `content`, `section`) and gradually incorporate specialized layouts as your presentation complexity grows.
