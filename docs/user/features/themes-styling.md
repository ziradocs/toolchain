# 🎨 Themes & Styling

ZiraDocs provides a comprehensive theming system that allows you to customize every aspect of your presentations. From built-in themes to completely custom styling, you have full control over the visual appearance.

## 🚀 **Quick Start**

### Using Built-in Themes
```bash
# Use embedded themes
slidelang build presentation.slidelang --theme default
slidelang build presentation.slidelang --theme dark
slidelang build presentation.slidelang --theme minimal

# Theme with number reference
slidelang build presentation.slidelang --theme embedded:2
```

### Using External Themes
```bash
# Custom theme file
slidelang build presentation.slidelang --theme external:my-theme.json

# Theme from URL
slidelang build presentation.slidelang --theme external:https://themes.example.com/corporate.json
```

### Setting Theme in Frontmatter
```yaml
---
theme: corporate-blue
title: My Presentation
---
```

## 🎯 **Theme Types**

### 1. **Embedded Themes**
Built into ZiraDocs CLI, optimized for common use cases:

| Theme | Description | Best For |
|-------|-------------|----------|
| `default` | Clean, professional blue theme | Business presentations |
| `dark` | Dark mode with high contrast | Technical presentations |
| `minimal` | Ultra-clean minimal design | Academic/research presentations |
| `corporate` | Professional corporate styling | Company presentations |
| `creative` | Colorful, modern design | Creative/marketing presentations |

### 2. **External Themes**
Custom JSON-based themes that you can create or download:

```json
{
  "name": "my-custom-theme",
  "description": "A theme for my company",
  "author": "Your Name",
  "version": "1.0.0",
  "variables": {
    "--slidelang-primary-color": "#2563eb",
    "--slidelang-bg-content-slide": "#ffffff",
    "--slidelang-text-color": "#1f2937"
  }
}
```

### 3. **Advanced Custom Themes**
Themes with complete CSS customization:

```
my-theme/
├── theme.json          # Variables and metadata
├── styles.css          # Complete custom CSS
├── assets/
│   ├── fonts/         # Custom fonts
│   └── images/        # Theme images
└── README.md          # Theme documentation
```

## 🎨 **Styling System Overview**

### CSS Variables Architecture
ZiraDocs uses CSS custom properties with the `--slidelang-` namespace:

```css
:root {
  /* Color System */
  --slidelang-primary-color: #3b82f6;
  --slidelang-secondary-color: #64748b;
  --slidelang-accent-color: #06b6d4;
  
  /* Typography */
  --slidelang-font-main: 'Inter', sans-serif;
  --slidelang-font-heading: 'Inter', sans-serif;
  --slidelang-font-code: 'JetBrains Mono', monospace;
  
  /* Layout */
  --slidelang-border-radius: 0.75rem;
  --slidelang-shadow-main: 0 4px 6px rgba(0, 0, 0, 0.1);
  --slidelang-transition: all 0.3s ease;
}
```

### Element Targeting
All ZiraDocs elements use consistent CSS classes:

```css
/* Slide Types */
.slidelang-slide.slidelang-title-slide { /* Title slides */ }
.slidelang-slide.slidelang-content-slide { /* Content slides */ }
.slidelang-slide.slidelang-section-slide { /* Section dividers */ }

/* Content Elements */
.slidelang-element.slidelang-text { /* Text blocks */ }
.slidelang-element.slidelang-points { /* Bullet points */ }
.slidelang-element.slidelang-table { /* Data tables */ }
.slidelang-element.slidelang-code { /* Code blocks */ }

/* Interactive Elements */
.slidelang-element.slidelang-quiz { /* Quiz components */ }
.slidelang-element.slidelang-poll { /* Poll widgets */ }
```

## 🛠️ **Customization Levels**

### Level 1: Variable Override (Easiest)
Change colors, fonts, and spacing without CSS knowledge:

