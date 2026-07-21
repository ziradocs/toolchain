# 🎨 Theme Specification

This document defines the complete CSS variables specification for ZiraDocs themes. Use this as your reference when creating custom themes or implementing ZiraDocs renderers.

## 🔑 **Core Concepts**

### CSS Variables Namespace
All ZiraDocs theme variables use the `--slidelang-` prefix to avoid conflicts:

```css
:root {
  --slidelang-primary-color: #3b82f6;
  --slidelang-background-color: #ffffff;
  /* ... more variables */
}
```

### Variable Categories
Variables are organized into logical groups:
- **Colors:** Primary, secondary, accent colors
- **Typography:** Font families, sizes, weights
- **Layout:** Spacing, borders, shadows
- **Components:** Slide types, UI elements
- **Syntax:** Code highlighting colors

## 📖 **Complete Variables Reference**

### 🎨 **Color System**

#### Primary Colors
```css
--slidelang-primary-color: #3b82f6;      /* Main brand color */
--slidelang-secondary-color: #64748b;    /* Secondary accent */
--slidelang-accent-color: #06b6d4;       /* Highlight color */
```

#### Semantic Colors
```css
--slidelang-success-color: #10b981;      /* Success states */
--slidelang-warning-color: #f59e0b;      /* Warning states */
--slidelang-danger-color: #ef4444;       /* Error states */
--slidelang-info-color: #3b82f6;         /* Information */
--slidelang-tip-color: #8b5cf6;          /* Tips and hints */
```

#### Text Colors
```css
--slidelang-text-color: #1f2937;         /* Primary text */
--slidelang-text-light: #6b7280;         /* Secondary text */
--slidelang-text-muted: #9ca3af;         /* Muted text */
--slidelang-text-on-primary: #ffffff;    /* Text on primary color */
--slidelang-text-on-accent: #ffffff;     /* Text on accent color */
--slidelang-text-on-dark: #ffffff;       /* Text on dark backgrounds */
```

#### Background Colors
```css
--slidelang-background-color: #ffffff;   /* Main background */
--slidelang-bg-white: #ffffff;           /* Pure white */
--slidelang-bg-gray-50: #f9fafb;         /* Light gray */
--slidelang-bg-gray-100: #f3f4f6;        /* Lighter gray */
--slidelang-bg-code: #1f2937;            /* Code block background */
--slidelang-bg-light: #f9fafb;           /* Light sections */
```

#### Slide Type Backgrounds
```css
--slidelang-bg-title-slide: #1f2937;     /* Title slide background */
--slidelang-bg-section-slide: #3b82f6;   /* Section slide background */
--slidelang-bg-content-slide: #ffffff;   /* Content slide background */
--slidelang-bg-end-slide: #10b981;       /* End slide background */
--slidelang-bg-closing-slide: #3b82f6;   /* Closing slide background */
```

#### Component Backgrounds
```css
--slidelang-bg-info: #eff6ff;            /* Info callout background */
--slidelang-bg-success: #ecfdf5;         /* Success callout background */
--slidelang-bg-warning: #fffbeb;         /* Warning callout background */
--slidelang-bg-danger: #fef2f2;          /* Danger callout background */
--slidelang-bg-tip: #f3e8ff;             /* Tip callout background */
--slidelang-bg-note: #f9fafb;            /* Note background */
--slidelang-bg-quote: #f9fafb;           /* Quote background */
--slidelang-bg-placeholder: rgba(59, 130, 246, 0.05); /* Placeholder */
```

#### Interactive Elements
```css
--slidelang-bg-hover: #f3f4f6;           /* Hover states */
--slidelang-bg-progress: #e5e7eb;        /* Progress bars */
--slidelang-link-color: #3b82f6;         /* Link color */
--slidelang-link-bg: rgba(59, 130, 246, 0.05);        /* Link background */
--slidelang-link-hover-color: #2563eb;   /* Link hover color */
--slidelang-link-hover-bg: rgba(59, 130, 246, 0.1);   /* Link hover background */
```

### 🔤 **Typography System**

#### Font Families
```css
--slidelang-font-main: 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
--slidelang-font-code: 'JetBrains Mono', 'Fira Code', 'SF Mono', Consolas, monospace;
--slidelang-font-heading: 'Inter', -apple-system, BlinkMacSystemFont, sans-serif;
```

### 🎭 **Visual Effects**

#### Gradients
```css
--slidelang-gradient-bg: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
--slidelang-title-gradient: linear-gradient(135deg, #1f2937 0%, #374151 100%);
--slidelang-accent-gradient: linear-gradient(135deg, #06b6d4 0%, #0891b2 100%);
```

