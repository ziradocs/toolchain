// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/linter"
	"go.ziradocs.com/core/v2/util"
	"go.ziradocs.com/doclang/v2/internal/pipeline"
)

// maxInputSizeEnvVar permite ajustar el límite de tamaño de entrada sin
// recompilar (prioridad: --max-size > env var > util.DefaultMaxInputBytes).
// Nombre distinto del de slidelang (SLIDELANG_MAX_SIZE): son binarios
// independientes y compartir el mismo nombre de env var confundiría a un
// operador que solo usa uno de los dos CLIs.
const maxInputSizeEnvVar = pipeline.MaxInputSizeEnvVar

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
		diagramBackend  string   // "chromium" (default) or "kroki" (no Chromium needed for mermaid/plantuml)
		krokiServer     string   // Custom Kroki server URL (default: public https://kroki.io)
		chromiumPath    string   // Custom Chromium/Chrome path
		installChromium bool     // Auto-install Chromium if not found
		maxSizeMB       int      // Maximum input file size in MB (0 = use default/env)
		assetRoot       string   // Confinement root for local image sources (default: --base-dir, else the input file's directory)
		baseDir         string   // Directory relative image sources and @include paths resolve from (default: input file's directory)
		lintOnly        bool     // Only run the linter, don't generate output
		lintConfig      string   // Path to a YAML linter policy file (flag > frontmatter lint_policy: > default, ver linter.ResolvePolicyConfig)
		reportFormat    string   // Formato de reporte de linter (json, sarif)
		reportOut       string   // Archivo de salida para el reporte
		filters         []string // Rutas a binarios de filtro externo (issue #240, decisión C) — ver core/transform
		includeRoot     string   // Confinement root for @include (issue #238, decisión 3) — default: --base-dir, else the input file's directory
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
  doclang build report.doclang --toc --numbering

  # Temporary copy of docs/guide.md: images and @include resolve from docs/
  doclang build /tmp/guide-unsaved.md --base-dir docs --format docx`,
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

			// Directorios de resolución y confinamiento: el ancla es
			// --base-dir o, sin él, el directorio del archivo de entrada;
			// las imágenes relativas se resuelven desde ella y
			// --asset-root/--include-root valen el ancla si no se pasan
			// (ver pipeline.ResolveRoots). Se valida antes de parsear para
			// que un --base-dir inválido falle también con --lint-only.
			roots, err := pipeline.ResolveRoots(inputFile, baseDir, assetRoot, includeRoot)
			if err != nil {
				return err
			}

			// include → parse → transform → tema → lint: el mismo pipeline
			// que usa el tool `export` del servidor MCP (internal/pipeline).
			themeFlag, _ := cmd.Flags().GetString("theme")
			prepared, err := pipeline.Prepare(log, content, pipeline.PrepareOptions{
				InputFile:         inputFile,
				MaxInputBytes:     maxInputBytes,
				IncludeRoot:       roots.IncludeRoot,
				Filters:           filters,
				ThemeName:         themeFlag,
				ThemeTrusted:      true, // --theme es el operador en su propia terminal
				CustomRules:       customRules,
				RulePacks:         rulePacks,
				ExternalRulepacks: externalRulepacks,
				LintConfig:        lintConfig,
				PolicyResolver:    policyResolver,
				PostLint:          postLint,
				ReportFormat:      reportFormat,
				ReportOut:         reportOut,
			})
			if err != nil {
				return err
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
			outputFile, err := OutputFileName(output, inputFile, format)
			if err != nil {
				return err
			}

			// Un flag no pasado explícitamente deja que mande el
			// frontmatter (ver pipeline.ResolveTOCAndNumbering).
			var tocOpt, numberingOpt *bool
			if cmd.Flags().Changed("toc") {
				tocOpt = &toc
			}
			if cmd.Flags().Changed("numbering") {
				numberingOpt = &numbering
			}

			// Confinamiento de fuentes de imagen (ver docs/SECURITY_AUDIT_2026-07.md,
			// AL-4): roots.AssetRoot es el ancla por defecto; --asset-root
			// permite ampliarlo explícitamente (p. ej. a un directorio de
			// assets compartido) sin dejar de rechazar rutas absolutas/"..".
			// Las fuentes relativas se resuelven contra roots.SourceDir, no
			// contra la raíz: la raíz solo dice hasta dónde se puede leer.
			if err := pipeline.ValidateRenderOptions(renderMode, diagramBackend); err != nil {
				return err
			}

			if err := pipeline.Generate(log, prepared, inputFile, pipeline.GenerateOptions{
				Format:          format,
				OutputFile:      outputFile,
				TOC:             tocOpt,
				Numbering:       numberingOpt,
				PageBreaks:      pageBreaks,
				RenderMode:      renderMode,
				ImageFormat:     imageFormat,
				WebPQuality:     webpQuality,
				PlantUMLServer:  plantumlServer,
				PlantUMLFormat:  plantumlFormat,
				DiagramBackend:  diagramBackend,
				KrokiServer:     krokiServer,
				ChromiumPath:    chromiumPath,
				InstallChromium: installChromium,
				AssetRoot:       roots.AssetRoot,
				SourceDir:       roots.SourceDir,
			}); err != nil {
				return err
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

	// Diagram backend flags: an alternative to Chromium for mermaid/plantuml
	// in offline-* render modes (Chromium stays required for chart/map/math
	// and for --format pdf itself).
	cmd.Flags().StringVar(&diagramBackend, "diagram-backend", "chromium", "Backend for mermaid/plantuml diagrams in offline-* render modes: chromium (default) or kroki (no Chromium needed; requires a Kroki server, see --kroki-server)")
	cmd.Flags().StringVar(&krokiServer, "kroki-server", "", "Custom Kroki server URL (default: public https://kroki.io); only used when --diagram-backend=kroki")

	// Image format flags (for charts and maps in offline modes)
	cmd.Flags().StringVar(&imageFormat, "image-format", "png", "Image format for charts and maps: png or webp (only affects offline modes)")
	cmd.Flags().IntVar(&webpQuality, "webp-quality", 85, "WebP quality: 1-100 (higher = better quality, larger file)")

	// Chromium flags (for PDF generation and offline rendering)
	cmd.Flags().StringVar(&chromiumPath, "chromium-path", "", "Custom path to Chromium/Chrome/Edge executable")
	cmd.Flags().BoolVar(&installChromium, "install-chromium", false, "Auto-install Chromium if not found")
	cmd.Flags().IntVar(&maxSizeMB, "max-size", 0, "Maximum input file size in MB (default: 10MB, override via DOCLANG_MAX_SIZE env var)")
	cmd.Flags().StringVar(&assetRoot, "asset-root", "", "Directory local image sources are confined to (default: --base-dir, else the input file's directory); relative sources resolve from --base-dir, and absolute paths or '..' outside the root are rejected")
	cmd.Flags().StringVar(&baseDir, "base-dir", "", "Directory relative image sources and @include paths resolve from (default: the input file's directory). For building a temporary copy of a file: it also becomes the default --asset-root and --include-root, and must be inside --asset-root")
	cmd.Flags().BoolVar(&lintOnly, "lint-only", false, "Only run the linter, don't generate output")
	cmd.Flags().StringVar(&lintConfig, "lint-config", "", "Path to a YAML linter policy file. If unset, a 'lint_policy:' block embedded in the document's own frontmatter is used instead, if present")
	cmd.Flags().StringVar(&reportFormat, "report", "", "Generate a machine-readable linting report (json, sarif)")
	cmd.Flags().StringVar(&reportOut, "report-out", "", "Output path for the report (default: stdout)")
	cmd.Flags().StringArrayVar(&filters, "filter", nil, "Path to an external filter binary that transforms the AST between parse and lint (repeatable; runs in the order given, each filter's output feeds the next). Communicates via JSON on stdin/stdout — see https://ziradocs.com/docs/architecture/json-ast-contract/")
	cmd.Flags().StringVar(&includeRoot, "include-root", "", "Directory @include paths resolve from and are confined to (default: --base-dir, else the input file's directory); absolute paths and '..' outside it are rejected")

	return cmd
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// OutputFileName arma la ruta de salida de `build`: dir/<nombre del
// input sin extensión>.<extensión del formato>.
func OutputFileName(dir, inputFile, format string) (string, error) {
	baseName := strings.TrimSuffix(filepath.Base(inputFile), filepath.Ext(inputFile))
	switch format {
	case "html":
		return filepath.Join(dir, baseName+".html"), nil
	case "pdf":
		return filepath.Join(dir, baseName+".pdf"), nil
	case "docx":
		return filepath.Join(dir, baseName+".docx"), nil
	case "markdown", "md":
		return filepath.Join(dir, baseName+".md"), nil
	default:
		return "", fmt.Errorf("unsupported format: %s", format)
	}
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
