# DocLang Documentation

**DocLang** is a Domain-Specific Language (DSL) for generating professional documents (HTML, PDF, DOCX) with simple, powerful syntax.

## 🎯 What is DocLang?

DocLang is ZiraDocs's sibling — designed specifically for **documents** instead of presentations. It reuses all of ZiraDocs's mature infrastructure but produces continuous documents with:

- ✅ Automatic table of contents
- ✅ Smart section numbering
- ✅ Customizable headers and footers
- ✅ Cross-references
- ✅ Footnotes
- ✅ Bibliography
- ✅ Export to HTML, PDF, DOCX

## 🚀 Quick Start

### Installation

```bash
go install go.ziradocs.com/doclang/cmd/doclang@latest
```

Verify the installation:

```bash
doclang version
```

> **Note:** DocLang is a CLI independent from ZiraDocs. For presentations, install `slidelang`.

```bash
# For documents
go install go.ziradocs.com/doclang/cmd/doclang@latest

# For presentations
go install go.ziradocs.com/slidelang/cmd/slidelang@latest
```

### Your First Document

Create a file `hello.doclang`:

```doclang
---
mode: flex
doctype: document
title: "My First Document"
author: "Your Name"
---

# Introduction

Welcome to **DocLang**! This is your first document.

## Getting Started

DocLang makes document creation:

- Fast and efficient
- Professional and polished
- Easy to maintain

## Next Steps

Ready to learn more? Check out the [full documentation](DOCLANG_OVERVIEW.md).
```

### Generating the Document

```bash
# Generate HTML
doclang build hello.doclang --format html

# Generate PDF
doclang build hello.doclang --format pdf

# Generate multiple formats
doclang build hello.doclang --format html,pdf,docx
```

## 📚 Documentation

### Core Concepts
- [**Overview**](DOCLANG_OVERVIEW.md) - full introduction to DocLang
- [**Strict Mode Syntax**](DOCLANG_SYNTAX_STRICT.md) - formal, structured syntax
- [**Flex Mode Syntax**](DOCLANG_SYNTAX_FLEX.md) - extended Markdown syntax
- [**FrontMatter Configuration**](DOCLANG_FRONTMATTER.md) - configuration and metadata

### Elements Reference
- **Text Elements** - paragraphs, lists, inline formatting
- **Code Blocks** - syntax highlighting, multiple languages
- **Tables** - tables with captions and IDs
- **Images** - images with automatic captions
- **Advanced Elements** - charts, Mermaid, maps

### Advanced Features
- **Table of Contents** - automatic generation
- **Cross References** - links between sections
- **Footnotes** - automatic footnotes
- **Bibliography** - reference system
- **Page Layout** - headers, footers, numbering

## 🎨 Syntax Modes

DocLang offers two syntax modes:

### Flex Mode (Markdown)
Familiar, fast, ideal for most cases.

```doclang
---
mode: flex
doctype: document
---

# Section Title

Regular paragraph text.

## Subsection

- Bullet point one
- Bullet point two
```

### Strict Mode (Structured)
Formal, explicit, ideal for automated generation.

```doclang
---
mode: strict
doctype: document
---

SECTION "Section Title"
  level: 1

  TEXT
    Regular paragraph text.

SECTION "Subsection"
  level: 2

  POINTS
    - Bullet point one
    - Bullet point two
```

## 🔧 CLI Commands

```bash
# Build document
doclang build <file> [options]

# Options:
#   --format, -f    Output format (html, pdf, docx)
#   --output, -o    Output directory
#   --theme, -t     Theme name
#   --watch, -w     Watch mode

# Preview document
doclang preview <file>

# Lint document
doclang lint <file> --strict

# Convert between modes
doclang convert <file> --to strict

# Generate template
doclang init --template technical-doc
```

## 📊 Output Formats

### HTML
Interactive web document with:
- Clickable table of contents
- In-document search
- Responsive design
- Print-friendly styles

### PDF
Professional PDF document with:
- Automatic pagination
- Headers and footers
- Table of contents with page numbers
- Bookmarks

### DOCX
Microsoft Word document with:
- Consistent styles
- Editable table of contents
- Full Word compatibility

## 🎨 Available Themes

- `professional` - formal corporate
- `academic` - academic/scientific
- `technical` - technical documentation
- `minimal` - clean and minimal
- `modern` - modern design
- `legal` - legal/contractual
- `report` - business reports

## 💡 Use Cases

### Technical Documentation
```bash
doclang init --template api-docs
```
Ideal for:
- API documentation
- Technical specifications
- Developer guides
- System architecture docs

### Business Reports
```bash
doclang init --template business-report
```
Ideal for:
- Quarterly reports
- Business analysis
- Project status reports
- Executive summaries

### Academic Papers
```bash
doclang init --template academic-paper
```
Ideal for:
- Research papers
- Thesis documents
- Technical articles
- Conference papers

### User Documentation
```bash
doclang init --template user-manual
```
Ideal for:
- User manuals
- How-to guides
- Product documentation
- Training materials

## 🔗 Advanced Elements

### Interactive Charts
```doclang
<<chart: bar>>
  data: [["Q1", 125], ["Q2", 145], ["Q3", 167]]
  series: ["Revenue"]
<<>>
```

### Mermaid Diagrams
```doclang
<<mermaid>>
  graph TD
    A[Start] --> B[Process]
    B --> C[End]
<<>>
```

### Maps
```doclang
<<map>>
  type: world
  markers:
    - lat: 40.7128
      lng: -74.0060
      label: "New York"
<<>>
```

### Table of Contents
```doclang
<<toc>>
  depth: 3
  title: "Contents"
<<>>
```

### Cross References
```doclang
See [Section 2](#section-2) for details.

Or use: <<ref: section-2>>
```

## 🌟 Highlighted Features

### Inherited from ZiraDocs
- ✅ Robust parser (Strict & Flex modes)
- ✅ Complete element registry
- ✅ Variables and templates
- ✅ Diagnostics and linting
- ✅ Theme system
- ✅ AI content detection

### Specific to DocLang
- 🆕 Document-specific layouts
- 🆕 Table of contents generation
- 🆕 Page numbering system
- 🆕 Header/footer management
- 🆕 Cross-referencing
- 🆕 Footnotes support
- 🆕 Bibliography management
- 🆕 Multi-format export

## 🛠️ Integration

### VS Code Extension
```bash
# Install the extension
code --install-extension slidelang.doclang-vscode
```

Features:
- Syntax highlighting
- Auto-completion
- Live preview
- Linting
- Snippets

## 📖 Comparison with Other Tools

| Feature | DocLang | Markdown | LaTeX | Word |
|---------|---------|----------|-------|------|
| **Easy to Learn** | ✅ | ✅ | ❌ | ✅ |
| **Professional Output** | ✅ | ⚠️ | ✅ | ✅ |
| **Version Control** | ✅ | ✅ | ✅ | ❌ |
| **Charts/Diagrams** | ✅ | ⚠️ | ⚠️ | ⚠️ |
| **Auto TOC** | ✅ | ⚠️ | ✅ | ✅ |
| **Multi-format Export** | ✅ | ⚠️ | ❌ | ⚠️ |
| **Templates** | ✅ | ❌ | ⚠️ | ✅ |

## 🤝 Contributing

Contributions are welcome! See [CONTRIBUTING.md](../CONTRIBUTING.md) for guidelines.

## 📄 License

Apache-2.0 — see [LICENSE](../../LICENSE) for details.

---

**DocLang** - Professional documents made simple.
