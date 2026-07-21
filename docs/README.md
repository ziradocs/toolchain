# ZiraDocs / DocLang — Documentation

Welcome to the documentation for both DSLs in this monorepo. This documentation is organized by
audience and use case.

> **Where this is headed:** user-facing guides (getting started, language reference, features,
> tutorials) are moving to **docs.ziradocs.com** as their canonical home, once that site has
> equivalent coverage — see the [`docs/user/` migration issue](https://github.com/ziradocs/toolchain/issues)
> for status. Until then, `docs/user/` here stays current and complete. Developer/architecture
> docs, the formal spec, and DocLang's own reference stay in this repo either way.

## Quick Navigation

### For Users
- **[Getting Started](user/getting-started/)** — Installation, quickstart, and first presentation
- **[Language Reference](user/language-reference/)** — Complete DSL syntax guide
- **[Features](user/features/)** — Themes, variables, interactive elements
- **[Guides](user/guides/)** — Playground, generating with LLMs, syntax-mode choice, migration

### DocLang
- **[DocLang docs](doclang/)** — overview, frontmatter, strict/flex syntax reference, examples

### For Developers
- **[Developer docs](developer/)** — Chromium integration, docxgo fork, PlantUML offline, theme system
- **[Architecture](architecture/)** — JSON/AST contract, HTML sanitization
- **[Language Specification](../core/spec/)** — the formal grammar/semantics (v0.1) and the versioned AST/JSON contract

### For Theme Creators
- **[Theme Implementation Guide](user/theme-implementation/)** — Complete guide for creating custom themes

## Quick Start

1. **New to ZiraDocs?** → Start with [Getting Started](user/getting-started/quickstart.md)
2. **Want to contribute?** → Check the root [Contributing Guide](../CONTRIBUTING.md)
3. **Creating themes?** → See [Theme Implementation Guide](user/theme-implementation/)
4. **Looking for specific features?** → Browse [Features](user/features/)
5. **Security concerns?** → Read [SECURITY.md](../SECURITY.md)

## What is ZiraDocs?

ZiraDocs is a Domain Specific Language (DSL) for creating presentations, optimized for both AI generation and human authoring. It offers:

- **Two syntax modes:** Strict (structured) and Flex (markdown-like)
- **Rich features:** Interactive elements, charts, maps, themes, variables
- **AI-optimized:** Designed for AI content generation, with a dedicated [`llm-kit/`](../llm-kit/)
- **Multiple outputs:** HTML, PDF, DOCX (doclang), Markdown (doclang, partial)
- **Secure by default:** HTML sanitization, URL scheme blocking, CSP on generated output

## Security

See [SECURITY.md](../SECURITY.md) for the sanitization model and how to report a vulnerability.

## Documentation Status

If you find an issue or gap in this documentation, search or open an issue in the
[issue tracker](https://github.com/ziradocs/toolchain/issues) with the `documentation` label.
