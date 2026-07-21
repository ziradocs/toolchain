# 🎨 Theme Examples & Templates

This directory contains example themes and code templates to help you get started with ZiraDocs theme creation.

## 📋 **Available Examples**

### 🎯 **Complete Themes**
- **[Minimal Theme](minimal-theme.json)** - Bare minimum variables for a working theme
- **[Professional Blue](professional-blue.json)** - Corporate-friendly blue theme
- **[Dark Mode](dark-mode.json)** - Dark theme for modern presentations
- **[Creative Gradient](creative-gradient.json)** - Colorful theme with gradients

### 🔧 **Code Templates**
- **[Theme Loader](theme-loader.js)** - JavaScript theme loading utility
- **[CSS Base](slidelang-base.css)** - Base CSS for ZiraDocs presentations
- **[React Component](react-renderer.jsx)** - React theme integration example

### 📝 **Documentation Templates**
- **[Theme README](theme-readme-template.md)** - Template for documenting your themes
- **[Package JSON](package-template.json)** - NPM package template for theme distribution

## 🚀 **Quick Start Templates**

### Minimal Theme (Just 20 variables)
Perfect starting point - includes only the essential variables:

```json
{
  "name": "minimal",
  "description": "Minimal theme with essential variables only",
  "author": "ZiraDocs",
  "version": "1.0.0",
  "variables": {
    "--slidelang-primary-color": "#3b82f6",
    "--slidelang-secondary-color": "#64748b",
    "--slidelang-accent-color": "#06b6d4",
    "--slidelang-background-color": "#ffffff",
    "--slidelang-text-color": "#1f2937",
    "--slidelang-text-on-primary": "#ffffff",
    "--slidelang-bg-title-slide": "#1f2937",
    "--slidelang-bg-section-slide": "#3b82f6",
    "--slidelang-bg-content-slide": "#ffffff",
    "--slidelang-bg-end-slide": "#06b6d4",
    "--slidelang-font-main": "system-ui, sans-serif",
    "--slidelang-font-code": "monospace",
    "--slidelang-border-radius": "0.5rem",
    "--slidelang-shadow-main": "0 1px 3px rgba(0,0,0,0.1)",
    "--slidelang-bg-info": "#eff6ff",
    "--slidelang-bg-success": "#ecfdf5",
    "--slidelang-bg-warning": "#fffbeb",
    "--slidelang-bg-danger": "#fef2f2",
    "--slidelang-info-text-color": "#1e40af",
    "--slidelang-success-text-color": "#065f46"
  }
}
```

### Theme Color Generator
Use this template to quickly generate theme variations:

```javascript
// Color palette generator
function generateTheme(primaryColor, name) {
  return {
    "name": name,
    "description": `Auto-generated theme based on ${primaryColor}`,
    "author": "Theme Generator",
    "version": "1.0.0",
    "variables": {
      "--slidelang-primary-color": primaryColor,
      "--slidelang-secondary-color": adjustBrightness(primaryColor, -20),
      "--slidelang-accent-color": complementaryColor(primaryColor),
      // ... generate all other variables
    }
  };
}
```

## 🎨 **Theme Variations**

### Light vs Dark Themes
```json
// Light theme base
"--slidelang-background-color": "#ffffff",
"--slidelang-text-color": "#1f2937",

// Dark theme base  
"--slidelang-background-color": "#1f2937", 
"--slidelang-text-color": "#f9fafb"
```

### Brand Color Themes
```json
// Tech company (blue/purple)
"--slidelang-primary-color": "#6366f1",
"--slidelang-accent-color": "#8b5cf6",

// Healthcare (green/blue)
"--slidelang-primary-color": "#059669", 
"--slidelang-accent-color": "#0891b2",

// Finance (navy/gold)
"--slidelang-primary-color": "#1e40af",
"--slidelang-accent-color": "#f59e0b"
```

## 🧪 **Testing Utilities**

### Theme Validator
```javascript
function validateTheme(theme) {
  const requiredVariables = [
    '--slidelang-primary-color',
    '--slidelang-background-color', 
    '--slidelang-text-color',
    '--slidelang-bg-title-slide',
    '--slidelang-bg-content-slide',
    '--slidelang-font-main'
  ];
  
  const missing = requiredVariables.filter(
    variable => !theme.variables[variable]
  );
  
  return {
    valid: missing.length === 0,
    missing: missing,
    totalVariables: Object.keys(theme.variables).length
  };
}
```

### Color Contrast Checker
```javascript
function checkContrast(backgroundColor, textColor) {
  // Calculate WCAG contrast ratio
  const ratio = getContrastRatio(backgroundColor, textColor);
  return {
    ratio: ratio,
    AA: ratio >= 4.5,  // WCAG AA compliance
    AAA: ratio >= 7.0  // WCAG AAA compliance
  };
}
```

## 📦 **Distribution Templates**

### NPM Package Structure
```
my-slidelang-theme/
├── package.json
├── README.md
├── LICENSE
├── themes/
│   ├── light.json
│   ├── dark.json
│   └── variants/
├── examples/
│   └── sample-presentation.slidelang
├── screenshots/
│   ├── light-preview.png
│   └── dark-preview.png
└── tools/
    └── theme-builder.js
```

### Package.json Template
```json
{
  "name": "@yourname/slidelang-theme-ocean",
  "version": "1.0.0",
  "description": "Ocean-inspired themes for ZiraDocs presentations",
  "main": "themes/ocean.json",
  "files": ["themes/", "README.md", "LICENSE"],
  "keywords": ["slidelang", "theme", "presentation", "ocean", "blue"],
  "author": "Your Name <your.email@example.com>",
  "license": "MIT",
  "repository": {
    "type": "git",
    "url": "https://github.com/yourname/slidelang-theme-ocean"
  }
}
```

## 🎯 **Usage Examples**

### CLI Usage
```bash
# Use local theme file
slidelang build presentation.slidelang --theme external:./themes/ocean.json

# Use from npm package
npm install @yourname/slidelang-theme-ocean
slidelang build presentation.slidelang --theme external:./node_modules/@yourname/slidelang-theme-ocean/themes/ocean.json
```

### Programmatic Usage
```javascript
const ZiraDocsRenderer = require('slidelang-renderer');
const oceanTheme = require('./themes/ocean.json');

const renderer = new ZiraDocsRenderer();
renderer.setTheme(oceanTheme);
renderer.renderPresentation(slideLangContent);
```

## 🎨 **Design Resources**

### Color Palette Tools
- **Adobe Color:** color.adobe.com
- **Coolors:** coolors.co  
- **Material Design Colors:** material.io/design/color
- **Tailwind CSS Colors:** tailwindcss.com/docs/customizing-colors

### Font Pairing Resources
- **Google Fonts:** fonts.google.com
- **Font Pair:** fontpair.co
- **Typewolf:** typewolf.com

### Design Inspiration
- **Dribbble:** dribbble.com/tags/presentation
- **Behance:** behance.net/search/projects/?field=presentation
- **Slide Design Gallery:** Various presentation showcases

---

**Ready to create your theme?** Start with the [minimal theme template](minimal-theme.json) or follow the complete [Theme Creation Guide](../theme-creation.md).
