# Headers and Footers

Headers and footers provide consistent elements across your entire presentation, including logos, page numbers, author information, and navigation elements. ZiraDocs offers a flexible system for configuring these elements both globally and per-slide.

> **Note:** Basic header/footer configuration is covered in [FrontMatter Configuration](../language-reference/frontmatter.md#layout-configuration). This guide focuses on advanced usage patterns and customization.

## Quick Start

Simple configuration in your FrontMatter:

```yaml
---
title: "My Corporate Presentation"
author: "John Doe"
company: "TechCorp Inc."
date: "2024-12-27"
mode: flex

header:
  enabled: true
  text:
    position: "right"
    content: "{{author}} • {{date}}"
    style: "subtitle"

footer:
  enabled: true
  page_numbers:
    enabled: true
    format: "{{current}} / {{total}}"
    position: "right"
    exclude_title_slides: true
---
```

## Advanced Header Configuration

### Logo Integration

```yaml
header:
  enabled: true
  logo:
    src: "./assets/company-logo.png"
    alt: "Company Logo"
    position: "left"              # left, center, right
    height: "40px"
    width: "auto"                 # maintains aspect ratio
    link: "https://company.com"   # optional clickable link
    retina: "./assets/logo@2x.png" # high-resolution version
  text:
    position: "right"
    content: "{{department}} • {{quarter}}"
    style: "subtitle"
    color: "#666666"
    font_size: "14px"
  divider: true                   # separator line below header
```

### Dynamic Text Content

Headers support rich variable interpolation:

```yaml
variables:
  department: "Sales"
  quarter: "Q4 2024"
  presenter: "Maria González"
  project_status: "In Progress"

header:
  text:
    content: "{{department}} - {{presenter}}"
    style: "subtitle"
```

Available variables include:
- `{{title}}` - Presentation title
- `{{author}}` - Author name
- `{{date}}` - Presentation date
- `{{company}}` - Company name
- `{{section}}` - Current section
- `{{slide_title}}` - Current slide title
- `{{time}}` - Current time
- Any custom variables defined in `variables:`

## Advanced Footer Configuration

### Page Number Formatting

```yaml
footer:
  page_numbers:
    enabled: true
    format: "{{current}} / {{total}}"    # "5 / 20"
    position: "right"                    # left, center, right
    exclude_title_slides: true           # skip title slides
    exclude_section_slides: false       # include section slides
    start_from: 2                        # start counting from slide 2
    style: "caption"                     # text styling
    prefix: "Page "                      # "Page 5 / 20"
    suffix: ""                           # custom suffix
```

**Format Examples:**
- `"{{current}} / {{total}}"` → "5 / 20"
- `"{{current}}"` → "5"
- `"Slide {{current}} of {{total}}"` → "Slide 5 of 20"
- `"{{current}} | {{section_name}}"` → "5 | Introduction"

### Multi-Position Text

```yaml
footer:
  text:
    left: "© 2024 {{company}}"      # left-aligned text
    center: "{{title}}"             # center-aligned text
    right: "{{date}}"               # right-aligned text
    style: "caption"                # consistent styling
    color: "#999999"                # custom color
```

### Social Media Links

```yaml
footer:
  social_links:
    - platform: "twitter"
      handle: "@company"
      position: "left"
      icon: true                    # show platform icon
    - platform: "linkedin"
      url: "company/mycompany"
      position: "left"
    - platform: "github"
      url: "mycompany"
      position: "right"
    - platform: "website"
      url: "https://mycompany.com"
      text: "mycompany.com"         # custom display text
      position: "center"
```

## Per-Slide Configuration

### Flex Mode Override

```markdown
---
layout: content
header:
  text:
    content: "Special Section"
    color: "#cc0000"
    position: "left"
footer:
  text:
    left: "CONFIDENTIAL INFORMATION"
    center: "⚠️ RESTRICTED ACCESS ⚠️"
    right: "CLASSIFIED"
---

# My Special Slide
Content with custom header/footer...
```

### Strict Mode Configuration

```slidelang
SLIDE content
  title: "My Special Slide"
  header_text: "Special Section"
  footer_left: "Confidential Material"
  exclude_page_numbers: false
  
  TEXT
    Slide content...
```

### Conditional Display with Directives

```markdown
@header: false
@footer: false

# Slide Without Header/Footer
Clean slide content...

---

@header_text: "Special Presentation"
@footer_left: "Draft - Do Not Distribute"

# Slide with Custom Text
Content with modified headers/footers...
```

## Layout-Specific Configuration

Different slide types can have unique header/footer settings:

```yaml
layout_defaults:
  title:
    header:
      enabled: false             # no header on title slides
    footer:
      page_numbers:
        enabled: false           # no page numbers on title slides
  
  title_slide:
    header:
      enabled: false
    footer:
      text:
        center: ""               # empty center text
  
  content:
    header:
      enabled: true              # full header on content slides
    footer:
      enabled: true
  
  section:
    header:
      minimal: true              # simplified version
    footer:
      page_numbers:
        format: "Section {{section}} - {{current}}/{{total}}"
```

## Visual Customization

### Predefined Styles

Available text styles:
- `title` - Large text for titles
- `subtitle` - Medium text for subtitles  
- `body` - Normal body text
- `caption` - Small text, ideal for footers
- `minimalist` - Very subtle styling

### Custom CSS Integration

```yaml
header:
  custom_css: |
    .header-container {
      background: linear-gradient(90deg, #1e3c72, #2a5298);
      color: white;
      padding: 8px 16px;
      border-radius: 4px;
    }
    .header-logo {
      filter: brightness(0) invert(1);
    }

footer:
  custom_css: |
    .footer-container {
      border-top: 2px solid #1e3c72;
      background: rgba(30, 60, 114, 0.05);
    }
    .page-numbers {
      font-weight: bold;
      color: #1e3c72;
    }
```

## Common Use Cases

### Corporate Presentation

```yaml
header:
  logo:
    src: "./assets/company-logo.png"
    position: "left"
    height: "35px"
  text:
    position: "right"
    content: "{{department}} • {{quarter}}"
    style: "subtitle"

footer:
  page_numbers:
    enabled: true
    format: "{{current}} / {{total}}"
    position: "right"
  text:
    left: "© 2024 {{company}}"
    center: "{{title}}"
    right: ""
```

### Academic Presentation

```yaml
header:
  text:
    position: "center"
    content: "{{author}} • {{institution}}"
    style: "caption"

footer:
  page_numbers:
    enabled: true
    format: "{{current}}"
    position: "center"
  text:
    left: "{{conference}} 2024"
    right: "{{date}}"
```

### Minimalist Design

```yaml
header:
  enabled: false

footer:
  page_numbers:
    enabled: true
    format: "{{current}}"
    position: "right"
    style: "minimalist"
  text:
    left: ""
    center: ""
    right: ""
```

### Strong Branding

```yaml
header:
  logo:
    src: "./assets/brand-header.png"
    position: "center"
    height: "60px"
  divider: true

footer:
  logo:
    src: "./assets/brand-footer.png"
    position: "center"
    height: "30px"
  social_links:
    - platform: "twitter"
      handle: "@brand"
      position: "left"
    - platform: "website"
      url: "brand.com"
      position: "right"
  divider: true
```

## Responsive Design

Headers and footers automatically adapt to different formats:

```yaml
responsive:
  mobile:
    header:
      logo:
        height: "30px"           # smaller logo on mobile
      text:
        font_size: "12px"        # smaller text
    footer:
      text:
        center: ""               # hide center text on mobile
  
  print:
    header:
      enabled: false             # no header when printing
    footer:
      page_numbers:
        enabled: true
        format: "Page {{current}} of {{total}}"
      text:
        left: "Printed: {{timestamp}}"
        center: ""
        right: ""
```

## Best Practices

### ✅ Recommended Practices

- **Consistency**: Maintain the same header/footer style throughout your presentation
- **Legibility**: Use colors that contrast well with your background
- **Useful Information**: Include only relevant information (author, date, page numbers)
- **Responsive Design**: Consider how headers/footers will appear on different screen sizes
- **Appropriate Branding**: Use corporate logos and colors when appropriate
- **Variable Usage**: Leverage dynamic variables to keep information current

### ❌ Avoid These Pitfalls

- **Excessive Height**: Headers/footers that take too much space from content
- **Information Overload**: Too much information in small spaces
- **Distracting Colors**: Colors that draw attention away from main content
- **Poor Quality Images**: Pixelated or low-quality logos
- **Outdated Information**: Static dates or information that becomes stale
- **Inconsistent Positioning**: Different layouts for similar slide types

## Theme Integration

Themes can provide default header/footer configurations:

```yaml
# In theme definition
theme_defaults:
  header:
    style: "corporate"
    logo:
      height: "35px"
    text:
      style: "subtitle"
      color: "#333333"
  
  footer:
    style: "professional"
    page_numbers:
      style: "elegant"
    divider: true
```

## Related Documentation

- [FrontMatter Configuration](../language-reference/frontmatter.md) - Basic configuration
- [Variables and Templates](variables-templates.md) - Dynamic content
- [Specialized Layouts](../language-reference/specialized-layouts.md) - Layout-specific settings
- [Directives and Configuration](../language-reference/directives-configuration.md) - Per-slide control

## Examples in Action

See the practical examples in the repository:

- `examples/18_specialized_layouts/18.3_headers_footers_basic_flex.slidelang` - Basic setup
- `examples/18_specialized_layouts/18.4_headers_footers_advanced_flex.slidelang` - Advanced configuration
- `examples/18_specialized_layouts/18.5_headers_footers_granular_flex.slidelang` - Per-slide overrides
