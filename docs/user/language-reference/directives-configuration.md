# Directives and Advanced Configuration

ZiraDocs supports advanced directives and configuration options that provide granular control over presentation behavior, layout, and content inclusion beyond the global [FrontMatter configuration](frontmatter.md).

## Overview

Directives are special markers within the document body that control specific aspects of rendering, behavior, or content processing for individual slides or sections. They complement global FrontMatter settings with slide-level precision.

## Inline Directives

### Slide-Specific Layout Override

Override layout for individual slides while maintaining global theme consistency.

**Flex Mode:**
```slidelang
---
layout: two_columns_image_right
---

# Special Layout Slide

**Left Column:**
- Main content point
- Supporting details

**Right Column:**  
![Featured Image](assets/feature.png)
```

**Strict Mode:**
```slidelang
SLIDE content
  layout_override: "two_columns_image_right"
  title: "Special Layout Slide"
  
  COLUMN_LEFT
    POINTS
      - Main content point
      - Supporting details
  
  COLUMN_RIGHT
    IMAGE "assets/feature.png"
```

### Style and CSS Control

Apply specific styling or CSS classes to content blocks.

**Flex Mode:**
```slidelang
::: style class="highlight-box warning" css="border: 2px solid red; padding: 1em;" :::
**Warning:** This message requires immediate attention.
:::

::: style class="branded-callout" :::
Key insight that aligns with brand guidelines.
:::
```

**Strict Mode:**
```slidelang
SLIDE content
  title: "Important Information"
  
  TEXT style_class="highlight-box warning" custom_css="border: 2px solid red; padding: 1em;"
    Warning: This message requires immediate attention.
  
  TEXT style_class="branded-callout"
    Key insight that aligns with brand guidelines.
```

## Content Inclusion Directives

### External File Inclusion

Include content from external files for modular presentation development.

**Syntax (Both Modes):**
```slidelang
# Main Presentation Content

<!-- Include detailed analysis from external file -->
::: include path="./sections/market-analysis.md" :::

## Code Examples

<!-- Include specific lines from code file -->
::: include_code path="./examples/api-demo.py" language="python" lines="15-35" :::

<!-- Include entire file with syntax highlighting -->
::: include_code path="./config/database-setup.yaml" language="yaml" :::
```

### Conditional Content

Include content based on output format or conditions.

```slidelang
::: include_if format="html" :::
Interactive elements appear here in web version.
:::

::: include_if format="pdf" :::
Static alternative content for PDF export.
:::

::: include_unless format="print" :::
This content is excluded from printed versions.
:::
```

## Block Processing Control

### Raw Content Blocks

Prevent ZiraDocs parsing for literal content preservation.

```slidelang
::: raw :::
This content bypasses ZiraDocs processing entirely.
Even --- slide separators and # headers are ignored.
SLIDE fake_element
  TEXT "This won't be processed as ZiraDocs syntax"
::: end_raw :::
```

### Comment Blocks

Add development notes that won't appear in output.

```slidelang
::: comment :::
TODO: Update this section with Q4 data
Designer feedback: Consider different color scheme
Review with legal team before final presentation
::: end_comment :::
```

## Section-Level Configuration

Apply temporary configuration changes to specific sections.

### Theme Override
```slidelang
::: config theme="dark_presentation" transition="fade" :::

# Dark Theme Section
Content using dark theme styling...

---

# Another Dark Slide
Continues with dark theme...

::: end_config :::

---

# Back to Default
Returns to global theme configuration.
```

### Animation and Transition Control
```slidelang
::: config transition="zoom" animation_speed="slow" :::

# Slow Zoom Transitions
This section uses slower, zoom-based transitions.

::: end_config :::
```

## Advanced Presenter Features

### Enhanced Presenter Notes

While basic presenter notes use the `@notes:` directive, advanced configuration allows structured presenter information.

```slidelang
SLIDE content
  title: "Financial Summary"
  
  @notes:
  Key talking points:
  - Emphasize 25% growth over last quarter
  - Address concerns about market volatility
  - Estimated speaking time: 3 minutes
  
  @notes_timing: 180  # seconds
  @notes_priority: high
  
  TEXT
    Our revenue has increased significantly this quarter.
```