#### Borders & Radius
```css
--slidelang-border-radius: 0.5rem;       /* Standard radius */
--slidelang-border-radius-lg: 1rem;      /* Large radius */
--slidelang-border-radius-sm: 0.25rem;   /* Small radius */
--slidelang-border-color: #e5e7eb;       /* Border color */
```

#### Shadows
```css
--slidelang-shadow-main: 0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -1px rgba(0, 0, 0, 0.06);
--slidelang-shadow-lg: 0 25px 50px -12px rgba(0, 0, 0, 0.25);
--slidelang-shadow-xl: 0 35px 60px -12px rgba(0, 0, 0, 0.3);
--slidelang-shadow-text: rgba(0, 0, 0, 0.25);
--slidelang-shadow-light: rgba(0, 0, 0, 0.05);
--slidelang-shadow-medium: rgba(0, 0, 0, 0.15);
```

#### Transitions
```css
--slidelang-transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
--slidelang-transition-fast: all 0.1s cubic-bezier(0.4, 0, 0.2, 1);
```

### 🎨 **Component-Specific Colors**

#### Callouts & Notes
```css
--slidelang-note-color: #6b7280;
--slidelang-note-text-color: #374151;
--slidelang-details-border-color: #d1d5db;
--slidelang-details-text-color: #374151;
--slidelang-success-text-color: #065f46;
--slidelang-warning-text-color: #92400e;
--slidelang-danger-text-color: #991b1b;
--slidelang-info-text-color: #1e40af;
```

#### Highlighting
```css
--slidelang-highlight-bg: #fbbf24;
--slidelang-highlight-text: #92400e;
```

#### Code Syntax Highlighting
```css
--slidelang-syntax-comment: #6b7280;     /* Comments */
--slidelang-syntax-keyword: #8b5cf6;     /* Keywords */
--slidelang-syntax-string: #10b981;      /* Strings */
--slidelang-syntax-number: #f59e0b;      /* Numbers */
--slidelang-syntax-operator: #ef4444;    /* Operators */
--slidelang-syntax-function: #06b6d4;    /* Functions */
--slidelang-syntax-variable: #8b5cf6;    /* Variables */
--slidelang-syntax-type: #10b981;        /* Types */
```

#### Advanced UI Elements
```css
--slidelang-bg-copy-button: rgba(59, 130, 246, 0.15);
--slidelang-border-copy-button: rgba(59, 130, 246, 0.3);
--slidelang-bg-copy-button-hover: rgba(59, 130, 246, 0.25);
--slidelang-bg-line-highlight: rgba(16, 185, 129, 0.1);
--slidelang-bg-diff-added: rgba(16, 185, 129, 0.15);
--slidelang-bg-diff-removed: rgba(239, 68, 68, 0.15);
--slidelang-bg-image-overlay: rgba(31, 41, 55, 0.7);
--slidelang-bg-comparison-label: rgba(31, 41, 55, 0.9);
--slidelang-bg-lightbox: rgba(31, 41, 55, 0.95);
```

## 📊 **Required vs Optional Variables**

### ✅ **Required Variables** (Must be defined)
- All primary colors (`--slidelang-primary-color`, etc.)
- Text colors (`--slidelang-text-color`, etc.)
- Background colors (`--slidelang-background-color`, etc.)
- Font families (`--slidelang-font-main`, etc.)
- Slide backgrounds (`--slidelang-bg-*-slide`)

### 🔸 **Optional Variables** (Have defaults)
- Advanced UI elements (copy buttons, diff highlighting)
- Extended syntax highlighting
- Complex shadows and gradients

## 🎯 **Validation**

Your theme should define at minimum 60+ variables. Missing required variables will fall back to defaults, but may cause visual inconsistencies.

### Quick Validation Checklist
- [ ] All slide type backgrounds defined
- [ ] Primary, secondary, accent colors set
- [ ] Text colors for all backgrounds
- [ ] Font families specified
- [ ] Basic component colors defined

## 📝 **Theme JSON Format**

External themes use this JSON structure:

```json
{
  "name": "my-custom-theme",
  "description": "My awesome presentation theme",
  "author": "Your Name",
  "version": "1.0.0",
  "variables": {
    "--slidelang-primary-color": "#3b82f6",
    "--slidelang-background-color": "#ffffff",
    "...": "all other variables"
  }
}
```

---

**Next:** Learn how to create themes in the [Theme Creation Guide](theme-creation.md).
