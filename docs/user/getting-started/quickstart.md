# Quickstart: Create Your First Presentation

Get up and running with ZiraDocs in under 5 minutes! This guide will walk you through creating and building your first presentation.

## 🚀 **Prerequisites**

- ZiraDocs CLI installed ([Installation Guide](installation.md))
- Text editor of your choice
- Basic familiarity with Markdown (optional, but helpful)

## 📝 **Step 1: Create Your First Presentation**

Create a new file called `my-first-presentation.slidelang` and add the following content:

```slidelang
---
title: My First ZiraDocs Presentation
author: Your Name
theme: default
mode: strict
---

SLIDE title
  heading: "Welcome to ZiraDocs!"
  subtitle: "Creating beautiful presentations made simple"

SLIDE content
  title: "Why ZiraDocs?"
  TEXT
    ZiraDocs makes creating presentations fast and enjoyable with:
  POINTS
    - Clean, readable syntax
    - Beautiful default themes
    - AI-powered content assistance
    - Multiple output formats

SLIDE content
  title: "Getting Started is Easy"
  TEXT
    Just write your content in simple text files and let ZiraDocs handle the rest.
  
  CODE language="bash"
    # Build your presentation
    slidelang build my-presentation.slidelang
```

## 🔨 **Step 2: Build Your Presentation**

Open your terminal and run:

```bash
slidelang build my-first-presentation.slidelang
```

This will generate:
- `dist/my-first-presentation.html` - Your presentation
- `dist/presentation.css` - Styles
- `dist/presentation.js` - Interactive features

## 🌐 **Step 3: View Your Presentation**

Open the generated HTML file in your browser:

```bash
# On Windows
start dist/my-first-presentation.html

# On macOS
open dist/my-first-presentation.html

# On Linux
xdg-open dist/my-first-presentation.html
```

**🎉 Congratulations!** You've created your first ZiraDocs presentation!

## ⚡ **Quick Commands Reference**

```bash
# Basic build
slidelang build presentation.slidelang

# Build with custom output directory
slidelang build presentation.slidelang --output ./build

# Build with embedded assets (single file)
slidelang build presentation.slidelang --embed-assets

# Use a different theme
slidelang build presentation.slidelang --theme dark

# Check for errors without building
slidelang build presentation.slidelang --lint-only
```

## 🎨 **Try Different Syntax Modes**

### **Flex Mode (Markdown-like)**

Create `my-flex-presentation.md`:

```markdown
---
title: Flex Mode Example
mode: flex
theme: minimal
---

# Hello, Flex Mode!
## This feels just like Markdown

- Easy to write
- Familiar syntax
- Perfect for quick presentations

---

## Second Slide

You can use standard Markdown elements:

![Example Image](https://via.placeholder.com/400x300)

> "ZiraDocs makes presentations simple and beautiful"

---

## Code Examples

```javascript
function greet(name) {
    return `Hello, ${name}!`;
}

console.log(greet("ZiraDocs"));
```
```

Build it the same way:
```bash
slidelang build my-flex-presentation.md
```

## 🎯 **What's Next?**

Now that you have a working presentation, explore these topics:

### **📖 Learn the Language**
- **[Strict Mode Syntax](../language-reference/strict-mode.md)** - Structured, precise syntax
- **[Flex Mode Syntax](../language-reference/flex-mode.md)** - Markdown-like syntax
- **[FrontMatter Configuration](../language-reference/frontmatter.md)** - Customize your presentations

### **✨ Add Advanced Features**
- **[Themes & Styling](../features/themes-styling.md)** - Customize the look and feel
- **[Charts & Diagrams](../features/charts-diagrams.md)** - Add data visualizations
- **[Interactive Elements](../features/dynamic-interactive.md)** - Polls, Q&A, and more

### **📚 See Examples**
- **[Corporate Presentation](../guides/examples/corporate-deck.md)**
- **[Technical Talk](../guides/examples/technical-talk.md)**
- **[Interactive Workshop](../guides/examples/interactive-workshop.md)**

### **⚙️ Advanced Usage**
- **[CLI Reference](../cli-reference/commands.md)** - Complete command documentation
- **[Configuration](../cli-reference/configuration.md)** - Project-level settings
- **[Best Practices](../guides/best-practices.md)** - Writing effective presentations

## 🆘 **Need Help?**

- **Common issues:** Check the [Troubleshooting Guide](../guides/troubleshooting.md)
- **Ask questions:** Open an issue on [GitHub](https://github.com/ziradocs/toolchain/issues)
- **Join the community:** [Discord](https://discord.gg/slidelang) (coming soon)

---

**🚀 Ready to create amazing presentations? Let's go!**
