# DocLang Documentation Index

Complete index of all DocLang documentation.

## 🎯 Start Here

- **[README.md](README.md)** - main entry point with quick start
- **[DOCLANG_OVERVIEW.md](DOCLANG_OVERVIEW.md)** - full overview of the DSL

## 📖 User Documentation

### Syntax and Language
- **[DOCLANG_SYNTAX_FLEX.md](DOCLANG_SYNTAX_FLEX.md)** - extended Markdown syntax (Flex mode)
  - Headings and hierarchy
  - Basic elements (text, lists, code)
  - Advanced elements (charts, diagrams, maps)
  - DocLang-specific elements (TOC, references)
  - Full examples

- **[DOCLANG_SYNTAX_STRICT.md](DOCLANG_SYNTAX_STRICT.md)** - structured syntax (Strict mode)
  - Section declarations (SECTION)
  - Explicit keywords (TEXT, POINTS, CODE)
  - Layouts and validation
  - DocLang-specific elements
  - Best practices

- **[DOCLANG_FRONTMATTER.md](DOCLANG_FRONTMATTER.md)** - configuration and metadata
  - Essential fields
  - Page configuration (size, margins)
  - Headers and footers
  - Table of contents (TOC)
  - Numbering
  - References and bibliography
  - Custom variables
  - Full examples

## 📚 Examples

### Complete Documents
- **[examples/technical-specification.doclang](examples/technical-specification.doclang)** - complete technical specification
  - ~650 lines of real-world example
  - API documentation
  - Every element in use
  - Headers/footers configured
  - Automatic TOC
  - Charts, diagrams, maps
  - Cross-references

## 🎨 Themes and Design

### Available Themes
- `professional` - formal corporate
- `academic` - academic/scientific
- `technical` - technical documentation
- `minimal` - clean and minimal
- `modern` - modern design
- `legal` - legal/contractual
- `report` - business reports

## 🔧 CLI Reference

### Main Commands
```bash
# Build
doclang build <file> --format html,pdf,docx

# Preview
doclang preview <file>

# Watch mode
doclang watch <file>

# Lint
doclang lint <file>

# Init
doclang init --template <template-name>
```

### Options and Flags
```bash
--format, -f    Output format (html, pdf, docx)
--output, -o    Output directory
--theme, -t     Theme name
--watch, -w     Watch mode
--verbose, -v   Verbose output
```

## 📊 Comparisons

### vs. Other Tools
| Feature | DocLang | Markdown | LaTeX | Word |
|---------|---------|----------|-------|------|
| Ease of use | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐ | ⭐⭐⭐⭐ |
| Professional output | ⭐⭐⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐ |
| Git-friendly | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐ |
| Charts | ⭐⭐⭐⭐⭐ | ⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐ |

### Modes: Strict vs. Flex
| Aspect | Strict Mode | Flex Mode |
|--------|-------------|-----------|
| Syntax | Explicit keywords | Natural Markdown |
| Validation | Strict | Flexible |
| Use case | Automated generation | Manual writing |
| Learning curve | Medium | Low |

## 🚀 Getting Started

### For New Users
1. Read [README.md](README.md) for a quick start
2. Review [DOCLANG_SYNTAX_FLEX.md](DOCLANG_SYNTAX_FLEX.md) for syntax
3. See the [examples](examples/) for real cases
4. Check [DOCLANG_FRONTMATTER.md](DOCLANG_FRONTMATTER.md) for configuration

### For Developers
1. Review ZiraDocs's code (reused infrastructure)
2. Study the [technical examples](examples/)
3. Contribute following [CONTRIBUTING.md](../CONTRIBUTING.md)

## 🔗 External Links

### Resources
- [ZiraDocs Documentation](../user/README.md) - sibling DSL
- [Markdown Guide](https://www.markdownguide.org/) - Markdown syntax
- [Mermaid Documentation](https://mermaid-js.github.io/) - diagrams
- [Chart.js Documentation](https://www.chartjs.org/) - charts

### Community
- GitHub Repository: github.com/ziradocs/toolchain
- Issue Tracker: github.com/ziradocs/toolchain/issues
- Discussions: github.com/ziradocs/toolchain/discussions

## 📝 Contributing to the Documentation

### Adding Documentation
1. Fork the repository
2. Create a branch: `git checkout -b docs/new-feature`
3. Add documentation under `/docs/doclang/`
4. Update this index
5. Submit a PR

### Standards
- Use Markdown
- Include code examples
- Add screenshots where applicable
- Keep consistency with existing docs

## 🐛 Reporting Documentation Issues

If you find errors in the documentation:

1. Check it hasn't already been reported
2. Open an issue with the `documentation` label
3. Describe the problem clearly
4. Suggest a fix if possible

---

*DocLang: Professional documents made simple.*
