# 🎨 Theme Creation Guide

This step-by-step guide will walk you through creating a custom ZiraDocs theme from scratch. No CLI knowledge required!

## 🎯 **What You'll Learn**

- How to design a cohesive theme color palette
- Creating your first theme JSON file
- Testing your theme with ZiraDocs
- Best practices for theme design
- Publishing and sharing themes

## 🚀 **Prerequisites**

- Basic understanding of colors (hex codes, RGB)
- Text editor (VS Code, Sublime, etc.)
- Optional: Color palette tools (Adobe Color, Coolors.co)

## 📋 **Step 1: Planning Your Theme**

### Define Your Theme Concept
Ask yourself:
- **Purpose:** Professional, creative, educational, branded?
- **Mood:** Modern, classic, playful, serious?
- **Audience:** Corporate, academic, design-focused?

### Choose Your Color Palette
Pick 3-5 main colors:
- **Primary:** Your main brand/theme color
- **Secondary:** Supporting color (often neutral)
- **Accent:** Highlight color for emphasis
- **Success/Warning/Error:** Semantic colors

### Example: "Ocean Blue" Theme
```
Primary: #0ea5e9 (sky blue)
Secondary: #64748b (slate gray)  
Accent: #06b6d4 (cyan)
Success: #10b981 (emerald)
Warning: #f59e0b (amber)
Error: #ef4444 (red)
```

## 📝 **Step 2: Create Your Theme File**

### Basic Structure
Create a new file named `my-theme.json`:

```json
{
  "name": "ocean-blue",
  "description": "A professional ocean-inspired theme with blue gradients",
  "author": "Your Name",
  "version": "1.0.0",
  "variables": {
    // Variables go here
  }
}
```

### Add Essential Colors
Start with the core color variables:

```json
{
  "name": "ocean-blue",
  "description": "A professional ocean-inspired theme with blue gradients",
  "author": "Your Name", 
  "version": "1.0.0",
  "variables": {
    // Primary colors
    "--slidelang-primary-color": "#0ea5e9",
    "--slidelang-secondary-color": "#64748b",
    "--slidelang-accent-color": "#06b6d4",
    
    // Semantic colors
    "--slidelang-success-color": "#10b981",
    "--slidelang-warning-color": "#f59e0b",
    "--slidelang-danger-color": "#ef4444",
    "--slidelang-info-color": "#0ea5e9",
    "--slidelang-tip-color": "#8b5cf6",
    
    // Background colors
    "--slidelang-background-color": "#ffffff",
    "--slidelang-bg-white": "#ffffff",
    "--slidelang-bg-gray-50": "#f8fafc",
    "--slidelang-bg-gray-100": "#f1f5f9",
    "--slidelang-bg-light": "#f8fafc",
    
    // Text colors
    "--slidelang-text-color": "#1e293b",
    "--slidelang-text-light": "#64748b",
    "--slidelang-text-muted": "#94a3b8",
    "--slidelang-text-on-primary": "#ffffff",
    "--slidelang-text-on-accent": "#ffffff",
    "--slidelang-text-on-dark": "#ffffff"
  }
}
```

## 🎨 **Step 3: Design Slide Backgrounds**

Each slide type needs its own background. Think about the flow:

```json
// Add to your variables section:
"--slidelang-bg-title-slide": "#0f172a",     // Dark for impact
"--slidelang-bg-section-slide": "#0ea5e9",   // Primary color
"--slidelang-bg-content-slide": "#ffffff",   // Clean white
"--slidelang-bg-end-slide": "#10b981",       // Success green
"--slidelang-bg-closing-slide": "#0ea5e9"    // Back to primary
```

### Pro Tips for Slide Backgrounds:
- **Title slides:** Dark or bold colors for impact
- **Content slides:** Light colors for readability
- **Section dividers:** Medium contrast, brand colors
- **End slides:** Positive colors (green, blue)

## 🔤 **Step 4: Typography Settings**

Choose fonts that match your theme's personality:

```json
// Add to your variables:
"--slidelang-font-main": "'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif",
"--slidelang-font-code": "'JetBrains Mono', 'Fira Code', 'SF Mono', Consolas, monospace",
"--slidelang-font-heading": "'Inter', -apple-system, BlinkMacSystemFont, sans-serif"
```

