// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
	"go.ziradocs.com/slidelang/v2/internal/generator"
	"go.ziradocs.com/slidelang/v2/internal/generator/config"
	"go.ziradocs.com/slidelang/v2/internal/generator/css/themes"
	"go.ziradocs.com/slidelang/v2/internal/generator/data"
	templateBuilder "go.ziradocs.com/slidelang/v2/internal/generator/template"
)

// PreviewThemeCmd creates a new theme preview command
func PreviewThemeCmd() *cobra.Command {
	var (
		host         string
		port         int
		watchMode    bool
		sampleSlides string
		browser      bool
	)

	cmd := &cobra.Command{
		Use:   "preview <theme-path>",
		Short: "Preview a theme with live reload",
		Long: `Preview a theme in your browser with optional live reload.

This command starts a local HTTP server to preview your theme with sample slides.
When watch mode is enabled, changes to theme files trigger automatic browser refresh.

Examples:
  slidelang themes preview ./my-theme --watch
  slidelang themes preview ./corporate-theme --port 8080
  slidelang themes preview ./themes/academic --sample ./my-slides.slidelang
  slidelang themes preview ./minimal-theme --watch --browser`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			themePath := args[0]

			// Validate theme path
			if err := validateThemePath(themePath); err != nil {
				return fmt.Errorf("invalid theme path: %v", err)
			}

			// Create preview server
			log := util.NewConsoleLogger(util.LevelInfo, true) // Initialize logger
			server := &PreviewServer{
				ThemePath:      themePath,
				Host:           host,
				Port:           port,
				SampleSlides:   sampleSlides,
				WatchMode:      watchMode,
				Browser:        browser,
				logger:         log,
				parser:         parser.New(log),            // Initialize parser
				generator:      generator.New(log),         // Initialize generator
				lastEventTimes: make(map[string]time.Time), // Initialize event tracking map
			}

			return server.Start()
		},
	}

	cmd.Flags().StringVar(&host, "host", "127.0.0.1", "Interface to bind the preview server to (use 0.0.0.0 to expose it on the network)")
	cmd.Flags().IntVarP(&port, "port", "p", 8080, "Preview server port")
	cmd.Flags().BoolVarP(&watchMode, "watch", "w", false, "Enable live reload on file changes")
	cmd.Flags().StringVarP(&sampleSlides, "sample", "s", "", "Custom sample slides file (default: built-in samples)")
	cmd.Flags().BoolVarP(&browser, "browser", "b", true, "Automatically open browser")

	return cmd
}

// PreviewServer handles theme preview functionality
type PreviewServer struct {
	ThemePath      string
	Host           string
	Port           int
	SampleSlides   string
	WatchMode      bool
	Browser        bool
	server         *http.Server
	watcher        *fsnotify.Watcher
	logger         util.Logger
	parser         *parser.Parser
	generator      *generator.Generator
	lastEventTimes map[string]time.Time // Track last event times per file
	hasChanges     bool                 // Track if there are pending changes
	lastChangeTime time.Time            // Track when the last change occurred

	// Cache to avoid regenerating content on every request
	cachedPresentation string
	cacheValid         bool
}