```json
{
  "name": "company-brand",
  "variables": {
    "--slidelang-primary-color": "#ff6b35",
    "--slidelang-font-main": "'Roboto', sans-serif",
    "--slidelang-border-radius": "0.5rem"
  }
}
```

### Level 2: Targeted CSS (Intermediate)
Add specific styling for elements:

```css
/* Custom table styling */
.slidelang-element.slidelang-table {
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  color: white;
}

.slidelang-element.slidelang-table th {
  background: rgba(255, 255, 255, 0.2);
}

/* Custom code blocks */
.slidelang-element.slidelang-code {
  background: #1a1a1a;
  border: 2px solid #333;
  box-shadow: 0 0 20px rgba(0, 255, 255, 0.3);
}
```

### Level 3: Complete Theme (Advanced)
Full control with custom layouts and animations:

```css
/* Custom slide transitions */
.slidelang-slide {
  transition: transform 0.8s cubic-bezier(0.25, 0.46, 0.45, 0.94);
}

/* Animated backgrounds */
.slidelang-slide.slidelang-title-slide {
  background: linear-gradient(-45deg, #ee7752, #e73c7e, #23a6d5, #23d5ab);
  background-size: 400% 400%;
  animation: gradientShift 15s ease infinite;
}

@keyframes gradientShift {
  0% { background-position: 0% 50%; }
  50% { background-position: 100% 50%; }
  100% { background-position: 0% 50%; }
}

/* Custom layouts */
.slidelang-slide.slidelang-content-slide {
  display: grid;
  grid-template-columns: 1fr 300px;
  gap: 2rem;
}
```

## 🎭 **Interactive Element Styling**

### Quiz Components
```css
.slidelang-element.slidelang-quiz .question {
  background: var(--slidelang-bg-info);
  border-left: 4px solid var(--slidelang-info-color);
}

.slidelang-element.slidelang-quiz .option {
  background: var(--slidelang-bg-light);
  border: 1px solid var(--slidelang-border-color);
  transition: var(--slidelang-transition);
}

.slidelang-element.slidelang-quiz .option:hover {
  background: var(--slidelang-bg-hover);
  border-color: var(--slidelang-accent-color);
}

.slidelang-element.slidelang-quiz .option.correct {
  background: var(--slidelang-bg-success);
  border-color: var(--slidelang-success-color);
}
```

### Poll Widgets
```css
.slidelang-element.slidelang-poll .poll-option {
  background: var(--slidelang-bg-white);
  border: 2px solid var(--slidelang-border-color);
  border-radius: var(--slidelang-border-radius);
  transition: var(--slidelang-transition);
}

.slidelang-element.slidelang-poll .poll-results {
  background: var(--slidelang-bg-gray-50);
  padding: 1rem;
  border-radius: var(--slidelang-border-radius);
}

.slidelang-element.slidelang-poll .progress-bar {
  background: var(--slidelang-accent-color);
  height: 8px;
  border-radius: 4px;
  transition: width 0.5s ease;
}
```

## 📱 **Responsive Design**

Themes should consider mobile and tablet displays:

```css
/* Mobile optimizations */
@media (max-width: 768px) {
  .slidelang-slide {
    padding: 1.5rem;
    font-size: 0.9rem;
  }
  
  .slidelang-slide.slidelang-title-slide h1 {
    font-size: 2rem;
  }
  
  .slidelang-element.slidelang-table {
    overflow-x: auto;
  }
}

/* Tablet optimizations */
@media (max-width: 1024px) {
  .slidelang-slide {
    padding: 2rem;
  }
  
  .slidelang-element.slidelang-code {
    font-size: 0.85rem;
  }
}
```

## 🎨 **Theme Gallery**

### Dark Professional
```json
{
  "name": "dark-professional",
  "variables": {
    "--slidelang-bg-content-slide": "#1a1a1a",
    "--slidelang-bg-title-slide": "linear-gradient(135deg, #1e293b 0%, #0f172a 100%)",
    "--slidelang-text-color": "#e2e8f0",
    "--slidelang-primary-color": "#3b82f6",
    "--slidelang-accent-color": "#06b6d4"
  }
}
```