### Slide Metadata
```slidelang
SLIDE content
  title: "Market Analysis"
  slide_id: "market_overview_2024"
  tags: ["analysis", "market", "strategy"]
  review_status: "approved"
  last_updated: "2024-01-15"
  
  TEXT
    Market conditions show positive trends...
```

## Project Configuration Files

For organization-wide or project-level settings, ZiraDocs supports configuration files.

### `slidelang.config.yaml`

```yaml
# Project-wide ZiraDocs configuration

# Default settings for all presentations
defaults:
  mode: flex
  theme: "corporate_standard"
  slide_aspect_ratio: "16:9"

# Asset management
assets:
  base_paths:
    - "./shared/images"
    - "./brand/logos"
    - "./templates"
  
  # Optimization settings
  image_optimization:
    max_width: 1920
    quality: 85
    format_preference: ["webp", "png", "jpg"]

# Plugin configuration
plugins:
  enabled:
    - "slidelang-charts"
    - "slidelang-analytics"
  
  config:
    slidelang_charts:
      default_library: "chartjs"
      color_scheme: "corporate"
    
    slidelang_analytics:
      tracking_id: "GA-XXXX-YYYY"
      anonymize_ip: true

# Build and processing
build:
  # Pre-processing hooks
  pre_process:
    - "./scripts/validate_content.js"
    - "./scripts/fetch_data.py"
  
  # Post-processing hooks  
  post_process:
    - "./scripts/optimize_assets.js"
    - "./scripts/generate_index.html"
  
  # Output optimization
  optimization:
    minify_css: true
    compress_images: true
    bundle_assets: true

# Development settings
development:
  auto_reload: true
  log_level: "debug"
  serve_port: 3000
  
# Security policies  
security:
  allow_external_scripts: false
  sanitize_html: true
  validate_urls: true
```

### Environment-Specific Configuration

```yaml
# ziradocs.com.yaml - Development overrides
defaults:
  theme: "debug_theme"

development:
  log_level: "verbose"
  show_build_time: true
  enable_source_maps: true
```

```yaml
# slidelang.prod.yaml - Production settings
build:
  optimization:
    minify_css: true
    compress_images: true
    
security:
  strict_mode: true
  validate_all_links: true
```

## Validation and Linting

Directives integrate with ZiraDocs's validation system.

### Directive Validation
```bash
# Validate directive usage
slidelang lint presentation.slidelang

# Check for unsupported directives
slidelang lint --strict presentation.slidelang

# Validate included files exist
slidelang lint --check-includes presentation.slidelang
```

### Configuration Validation
```bash
# Validate configuration files
slidelang config validate

# Test configuration with dry-run
slidelang build --dry-run --config=slidelang.prod.yaml
```

## Best Practices

### Performance Considerations
- **Minimize external includes** - Each inclusion adds processing time
- **Use conditional content wisely** - Excessive conditions can complicate maintenance
- **Cache included content** - Configure build system to cache frequently included files

### Organization
- **Group related directives** - Keep similar directives together for readability
- **Document complex configurations** - Use comment blocks to explain advanced setups
- **Version configuration files** - Track configuration changes alongside content

### Security
- **Validate external content** - Ensure included files are safe and authorized
- **Limit raw blocks** - Use raw directives sparingly to maintain security
- **Review custom CSS** - Validate inline styles don't introduce vulnerabilities

## Migration from Legacy Configurations

If updating from earlier versions:

1. **Presenter Notes**: Legacy HTML-based notes automatically migrate to metadata
2. **Layout Specifications**: Old layout syntax remains compatible
3. **Include Directives**: File paths may need updating for new asset resolution

## Error Handling

Common directive issues and solutions:

- **File not found**: Check include paths and asset configuration
- **Invalid CSS**: Validate custom styling syntax  
- **Conflicting configurations**: Later directives override earlier ones
- **Unsupported directive**: Update ZiraDocs or check plugin availability

## Related Documentation

- [FrontMatter Configuration](frontmatter.md) - Global presentation settings
- [Specialized Layouts](specialized-layouts.md) - Built-in layout options  
- [Dynamic & Interactive Features](../features/dynamic-interactive.md) - Animation and interaction directives
- [Advanced Elements](advanced-elements.md) - Complex content elements
- [CLI Reference](../cli-reference/) - Command-line tools and options

---

**💡 Pro Tip:** Start with simple directives like style classes and external includes. Gradually incorporate advanced configuration as your presentation complexity grows.