// Start initializes and starts the preview server
func (ps *PreviewServer) Start() error {
	fmt.Printf("🎨 Starting theme preview server...\n")
	fmt.Printf("📁 Theme: %s\n", ps.ThemePath)
	fmt.Printf("🌐 Port: %d\n", ps.Port)

	if ps.WatchMode {
		fmt.Printf("👀 Watch mode: enabled\n")
	}

	// Setup HTTP routes
	mux := http.NewServeMux()
	mux.HandleFunc("/", ps.handleIndex)
	mux.HandleFunc("/preview", ps.handlePreview)
	mux.HandleFunc("/styles.css", ps.handleStylesCSS)             // Theme-specific styles
	mux.HandleFunc("/presentation.css", ps.handlePresentationCSS) // Full system CSS
	mux.HandleFunc("/navigation.css", ps.handleNavigationCSS)     // Navigation CSS
	mux.HandleFunc("/theme.json", ps.handleThemeJSON)
	mux.HandleFunc("/reload", ps.handleReload)
	mux.HandleFunc("/ws", ps.handleWebSocket)

	// Create server. Default host is 127.0.0.1 (loopback-only) — binding to
	// all interfaces (host "" or "0.0.0.0") requires an explicit --host flag
	// so a preview server isn't reachable by anyone else on the LAN by default.
	bindHost := ps.Host
	if bindHost == "" {
		bindHost = "127.0.0.1"
	}
	ps.server = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", bindHost, ps.Port),
		Handler: mux,
	}

	// Setup file watcher if enabled
	if ps.WatchMode {
		if err := ps.setupWatcher(); err != nil {
			return fmt.Errorf("failed to setup file watcher: %v", err)
		}
		defer func() { _ = ps.watcher.Close() }()
	}

	// Open browser if requested
	if ps.Browser {
		go ps.openBrowser()
	}

	fmt.Printf("✅ Preview server running at http://%s:%d\n", bindHost, ps.Port)
	fmt.Printf("💡 Press Ctrl+C to stop\n\n")

	// Start server
	if err := ps.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server error: %v", err)
	}

	return nil
}

// handleIndex serves the main preview page
func (ps *PreviewServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	html := ps.generatePreviewHTML()
	w.Header().Set("Content-Type", "text/html")
	_, _ = w.Write([]byte(html))
}

// handlePreview serves the presentation preview
func (ps *PreviewServer) handlePreview(w http.ResponseWriter, r *http.Request) {
	// Use cached presentation if valid
	if ps.cacheValid && ps.cachedPresentation != "" {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(ps.cachedPresentation))
		return
	}

	// Generate presentation and cache it
	slides := ps.getSampleSlides()
	presentation := ps.generatePresentation(slides)

	// Cache the result
	ps.cachedPresentation = presentation
	ps.cacheValid = true

	w.Header().Set("Content-Type", "text/html")
	_, _ = w.Write([]byte(presentation))
}

// handleStylesCSS serves the theme's styles.css file (theme-specific styles)
func (ps *PreviewServer) handleStylesCSS(w http.ResponseWriter, r *http.Request) {
	ps.logger.Info("PREVIEW", "Serving theme styles.css: %s", r.URL.Path)
	cssPath := filepath.Join(ps.ThemePath, "styles.css")
	ps.serveCSS(w, r, cssPath, "styles.css")
}

// handlePresentationCSS serves the presentation.css file (full system CSS)
func (ps *PreviewServer) handlePresentationCSS(w http.ResponseWriter, r *http.Request) {
	ps.logger.Info("PREVIEW", "Serving presentation.css: %s", r.URL.Path)
	cssPath := filepath.Join(ps.ThemePath, "presentation.css")
	ps.serveCSS(w, r, cssPath, "presentation.css")
}

// handleNavigationCSS serves the navigation.css file (navigation controls)
func (ps *PreviewServer) handleNavigationCSS(w http.ResponseWriter, r *http.Request) {
	ps.logger.Info("PREVIEW", "Serving navigation.css: %s", r.URL.Path)
	cssPath := filepath.Join(ps.ThemePath, "navigation.css")
	ps.serveCSS(w, r, cssPath, "navigation.css")
}

// serveCSS is a helper method to serve CSS files with proper headers
func (ps *PreviewServer) serveCSS(w http.ResponseWriter, r *http.Request, filePath, fileName string) {
	ps.logger.Info("PREVIEW", "CSS file path: %s", filePath)

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		ps.logger.Warn("PREVIEW", "CSS file does not exist: %s", filePath)
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprintf(w, "/* %s not found */", fileName)
		return
	}

	ps.logger.Info("PREVIEW", "CSS file found, serving...")
	w.Header().Set("Content-Type", "text/css")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	http.ServeFile(w, r, filePath)
}

// handleThemeJSON serves the theme's manifest
func (ps *PreviewServer) handleThemeJSON(w http.ResponseWriter, r *http.Request) {
	jsonPath := filepath.Join(ps.ThemePath, "theme.json")
	http.ServeFile(w, r, jsonPath)
}

