# 🌟 ZiraDocs Features

This section covers all the powerful features available in ZiraDocs for creating dynamic, interactive, and visually stunning presentations.

## 📋 **Feature Categories**

### 🎨 **Styling & Theming**
- **[Themes & Styling](themes-styling.md)** - Complete theming system, custom CSS, and visual customization
- **[Headers & Footers](headers-footers.md)** - Persistent slide elements and branding

### 📊 **Content & Layout** 
- **[Variables & Templates](variables-templates.md)** - Dynamic content, data binding, and reusable templates
- **[Infographics](infographics.md)** - Complete presentation templates for specific business scenarios

### 🎯 **Interactive Elements**
- **[Dynamic & Interactive](dynamic-interactive.md)** - Quizzes, polls, forms, and audience engagement tools

## 🚀 **Quick Feature Overview**

| Feature | Description | Best For |
|---------|-------------|----------|
| **Custom Themes** | CSS variables and complete styling control | Branding, visual consistency |
| **Variables** | Dynamic content and data binding | Data-driven presentations |
| **Interactive Elements** | Quizzes, polls, and audience engagement | Training, workshops, meetings |
| **Infographics** | Pre-built templates for business scenarios | Reports, analytics, pitches |
| **Headers/Footers** | Persistent branding and navigation | Corporate presentations |

## 💡 **Getting Started**

### New to ZiraDocs?
1. Start with **[Themes & Styling](themes-styling.md)** to understand the visual system
2. Learn **[Variables & Templates](variables-templates.md)** for dynamic content
3. Add engagement with **[Interactive Elements](dynamic-interactive.md)**

### Looking for Specific Features?
- **Brand consistency** → [Themes & Styling](themes-styling.md) + [Headers & Footers](headers-footers.md)
- **Data presentations** → [Variables & Templates](variables-templates.md) + [Infographics](infographics.md)  
- **Training materials** → [Interactive Elements](dynamic-interactive.md)
- **Business reports** → [Infographics](infographics.md)

## 🔧 **Feature Combination Examples**

### Corporate Presentation
```yaml
---
theme: corporate-blue
variables:
  company: "Acme Corp"
  quarter: "Q4 2025"
header:
  logo: ./assets/logo.png
  text: "{{company}} - {{quarter}} Review"
---
```

### Interactive Training
```markdown
---
theme: educational
---

# Training Module

<<quiz>>
question: What is ZiraDocs?
options:
  - A programming language
  - A presentation DSL ✓
  - A database system
feedback: Correct! ZiraDocs is a Domain-Specific Language for presentations.
<</quiz>>
```

### Data Dashboard
```yaml
---
theme: data-insights
variables:
  metrics:
    revenue: "$2.4M"
    growth: "+15%"
    customers: "1,847"
---
```

## 📚 **Advanced Combinations**

### Multi-Language Presentations
Combine **Variables** + **Themes** for localized content:

```yaml
---
variables:
  lang: "es"
  content: !include ./locales/{{lang}}.yaml
theme: minimal-clean
---
```

### Interactive Data Stories
Combine **Infographics** + **Interactive Elements** + **Variables**:

```markdown
---
template: data-story
theme: analytics-pro
variables:
  dataset: ./data/sales-q4.json
---

# Sales Performance

<<chart: bar>>
data: {{dataset.monthly_sales}}
<</chart>>

<<poll>>
question: Which month performed best?
options: ["October", "November", "December"]
<</poll>>
```

## 🎯 **Feature Roadmap**

### Current (v0.8.0+)
- ✅ Complete theming system
- ✅ Variable substitution and templates
- ✅ Interactive quizzes and polls
- ✅ Business infographic templates
- ✅ Headers and footers

### Coming Soon (v0.9.0)
- 🔄 Animation and transition system
- 🔄 Advanced chart types
- 🔄 Real-time collaboration features
- 🔄 Export to PowerPoint/PDF

### Future (v1.0.0+)
- 📋 Plugin system for custom elements
- 📋 AI-powered content suggestions
- 📋 Advanced analytics and tracking
- 📋 Multi-presenter support

## 🔗 **Related Documentation**

- **[Getting Started](../getting-started/)** - Installation and first presentation
- **[Language Reference](../language-reference/)** - Complete syntax guide
- **[CLI Reference](../cli-reference/)** - Command-line interface
- **[Theme Implementation](../theme-implementation/)** - Creating custom themes

---

**Need help?** Check the [troubleshooting guide](../guides/troubleshooting.md) or visit our [community forum](https://github.com/ziradocs/toolchain/discussions).