### Font Pairing Guide:
- **Professional:** Inter, Roboto, Open Sans
- **Creative:** Poppins, Nunito, Comfortaa  
- **Technical:** JetBrains Mono, Source Code Pro
- **Classic:** Georgia, Times New Roman, serif fonts

## 🎭 **Step 5: Visual Effects**

Add personality with gradients, shadows, and borders:

```json
// Gradients
"--slidelang-gradient-bg": "linear-gradient(135deg, #0ea5e9 0%, #0284c7 100%)",
"--slidelang-title-gradient": "linear-gradient(135deg, #0f172a 0%, #1e293b 100%)",
"--slidelang-accent-gradient": "linear-gradient(135deg, #06b6d4 0%, #0891b2 100%)",

// Borders and radius
"--slidelang-border-radius": "0.75rem",      // Slightly rounded
"--slidelang-border-radius-lg": "1.25rem",   // More rounded
"--slidelang-border-radius-sm": "0.5rem",    // Less rounded
"--slidelang-border-color": "#e2e8f0",

// Shadows (subtle for professional look)
"--slidelang-shadow-main": "0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -1px rgba(0, 0, 0, 0.06)",
"--slidelang-shadow-lg": "0 20px 25px -5px rgba(0, 0, 0, 0.1), 0 10px 10px -5px rgba(0, 0, 0, 0.04)",

// Smooth transitions
"--slidelang-transition": "all 0.3s cubic-bezier(0.4, 0, 0.2, 1)"
```

## 🎨 **Step 6: Component Colors**

Style callouts, notes, and other components:

```json
// Callout backgrounds
"--slidelang-bg-info": "#eff6ff",
"--slidelang-bg-success": "#ecfdf5", 
"--slidelang-bg-warning": "#fffbeb",
"--slidelang-bg-danger": "#fef2f2",
"--slidelang-bg-tip": "#f0f9ff",
"--slidelang-bg-note": "#f8fafc",

// Text colors for callouts
"--slidelang-info-text-color": "#1e40af",
"--slidelang-success-text-color": "#065f46",
"--slidelang-warning-text-color": "#92400e", 
"--slidelang-danger-text-color": "#991b1b",

// Code syntax highlighting
"--slidelang-syntax-comment": "#64748b",
"--slidelang-syntax-keyword": "#0ea5e9",
"--slidelang-syntax-string": "#10b981",
"--slidelang-syntax-number": "#f59e0b",
"--slidelang-syntax-function": "#06b6d4"
```

## ✅ **Step 7: Complete Theme Template**

Here's your complete `ocean-blue.json` theme:

<details>
<summary>Click to expand complete theme</summary>

