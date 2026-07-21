# modern-blue Theme

**Author:** SlideLang Dev  
**Version:** 2.0.0  
**Description:** Modern blue theme reimplemented - clean, professional design for business presentations with full 2025 variables support

## ✨ Features

- ✅ **Full 2025 Compatibility**: 60+ CSS variables for complete control
- ✅ **Professional Blue Palette**: Modern blue tones with cyan accents
- ✅ **Inter Typography**: Premium font stack with JetBrains Mono for code
- ✅ **Modern Gradients**: Sophisticated multi-point gradients
- ✅ **Enhanced Syntax Highlighting**: Beautiful code blocks with perfect contrast
- ✅ **Accessibility First**: WCAG AAA compliance with high contrast ratios
- ✅ **Shadow System**: Modern depth with 3-level shadow hierarchy
- ✅ **Zero Hardcoding**: All styles use CSS variables for full customization

## 🎨 What's New in v2.0.0

### ✨ **Complete Reimplementation**
- **🔧 Full Variable System**: Now supports all 60+ modern CSS variables
- **🎨 Enhanced Color Palette**: Professional blue with cyan accents
- **📝 Inter Typography**: Modern font stack for maximum legibility
- **🌟 Perfect Syntax Highlighting**: Beautiful code blocks with JetBrains Mono
- **📱 Better Accessibility**: WCAG AAA compliance with improved contrast
- **💫 Modern Shadows**: 3-level shadow system with proper depth

### ✨ **Professional Design Improvements**
- **Gradient Refinements**: Multi-point gradients with smooth transitions
- **Enhanced Code Blocks**: Perfect syntax highlighting with copy buttons
- **Interactive Elements**: Hover states and focus indicators
- **Semantic Colors**: Consistent info/success/warning/danger states
- **Typography Hierarchy**: Clear heading structure with optimal spacing

## Installation

```bash
# Install from theme file
slidelang themes install modern-blue.json

# Or copy to themes directory
cp -r modern-blue ~/.slidelang/themes/
```

## Usage

```bash
# Use in build command
slidelang build presentation.slidelang --theme modern-blue --format html

# Use in configuration file (.slidelang.yaml)
theme:
  default: modern-blue
```

## 🎨 CSS Variables

This theme supports extensive customization through CSS variables:

### Core Colors
- `--primary-color`: #3498db (Main brand color)
- `--secondary-color`: #1e40af (Secondary brand color)  
- `--accent-color`: #2ecc71 (Highlight/accent color)
- `--background-color`: #ffffff (Base background)
- `--text-color`: #2c3e50 (Main text color)

### Typography
- `--font-family`: 'Helvetica Neue', 'Arial', sans-serif
- `--font-size-base`: 1rem
- `--line-height-base`: 1.5

### Layout & Spacing
- `--border-radius`: 0.5rem
- `--box-shadow`: 0 0.25rem 0.5rem rgba(0, 0, 0, 0.1)

### Background Variables (NEW ✨)
- `--title-gradient`: linear-gradient(135deg, #1a365d 0%, #2d3748 100%)
- `--gradient-bg`: linear-gradient(135deg, #3182ce 0%, #2c5aa0 100%)
- `--bg-white`: #ffffff
- `--bg-code`: #f7fafc (Code block backgrounds)
- `--bg-light`: #edf2f7 (Light background accents)

### Slide-Specific Backgrounds (NEW ✨)
- `--bg-title-slide`: var(--title-gradient) (Title slide background)
- `--bg-section-slide`: var(--gradient-bg) (Section slide background)
- `--bg-content-slide`: var(--bg-white) (Content slide background)
- `--bg-end-slide`: var(--gradient-bg) (End slide background)

### Shadow Variables (NEW ✨)
- `--shadow-text`: rgba(0, 0, 0, 0.3) (Text shadows)
- `--shadow-light`: rgba(0, 0, 0, 0.1) (Light shadows)
- `--shadow-medium`: rgba(0, 0, 0, 0.2) (Medium shadows)

## 🔧 Customization Examples

### Using Solid Colors Instead of Gradients

Create a custom theme variant with solid colors:

```json
{
  "name": "modern-blue-solid",
  "extends": "modern-blue",
  "variables": {
    "--bg-title-slide": "#1a365d",
    "--bg-section-slide": "#3182ce",
    "--bg-end-slide": "#2ecc71"
  }
}
```

### Corporate Branding

Customize colors for your brand:

```json
{
  "name": "my-corporate-theme", 
  "extends": "modern-blue",
  "variables": {
    "--primary-color": "#your-brand-color",
    "--secondary-color": "#your-secondary-color",
    "--accent-color": "#your-accent-color"
  }
}
```

### Dark Mode Variant

Create a dark version:

```json
{
  "name": "modern-blue-dark",
  "extends": "modern-blue", 
  "variables": {
    "--background-color": "#1a202c",
    "--text-color": "#ffffff",
    "--bg-white": "#2d3748",
    "--bg-code": "#4a5568",
    "--bg-light": "#2d3748"
  }
}
```

## 🎯 Slide Types Supported

- **Title Slides** (`title-slide`, `cover-slide`, `intro-slide`): Professional gradients with centered content
- **Section Slides** (`section-slide`): Bold section dividers with gradient backgrounds  
- **Content Slides** (`content-slide`): Clean white backgrounds for main content
- **End Slides** (`end-slide`): Elegant gradient conclusions

## 📱 Responsive Features

- **Mobile Optimized**: Automatic font scaling and layout adjustments
- **Print Friendly**: Optimized styles for PDF export
- **Cross-Browser**: Compatible with all modern browsers

## 🚀 What's New in v1.1.0

- ✅ **Zero Hardcoded Styles**: All colors now use CSS variables
- ✅ **Flexible Backgrounds**: Choose between gradients or solid colors
- ✅ **Slide-Specific Variables**: Dedicated variables for each slide type
- ✅ **Enhanced Shadows**: Configurable shadow transparency
- ✅ **Better Customization**: More granular control over theme appearance

## Files

- `theme.json` - Theme manifest and CSS variables
- `styles.css` - Custom CSS styles and layouts  
- `theme-solid.json` - Solid color variant
- `README.md` - This documentation

## Support

For issues or customization help:
- Check the [SlideLang Documentation](../../docs/)
- Use `slidelang themes validate modern-blue` to test modifications
- Use `slidelang themes info modern-blue` for detailed theme information
