# Choosing Your Syntax Mode

ZiraDocs offers two syntax modes to accommodate different workflows and preferences. This guide helps you choose the right mode for your project and understand when to switch between them.

## Quick Decision Guide

**Choose Flex Mode if you:**
- ✅ Are familiar with Markdown
- ✅ Need rapid content creation and prototyping  
- ✅ Prefer minimal syntax overhead
- ✅ Work on personal or small team projects
- ✅ Create simple to medium complexity presentations

**Choose Strict Mode if you:**
- ✅ Need maximum parser predictability
- ✅ Generate presentations programmatically or via AI
- ✅ Work in large teams requiring consistent structure
- ✅ Create complex presentations with specialized layouts
- ✅ Integrate with automated workflows

## Detailed Comparison

| Feature | Strict Mode | Flex Mode | Notes |
|---------|-------------|-----------|-------|
| **Syntax Style** | Formal, explicit keywords | Extended Markdown | Flex feels more natural to Markdown users |
| **Verbosity** | More verbose | ~60% less verbose | Significant reduction in markup required |
| **Learning Curve** | Steeper initially | Easier for Markdown users | Clear rules vs. familiar patterns |
| **Error Handling** | Strict validation | More tolerant | Strict catches errors early; Flex is forgiving |
| **Parser Predictability** | 100% predictable | Type inference based | Important for automated tools |
| **Team Collaboration** | Enforces consistency | Allows style variation | Consider team size and standards |
| **Content Generation** | Ideal for AI/automated | Natural for human writing | Different optimization targets |

## Use Case Scenarios

### Individual Content Creators

**Recommendation: Flex Mode**

Perfect for personal presentations, blog-to-slides conversion, and rapid iteration:

```yaml
---
mode: flex
title: "My Quick Presentation"
---

# Welcome Slide
Quick content creation with familiar Markdown.

---

## Main Points
- Point one with **bold emphasis**
- Point two with *italic styling*
- Point three with `code snippets`
```

### Enterprise Teams

**Recommendation: Strict Mode**

Ensures consistency across team members and integrates with corporate workflows:

```yaml
---
mode: strict
title: "Quarterly Business Review"
---

SLIDE title
  heading: "Q4 2024 Results"
  subtitle: "Exceeding all targets"

SLIDE stats
  title: "Key Metrics"
  TABLE
    headers: ["Metric", "Target", "Actual", "Variance"]
    rows: [
      ["Revenue", "$2.5M", "$3.1M", "+24%"],
      ["Users", "10K", "12.5K", "+25%"]
    ]
```

### Technical Documentation

**Recommendation: Depends on complexity**

- **Simple docs:** Flex Mode for natural writing flow
- **Complex specs:** Strict Mode for precise structure

**Flex Example:**
```slidelang
# API Overview

## Authentication

Use Bearer tokens in the Authorization header:

```bash
curl -H "Authorization: Bearer your-token" \
  https://api.example.com/data
```
```

**Strict Example:**
```slidelang
SLIDE code_example
  title: "API Authentication"
  TEXT
    Use Bearer tokens in the Authorization header:
  CODE bash
    curl -H "Authorization: Bearer your-token" \
      https://api.example.com/data
```

### AI-Generated Content

**Recommendation: Strict Mode**

AI models benefit from explicit structure and predictable syntax:

```yaml
---
mode: strict
title: "AI-Generated Market Analysis"
---

SLIDE title
  heading: "Market Trends 2024"
  subtitle: "Data-driven insights"

SLIDE stats
  title: "Growth Metrics"
  <<chart: bar>>
    data: [["Q1", 45], ["Q2", 52], ["Q3", 61], ["Q4", 73]]
    series: ["Revenue (millions)"]
```

## Performance Characteristics

### Development Speed

- **Flex Mode:** ~60% faster to write initially
- **Strict Mode:** More time upfront, faster to maintain

### Error Detection

- **Flex Mode:** Runtime discovery of issues
- **Strict Mode:** Compile-time validation and early error detection

### Consistency

- **Flex Mode:** Style variations between authors
- **Strict Mode:** Enforced structural consistency

## Migration Between Modes

### Flex to Strict Migration

Common when projects grow in complexity:

**Before (Flex):**
```slidelang
# My Presentation

## Key Features
- Feature A with benefits
- Feature B with advantages

```python
def example():
    return "Hello World"
```
```

**After (Strict):**
```slidelang
SLIDE content
  title: "My Presentation"

SLIDE content
  title: "Key Features"
  POINTS
    - Feature A with benefits
    - Feature B with advantages

SLIDE code_example
  title: "Code Example"
  CODE python
    def example():
        return "Hello World"
```

### Strict to Flex Migration

Less common, typically for simplification:

**Before (Strict):**
```slidelang
SLIDE content
  title: "Simple Content"
  TEXT
    Basic presentation content.
  POINTS
    - First point
    - Second point
```

**After (Flex):**
```slidelang
# Simple Content

Basic presentation content.

- First point
- Second point
```

## Hybrid Approaches

### Project-Level Standards

Large organizations often standardize by project type:

- **Executive presentations:** Strict Mode for consistency
- **Technical demos:** Flex Mode for rapid development  
- **Training materials:** Strict Mode for structure
- **Internal docs:** Flex Mode for ease of authoring

### Template Systems

Create mode-specific templates for common use cases:

**Executive Template (Strict):**
```yaml
---
mode: strict
template: "executive-quarterly"
variables:
  quarter: "Q4 2024"
  presenter: "{{author}}"
---
```

**Demo Template (Flex):**
```yaml
---
mode: flex
template: "product-demo"
variables:
  product: "{{product_name}}"
  demo_date: "{{date}}"
---
```

## Best Practices by Mode

### Strict Mode Best Practices

1. **Use layout validation** - Leverage built-in layout rules
2. **Define clear templates** - Create reusable slide templates  
3. **Implement linting** - Use automated validation in CI/CD
4. **Document conventions** - Establish team coding standards
5. **Version control integration** - Track structural changes

### Flex Mode Best Practices

1. **Maintain heading hierarchy** - Use consistent heading levels
2. **Leverage automatic detection** - Trust layout auto-detection
3. **Use consistent separators** - Standardize slide breaks
4. **Test rendering frequently** - Preview changes regularly
5. **Create style guides** - Document team Markdown conventions

## Decision Framework

### For New Projects

1. **Assess team Markdown expertise**
   - High expertise → Consider Flex Mode
   - Mixed expertise → Consider Strict Mode

2. **Evaluate project complexity**
   - Simple content → Flex Mode
   - Complex layouts → Strict Mode

3. **Consider maintenance requirements**
   - Long-term maintenance → Strict Mode
   - Short-term projects → Either mode

4. **Review automation needs**
   - Heavy automation → Strict Mode
   - Manual authoring → Flex Mode

### For Existing Projects

1. **Audit current content patterns**
2. **Assess author satisfaction with current mode**
3. **Evaluate error rates and maintenance burden**
4. **Consider migration costs vs. benefits**

## Related Documentation

- [Strict Mode Syntax](strict-mode.md) - Complete Strict Mode reference
- [Flex Mode Syntax](flex-mode.md) - Complete Flex Mode reference  
- [Syntax Overview](syntax-overview.md) - Quick comparison and examples
- [FrontMatter Configuration](frontmatter.md) - Mode selection and configuration
- [Specialized Layouts](specialized-layouts.md) - Layout behavior in both modes

---

**💡 Pro Tip:** Start with the mode that matches your team's existing skills. You can always migrate later as your needs evolve.
