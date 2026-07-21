# Change Log

All notable changes to the "doclang-vscode" extension will be documented in this file.

## [0.1.0] - 2025-10-10

### Added
- Initial release of DocLang Preview extension
- Live preview for Markdown files using DocLang CLI
- Auto-refresh on file save
- Manual refresh command
- Preview to the side functionality
- Table of contents support (configurable)
- Custom DocLang CLI path configuration
- Auto-detection of workspace DocLang installation
- Error handling with retry functionality
- Scroll position preservation on refresh
- Professional HTML rendering
- Webview-based preview panel

### Commands
- `DocLang: Open Preview` - Open preview in active column
- `DocLang: Open Preview to the Side` - Open preview beside editor
- `DocLang: Refresh Preview` - Manually refresh the preview

### Settings
- `doclang.executablePath` - Path to DocLang CLI executable
- `doclang.autoRefresh` - Enable/disable auto-refresh on save
- `doclang.tocEnabled` - Enable/disable table of contents
- `doclang.previewColumn` - Where to open preview (beside/active)

### Features
- Real-time Markdown preview
- Professional document rendering
- Hierarchical table of contents
- Mermaid diagram support
- Code syntax highlighting
- Responsive layout
- Dark mode support

## [Unreleased]

### Planned
- PDF export from preview
- Custom CSS themes
- Split view synchronization
- Outline view integration
- Snippet support
- Live collaboration
