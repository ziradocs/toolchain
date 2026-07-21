# DocLang Theme System - Quick Reference

**Updated:** October 13, 2025  
**Version:** 1.1.0

---

## 🎨 Available Themes

### 1. Professional (Default)
**Best for:** Corporate reports, business documents, proposals

**Characteristics:**
- Modern sans-serif (Segoe UI)
- Vibrant blue accents (#3498db)
- Clean spacing and shadows
- Professional gradients in tables

**TOC Style:** Modern blue with gradient hover effects

```bash
./doclang build document.doclang --theme professional --toc
```

---

### 2. Academic
**Best for:** Research papers, academic publications, theses

**Characteristics:**
- Classic serif fonts (Georgia, Times)
- Formal black and white
- High contrast for readability
- Traditional academic formatting

**TOC Style:** Classic with black borders and formal hierarchy

```bash
./doclang build paper.doclang --theme academic --toc --numbering
```

---

### 3. Technical
**Best for:** API docs, technical manuals, developer documentation

**Characteristics:**
- Monospace everywhere (Courier New)
- Terminal-style flat design
- Sharp corners, no shadows
- Classic blue hyperlinks (#0000ee)

**TOC Style:** Flat terminal aesthetic with minimal decoration

```bash
./doclang build api-docs.doclang --theme technical --toc
```

---

### 4. Page-View
**Best for:** Print-ready documents, formal reports, Word-style output

**Characteristics:**
- Calibri fonts (Word-like)
- Visual page separation
- Gray canvas background
- Headers and footers with page numbers
- Page shadows (paper effect)

**TOC Style:** Office blue with clean spacing

```bash
./doclang build report.doclang --theme page-view --toc --page-breaks
```

**Special Features:**
- ✅ Visual page containers (210mm × 297mm - A4)
- ✅ Page shadows for depth
- ✅ Automatic headers with document title
- ✅ Automatic footers with page numbers
- ✅ Gray background canvas (#f5f5f5)
- ✅ 40px spacing between pages

---

## 📊 Theme Comparison

| Feature | Professional | Academic | Technical | Page-View |
|---------|-------------|----------|-----------|-----------|
| **Font Family** | Segoe UI | Georgia Serif | Courier Mono | Calibri |
| **Primary Color** | Blue (#3498db) | Black (#000000) | Monochrome | Office Blue (#4472c4) |
| **Shadows** | Yes (subtle) | Minimal | None (flat) | Yes (paper effect) |
| **Tables** | Gradient headers | Simple borders | Terminal style | Office style |
| **TOC Bullets** | ▸ Modern arrow | • Classic dot | • Simple dot | ▸ Modern arrow |
| **Visual Pages** | No | No | No | **Yes** |
| **Headers/Footers** | No | No | No | **Yes** |
| **Best For** | Business | Academia | Tech Docs | Print-Ready |

---

## 🎯 TOC Design by Theme

### Professional
```
📘 Tabla de Contenidos
  ▸ 1. Introduction
    • 1.1 Background
    • 1.2 Objectives
  ▸ 2. Methodology
```
- **Background:** Light gray (#f8f9fa)
- **Accent:** Vibrant blue (#3498db)
- **Hover:** Blue transitions

### Academic
```
📕 Tabla de Contenidos
  • 1. Introduction
    ◦ 1.1 Background
    ◦ 1.2 Objectives
  • 2. Methodology
```
- **Background:** Gray (#f5f5f5)
- **Border:** Black (#000000)
- **Hover:** Classic blue (#0066cc)

### Technical
```
📗 Tabla de Contenidos
  • 1. Introduction
    • 1.1 Background
    • 1.2 Objectives
  • 2. Methodology
```
- **Background:** Terminal gray (#f0f0f0)
- **Font:** Courier New monospace
- **Hover:** Classic link blue (#0000ee)

### Page-View
```
📙 Tabla de Contenidos
  ▸ 1. Introduction
    • 1.1 Background
    • 1.2 Objectives
  ▸ 2. Methodology
```
- **Background:** Light (#fafafa)
- **Accent:** Office blue (#4472c4)
- **Hover:** Link blue (#0563c1)

---

## 🚀 Usage Examples

### Basic Usage
```bash
# Default professional theme
./doclang build document.doclang

# With table of contents
./doclang build document.doclang --toc

# With section numbering
./doclang build document.doclang --toc --numbering
```

### Theme Selection
```bash
# Professional (default)
./doclang build doc.doclang --theme professional

# Academic
./doclang build paper.doclang --theme academic

# Technical
./doclang build api.doclang --theme technical

# Page-view with visual pages
./doclang build report.doclang --theme page-view --page-breaks
```

### Advanced Options
```bash
# Full-featured document
./doclang build document.doclang \
  --theme page-view \
  --toc \
  --numbering \
  --page-breaks \
  --output ./dist

# With custom output directory
./doclang build doc.doclang \
  --theme academic \
  --output ./output/documents
```

### Frontmatter Configuration
```yaml
---
title: "My Document"
author: "John Doe"
date: 2025-10-13
theme: page-view  # Auto-activates visual pages
---

# Introduction

Your content here...
```

---

## 🎨 CSS Variables Reference

### TOC Variables (All Themes)

| Variable | Professional | Academic | Technical | Page-View |
|----------|-------------|----------|-----------|-----------|
| `--doclang-toc-bg` | #f8f9fa | #f5f5f5 | #f0f0f0 | #fafafa |
| `--doclang-toc-border` | #3498db | #000000 | #000000 | #4472c4 |
| `--doclang-toc-title-color` | #1a202c | #000000 | #000000 | #1f4788 |
| `--doclang-toc-link-color` | #2c3e50 | #1a1a1a | #1a1a1a | #000000 |
| `--doclang-toc-link-hover` | #3498db | #0066cc | #0000ee | #0563c1 |
| `--doclang-toc-accent` | #3498db | #666666 | #000000 | #4472c4 |

### Page-View Variables

| Variable | Default Value | Description |
|----------|--------------|-------------|
| `--doclang-page-shadow` | `0 2px 8px rgba(0,0,0,0.15)` | Paper shadow effect |
| `--doclang-page-break-margin` | `40px` | Space between pages |
| `--doclang-header-height` | `15mm` | Header area height |
| `--doclang-footer-height` | `15mm` | Footer area height |
| `--doclang-header-footer-bg` | `#fafafa` | Header/footer background |

---

## 📁 File Structure

```
doclang/
├── themes/
│   └── document/
│       ├── variables.go    # 74 CSS variables
│       ├── themes.go       # 4 embedded themes
│       └── loader.go       # Theme loading system
├── internal/
│   ├── cli/
│   │   └── build.go        # --theme flag
│   └── generator/
│       ├── generator.go    # Theme options
│       └── html.go         # Theme variables pass-through
└── output/
    └── your-document.html

core/
└── renderer/
    └── document_html.go    # Theme-aware CSS + Page-view logic
```

---

## 🔧 Customization

### Creating External Themes

Create a JSON file (e.g., `custom-theme.json`):

```json
{
  "name": "custom",
  "description": "My custom theme",
  "author": "Your Name",
  "version": "1.0.0",
  "variables": {
    "--doclang-page-bg": "#ffffff",
    "--doclang-font-main": "Arial, sans-serif",
    "--doclang-h1-color": "#ff6600",
    "--doclang-toc-bg": "#fff5f0",
    "--doclang-toc-border": "#ff6600",
    ...
  }
}
```

Install and use:
```bash
# Future feature (planned)
./doclang theme install custom-theme.json
./doclang build doc.doclang --theme custom
```

---

## 🐛 Troubleshooting

### TOC not showing?
```bash
# Make sure to add --toc flag
./doclang build document.doclang --toc
```

### Page-view not rendering visual pages?
```bash
# Make sure to use page-view theme AND --page-breaks
./doclang build document.doclang --theme page-view --page-breaks
```

### Theme colors not applying?
- Check CSS variable fallbacks are working
- Verify theme is loading: look for log message "Using theme: X v1.0.0"
- Try rebuilding: `go build -o doclang ./cmd/doclang`

### Headers/footers not showing in page-view?
- Headers/footers auto-enable with `page-view` theme
- Check that `ShowHeaders` and `ShowFooters` are true in options
- Verify `page-view-mode` class on `<body>`

---

## 📊 Performance Notes

- **Theme Loading:** < 1ms (embedded themes)
- **CSS Generation:** < 5ms (74 variables)
- **Page-View Rendering:** +10-20% time (HTML structure)
- **TOC Generation:** +5ms per 100 sections
- **File Size:** +2-3KB (theme CSS variables)

---

## ✅ Quality Checklist

Before generating final documents:

- [ ] Theme selected matches document purpose
- [ ] TOC enabled if document is long (--toc)
- [ ] Section numbering if needed (--numbering)
- [ ] Page-view for print-ready output (--theme page-view --page-breaks)
- [ ] Output directory specified (--output)
- [ ] Frontmatter complete (title, author, date)
- [ ] Test in browser before distributing
- [ ] Verify all charts/diagrams render correctly
- [ ] Check page breaks don't split important content

---

## 🔗 Related Documentation

- **TOC & page breaks:** the `--toc`, `--numbering` and `--page-breaks` flags, and the
  `page-view` theme — see [`doclang/README.md`](../../doclang/README.md)

---

## 🎓 Tips & Best Practices

### For Business Documents
```bash
./doclang build quarterly-report.doclang \
  --theme professional \
  --toc \
  --numbering
```

### For Academic Papers
```bash
./doclang build research-paper.doclang \
  --theme academic \
  --toc \
  --numbering
```

### For API Documentation
```bash
./doclang build api-reference.doclang \
  --theme technical \
  --toc
```

### For Print-Ready Reports
```bash
./doclang build annual-report.doclang \
  --theme page-view \
  --toc \
  --numbering \
  --page-breaks
```

---

**Last Updated:** October 13, 2025  
**Version:** 1.1.0  
**Status:** Production Ready ✅