// handleReload provides reload endpoint for watch mode
func (ps *PreviewServer) handleReload(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Check if there are pending changes
	if ps.hasChanges {
		// Reset the changes flag
		ps.hasChanges = false
		_, _ = w.Write([]byte(`{"status": "reload", "hasChanges": true}`))
	} else {
		_, _ = w.Write([]byte(`{"status": "ok", "hasChanges": false}`))
	}
}

// handleWebSocket handles WebSocket connections for live reload.
// NOTE: not implemented yet (returns 501 below) — when this is built out,
// it must set Upgrader.CheckOrigin to reject cross-origin upgrade requests,
// same as any other same-origin-only endpoint on this server.
func (ps *PreviewServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement WebSocket for real-time updates
	w.WriteHeader(http.StatusNotImplemented)
	_, _ = w.Write([]byte("WebSocket support coming soon"))
}

// setupWatcher configures file system watching
func (ps *PreviewServer) setupWatcher() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	ps.watcher = watcher

	// Watch specific theme files instead of the entire directory
	themeFiles := []string{
		filepath.Join(ps.ThemePath, "theme.json"),
		filepath.Join(ps.ThemePath, "styles.css"),
		filepath.Join(ps.ThemePath, "presentation.css"),
		filepath.Join(ps.ThemePath, "navigation.css"),
		filepath.Join(ps.ThemePath, "README.md"),
	}

	for _, file := range themeFiles {
		if _, err := os.Stat(file); err == nil {
			// File exists, watch it
			if err := watcher.Add(file); err != nil {
				ps.logger.Warn("Failed to watch file %s: %v", file, err)
			} else {
				ps.logger.Debug("PREVIEW", "Watching file: %s", file)
			}
		}
	}

	// Also watch the theme directory for new files
	err = watcher.Add(ps.ThemePath)
	if err != nil {
		return err
	}

	// Start watching in background
	go ps.watchFiles()

	return nil
}

// watchFiles monitors file changes and triggers updates
func (ps *PreviewServer) watchFiles() {
	// Increased debounce delay for Windows
	debounceDelay := 1500 * time.Millisecond

	for {
		select {
		case event, ok := <-ps.watcher.Events:
			if !ok {
				return
			}

			// Filter only relevant file types and operations
			if ps.isRelevantFileChange(event) {
				// Advanced debouncing per file
				now := time.Now()
				fileName := event.Name

				// Check if we've already processed this file recently
				if lastTime, exists := ps.lastEventTimes[fileName]; exists {
					if now.Sub(lastTime) < debounceDelay {
						ps.logger.Debug("PREVIEW", "Debouncing event for %s", fileName)
						continue
					}
				}

				// Update last event time for this file
				ps.lastEventTimes[fileName] = now

				// Mark that there are pending changes and invalidate cache
				ps.hasChanges = true
				ps.lastChangeTime = now
				ps.cacheValid = false // Invalidate presentation cache

				ps.logger.Debug("PREVIEW", "Processing file change: %s (%s)", fileName, event.Op.String())
				fmt.Printf("📝 File changed: %s\n", event.Name)
				fmt.Printf("🔄 Refresh your browser to see changes\n")
			} else {
				ps.logger.Debug("PREVIEW", "Ignoring irrelevant file change: %s (%s)", event.Name, event.Op.String())
			}

		case err, ok := <-ps.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("Watcher error: %v", err)
		}
	}
}

// isRelevantFileChange checks if the file change is relevant for preview updates
func (ps *PreviewServer) isRelevantFileChange(event fsnotify.Event) bool {
	// Only watch for Write operations (ignore Create, Remove, Rename, Chmod)
	if event.Op&fsnotify.Write != fsnotify.Write {
		return false
	}

	// Get absolute path to avoid issues with relative paths
	absPath, err := filepath.Abs(event.Name)
	if err != nil {
		return false
	}

	// Only watch files inside the theme directory
	themeAbsPath, err := filepath.Abs(ps.ThemePath)
	if err != nil {
		return false
	}

	if !strings.HasPrefix(absPath, themeAbsPath) {
		return false
	}

	// Only watch theme-related files
	fileName := filepath.Base(event.Name)
	relevantFiles := []string{"theme.json", "styles.css", "presentation.css", "navigation.css", "README.md"}

	for _, file := range relevantFiles {
		if fileName == file {
			return true
		}
	}

	// Ignore temporary files, system files, hidden files, and backup files
	if strings.HasPrefix(fileName, ".") ||
		strings.HasPrefix(fileName, "~") ||
		strings.HasSuffix(fileName, ".tmp") ||
		strings.HasSuffix(fileName, ".swp") ||
		strings.HasSuffix(fileName, ".bak") ||
		strings.HasSuffix(fileName, ".orig") ||
		strings.Contains(fileName, "~") {
		return false
	}

	return false
}

