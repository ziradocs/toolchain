# FrontMatter Configuration

FrontMatter is a YAML block at the beginning of ZiraDocs files that defines metadata and global configuration for presentations. This block is delimited by three dashes (`---`) and is crucial for defining the [operating mode (Strict or Flex)](syntax-overview.md#operating-modes) and other essential settings.

## Quick Example

```yaml
---
title: My Amazing Presentation
author: John Doe
date: "2024-07-15"
mode: flex  # Required: strict or flex
theme: default
output:
  format: [html, pdf]
  path: "./dist"
---

# First slide content starts here...
```

## Essential Configuration

### `mode` (required)
Specifies the syntax mode for the document body. **This field is mandatory**.

- `flex`: Enables [extended Markdown mode](flex-mode.md)
- `strict`: Enables [formal keyword-based syntax](strict-mode.md)

```yaml
mode: flex
```

### `title` (string)
The main title of the presentation. Often displayed on the first slide or browser title.

```yaml
title: "Introduction to ZiraDocs"
```

### `author` (string)
The name of the author(s) of the presentation.

```yaml
author: "The ZiraDocs Team"
```

### `date` (string)
The presentation date or last modification date. Can be free text or formatted date (e.g., `YYYY-MM-DD`).

```yaml
date: "July 2024"
```

## Output Configuration

### `output` (object)
Settings related to presentation compilation and output.

```yaml
output:
  format: ["html", "pdf"]     # Output formats
  path: "./build"             # Output directory
  filename: "my_presentation" # Base filename (without extension)
```

**Available formats:**
- `html` - Interactive web presentation
- `pdf` - Static PDF document

### `theme` (string)
The visual theme name to apply to the presentation.

```yaml
theme: "ocean_blue"
```

## Customization

### `custom_css` (string | array)
Path to custom CSS file(s) for appearance customization.

```yaml
custom_css: "./assets/css/my_styles.css"
# or multiple files
custom_css:
  - "./base_styles.css"
  - "./theme_overrides.css"
```

### `custom_js` (string | array)
Path to custom JavaScript file(s) for additional functionality.

```yaml
custom_js: "./scripts/interactivity.js"
```

## Variables and Templating

### `variables` (object)
Define custom variables accessible throughout the presentation using template syntax `{{variable_name}}`.

```yaml
variables:
  company: "Acme Corp"
  department: "Sales"
  quarter: "Q3"
  version: "1.0"
```

**Usage in content:**
```markdown
Welcome to {{company}}'s {{department}} presentation for {{quarter}} {{date}}.
```

## Layout Configuration

### `slide_aspect_ratio` (string)
Defines the aspect ratio for slides.

```yaml
slide_aspect_ratio: "16:9"  # or "4:3"
```

### `header` (object)
Global header configuration for all slides.

```yaml
header:
  enabled: true
  logo:
    src: "./assets/logo.png"
    alt: "Company Logo"
    position: "left"          # left, center, right
    height: "40px"
  text:
    position: "right"
    content: "{{author}} • {{date}}"
    style: "subtitle"
  divider: true               # Separator line below header
```

### `footer` (object)
Global footer configuration for all slides.

```yaml
footer:
  enabled: true
  page_numbers:
    enabled: true
    format: "{{current}} / {{total}}"
    position: "right"
    exclude_title_slides: true
  text:
    left: "Confidential - {{company}}"
    center: "{{title}}"
    right: "{{date}}"
  divider: true               # Separator line above footer
```

### `layout_defaults` (object)
Override header/footer settings for specific slide types.

```yaml
layout_defaults:
  title:                      # Title slides
    header:
      enabled: false
    footer:
      page_numbers:
        enabled: false
  section:                    # Section divider slides
    header:
      minimal: true           # Simplified header
    footer:
      page_numbers:
        format: "Section {{section}} - {{current}}/{{total}}"
```

## Advanced Configuration

### `language` (string)
Main content language for accessibility and tools.

```yaml
language: "en-US"
```

### `presenter_notes` (boolean)
Controls whether presenter notes are generated or processed.

```yaml
presenter_notes: true
```

### `plugins` (array)
Configuration for plugins that extend ZiraDocs functionality.

```yaml
plugins:
  - name: "slidelang-chart-plugin"
    version: "^1.2"
    config:
      default_chart_type: "bar"
  - "slidelang-seo-enhancer"
```

### `seo` (object)
SEO optimization settings for HTML output.

```yaml
seo:
  meta_description: "A presentation about the wonders of ZiraDocs."
  keywords: ["slidelang", "presentations", "dsl", "ai"]
```

### `security` (object)
Security policies for embedded content or scripts.

```yaml
security:
  iframe_sandbox_policy: "allow-scripts allow-same-origin"
```

### `analytics` (object)
Configuration for web analytics tools.

```yaml
analytics:
  google_analytics_id: "UA-XXXXX-Y"
```

## Complete Example

Here's a comprehensive FrontMatter example showing most available options:

```yaml
---
title: "Q3 Sales Performance Review"
author: "Sales Team"
date: "2024-09-15"
mode: flex
theme: "corporate"
language: "en-US"

output:
  format: ["html", "pdf"]
  path: "./presentations/q3-review"
  filename: "sales-performance-q3-2024"

variables:
  company: "Acme Corporation"
  quarter: "Q3"
  year: "2024"
  presenter: "John Smith"
  department: "Sales"

header:
  enabled: true
  logo:
    src: "./assets/acme-logo.png"
    alt: "Acme Corporation"
    position: "left"
    height: "35px"
  text:
    position: "right"
    content: "{{department}} • {{quarter}} {{year}}"
    style: "subtitle"
  divider: true

footer:
  enabled: true
  page_numbers:
    enabled: true
    format: "{{current}} / {{total}}"
    position: "right"
    exclude_title_slides: true
  text:
    left: "Confidential - {{company}}"
    center: "{{title}}"
    right: "{{date}}"
  divider: true

layout_defaults:
  title:
    header:
      enabled: false
    footer:
      page_numbers:
        enabled: false

slide_aspect_ratio: "16:9"
presenter_notes: true

custom_css: "./styles/corporate-theme.css"

seo:
  meta_description: "{{quarter}} {{year}} sales performance review for {{company}}"
  keywords: ["sales", "performance", "quarterly", "review"]
---

# {{title}}

Welcome to {{company}}'s {{quarter}} {{year}} Sales Performance Review.

**Presenter:** {{presenter}}  
**Date:** {{date}}
```

## Best Practices

1. **Always specify mode** - The `mode` field is required for proper parsing
2. **Use meaningful variables** - Define variables for repeated content
3. **Organize output settings** - Keep all output-related configuration in the `output` object
4. **Test your themes** - Ensure custom CSS and themes work with your content
5. **Validate YAML syntax** - Invalid YAML will prevent presentation compilation

## Related Documentation

- [Syntax Overview](syntax-overview.md) - Learn about operating modes
- [Strict Mode Syntax](strict-mode.md) - Formal keyword-based syntax
- [Flex Mode Syntax](flex-mode.md) - Extended Markdown syntax
- [Variable Templates](../advanced/variables-templates.md) - Advanced templating features

## Migration Notes

If you're updating from older versions:
- The `mode` field is now required
- Some theme names may have changed
- Output format specifications have been standardized

Check the [CHANGELOG](../../CHANGELOG.md) for breaking changes between versions.
