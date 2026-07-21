# Migration Guide

This guide helps you migrate presentations from other formats to ZiraDocs. Whether you're coming from PowerPoint, Markdown, or other tools, this guide will help you make the transition smoothly.

## 🎯 **Quick Reference**

| Coming from... | Best Strategy | Time Estimate |
|----------------|---------------|---------------|
| **Markdown files** | Direct conversion with minimal changes | 15-30 minutes |
| **PowerPoint/Keynote/Google Slides** | Manual reconstruction | 1-3 hours |
| **Other text formats** | Copy-paste and restructure | 30-60 minutes |
| **Between ZiraDocs modes** | See [Choosing Your Syntax Mode](choosing-syntax-mode.md) | 15 minutes |

## 📝 **Migrating from Markdown**

If you have existing presentations in standard Markdown, migration to **ZiraDocs Flex Mode** is usually straightforward.

### Step-by-Step Process

#### 1. Add FrontMatter Configuration
Add ZiraDocs metadata at the beginning of your file:

```yaml
---
mode: flex
title: My Migrated Presentation
author: Your Name
theme: default
---
```

#### 2. Review Slide Separators
- **Standard Markdown**: `---` creates horizontal rules
- **ZiraDocs**: `---` on its own line separates slides

**Before (Markdown):**
```markdown
# Slide 1
Content here
---
More content on same slide
```

**After (ZiraDocs):**
```markdown
# Slide 1
Content here

---

# Slide 2
More content on new slide
```

#### 3. Validate Heading Structure
Check how your headings will be interpreted:

| Markdown | ZiraDocs Flex Interpretation |
|----------|-------------------------------|
| `# Title` | Slide title (prominent) |
| `## Subtitle` | Content heading or subtitle |
| `### Section` | Section heading within slide |

#### 4. Enhance with ZiraDocs Features
Add ZiraDocs-specific elements for better presentations:

```markdown
# Welcome Slide

@notes:
Remember to introduce yourself and thank the audience.

## Content Slide

::: left
Left column content
:::

::: right
Right column content
:::

---

# Charts & Diagrams

<<chart: bar>>
{
  "title": "Quarterly Results",
  "data": {
    "labels": ["Q1", "Q2", "Q3", "Q4"],
    "datasets": [{
      "label": "Revenue",
      "data": [100, 150, 180, 220]
    }]
  }
}
```

### Common Compatibility Issues

| Element | Markdown | ZiraDocs Solution |
|---------|----------|-------------------|
| **Horizontal rules within slides** | `---` | Use `___` or `***` instead |
| **Multiple headings per slide** | Multiple `#` | Use `##` or `###` for subsections |
| **Speaker notes** | Not supported | Add `@notes:` directives |
| **Columns** | Not supported | Use `::: left` and `::: right` |

## 🖼️ **Migrating from PowerPoint/Keynote/Google Slides**

Automated migration from proprietary formats is complex, but manual reconstruction is highly effective.

### Recommended Strategy: Structured Reconstruction

#### Phase 1: Planning (15 minutes)
1. **Choose your mode**: [Strict or Flex](choosing-syntax-mode.md)
2. **Count slides**: Plan your slide structure
3. **Identify special elements**: Charts, images, animations
4. **Select theme**: Browse [available themes](../features/themes-styling.md)

#### Phase 2: Content Extraction (30-60 minutes)
1. **Text content**: Copy slide titles and bullet points
2. **Images**: Export and save to `assets/images/`
3. **Charts**: Note data for recreation with ZiraDocs charts
4. **Layouts**: Identify which ZiraDocs layouts match your needs

#### Phase 3: Recreation (60-120 minutes)

**Basic slide structure:**
```markdown
---
mode: flex
title: My Presentation
author: John Doe
theme: corporate
---

# Title Slide
## Subtitle

@notes:
Opening remarks and agenda overview.

---

# Agenda
- Introduction
- Main topics
- Q&A session

---

# Key Points

::: highlight
Important takeaway message
:::

<<chart: pie>>
{
  "title": "Market Share",
  "data": {
    "labels": ["Product A", "Product B", "Product C"],
    "datasets": [{
      "data": [45, 30, 25]
    }]
  }
}
```

### Handling Complex Elements

| PowerPoint Feature | ZiraDocs Equivalent | Notes |
|---------------------|---------------------|--------|
| **Bullet animations** | Use `@reveal:` directive | [See directives guide](../language-reference/directives-configuration.md) |
| **Master slides** | Use themes and layouts | [See themes guide](../features/themes-styling.md) |
| **Complex charts** | `<<chart>>` blocks | [See charts guide](../features/charts-diagrams.md) |
| **Speaker notes** | `@notes:` directive | Automatically appear in presenter mode |
| **Slide transitions** | `@transition:` directive | Fade, slide, zoom effects |
| **Multiple columns** | `::: left` / `::: right` | Or use specialized layouts |

### Visual Fidelity Tips