// openBrowser attempts to open the preview in the default browser
func (ps *PreviewServer) openBrowser() {
	// Wait a moment for server to start
	time.Sleep(500 * time.Millisecond)

	url := fmt.Sprintf("http://localhost:%d", ps.Port)

	// TODO: Implement cross-platform browser opening
	fmt.Printf("🌐 Open in browser: %s\n", url)
}

// validateThemePath checks if the theme path is valid
func validateThemePath(path string) error {
	// Check if path exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("path does not exist: %s", path)
	}

	// Check for theme.json
	themeFile := filepath.Join(path, "theme.json")
	if _, err := os.Stat(themeFile); os.IsNotExist(err) {
		return fmt.Errorf("theme.json not found in %s", path)
	}

	// Check for at least one CSS file (styles.css or presentation.css)
	stylesFile := filepath.Join(path, "styles.css")
	presentationFile := filepath.Join(path, "presentation.css")

	hasStyles := false
	hasPresentation := false

	if _, err := os.Stat(stylesFile); err == nil {
		hasStyles = true
	}
	if _, err := os.Stat(presentationFile); err == nil {
		hasPresentation = true
	}

	if !hasStyles && !hasPresentation {
		return fmt.Errorf("no CSS files found in %s (expected styles.css or presentation.css)", path)
	}

	return nil
}

// getSampleSlides returns sample slides for preview
func (ps *PreviewServer) getSampleSlides() string {
	if ps.SampleSlides != "" {
		// Load custom sample slides
		content, err := os.ReadFile(ps.SampleSlides)
		if err != nil {
			fmt.Printf("⚠️ Failed to load custom slides, using default samples\n")
			return ps.getDefaultSampleSlides()
		}
		return string(content)
	}

	return ps.getDefaultSampleSlides()
}

// getDefaultSampleSlides returns built-in sample slides
func (ps *PreviewServer) getDefaultSampleSlides() string {
	return `---
mode: flex
title: "Theme Preview"
theme: preview
---

# Welcome to SlideLang Theme Preview

This is a sample presentation to showcase your theme's styling.

---

## Text Formatting Examples

This slide demonstrates various **text formatting** options:

- *Italic text* for emphasis
- **Bold text** for importance  
- ` + "`inline code`" + ` for technical terms
- [Links to external resources](https://slidelang.com)

Regular paragraph text looks like this. It should be readable and well-spaced according to your theme's typography settings.

---

## Lists and Structure

### Bullet Points
- First item with standard styling
- Second item with **bold emphasis**
- Third item with *italic styling*
  - Nested sub-item
  - Another nested item

### Numbered Lists
1. First numbered item
2. Second numbered item
3. Third numbered item with more content to test line wrapping behavior

---

## Code Examples

Here's a code block to test syntax highlighting:

` + "```" + `javascript
function themePreview() {
    const elements = document.querySelectorAll('.slide');
    elements.forEach(el => {
        el.style.transition = 'all 0.3s ease';
    });
    
    console.log('Theme preview loaded!');
}
` + "```" + `

Inline code like ` + "`const x = 42;`" + ` should also be styled properly.

---

## Special Blocks

::: info Theme Information
This is an info block that shows how your theme handles special content blocks and callouts.
:::

::: warning Important Note
Warning blocks should grab attention while maintaining readability and fitting your theme's visual style.
:::

---

## Tables and Data

| Feature | Status | Description |
|---------|--------|-------------|
| Colors | ✅ Working | Theme colors applied correctly |
| Typography | ✅ Working | Font families and sizes |
| Spacing | ✅ Working | Margins and padding |
| Responsive | 🔄 Testing | Mobile-friendly layouts |

---

## Final Thoughts

This concludes the theme preview. Your theme should now be displaying:

- Consistent color scheme throughout
- Readable typography and spacing
- Proper element styling and hierarchy
- Responsive design elements

**Happy theming!** 🎨`
}