```json
{
  "name": "ocean-blue",
  "description": "A professional ocean-inspired theme with blue gradients and modern typography",
  "author": "Your Name",
  "version": "1.0.0",
  "variables": {
    "--slidelang-primary-color": "#0ea5e9",
    "--slidelang-secondary-color": "#64748b",
    "--slidelang-accent-color": "#06b6d4",
    "--slidelang-success-color": "#10b981",
    "--slidelang-warning-color": "#f59e0b",
    "--slidelang-danger-color": "#ef4444",
    "--slidelang-info-color": "#0ea5e9",
    "--slidelang-tip-color": "#8b5cf6",

    "--slidelang-gradient-bg": "linear-gradient(135deg, #0ea5e9 0%, #0284c7 100%)",
    "--slidelang-title-gradient": "linear-gradient(135deg, #0f172a 0%, #1e293b 100%)",
    "--slidelang-accent-gradient": "linear-gradient(135deg, #06b6d4 0%, #0891b2 100%)",

    "--slidelang-font-main": "'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif",
    "--slidelang-font-code": "'JetBrains Mono', 'Fira Code', 'SF Mono', Consolas, monospace",
    "--slidelang-font-heading": "'Inter', -apple-system, BlinkMacSystemFont, sans-serif",

    "--slidelang-border-radius": "0.75rem",
    "--slidelang-border-radius-lg": "1.25rem",
    "--slidelang-border-radius-sm": "0.5rem",
    "--slidelang-border-color": "#e2e8f0",

    "--slidelang-shadow-main": "0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -1px rgba(0, 0, 0, 0.06)",
    "--slidelang-shadow-lg": "0 20px 25px -5px rgba(0, 0, 0, 0.1), 0 10px 10px -5px rgba(0, 0, 0, 0.04)",
    "--slidelang-transition": "all 0.3s cubic-bezier(0.4, 0, 0.2, 1)",

    "--slidelang-text-color": "#1e293b",
    "--slidelang-text-light": "#64748b",
    "--slidelang-text-muted": "#94a3b8",
    "--slidelang-text-on-primary": "#ffffff",
    "--slidelang-text-on-accent": "#ffffff",
    "--slidelang-text-on-dark": "#ffffff",

    "--slidelang-background-color": "#ffffff",
    "--slidelang-bg-white": "#ffffff",
    "--slidelang-bg-gray-50": "#f8fafc",
    "--slidelang-bg-gray-100": "#f1f5f9",
    "--slidelang-bg-light": "#f8fafc",

    "--slidelang-bg-title-slide": "#0f172a",
    "--slidelang-bg-section-slide": "#0ea5e9",
    "--slidelang-bg-content-slide": "#ffffff",
    "--slidelang-bg-end-slide": "#10b981",
    "--slidelang-bg-closing-slide": "#0ea5e9",

    "--slidelang-bg-info": "#eff6ff",
    "--slidelang-bg-success": "#ecfdf5",
    "--slidelang-bg-warning": "#fffbeb",
    "--slidelang-bg-danger": "#fef2f2",
    "--slidelang-bg-tip": "#f0f9ff",
    "--slidelang-bg-note": "#f8fafc",

    "--slidelang-info-text-color": "#1e40af",
    "--slidelang-success-text-color": "#065f46",
    "--slidelang-warning-text-color": "#92400e",
    "--slidelang-danger-text-color": "#991b1b",

    "--slidelang-syntax-comment": "#64748b",
    "--slidelang-syntax-keyword": "#0ea5e9",
    "--slidelang-syntax-string": "#10b981",
    "--slidelang-syntax-number": "#f59e0b",
    "--slidelang-syntax-operator": "#ef4444",
    "--slidelang-syntax-function": "#06b6d4",
    "--slidelang-syntax-variable": "#8b5cf6"
  }
}
```
</details>

## 🧪 **Step 8: Testing Your Theme**

### Test with ZiraDocs CLI
```bash
slidelang build presentation.slidelang --theme external:ocean-blue.json
```

### Preview in Browser
1. Generate your presentation
2. Open the HTML file
3. Check all slide types
4. Test responsive behavior
5. Verify code highlighting

### Testing Checklist:
- [ ] All slide types look good
- [ ] Text is readable on all backgrounds
- [ ] Colors work well together
- [ ] Code blocks are highlighted properly
- [ ] Callouts and notes stand out appropriately

## 🎨 **Design Best Practices**

### Color Harmony
- **Analogous:** Colors next to each other (blues and greens)
- **Complementary:** Opposite colors (blue and orange)
- **Triadic:** Three evenly spaced colors
- **Monochromatic:** Different shades of one color

### Contrast Guidelines
- **High contrast:** Dark text on light backgrounds
- **Medium contrast:** For secondary information
- **Low contrast:** For disabled or muted elements

### Typography Rules
- **Max 2-3 fonts:** Keep it simple
- **Hierarchy:** Different sizes/weights for headings
- **Readability:** Adequate line spacing and size

## 🚀 **Advanced Techniques**

### Dynamic Gradients
```json
"--slidelang-gradient-bg": "linear-gradient(45deg, #0ea5e9 0%, #06b6d4 50%, #10b981 100%)"
```

### Custom Shadows
```json
"--slidelang-shadow-main": "0 8px 32px rgba(14, 165, 233, 0.2)"
```

### Brand Integration
- Use your brand colors exactly
- Match your website/app styling
- Include brand-specific gradients

## 📤 **Publishing Your Theme**

### Package Structure
```
my-theme/
├── theme.json          # Main theme file
├── README.md          # Usage instructions
├── preview.png        # Theme screenshot
└── examples/          # Sample presentations
```

### Documentation Template
```markdown
# Ocean Blue Theme

Professional ocean-inspired theme for ZiraDocs presentations.

## Installation
slidelang build --theme external:ocean-blue.json

## Preview
![Preview](preview.png)

## Features
- Ocean-inspired color palette
- Modern typography with Inter font
- Smooth gradients and shadows
- Professional callout styling
```

---

**Next:** Learn about [Renderer Implementation](renderer-implementation.md) if you're building custom rendering tools.
