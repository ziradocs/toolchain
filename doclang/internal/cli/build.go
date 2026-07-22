// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/include"
	"go.ziradocs.com/core/v2/linter"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/report"
	"go.ziradocs.com/core/v2/transform"
	"go.ziradocs.com/core/v2/util"
	"go.ziradocs.com/core/v2/xref"
	"go.ziradocs.com/doclang/v2/internal/generator"
	"go.ziradocs.com/doclang/v2/themes/document" // 🆕 Theme system
)

// maxInputSizeEnvVar permite ajustar el límite de tamaño de entrada sin
// recompilar (prioridad: --max-size > env var > util.DefaultMaxInputBytes).
// Nombre distinto del de slidelang (SLIDELANG_MAX_SIZE): son binarios
// independientes y compartir el mismo nombre de env var confundiría a un
// operador que solo usa uno de los dos CLIs.
const maxInputSizeEnvVar = "DOCLANG_MAX_SIZE"

func NewBuildCommand(customRules []linter.Rule, rulePacks []linter.RulePack, externalRulepacks []string,
	policyResolver func(flagPath string, fm *ast.FrontMatterNode) (*linter.PolicyConfig, error),
	postLint func(doc *ast.AST, active []diagnostics.Diagnostic, waived []linter.WaivedDiagnostic) error) *cobra.Command {
	var (
		format          string
		output          string
		toc             bool
		numbering       bool
		pageBreaks      bool
		logLevel        string
		renderMode      string   // Global: "browser", "offline-assets", "offline-inline" (applies to all: PlantUML, Mermaid, Charts, Maps)
		imageFormat     string   // "png" or "webp" (for offline modes)
		webpQuality     int      // WebP quality: 1-100 (default 85)
		plantumlServer  string   // Custom PlantUML server URL
		plantumlFormat  string   // "svg" or "png"
		chromiumPath    string   // Custom Chromium/Chrome path
		installChromium bool     // Auto-install Chromium if not found
		maxSizeMB       int      // Maximum input file size in MB (0 = use default/env)
		assetRoot       string   // Confinement root for local image sources (default: input file's directory)
		lintOnly        bool     // Only run the linter, don't generate output
		lintConfig      string   // Path to a YAML linter policy file (flag > frontmatter lint_policy: > default, ver linter.ResolvePolicyConfig)
		reportFormat    string   // Formato de reporte de linter (json, sarif)
		reportOut       string   // Archivo de salida para el reporte
		filters         []string // Rutas a binarios de filtro externo (issue #240, decisión C) — ver core/transform
		includeRoot     string   // Confinement root for @include (issue #238, decisión 3) — default: input file's directory
	)

	cmd := &cobra.Command{
		Use:   "build [file]",
		Short: "Build a document from a doclang file",
		Long: `Build generates a document from a .doclang DSL file.

Supported formats:
  - html: Single-page HTML document (default)
  - pdf: PDF document
  - docx: Microsoft Word document
  - markdown: Markdown output

Examples:
  # Basic build (browser mode, uses CDN)
  doclang build document.doclang
  
  # Offline mode with external assets
  doclang build doc.doclang --render-mode=offline-assets
  
  # Offline mode with everything embedded
  doclang build doc.doclang --render-mode=offline-inline
  
  # PDF with custom theme
  doclang build spec.doclang --format pdf --theme technical
  
  # Document with TOC and numbering
  doclang build report.doclang --toc --numbering`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			inputFile := filepath.ToSlash(filepath.Clean(args[0]))

			// Validate input file
			if !fileExists(inputFile) {
				return fmt.Errorf("input file not found: %s", inputFile)
			}

			// Setup logger
			log := util.NewConsoleLogger(util.LevelInfo, false)

			if level, err := parseLogLevel(logLevel); err != nil {
				return err
			} else {
				log.SetLevel(level)
			}

			log.Info("DOCLANG", "Building document: %s", inputFile)

			// Read input file
			content, err := os.ReadFile(inputFile)
			if err != nil {
				return fmt.Errorf("failed to read file: %w", err)
			}

			maxInputBytes := util.ResolveMaxInputBytes(maxSizeMB, maxInputSizeEnvVar)
			if err := util.CheckInputSize(len(content), maxInputBytes); err != nil {
				return err
			}

			// Expandir @include (issue #238, decisión 3) — build-time, fuera
			// del parser (ver core/include, doc del paquete): así
			// `fmt` preserva @include verbatim en vez de expandirlo.
			resolvedIncludeRoot := includeRoot
			if resolvedIncludeRoot == "" {
				resolvedIncludeRoot = filepath.Dir(inputFile)
			}
			absIncludeRoot, err := filepath.Abs(resolvedIncludeRoot)
			if err != nil {
				return fmt.Errorf("invalid include root: %w", err)
			}
			expandedContent, err := include.Expand(string(content), absIncludeRoot, os.ReadFile)
			if err != nil {
				return fmt.Errorf("failed to expand includes: %w", err)
			}
			content = []byte(expandedContent)
			// Re-chequear el tamaño DESPUÉS de expandir — ver el mismo
			// comentario en slidelang/internal/cli/build.go (amplificación
			// estilo "billion laughs"; include.MaxDepth no acota esto solo).
			if err := util.CheckInputSize(len(content), maxInputBytes); err != nil {
				return fmt.Errorf("expanded content exceeds size limit: %w", err)
			}

			// Parse document
			log.Info("PARSER", "Parsing document...")

			contentStr := string(content)

			var doc *ast.AST
			var diagnostics []diagnostics.Diagnostic

			// DocLang SIEMPRE usa DocumentFlexParser con normalización
			// - Maneja correctamente la jerarquía # → ## → ###
			// - ## y ### son subsecciones, NO secciones separadas
			// - Funciona con o sin frontmatter
			// - Aplica normalización automática (tags de cierre, formato, etc.)
			// La normalización corre dentro del constructor, así que el guard de
			// recover/timeout debe cubrir constructor + Parse() (ME-8/BA-5).
			log.Info("PARSER", "Using DocumentFlexParser with normalization for DocLang document")
			if err := util.RunGuarded(util.DefaultParseTimeout, func() error {
				docParser := parser.NewDocumentFlexParserWithNormalization(contentStr, log)
				doc, diagnostics = docParser.Parse()
				if doc != nil && doc.FilePath == "" {
					doc.FilePath = inputFile
				}
				return nil
			}); err != nil {
				return err
			}
			// Check for errors in diagnostics; print warnings too (e.g. CHART002
			// for malformed chart JSON).
			hasErrors := false
			for _, diag := range diagnostics {
				if diag.IsError() {
					log.Error("PARSER: Error at line %d: %s", diag.Position.Line, diag.Message)
					hasErrors = true
				} else if diag.IsWarning() {
					log.Warn("PARSER: Warning at line %d: %s", diag.Position.Line, diag.Message)
				}
			}
			if hasErrors {
				return fmt.Errorf("parse errors found")
			}

			// Etapa de transform (issue #240, decisión C): built-ins primero
			// (issue #239: numeración de figuras/tablas + resolución de
			// \ref), filtros de terceros (--filter) después, en orden. Corre
			// ANTES del lint deliberadamente (ver core/transform,
			// doc del paquete): el linter valida el AST YA transformado.
			doc, err = transform.RunBuiltins(doc, []transform.Transform{xref.Transform})
			if err != nil {
				return fmt.Errorf("built-in transform stage failed: %w", err)
			}
			if len(filters) > 0 {
				log.Info("TRANSFORM", "Aplicando %d filtro(s) externo(s)...", len(filters))
				doc, err = transform.RunFilters(doc, filters, transform.DefaultFilterTimeout)
				if err != nil {
					return fmt.Errorf("filter stage failed: %w", err)
				}
			}

			// Ejecutar el linter compartido (issue "doclang a la par" — antes
			// solo slidelang lo corría; el core es el mismo, así que
			// cablearlo acá es barato). Mismo motor de políticas configurable
			// que slidelang --lint-config (ver linter.PolicyConfig).
			log.Info("LINT", "Validating document...")
			var policy *linter.PolicyConfig
			if policyResolver != nil {
				p, err := policyResolver(lintConfig, doc.FrontMatter)
				if err != nil {
					return err
				}
				policy = p
			} else {
				p, err := linter.ResolvePolicyConfig(lintConfig, doc.FrontMatter)
				if err != nil {
					return err
				}
				policy = p
			}
			linterInstance := linter.New().WithPolicy(policy)
			for _, rule := range customRules {
				linterInstance.AddRule(rule)
			}
			for _, pack := range rulePacks {
				for _, rule := range pack.Rules() {
					linterInstance.AddRule(rule)
				}
			}
			if len(externalRulepacks) > 0 {
				linterInstance.WithRulepacks(externalRulepacks, 30*time.Second)
			}
			allDiagnostics := linterInstance.LintUnfiltered(doc)
			activeDiags, waivedDiags := policy.Evaluate(allDiagnostics, doc.FilePath, time.Now())

			if reportFormat != "" {
				outPath := reportOut
				if err := report.WriteReport(reportFormat, outPath, activeDiags, waivedDiags, doc, content, externalRulepacks); err != nil {
					return fmt.Errorf("failed to write report: %w", err)
				}
				if outPath != "" && outPath != "-" {
					log.Info("LINT", "Reporte de evidencia generado en '%s'", outPath)
				}
			}

			if postLint != nil {
				if err := postLint(doc, activeDiags, waivedDiags); err != nil {
					return fmt.Errorf("post-lint hook failed: %w", err)
				}
			}

			lintDiagnostics := activeDiags
			lintErrors := false
			for _, diag := range lintDiagnostics {
				if diag.IsError() {
					log.Error("LINT: Error at line %d: %s", diag.Position.Line, diag.Message)
					lintErrors = true
				} else if diag.IsWarning() {
					log.Warn("LINT: Warning at line %d: %s", diag.Position.Line, diag.Message)
				}
			}
			if lintErrors {
				return fmt.Errorf("lint errors found")
			}
			if lintOnly {
				return nil
			}

			// Setup output directory
			if output == "" {
				output = "./output"
			}
			if err := os.MkdirAll(output, 0755); err != nil {
				return fmt.Errorf("failed to create output directory: %w", err)
			}

			// Generate output filename
			baseName := strings.TrimSuffix(filepath.Base(inputFile), filepath.Ext(inputFile))
			var outputFile string
			switch format {
			case "html":
				outputFile = filepath.Join(output, baseName+".html")
			case "pdf":
				outputFile = filepath.Join(output, baseName+".pdf")
			case "docx":
				outputFile = filepath.Join(output, baseName+".docx")
			case "markdown", "md":
				outputFile = filepath.Join(output, baseName+".md")
			default:
				return fmt.Errorf("unsupported format: %s", format)
			}

			// Create generator
			gen := generator.New(log)

			// Extract options from frontmatter if available
			tocEnabled := toc
			numberingEnabled := numbering
			pageBreaksEnabled := pageBreaks

			// Override with frontmatter if not explicitly set via flags
			if doc.FrontMatter != nil {
				// Use frontmatter defaults if flags weren't provided
				if !cmd.Flags().Changed("toc") {
					tocEnabled = true // Enable TOC by default for documents
				}
				if !cmd.Flags().Changed("numbering") {
					numberingEnabled = true // Enable numbering by default
				}
			}

			// 🆕 Load theme
			themeName, themeTrusted := getThemeName(doc, cmd)
			themeLoader := document.NewThemeLoader()
			theme, err := themeLoader.LoadTheme(themeName, themeTrusted)
			if err != nil {
				log.Warn("THEME: Failed to load theme '%s': %v", themeName, err)
				// Continuar con fallback a professional
			}

			log.Info("THEME", "Using theme: %s v%s (type: %s)",
				theme.Name, theme.Version, func() string {
					if theme.IsExternal {
						return "external"
					}
					return "embedded"
				}())

			// 🆕 Determine if theme is page-view
			isPageView := theme.Name == "page-view"

			// Validate PlantUML mode
			// Validate render mode
			validModes := map[string]bool{
				"browser":        true,
				"offline-assets": true,
				"offline-inline": true,
			}
			if !validModes[renderMode] {
				return fmt.Errorf("invalid render-mode: %s (valid: browser, offline-assets, offline-inline)", renderMode)
			}

			// Confinamiento de fuentes de imagen (ver docs/SECURITY_AUDIT_2026-07.md,
			// AL-4): por defecto, el directorio del propio archivo de entrada;
			// --asset-root permite ampliarlo explícitamente (p. ej. a un directorio
			// de assets compartido) sin dejar de rechazar rutas absolutas/"..".
			resolvedAssetRoot := assetRoot
			if resolvedAssetRoot == "" {
				resolvedAssetRoot = filepath.Dir(inputFile)
			}
			absAssetRoot, err := filepath.Abs(resolvedAssetRoot)
			if err != nil {
				return fmt.Errorf("invalid asset root: %w", err)
			}

			opts := generator.GeneratorOptions{
				Format:            format,
				Theme:             theme.Name,
				ThemeVariables:    theme.Variables, // 🆕 Pass theme variables
				ShowHeaders:       isPageView,      // 🆕 Only for page-view
				ShowFooters:       isPageView,      // 🆕 Only for page-view
				InteractiveViewer: tocEnabled,      // 🆕 Enable viewer when TOC is enabled
				TOC:               tocEnabled,
				TOCDepth:          3, // Default depth
				Numbering:         numberingEnabled,
				PageBreaks:        pageBreaksEnabled,
				PlantUMLMode:      renderMode,      // 🆕 Global render mode
				PlantUMLServer:    plantumlServer,  // 🆕 Custom server URL
				PlantUMLFormat:    plantumlFormat,  // 🆕 Image format
				MermaidMode:       renderMode,      // 🆕 Global render mode
				ChartMode:         renderMode,      // 🆕 Global render mode
				MapMode:           renderMode,      // 🆕 Global render mode
				MathMode:          renderMode,      // issue #239-B: Global render mode
				ImageFormat:       imageFormat,     // 🆕 Image format (png/webp)
				WebPQuality:       webpQuality,     // 🆕 WebP quality
				ChromiumPath:      chromiumPath,    // 🆕 Custom Chromium path
				InstallChromium:   installChromium, // 🆕 Auto-install flag
				AssetRoot:         absAssetRoot,    // 🆕 Confinamiento de fuentes de imagen (AL-4)
			} // Generate document
			log.Info("GENERATOR", "Generating %s document...", format)
			if err := gen.Generate(doc, outputFile, opts); err != nil {
				return fmt.Errorf("generation failed: %w", err)
			}

			log.Info("SUCCESS", "Document generated: %s", outputFile)
			return nil
		},
	}

	// Flags
	cmd.Flags().StringVarP(&format, "format", "f", "html", "Output format (html, pdf, docx, markdown)")
	cmd.Flags().StringVarP(&output, "output", "o", "./output", "Output directory")
	cmd.Flags().StringP("theme", "t", "", "Theme to use (professional, academic, technical, page-view)") // 🆕
	cmd.Flags().BoolVar(&toc, "toc", false, "Generate table of contents")
	cmd.Flags().BoolVar(&numbering, "numbering", false, "Enable section numbering")
	cmd.Flags().BoolVar(&pageBreaks, "page-breaks", false, "Insert page breaks between sections")
	cmd.Flags().StringVar(&logLevel, "log-level", "info", "Log level (debug, info, warn, error)")

	// Rendering mode (applies globally to all interactive elements)
	cmd.Flags().StringVar(&renderMode, "render-mode", "browser", "Rendering mode for all interactive elements (diagrams, charts, maps):\n"+
		"  - browser: Use CDN, render in browser (default, smallest files, requires internet)\n"+
		"  - offline-assets: Render at build time, save to assets/ folder (portable with folder)\n"+
		"  - offline-inline: Render at build time, embed in HTML (single file, largest)")

	// PlantUML-specific flags
	cmd.Flags().StringVar(&plantumlServer, "plantuml-server", "", "Custom PlantUML server URL (default: https://www.plantuml.com/plantuml)")
	cmd.Flags().StringVar(&plantumlFormat, "plantuml-format", "svg", "PlantUML image format: svg or png")

	// Image format flags (for charts and maps in offline modes)
	cmd.Flags().StringVar(&imageFormat, "image-format", "png", "Image format for charts and maps: png or webp (only affects offline modes)")
	cmd.Flags().IntVar(&webpQuality, "webp-quality", 85, "WebP quality: 1-100 (higher = better quality, larger file)")

	// Chromium flags (for PDF generation and offline rendering)
	cmd.Flags().StringVar(&chromiumPath, "chromium-path", "", "Custom path to Chromium/Chrome/Edge executable")
	cmd.Flags().BoolVar(&installChromium, "install-chromium", false, "Auto-install Chromium if not found")
	cmd.Flags().IntVar(&maxSizeMB, "max-size", 0, "Maximum input file size in MB (default: 10MB, override via DOCLANG_MAX_SIZE env var)")
	cmd.Flags().StringVar(&assetRoot, "asset-root", "", "Directory local image sources are confined to (default: the input file's directory); absolute paths and '..' outside it are rejected")
	cmd.Flags().BoolVar(&lintOnly, "lint-only", false, "Only run the linter, don't generate output")
	cmd.Flags().StringVar(&lintConfig, "lint-config", "", "Path to a YAML linter policy file. If unset, a 'lint_policy:' block embedded in the document's own frontmatter is used instead, if present")
	cmd.Flags().StringVar(&reportFormat, "report", "", "Generate a machine-readable linting report (json, sarif)")
	cmd.Flags().StringVar(&reportOut, "report-out", "", "Output path for the report (default: stdout)")
	cmd.Flags().StringArrayVar(&filters, "filter", nil, "Path to an external filter binary that transforms the AST between parse and lint (repeatable; runs in the order given, each filter's output feeds the next). Communicates via JSON on stdin/stdout — see docs/architecture/json-ast-contract.md")
	cmd.Flags().StringVar(&includeRoot, "include-root", "", "Directory @include paths are confined to (default: the input file's directory); absolute paths and '..' outside it are rejected")

	return cmd
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// getThemeName extracts theme name with priority: CLI flag > frontmatter > default.
// The second return value indicates whether the name is trusted (came from
// the operator's own --theme flag) — the frontmatter source is not, since
// it's attacker-controlled document content (see ThemeLoader.LoadTheme).
func getThemeName(doc *ast.AST, cmd *cobra.Command) (name string, trusted bool) {
	// 1. CLI flag
	if themeFlag, _ := cmd.Flags().GetString("theme"); themeFlag != "" {
		return themeFlag, true
	}

	// 2. Frontmatter
	if doc.FrontMatter != nil && doc.FrontMatter.Theme != "" {
		return doc.FrontMatter.Theme, false
	}

	// 3. Default
	return "professional", true
}

func parseLogLevel(level string) (util.LogLevel, error) {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "error":
		return util.LevelError, nil
	case "warn", "warning":
		return util.LevelWarn, nil
	case "info", "":
		return util.LevelInfo, nil
	case "debug":
		return util.LevelDebug, nil
	default:
		return util.LevelInfo, fmt.Errorf("invalid log level: %s", level)
	}
}