// generatePreviewHTML creates the main preview interface
func (ps *PreviewServer) generatePreviewHTML() string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>SlideLang Theme Preview</title>
    <style>
        body {
            font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
            margin: 0;
            padding: 20px;
            background: #f5f5f5;
        }
        .preview-header {
            background: white;
            padding: 20px;
            border-radius: 8px;
            margin-bottom: 20px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .preview-controls {
            display: flex;
            gap: 10px;
            margin-top: 15px;
        }
        .btn {
            padding: 8px 16px;
            border: none;
            border-radius: 4px;
            cursor: pointer;
            text-decoration: none;
            display: inline-block;
        }
        .btn-primary {
            background: #007bff;
            color: white;
        }
        .btn-secondary {
            background: #6c757d;
            color: white;
        }
        .preview-frame {
            background: white;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
            height: 80vh;
        }
        iframe {
            width: 100%%;
            height: 100%%;
            border: none;
            border-radius: 8px;
        }
        .status {
            display: inline-block;
            padding: 4px 8px;
            border-radius: 12px;
            font-size: 12px;
            font-weight: bold;
        }
        .status.watching {
            background: #d4edda;
            color: #155724;
        }
        .status.static {
            background: #f8d7da;
            color: #721c24;
        }
    </style>
</head>
<body>
    <div class="preview-header">
        <h1>🎨 SlideLang Theme Preview</h1>
        <p><strong>Theme:</strong> %s</p>
        <p><strong>Status:</strong> 
            <span class="status %s">%s</span>
        </p>
        
        <div class="preview-controls">
            <a href="/preview" class="btn btn-primary" target="preview-frame">🔄 Refresh Preview</a>
            %s
        </div>
    </div>
    
    <div class="preview-frame">
        <iframe name="preview-frame" src="/preview"></iframe>
    </div>
    
    %s
</body>
</html>`,
		ps.ThemePath,
		ps.getStatusClass(),
		ps.getStatusText(),
		ps.getThemeControlButtons(),
		ps.getWatchScript())
}

// generatePresentation creates the actual presentation HTML using real SlideLang parser
func (ps *PreviewServer) generatePresentation(slides string) string {
	ps.logger.Info("PREVIEW", "Starting presentation generation...")

	// Guard the parser call the same way `build` does (size cap + timeout +
	// panic-recovery, via util.CheckInputSize/util.RunGuarded — see
	// build.go and docs/SECURITY_AUDIT_2026-07.md, ME-8/BA-5, issue #22/#65).
	// This preview server is a long-lived http.Server and `slides` can come
	// from a user-supplied --sample file, so an oversized or pathological
	// input here would hang or panic the whole server process, not just one
	// CLI invocation — arguably higher risk than the one-shot `build` path
	// this guard was originally added for (issue #69).
	maxInputBytes := util.ResolveMaxInputBytes(0, maxInputSizeEnvVar)
	if err := util.CheckInputSize(len(slides), maxInputBytes); err != nil {
		ps.logger.Warn("PREVIEW", "Rejecting oversized sample slides: %v", err)
		return ps.generateGuardErrorPresentation(err)
	}

	var astNode *ast.AST
	var diags []diagnostics.Diagnostic
	if err := util.RunGuarded(util.DefaultParseTimeout, func() error {
		astNode, diags = ps.parser.Parse(slides, "preview-sample.slidelang")
		return nil
	}); err != nil {
		ps.logger.Warn("PREVIEW", "Guarded parse failed: %v", err)
		return ps.generateGuardErrorPresentation(err)
	}

	// Log any parsing diagnostics
	if len(diags) > 0 {
		ps.logger.Info("PREVIEW", "Parsing diagnostics: %d issues", len(diags))
		for _, diag := range diags {
			ps.logger.Info("PREVIEW", "  %s: %s", diag.Severity, diag.Message)
		}
	}

	if astNode == nil {
		ps.logger.Warn("PREVIEW", "Failed to parse sample slides, falling back to simple HTML")
		return ps.generateFallbackPresentation()
	}

	// Extract theme name from the theme path
	themeName := ps.extractThemeNameFromPath()
	ps.logger.Info("PREVIEW", "Using theme: %s", themeName)

	// Generate the HTML using the real generator
	html, err := ps.generateHTMLFromAST(astNode, themeName)
	if err != nil {
		ps.logger.Warn("PREVIEW", "Failed to generate HTML from AST: %v, falling back", err)
		return ps.generateFallbackPresentation()
	}

	ps.logger.Info("PREVIEW", "Successfully generated presentation HTML")
	return html
}

// generateHTMLFromAST generates HTML from AST using the real SlideLang generator
func (ps *PreviewServer) generateHTMLFromAST(astNode *ast.AST, themeName string) (string, error) {
	ps.logger.Info("PREVIEW", "Loading theme: %s", themeName)

	// Load theme for the presentation
	themeLoader := themes.NewThemeLoader()
	theme, err := themeLoader.LoadTheme(themeName, true)
	if err != nil {
		ps.logger.Warn("PREVIEW", "Failed to load theme '%s': %v, using default", themeName, err)
		theme, _ = themeLoader.LoadTheme("default", true)
	}

	ps.logger.Info("PREVIEW", "Theme loaded successfully: %s", theme.Name)

	// Create template builder with theme - TODOS LOS MÓDULOS INCLUIDOS POR DEFECTO
	builder := templateBuilder.NewTemplateBuilder().
		WithTheme(theme.Name)

	htmlTemplateContent := builder.Build()
	ps.logger.Debug("PREVIEW", "Built HTML template (length: %d)", len(htmlTemplateContent))

	// Create Go template
	tmpl, err := template.New("preview").Funcs(config.HTMLTemplateFuncs()).Parse(htmlTemplateContent)
	if err != nil {
		return "", fmt.Errorf("failed to parse HTML template: %w", err)
	}

	// Prepare template data
	templateData := data.PrepareTemplateData(astNode, ps.logger)
	ps.logger.Info("PREVIEW", "Template data prepared successfully")

	// Execute template to string
	var htmlBuffer strings.Builder
	if err := tmpl.Execute(&htmlBuffer, templateData); err != nil {
		return "", fmt.Errorf("failed to execute HTML template: %w", err)
	}

	generatedHTML := htmlBuffer.String()
	ps.logger.Info("PREVIEW", "Generated HTML successfully (length: %d)", len(generatedHTML))

	// Debug: Check for potential JavaScript issues around line 885
	lines := strings.Split(generatedHTML, "\n")
	if len(lines) > 890 {
		ps.logger.Debug("PREVIEW", "HTML around line 885:")
		for i := 880; i < 890 && i < len(lines); i++ {
			ps.logger.Debug("PREVIEW", "  Line %d: %s", i+1, lines[i])
		}
	}

	// Debug: Check for '$' characters that might cause issues
	dollarCount := strings.Count(generatedHTML, "$")
	if dollarCount > 0 {
		ps.logger.Info("PREVIEW", "Found %d '$' characters in generated HTML", dollarCount)

		// Find and log the context around each '$' character
		dollarPositions := []int{}
		for i := 0; i < len(generatedHTML); i++ {
			if generatedHTML[i] == '$' {
				dollarPositions = append(dollarPositions, i)
			}
		}

		for i, pos := range dollarPositions {
			start := pos - 30
			if start < 0 {
				start = 0
			}
			end := pos + 30
			if end > len(generatedHTML) {
				end = len(generatedHTML)
			}
			context := generatedHTML[start:end]
			ps.logger.Info("PREVIEW", "Dollar #%d at position %d: ...%s...", i+1, pos, context)
		}
	}

	// PREVIEW MODE: Inject external theme CSS links into the generated HTML
	// This allows the preview to load the appropriate CSS files based on theme type
	ps.logger.Info("PREVIEW", "Injecting external theme CSS links for preview mode")

	// Find the </head> tag and inject the theme CSS links before it
	headCloseTag := "</head>"
	headIndex := strings.Index(generatedHTML, headCloseTag)
	if headIndex != -1 {
		// Get the appropriate CSS links based on theme type
		themeCSSLinks := ps.getThemeCSSSLinks()

		// Inject the theme CSS links with override styles
		cssInjection := themeCSSLinks + `
    <style>
        /* Override any inline styles with our theme CSS */
        body { background: var(--background-color, #ffffff) !important; }
    </style>
`
		modifiedHTML := generatedHTML[:headIndex] + cssInjection + generatedHTML[headIndex:]
		ps.logger.Info("PREVIEW", "Successfully injected theme CSS links")
		return modifiedHTML, nil
	} else {
		ps.logger.Warn("PREVIEW", "Could not find </head> tag to inject CSS links")
	}

	return generatedHTML, nil
}

// extractThemeNameFromPath extracts theme name from the theme path
func (ps *PreviewServer) extractThemeNameFromPath() string {
	// Try to read theme.json to get the theme name
	themeJSONPath := filepath.Join(ps.ThemePath, "theme.json")
	if content, err := os.ReadFile(themeJSONPath); err == nil {
		// Parse theme.json properly
		var themeManifest struct {
			Name string `json:"name"`
		}

		if err := json.Unmarshal(content, &themeManifest); err == nil && themeManifest.Name != "" {
			return themeManifest.Name
		}
	}

	// Fallback to directory name
	return filepath.Base(ps.ThemePath)
}

// generateGuardErrorPresentation renders a minimal, self-contained HTML error
// page for the /preview iframe when the input-size cap or the parse
// timeout/panic guard trips (see generatePresentation), instead of letting
// an oversized or pathological sample file reach the parser unbounded on
// this long-lived server process (docs/SECURITY_AUDIT_2026-07.md, issue #69).
func (ps *PreviewServer) generateGuardErrorPresentation(err error) string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Theme Preview - Error</title>
    <style>
        body { margin: 0; padding: 20px; font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; background: #f5f5f5; }
        .error-box {
            background: #f8d7da;
            border-left: 4px solid #dc3545;
            color: #721c24;
            padding: 20px;
            border-radius: 4px;
            max-width: 800px;
            margin: 40px auto;
        }
    </style>
</head>
<body>
    <div class="error-box">
        <strong>⚠️ Preview aborted</strong>
        <p>` + template.HTMLEscapeString(err.Error()) + `</p>
    </div>
</body>
</html>`
}

// generateFallbackPresentation creates a simple HTML fallback when parsing fails
func (ps *PreviewServer) generateFallbackPresentation() string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Theme Preview - Fallback</title>
    <link rel="stylesheet" href="/theme.css">
    <style>
        body { margin: 0; padding: 20px; font-family: var(--font-family, 'Segoe UI', sans-serif); }
        .slide { 
            max-width: 800px; 
            margin: 0 auto 40px auto; 
            padding: 40px; 
            background: var(--background-color, white);
            border-radius: var(--border-radius, 8px);
            box-shadow: var(--box-shadow, 0 2px 4px rgba(0,0,0,0.1));
        }
        h1 { color: var(--primary-color, #333); }
        h2 { color: var(--primary-color, #333); }
        .preview-note {
            background: #fff3cd;
            border-left: 4px solid #ffc107;
            padding: 15px;
            margin: 20px 0;
            border-radius: 4px;
        }
    </style>
</head>
<body>
    <div class="preview-note">
        <strong>⚠️ Fallback Mode</strong><br>
        Unable to parse sample slides with SlideLang parser. Showing basic preview.
    </div>
    
    <div class="slide">
        <h1>Theme Preview - Fallback Mode</h1>
        <p>This is a basic preview of your theme when the SlideLang parser encounters issues.</p>
    </div>
    
    <div class="slide">
        <h2>Sample Content</h2>
        <p>Your theme colors and typography should still be visible here.</p>
        <ul>
            <li>Sample bullet point</li>
            <li>Another bullet point</li>
            <li>Third bullet point</li>
        </ul>
    </div>
</body>
</html>`
}

// detectThemeType detects whether this is a basic theme or full CSS export
func (ps *PreviewServer) detectThemeType() string {
	presentationPath := filepath.Join(ps.ThemePath, "presentation.css")
	navigationPath := filepath.Join(ps.ThemePath, "navigation.css")
	stylesPath := filepath.Join(ps.ThemePath, "styles.css")

	hasPresentationCSS := false
	hasNavigationCSS := false
	hasStylesCSS := false

	if _, err := os.Stat(presentationPath); err == nil {
		hasPresentationCSS = true
	}
	if _, err := os.Stat(navigationPath); err == nil {
		hasNavigationCSS = true
	}
	if _, err := os.Stat(stylesPath); err == nil {
		hasStylesCSS = true
	}

	if hasPresentationCSS {
		if hasNavigationCSS {
			return "full-with-navigation"
		}
		return "full-without-navigation"
	}

	if hasStylesCSS {
		return "basic-theme"
	}

	return "unknown"
}

// getThemeCSSSLinks returns the appropriate CSS links for the detected theme type
func (ps *PreviewServer) getThemeCSSSLinks() string {
	themeType := ps.detectThemeType()
	ps.logger.Info("PREVIEW", "Detected theme type: %s", themeType)

	switch themeType {
	case "full-with-navigation":
		return `    <link rel="stylesheet" href="/presentation.css">
    <link rel="stylesheet" href="/navigation.css">
    <link rel="stylesheet" href="/styles.css">`

	case "full-without-navigation":
		return `    <link rel="stylesheet" href="/presentation.css">
    <link rel="stylesheet" href="/styles.css">`

	case "basic-theme":
		return `    <link rel="stylesheet" href="/styles.css">`

	default:
		return `    <!-- No CSS files detected -->`
	}
}

// Helper methods
func (ps *PreviewServer) getStatusClass() string {
	if ps.WatchMode {
		return "watching"
	}
	return "static"
}

func (ps *PreviewServer) getStatusText() string {
	if ps.WatchMode {
		return "👀 WATCHING FOR CHANGES"
	}
	return "📸 STATIC PREVIEW"
}

func (ps *PreviewServer) getWatchScript() string {
	if !ps.WatchMode {
		return ""
	}

	return `
<script>
// Auto-refresh functionality for watch mode
let lastRefresh = Date.now();

function checkForUpdates() {
    fetch('/reload')
        .then(response => response.json())
        .then(data => {
            // Only refresh if there are actual changes
            if (data.hasChanges === true) {
                console.log('🔄 Changes detected, refreshing preview...');
                document.querySelector('iframe[name="preview-frame"]').src = '/preview?t=' + Date.now();
                lastRefresh = Date.now();
            }
        })
        .catch(err => console.log('Preview update check failed:', err));
}

// Check for updates every 3 seconds in watch mode (increased interval)
setInterval(checkForUpdates, 3000);

console.log('🔄 Auto-refresh enabled - only refreshes when changes are detected');
</script>`
}

// getThemeControlButtons returns the appropriate control buttons based on theme type
func (ps *PreviewServer) getThemeControlButtons() string {
	themeType := ps.detectThemeType()

	var buttons []string

	// Always show theme.json if it exists
	buttons = append(buttons, `<a href="/theme.json" class="btn btn-secondary" target="_blank">📄 theme.json</a>`)

	// Add buttons based on available CSS files
	switch themeType {
	case "full-with-navigation":
		buttons = append(buttons,
			`<a href="/presentation.css" class="btn btn-secondary" target="_blank">⚙️ presentation.css</a>`,
			`<a href="/navigation.css" class="btn btn-secondary" target="_blank">🧭 navigation.css</a>`,
			`<a href="/styles.css" class="btn btn-secondary" target="_blank">🎨 styles.css</a>`)

	case "full-without-navigation":
		buttons = append(buttons,
			`<a href="/presentation.css" class="btn btn-secondary" target="_blank">⚙️ presentation.css</a>`,
			`<a href="/styles.css" class="btn btn-secondary" target="_blank">🎨 styles.css</a>`)

	case "basic-theme":
		buttons = append(buttons,
			`<a href="/styles.css" class="btn btn-secondary" target="_blank">🎨 styles.css</a>`)

	default:
		// No additional buttons for unknown theme type
	}

	return strings.Join(buttons, "\n            ")
}
