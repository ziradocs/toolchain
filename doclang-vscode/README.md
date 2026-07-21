# DocLang Preview - VS Code Extension

Real-time preview extension for DocLang markdown documents.

## Features

- 🔍 **Live Preview**: See your DocLang documents rendered in real-time
- 🔄 **Auto-Refresh**: Automatically updates preview when you save your markdown files
- 📑 **Table of Contents**: Optional TOC generation
- ⚡ **Fast**: Leverages DocLang CLI for instant rendering
- 🎨 **Professional Output**: Same quality HTML output as DocLang CLI

## Installation

### Prerequisites

1. Install DocLang CLI:
   ```bash
   cd doclang
   go build -o doclang cmd/doclang/main.go
   ```

2. Make sure `doclang` is in your PATH or configure the path in VS Code settings.

### Install Extension

1. From source:
   ```bash
   cd doclang-vscode
   npm install
   npm run compile
   ```

2. Press `F5` to launch Extension Development Host

## Usage

### Open Preview

1. Open any `.md` file
2. Use one of these methods:
   - **Command Palette** (`Cmd+Shift+P` / `Ctrl+Shift+P`): `DocLang: Open Preview`
   - **Editor Title Bar**: Click the preview icon
   - **Right-click**: Select `DocLang: Open Preview`

### Preview to the Side

Open preview in a split view:
- **Command Palette**: `DocLang: Open Preview to the Side`
- **Editor Title Bar**: Click the split preview icon

### Refresh Preview

Manually refresh the preview:
- **Command Palette**: `DocLang: Refresh Preview`
- **Preview Window**: Click the "↻ Refresh" button

## Configuration

Configure DocLang Preview in VS Code settings (`Cmd+,` / `Ctrl+,`):

```json
{
  // Path to DocLang CLI executable
  "doclang.executablePath": "doclang",
  
  // Auto-refresh preview on file save
  "doclang.autoRefresh": true,
  
  // Enable table of contents
  "doclang.tocEnabled": true,
  
  // Where to open preview: "beside" or "active"
  "doclang.previewColumn": "beside"
}
```

### Custom DocLang Path

If DocLang is not in your PATH:

```json
{
  "doclang.executablePath": "/Users/you/cli/doclang/doclang"
}
```

## Workspace Integration

The extension automatically looks for DocLang CLI in your workspace:

```
workspace/
├── doclang/
│   └── doclang          # Auto-detected
├── docs/
│   └── guide.md         # Your markdown files
└── doclang-vscode/      # This extension
```

## Features in Detail

### Auto-Refresh

When enabled (default), the preview automatically updates when you save your markdown file:

- ✅ Real-time updates
- ✅ Preserves scroll position
- ✅ No manual refresh needed

### Table of Contents

Hierarchical TOC with multiple levels:

```markdown
# Main Title
## Section 1
### Subsection 1.1
## Section 2
```

Enable/disable in settings:
```json
{
  "doclang.tocEnabled": true
}
```

### Error Handling

When DocLang CLI fails:

- 🔴 Clear error messages
- 🔄 Retry button
- 📝 Detailed error output

## Commands

| Command | Description | Keybinding |
|---------|-------------|------------|
| `DocLang: Open Preview` | Open preview in active column | - |
| `DocLang: Open Preview to the Side` | Open preview beside editor | - |
| `DocLang: Refresh Preview` | Manually refresh preview | - |

## Development

### Build from Source

```bash
cd doclang-vscode
npm install
npm run compile
```

### Watch Mode

```bash
npm run watch
```

### Run Tests

```bash
npm test
```

### Package Extension

```bash
npm run package
vsce package
```

## Troubleshooting

### "DocLang CLI not found"

**Solution**: Configure the path in settings:

```json
{
  "doclang.executablePath": "/absolute/path/to/doclang"
}
```

### Preview not updating

**Solution**: 
1. Check auto-refresh is enabled
2. Manually refresh with `DocLang: Refresh Preview`
3. Check DocLang CLI is working: `doclang build yourfile.md`

### Slow rendering

**Solution**:
- Large documents may take longer
- Consider splitting into smaller files
- Check DocLang CLI performance

## Architecture

```
┌─────────────────────┐
│   VS Code Editor    │
│   (Markdown File)   │
└──────────┬──────────┘
           │
           │ Save Event
           ▼
┌─────────────────────┐
│  Preview Manager    │
│  (File Watching)    │
└──────────┬──────────┘
           │
           │ Build Request
           ▼
┌─────────────────────┐
│  DocLang Builder    │
│  (CLI Execution)    │
└──────────┬──────────┘
           │
           │ HTML Output
           ▼
┌─────────────────────┐
│   Webview Panel     │
│  (HTML Rendering)   │
└─────────────────────┘
```

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Submit a pull request

## License

Apache-2.0 — see the [LICENSE](../LICENSE) at the repository root.

## Links

- [DocLang Documentation](../docs/doclang/)
- [SlideLang](../slidelang/)
- [Report Issues](https://github.com/ziradocs/toolchain/issues)

## Changelog

### 0.1.0 (Initial Release)

- ✨ Live preview for markdown files
- 🔄 Auto-refresh on save
- 📑 Table of contents support
- ⚙️ Configurable DocLang CLI path
- 🎨 Professional HTML rendering
