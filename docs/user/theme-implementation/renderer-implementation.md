# 🔧 Renderer Implementation Guide

This guide is for developers who want to implement ZiraDocs theme support in their own rendering engines, viewers, or presentation tools.

## 🎯 **Overview**

ZiraDocs's theme system is designed to be **renderer-agnostic**. Any tool can implement theme support by:

1. **Parsing theme JSON files** or using embedded themes
2. **Applying CSS variables** to your rendering pipeline
3. **Mapping slide types** to appropriate backgrounds
4. **Handling component styling** (callouts, code blocks, etc.)

## 🏗️ **Architecture Principles**

### CSS Variables Foundation
The entire theme system is built on CSS custom properties with the `--slidelang-` namespace:

```css
:root {
  --slidelang-primary-color: #3b82f6;
  --slidelang-background-color: #ffffff;
  /* 60+ more variables */
}
```

### Renderer Independence
- **No CLI dependency:** Themes work in any CSS-capable renderer
- **Standard CSS:** Uses only standard CSS custom properties
- **Flexible application:** Apply variables to any HTML structure

### Progressive Enhancement
- **Fallback values:** All variables have sensible defaults
- **Optional features:** Advanced styling is optional
- **Graceful degradation:** Missing variables don't break rendering

## 📋 **Implementation Steps**

### Step 1: Theme Loading

#### Parse Theme JSON
```javascript
// Example theme loader
class ThemeLoader {
  async loadTheme(themePath) {
    const response = await fetch(themePath);
    const theme = await response.json();
    
    return {
      name: theme.name,
      variables: theme.variables,
      metadata: {
        author: theme.author,
        version: theme.version,
        description: theme.description
      }
    };
  }
  
  applyTheme(theme) {
    const root = document.documentElement;
    
    // Apply all CSS variables
    Object.entries(theme.variables).forEach(([variable, value]) => {
      root.style.setProperty(variable, value);
    });
  }
}
```

#### Handle External vs Embedded Themes
```javascript
class ZiraDocsRenderer {
  async setTheme(themeSpec) {
    if (themeSpec.startsWith('external:')) {
      const themePath = themeSpec.replace('external:', '');
      const theme = await this.themeLoader.loadTheme(themePath);
      this.themeLoader.applyTheme(theme);
    } else if (themeSpec.startsWith('embedded:')) {
      const themeName = themeSpec.replace('embedded:', '');
      const theme = this.getEmbeddedTheme(themeName);
      this.themeLoader.applyTheme(theme);
    }
  }
}
```

### Step 2: HTML Structure

#### Basic Slide Structure
Your renderer should generate HTML that can be styled with ZiraDocs variables:

```html
<!-- Title Slide -->
<div class="slidelang-slide slidelang-slide--title">
  <h1 class="slidelang-title">Presentation Title</h1>
  <p class="slidelang-subtitle">Subtitle</p>
</div>

<!-- Content Slide -->
<div class="slidelang-slide slidelang-slide--content">
  <h2 class="slidelang-heading">Slide Title</h2>
  <div class="slidelang-content">
    <!-- Content here -->
  </div>
</div>

<!-- Section Slide -->
<div class="slidelang-slide slidelang-slide--section">
  <h2 class="slidelang-section-title">Section Name</h2>
</div>
```

#### Component Elements
```html
<!-- Callouts -->
<div class="slidelang-callout slidelang-callout--info">
  <p>Information callout</p>
</div>

<!-- Code Blocks -->
<pre class="slidelang-code">
  <code class="slidelang-code__content">
    console.log('Hello World');
  </code>
</pre>

<!-- Notes -->
<div class="slidelang-note">
  <p>This is a note</p>
</div>
```

### Step 3: CSS Integration

#### Base Styles
Create CSS that uses ZiraDocs variables:

```css
/* Slide backgrounds */
.slidelang-slide--title {
  background: var(--slidelang-bg-title-slide);
  color: var(--slidelang-text-on-dark);
}

.slidelang-slide--section {
  background: var(--slidelang-bg-section-slide);
  color: var(--slidelang-text-on-primary);
}

.slidelang-slide--content {
  background: var(--slidelang-bg-content-slide);
  color: var(--slidelang-text-color);
}

/* Typography */
.slidelang-slide {
  font-family: var(--slidelang-font-main);
}

.slidelang-code {
  font-family: var(--slidelang-font-code);
  background: var(--slidelang-bg-code);
  color: var(--slidelang-text-on-dark);
  border-radius: var(--slidelang-border-radius);
}

/* Callouts */
.slidelang-callout--info {
  background: var(--slidelang-bg-info);
  color: var(--slidelang-info-text-color);
  border-left: 4px solid var(--slidelang-info-color);
}

.slidelang-callout--success {
  background: var(--slidelang-bg-success);
  color: var(--slidelang-success-text-color);
  border-left: 4px solid var(--slidelang-success-color);
}
```

