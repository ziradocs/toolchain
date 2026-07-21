# 🎨 Theme Implementation Guide

This guide is designed for **external theme creators** and **renderer implementors** who want to create custom themes for ZiraDocs presentations or implement ZiraDocs rendering in their own applications.

You don't need to know the internal details of the ZiraDocs CLI - this guide focuses purely on the **theme specification** and **rendering requirements**.

## 📋 **Quick Navigation**

- **[Theme Specification](theme-specification.md)** - Complete CSS variables reference
- **[Theme Creation Guide](theme-creation.md)** - Step-by-step theme development
- **[Renderer Implementation](renderer-implementation.md)** - For building custom renderers
- **[Examples & Templates](examples/)** - Sample themes and code

## 🎯 **Who This Guide Is For**

### 🎨 **Theme Creators**
- Designers wanting to create custom presentation themes
- Organizations needing branded presentation templates
- Community contributors creating theme packages

### 🔧 **Renderer Implementors**
- Developers building alternative ZiraDocs renderers
- Integration teams adding ZiraDocs support to existing tools
- Platform developers creating ZiraDocs-compatible viewers

## 🚀 **Quick Start**

### For Theme Creators
1. Read the [Theme Specification](theme-specification.md)
2. Follow the [Theme Creation Guide](theme-creation.md)
3. Use the [theme templates](examples/) as starting points

### For Renderer Implementors
1. Understand the [CSS Variables System](theme-specification.md#css-variables)
2. Review [Renderer Implementation](renderer-implementation.md)
3. Test with [example themes](examples/)

## 🔑 **Key Concepts**

### CSS Variables System
ZiraDocs uses CSS custom properties (variables) with the `--slidelang-` namespace for theming. This ensures:
- **Isolation:** No conflicts with existing CSS
- **Consistency:** Standardized variable names across all themes
- **Flexibility:** Easy customization without CSS knowledge

### Theme Types
- **Embedded Themes:** Built into the CLI, used via `--theme embedded:<name>`
- **External Themes:** JSON files, used via `--theme external:<path>`
- **Custom Renderers:** Implement the variable system in any rendering engine

### Presentation Structure
ZiraDocs presentations have specific slide types that themes must support:
- Title slides
- Section dividers  
- Content slides
- Closing slides
- Special elements (code blocks, charts, callouts)

## 📚 **Complete Documentation**

For the shipped theme list and the `--theme` resolution order, see the
[root README](../../../README.md#themes). The CSS pipeline itself
(`slidelang/internal/generator/css/`) is documented in the code.

---

**Next:** Start with the [Theme Specification](theme-specification.md) to understand the CSS variables system.