### Warm Corporate
```json
{
  "name": "warm-corporate",
  "variables": {
    "--slidelang-primary-color": "#dc2626",
    "--slidelang-secondary-color": "#7c2d12",
    "--slidelang-accent-color": "#ea580c",
    "--slidelang-bg-title-slide": "linear-gradient(135deg, #dc2626 0%, #7c2d12 100%)",
    "--slidelang-font-heading": "'Playfair Display', serif"
  }
}
```

### Tech Startup
```json
{
  "name": "tech-startup",
  "variables": {
    "--slidelang-primary-color": "#8b5cf6",
    "--slidelang-secondary-color": "#06b6d4",
    "--slidelang-accent-color": "#10b981",
    "--slidelang-bg-title-slide": "linear-gradient(135deg, #667eea 0%, #764ba2 100%)",
    "--slidelang-border-radius": "1rem",
    "--slidelang-font-main": "'Space Grotesk', sans-serif"
  }
}
```

## 🔧 **Creating Custom Themes**

### Step 1: Start with Variables
```json
{
  "name": "my-theme",
  "description": "Custom theme for my presentations",
  "author": "Your Name",
  "version": "1.0.0",
  "variables": {
    // Start with core colors
    "--slidelang-primary-color": "#your-brand-color",
    "--slidelang-text-color": "#333333",
    "--slidelang-bg-content-slide": "#ffffff"
  }
}
```

### Step 2: Test and Iterate
```bash
# Build with your theme
slidelang build test.slidelang --theme external:my-theme.json

# Preview in browser
slidelang preview test.slidelang --theme external:my-theme.json --watch
```

### Step 3: Add Custom CSS (Optional)
Create `styles.css` alongside your `theme.json`:

```css
/* Custom animations */
.slidelang-slide {
  opacity: 0;
  animation: slideIn 0.5s ease forwards;
}

@keyframes slideIn {
  to { opacity: 1; }
}

/* Custom elements */
.slidelang-element.slidelang-text {
  background: rgba(255, 255, 255, 0.8);
  backdrop-filter: blur(10px);
  border: 1px solid rgba(255, 255, 255, 0.3);
}
```

## 📚 **Advanced Topics**

### Theme Inheritance
```json
{
  "name": "my-dark-theme",
  "extends": "dark",
  "variables": {
    "--slidelang-accent-color": "#ff6b35"
  }
}
```

### Asset Management
```json
{
  "name": "branded-theme",
  "assets": {
    "fonts": ["./fonts/custom-font.woff2"],
    "images": ["./images/logo.svg"],
    "css": ["./additional-styles.css"]
  },
  "variables": {
    "--slidelang-font-main": "'CustomFont', sans-serif"
  }
}
```

### Theme Validation
```bash
# Validate theme structure
slidelang theme validate my-theme.json

# Test theme compatibility
slidelang theme test my-theme.json --with examples/
```

## 🔗 **Related Documentation**

- **[Theme Implementation Guide](../theme-implementation/)** - Complete technical reference
- **[Variables & Templates](variables-templates.md)** - Using variables in themes
- **[Dynamic & Interactive Elements](dynamic-interactive.md)** - Styling interactive components

## 💡 **Best Practices**

1. **Start Simple**: Begin with variable overrides before custom CSS
2. **Test Responsive**: Always test on mobile and tablet sizes
3. **Use Namespacing**: Stick to `--slidelang-` variable names
4. **Version Control**: Use semantic versioning for theme releases
5. **Document Changes**: Maintain a changelog for theme updates
6. **Performance**: Optimize images and avoid heavy animations
7. **Accessibility**: Ensure sufficient color contrast and readable fonts

---

**Next Steps:**
- Create your first custom theme with the [Theme Creation Guide](../theme-implementation/theme-creation.md)
- Explore advanced techniques in [Theme Implementation Guide](../theme-implementation/)
- Browse theme examples in [Theme Gallery](../theme-implementation/examples/)