### Step 4: Advanced Features

#### Gradient Support
```css
.slidelang-slide--title {
  background: var(--slidelang-title-gradient, var(--slidelang-bg-title-slide));
}

.slidelang-slide--section {
  background: var(--slidelang-gradient-bg, var(--slidelang-bg-section-slide));
}
```

#### Syntax Highlighting
```css
.slidelang-code .token.comment {
  color: var(--slidelang-syntax-comment);
}

.slidelang-code .token.keyword {
  color: var(--slidelang-syntax-keyword);
}

.slidelang-code .token.string {
  color: var(--slidelang-syntax-string);
}
```

#### Interactive Elements
```css
.slidelang-link {
  color: var(--slidelang-link-color);
  background: var(--slidelang-link-bg);
  transition: var(--slidelang-transition);
}

.slidelang-link:hover {
  color: var(--slidelang-link-hover-color);
  background: var(--slidelang-link-hover-bg);
}
```

## 🧪 **Testing Your Implementation**

### Test Suite
Create a comprehensive test to verify theme support:

```javascript
class ThemeRenderer {
  testThemeSupport() {
    const tests = [
      'Slide backgrounds are applied correctly',
      'Text colors contrast properly',
      'Callouts are styled appropriately', 
      'Code highlighting works',
      'Gradients render properly',
      'Typography is applied'
    ];
    
    // Run each test...
  }
}
```

### Sample Presentations
Test with presentations containing:
- All slide types (title, section, content, end)
- All callout types (info, success, warning, danger, tip)
- Code blocks with syntax highlighting
- Various text elements
- Interactive components

## 🎨 **Renderer-Specific Considerations**

### Web Renderers (HTML/CSS)
- **Direct CSS variables:** Use CSS custom properties directly
- **Dynamic theming:** Change themes without page reload
- **Responsive design:** Test themes at different screen sizes

### Native Applications
- **Variable mapping:** Map CSS variables to native styling systems
- **Color conversion:** Convert hex/RGB values to native formats
- **Font loading:** Handle font family fallbacks

### PDF/Print Renderers
- **Static rendering:** Pre-process themes before generation
- **Color accuracy:** Ensure print-safe color profiles
- **Font embedding:** Include necessary font files

### React/Vue Components
```jsx
// React example
function ZiraDocsSlide({ type, children, theme }) {
  const slideStyle = {
    background: theme.variables[`--slidelang-bg-${type}-slide`],
    color: theme.variables['--slidelang-text-color'],
    fontFamily: theme.variables['--slidelang-font-main']
  };
  
  return (
    <div className={`slidelang-slide slidelang-slide--${type}`} style={slideStyle}>
      {children}
    </div>
  );
}
```

## 📚 **Reference Implementations**

### Minimal HTML Renderer
```html
<!DOCTYPE html>
<html>
<head>
  <style id="slidelang-theme">
    /* Theme variables will be injected here */
  </style>
  <link rel="stylesheet" href="slidelang-base.css">
</head>
<body>
  <div class="slidelang-presentation">
    <!-- Slides here -->
  </div>
</body>
</html>
```

### Theme Injection Script
```javascript
function injectTheme(themeJson) {
  const styleEl = document.getElementById('slidelang-theme');
  const cssVars = Object.entries(themeJson.variables)
    .map(([key, value]) => `  ${key}: ${value};`)
    .join('\n');
    
  styleEl.textContent = `:root {\n${cssVars}\n}`;
}
```

## 🚀 **Best Practices**

### Performance
- **Variable caching:** Cache parsed themes
- **CSS optimization:** Minimize CSS recomputation
- **Lazy loading:** Load themes on demand

### Compatibility
- **Fallback values:** Always provide fallbacks
- **Progressive enhancement:** Start with basic styling
- **Cross-browser:** Test in multiple environments

### User Experience
- **Theme previews:** Show theme samples
- **Validation:** Validate theme JSON format
- **Error handling:** Graceful theme loading failures

---

**Complete your implementation by testing with the [example themes](examples/) and referencing the full [Theme Specification](theme-specification.md).**