1. **Colors**: Choose themes that match your brand colors
2. **Fonts**: Use CSS customization for specific typography
3. **Spacing**: Leverage ZiraDocs's automatic spacing or add custom CSS
4. **Images**: Optimize for web and use consistent sizing

## 🔄 **Version Migration (Future-Proofing)**

*Note: This section is preparatory for future ZiraDocs versions.*

When ZiraDocs undergoes major updates, migration will be supported through:

### Built-in Migration Tools
```bash
# Check compatibility
slidelang check --version-compat presentation.slidelang

# Automatic migration (when available)
slidelang migrate presentation.slidelang --to-version 2.0

# Backup before migration
slidelang migrate presentation.slidelang --backup --to-version 2.0
```

### Version Indicators
```yaml
---
version: "1.0"  # Parser version compatibility
mode: flex
title: My Presentation
---
```

## 🛠️ **Migration Tools & Helpers**

### Community Tools
- **markdown-to-slidelang**: Basic Markdown converter (coming soon)
- **pptx-extractor**: PowerPoint content extraction utility (community)

### Manual Helpers
```bash
# Validate migrated content
slidelang build presentation.slidelang --lint-only

# Quick theme preview
slidelang build presentation.slidelang --theme dark --preview

# Check for common issues
slidelang build presentation.slidelang --verbose
```

## ✅ **Migration Checklist**

### Before Starting
- [ ] **Backup original files**
- [ ] **Choose ZiraDocs mode** (Strict vs Flex)
- [ ] **Install ZiraDocs CLI** ([Installation guide](../getting-started/installation.md))
- [ ] **Select target theme**

### During Migration
- [ ] **Add proper frontmatter**
- [ ] **Test slide separators**
- [ ] **Validate heading structure**
- [ ] **Add speaker notes**
- [ ] **Recreate special elements**
- [ ] **Test build process**

### After Migration
- [ ] **Build and preview presentation**
- [ ] **Check responsive design**
- [ ] **Test presenter mode**
- [ ] **Validate all links and media**
- [ ] **Archive original files**

## 💡 **Migration Best Practices**

### Start Small
- **Single slide**: Test with one slide first
- **Simple presentation**: Migrate a basic presentation before complex ones
- **Feature by feature**: Add advanced features gradually

### Focus on Content First
1. **Get structure right**: Slides and content organization
2. **Add basic styling**: Choose appropriate theme
3. **Enhance gradually**: Add charts, interactions, animations

### Leverage ZiraDocs Strengths
- **Version control**: Keep presentation in Git
- **Modularity**: Break large presentations into sections
- **Reusability**: Create template slides for common patterns
- **Automation**: Use variables and templates for repeated content

### Quality Assurance
- **Test on multiple devices**: Desktop, tablet, mobile
- **Check accessibility**: Screen readers, keyboard navigation
- **Validate performance**: Large presentations and media
- **Get feedback**: Test with actual audience

## 🆘 **Common Migration Issues**

### Issue: Slides Not Separating Properly
**Problem**: Content appearing on wrong slides
```markdown
# Slide 1
Content
--- (this creates a horizontal rule, not slide separator)
More content (still on slide 1)
```

**Solution**: Use `---` on its own line
```markdown
# Slide 1
Content

---

# Slide 2
More content
```

### Issue: Charts Not Displaying
**Problem**: Chart syntax errors or missing data

**Solution**: Validate JSON syntax
```bash
# Check syntax
slidelang build presentation.slidelang --lint-only

# Use online JSON validator for chart data
```

### Issue: Images Not Loading
**Problem**: Incorrect paths or missing files

**Solution**: Use relative paths and verify files exist
```markdown
# Correct
![Chart](assets/images/chart.png)

# Incorrect  
![Chart](C:/Users/me/Desktop/chart.png)
```

### Issue: Theme Not Applied
**Problem**: Frontmatter syntax errors

**Solution**: Validate YAML frontmatter
```yaml
---
mode: flex          # ✅ Correct
title: My Presentation  # ✅ Quoted strings optional for simple titles
theme: "corporate"  # ✅ Quotes recommended for safety
---
```

## 🔗 **Next Steps After Migration**

### Immediate Actions
1. **Build and test**: Ensure presentation works correctly
2. **Share preview**: Get feedback from colleagues
3. **Version control**: Commit to Git repository

### Optimization
1. **Performance**: Optimize images and media
2. **Accessibility**: Add alt text and proper headings
3. **Mobile**: Test responsive design
4. **SEO**: Add proper metadata for web sharing

### Advanced Features
1. **Interactive elements**: Add polls, Q&A, navigation
2. **Custom styling**: Create custom CSS themes
3. **Integration**: Connect with external tools and APIs
4. **Automation**: Set up CI/CD for presentation deployment

---

**🚀 Ready to migrate? Start with our [Quickstart Guide](../getting-started/quickstart.md) to set up your environment, then return here for specific migration steps.**
