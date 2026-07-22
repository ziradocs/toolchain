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
	"go.ziradocs.com/core/ast"
	"go.ziradocs.com/core/config"
	"go.ziradocs.com/core/diagnostics"
	"go.ziradocs.com/core/include"
	"go.ziradocs.com/core/linter"
	"go.ziradocs.com/core/parser"
	"go.ziradocs.com/core/renderer"
	"go.ziradocs.com/core/report"
	"go.ziradocs.com/core/transform"
	"go.ziradocs.com/core/util"
	"go.ziradocs.com/core/xref"
	"go.ziradocs.com/slidelang/internal/generator"
)

// maxInputSizeEnvVar permite ajustar el límite de tamaño de entrada sin
// recompilar (prioridad: --max-size > env var > util.DefaultMaxInputBytes).
const maxInputSizeEnvVar = "SLIDELANG_MAX_SIZE"

type BuildOptions struct {
	InputFile    string
	OutputDir    string
	Format       string
	Mode         string
	LogLevel     string // "silent", "basic", "detailed", "debug"
	LintOnly     bool
	LintConfig   string // Ruta a un archivo YAML de política del linter (flag > frontmatter lint_policy: > default, ver linter.ResolvePolicyConfig)
	ReportFormat string // Formato de reporte de linter (json, sarif)
	ReportOut    string // Archivo de salida para el reporte
	// Filters (issue #240, decisión C): rutas a binarios de filtro externo que
	// transforman el AST entre parse y lint, en el orden dado. Ver
	// core/transform para el contrato (JSON crudo por stdin/stdout,
	// *HTML siempre blanqueado tras decodificar la respuesta).
	Filters []string
	// IncludeRoot (issue #238, decisión 3): directorio de confinamiento para
	// @include — default el directorio del propio archivo de entrada, igual
	// que --asset-root en doclang. Ver core/include.
	IncludeRoot string
	// AssetRoot (issue #129): directorio de confinamiento para las fuentes de
	// imagen locales que --format pptx embebe — default el directorio del
	// propio archivo de entrada, mismo patrón que --asset-root en doclang
	// (docs/SECURITY_AUDIT_2026-07.md, AL-4). Los demás formatos no lo usan:
	// HTML solo emite una URL relativa, nunca lee bytes de imagen del lado
	// del servidor.
	AssetRoot string
	// EnableNormalization es el nombre canónico (decisión 2 del plan OSS).
	// EnableAI se mantiene como alias deprecado poblado por el flag
	// --enable-ai — ver el reconcile en runBuild.
	EnableNormalization bool
	EnableAI            bool
	NoColors            bool
	Theme               string
	EmbedAssets         bool // Controls if CSS/JS are embedded in HTML or separate files
	NoNavigation        bool // Disable basic slide navigation
	NoUtilities         bool // Disable utility functions (code tabs, etc.)
	MaxSizeMB           int  // Maximum input file size in MB (0 = use default/env)
	// Offline rendering (issue #92) — aplica solo a la salida HTML. Espeja los
	// flags de doclang; los diagramas/charts/mapas se pre-renderizan con Chromium
	// en modos offline en vez de renderizarse en el navegador contra CDNs.
	RenderMode      string // "browser" (default) | "offline-assets" | "offline-inline"
	ImageFormat     string // "png" | "webp" (solo charts/maps en modos offline)
	WebPQuality     int    // 1-100 (default 85)
	ChromiumPath    string // Ruta a un Chromium/Chrome/Edge propio
	InstallChromium bool   // Auto-instalar Chromium si no se encuentra
}

func NewBuildCommand(customRules []linter.Rule, rulePacks []linter.RulePack, externalRulepacks []string,
	policyResolver func(flagPath string, fm *ast.FrontMatterNode) (*linter.PolicyConfig, error),
	postLint func(doc *ast.AST, active []diagnostics.Diagnostic, waived []linter.WaivedDiagnostic) error) *cobra.Command {
	opts := &BuildOptions{}
	cmd := &cobra.Command{
		Use:   "build [file]",
		Short: "Build a presentation from a slidelang file", Long: `Build a presentation from a slidelang file and generate output in the specified format.

Logging Levels:
  error    - Only critical errors
  warn     - Errors and warnings
  info     - General information (default)
  debug    - Detailed debugging output

Asset Embedding:
  By default, HTML generation creates separate CSS and JS files for better caching and modularity.
  Use --embed-assets to inline CSS and JavaScript directly in the HTML file.

Examples:
  # Basic usage with default settings (external CSS/JS)
  slidelang build presentation.slidelang

  # Generate HTML with embedded CSS and JavaScript
  slidelang build slides.slidelang --format html --embed-assets

  # Specify output format and directory with external assets
  slidelang build slides.slidelang --format html --output ./build

  # Generar HTML y JSON en una sola invocación (mismo AST, sin re-parsear)
  slidelang build slides.slidelang --format html,json --output ./build

  # Export to PDF (one slide per page, requires Chrome/Chromium)
  slidelang build slides.slidelang --format pdf --output ./build
  slidelang build slides.slidelang --format pdf --install-chromium
    # Use different logging levels
  slidelang build presentation.slidelang --log-level info
  slidelang build presentation.slidelang --log-level debug --no-colors
  slidelang build presentation.slidelang --log-level error
  
  # Lint only (no output generation)
  slidelang build --lint-only presentation.slidelang
    # Enable content normalization with minimal output
  slidelang build slides.slidelang --enable-normalization --log-level warn`,
		Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.InputFile = args[0]
			}
			return runBuild(opts, customRules, rulePacks, externalRulepacks, policyResolver, postLint)
		},
	}
	cmd.Flags().StringVarP(&opts.OutputDir, "output", "o", "./dist", "Output directory")
	cmd.Flags().StringVarP(&opts.Format, "format", "f", "", "Output format (json, html, pdf, pptx). Acepta una lista separada por comas para emitir varios formatos en un solo build, p. ej. --format html,json")
	cmd.Flags().StringVarP(&opts.Mode, "mode", "m", "auto", "Force parsing mode (strict, flex, auto)")
	cmd.Flags().StringVarP(&opts.LogLevel, "log-level", "l", "info", "Logging level (error, warn, info, debug)")
	cmd.Flags().StringVarP(&opts.Theme, "theme", "t", "", "Theme name to use (default theme if not specified)")
	cmd.Flags().BoolVar(&opts.NoColors, "no-colors", false, "Disable colored output")
	cmd.Flags().BoolVar(&opts.LintOnly, "lint-only", false, "Only run linter, don't generate output")
	cmd.Flags().StringVar(&opts.LintConfig, "lint-config", "", "Path to a YAML linter policy file (enable/disable rules and override severity by diagnostic ID, e.g. IMG001). If unset, a 'lint_policy:' block embedded in the document's own frontmatter is used instead, if present")
	cmd.Flags().StringVar(&opts.ReportFormat, "report", "", "Generate a machine-readable linting report (json, sarif)")
	cmd.Flags().StringVar(&opts.ReportOut, "report-out", "", "Output path for the report (default: slidelang-report.json/sarif in current dir)")
	cmd.Flags().StringArrayVar(&opts.Filters, "filter", nil, "Path to an external filter binary that transforms the AST between parse and lint (repeatable; runs in the order given, each filter's output feeds the next). Communicates via JSON on stdin/stdout — see docs/architecture/json-ast-contract.md")
	cmd.Flags().StringVar(&opts.IncludeRoot, "include-root", "", "Directory @include paths are confined to (default: the input file's directory); absolute paths and '..' outside it are rejected")
	cmd.Flags().StringVar(&opts.AssetRoot, "asset-root", "", "Directory local image sources are confined to for --format pptx (default: the input file's directory); absolute paths and '..' outside it are rejected")
	cmd.Flags().BoolVar(&opts.EnableNormalization, "enable-normalization", false, "Enable content normalization and inference (detects patterns typical of loosely-structured/AI-generated content and applies deterministic fixes; no network calls)")
	cmd.Flags().BoolVar(&opts.EnableAI, "enable-ai", false, "Deprecated: use --enable-normalization instead")
	_ = cmd.Flags().MarkDeprecated("enable-ai", "use --enable-normalization instead")
	cmd.Flags().BoolVar(&opts.EmbedAssets, "embed-assets", false, "Embed CSS and JavaScript in HTML (default: external files)")
	cmd.Flags().BoolVar(&opts.NoNavigation, "no-navigation", false, "Disable basic slide navigation (arrows, keyboard)")
	cmd.Flags().BoolVar(&opts.NoUtilities, "no-utilities", false, "Disable utility functions (code tabs, collapsibles)")
	cmd.Flags().IntVar(&opts.MaxSizeMB, "max-size", 0, "Maximum input file size in MB (default: 10MB, override via SLIDELANG_MAX_SIZE env var)")

	// Rendering mode (issue #92) — aplica a todos los elementos interactivos
	// (mermaid, charts, maps) en la salida HTML. El JSON ignora el modo.
	cmd.Flags().StringVar(&opts.RenderMode, "render-mode", "browser", "Rendering mode for interactive elements (mermaid, charts, maps) in HTML:\n"+
		"  - browser: Use CDN, render in browser (default, smallest files, requires internet)\n"+
		"  - offline-assets: Render at build time, save to assets/ folder (portable with folder)\n"+
		"  - offline-inline: Render at build time, embed in HTML (single file, largest)")
	cmd.Flags().StringVar(&opts.ImageFormat, "image-format", "png", "Image format for charts and maps: png or webp (only affects offline modes)")
	cmd.Flags().IntVar(&opts.WebPQuality, "webp-quality", 85, "WebP quality: 1-100 (higher = better quality, larger file)")
	cmd.Flags().StringVar(&opts.ChromiumPath, "chromium-path", "", "Custom path to Chromium/Chrome/Edge executable (for offline rendering and --format pdf)")
	cmd.Flags().BoolVar(&opts.InstallChromium, "install-chromium", false, "Auto-install Chromium if not found (for offline rendering and --format pdf)")

	return cmd
}

// validRenderModes son los modos de rendering aceptados por --render-mode.
var validRenderModes = map[string]bool{
	"browser":        true,
	"offline-assets": true,
	"offline-inline": true,
}

// isOfflineRenderMode indica si un modo pre-renderiza en build time (necesita
// Chromium). Delega en el predicado único de core (issue #92).
func isOfflineRenderMode(mode string) bool {
	return renderer.IsOfflineRenderMode(mode)
}

// parseFormats separa un valor de --format en una lista deduplicada de formatos,
// preservando el orden de aparición. Permite --format html,json en una sola
// invocación (issue #10) manteniendo compatibilidad con un solo formato.
func parseFormats(raw string) []string {
	var formats []string
	seen := make(map[string]bool)
	for _, part := range strings.Split(raw, ",") {
		f := strings.ToLower(strings.TrimSpace(part))
		if f == "" || seen[f] {
			continue
		}
		seen[f] = true
		formats = append(formats, f)
	}
	return formats
}

func runBuild(opts *BuildOptions, customRules []linter.Rule, rulePacks []linter.RulePack, externalRulepacks []string,
	policyResolver func(flagPath string, fm *ast.FrontMatterNode) (*linter.PolicyConfig, error),
	postLint func(doc *ast.AST, active []diagnostics.Diagnostic, waived []linter.WaivedDiagnostic) error) error {
	// Validate input file
	if opts.InputFile == "" {
		return fmt.Errorf("input file is required")
	}

	// Load configuration file if available
	cfg, err := config.LoadConfig("")
	if err != nil {
		// Config file is optional, so we log but don't fail
		fmt.Printf("Info: Could not load config file: %v\n", err)
	}

	// Apply configuration defaults if values weren't specified via CLI
	if cfg != nil {
		// NOTE: Theme priority is handled in generator to respect frontmatter
		// Only apply config theme as absolute fallback if no frontmatter specifies theme
		if opts.OutputDir == "./dist" && cfg.Build.OutputDir != "" {
			opts.OutputDir = cfg.Build.OutputDir
		}
		// Only use config format if no format was specified via CLI. Validate
		// it here, at the point it's actually about to be consumed — not
		// eagerly via a whole-struct config.ValidateConfig(cfg) right after
		// LoadConfig, which would (a) fatally reject an unused config format
		// even when --format overrides it, (b) miss uppercase formats since
		// it wouldn't share parseFormats' lowercasing, and (c) fail a build
		// over unrelated Theme/Server fields the build command never reads.
		//
		// cfg.Build.Format is never empty here: applyDefaults/GetDefaultConfig
		// (core/config/config.go) always populate it (default
		// "html", issue #59), so there is no reachable "config present but no
		// format" case to fall back from.
		if opts.Format == "" {
			for _, f := range parseFormats(cfg.Build.Format) {
				if !config.IsValidBuildFormat(f) {
					return fmt.Errorf("invalid config file: invalid build format: %s (must be one of 'html', 'pdf', 'json', or a comma-separated combination)", f)
				}
			}
			opts.Format = cfg.Build.Format
		}
		if opts.LogLevel == "info" && cfg.Build.LogLevel != "" {
			opts.LogLevel = cfg.Build.LogLevel
		}
		if !opts.EnableNormalization && cfg.Build.EnableNormalization {
			opts.EnableNormalization = cfg.Build.EnableNormalization
		}
		if len(opts.Filters) == 0 && len(cfg.Build.Filters) > 0 {
			opts.Filters = cfg.Build.Filters
		}
	} else {
		// Config file exists but couldn't be read/parsed (see the err check
		// above) — fall back to the same default GetDefaultConfig() would
		// have given us, so a broken config file doesn't silently change the
		// output format from the one a working/absent config would produce.
		// Reads the default from GetDefaultConfig() itself (not a hardcoded
		// "html" literal) so this branch can't silently drift from it if the
		// default ever changes.
		if opts.Format == "" {
			opts.Format = config.GetDefaultConfig().Build.Format
		}
	}

	// Determinar nivel de logging basado en --log-level (niveles estándar)
	logLevel := util.LevelInfo // default

	switch strings.ToLower(opts.LogLevel) {
	case "error":
		logLevel = util.LevelError // Solo errores críticos
	case "warn", "warning":
		logLevel = util.LevelWarn // Errores + advertencias
	case "info":
		logLevel = util.LevelInfo // Información general (default)
	case "debug":
		logLevel = util.LevelDebug // Todo incluyendo detalles de debug

	// Mantener compatibilidad legacy
	case "silent":
		logLevel = util.LevelError
	case "basic":
		logLevel = util.LevelInfo
	case "detailed":
		logLevel = util.LevelDebug
	default:
		logLevel = util.LevelInfo
	}

	// Inicializar logger
	useColors := !opts.NoColors
	util.InitDefault(logLevel, useColors)

	// 1. Leer archivo de entrada
	util.Info("FILE", "Cargando '%s'", opts.InputFile)
	content, err := os.ReadFile(opts.InputFile)
	if err != nil {
		return fmt.Errorf("failed to read input file: %w", err)
	}

	fileSize := len(content)
	util.Info("FILE", "Archivo cargado → %s", util.SizeString(fileSize))

	maxInputBytes := util.ResolveMaxInputBytes(opts.MaxSizeMB, maxInputSizeEnvVar)
	if err := util.CheckInputSize(fileSize, maxInputBytes); err != nil {
		return err
	}

	// 1.5. Expandir @include (issue #238, decisión 3) — build-time, fuera del
	// parser (ver core/include, doc del paquete): así `fmt`
	// preserva @include verbatim en vez de expandirlo. Corre antes del
	// dispatch de modo/frontmatter para que el documento fusionado sea lo
	// único que el resto del pipeline ve.
	includeRoot := opts.IncludeRoot
	if includeRoot == "" {
		includeRoot = filepath.Dir(opts.InputFile)
	}
	absIncludeRoot, err := filepath.Abs(includeRoot)
	if err != nil {
		return fmt.Errorf("invalid include root: %w", err)
	}
	expandedContent, err := include.Expand(string(content), absIncludeRoot, os.ReadFile)
	if err != nil {
		return fmt.Errorf("failed to expand includes: %w", err)
	}
	content = []byte(expandedContent)

	// Confinamiento de fuentes de imagen para --format pptx (issue #129, ver
	// docs/SECURITY_AUDIT_2026-07.md AL-4) — mismo patrón que includeRoot
	// arriba y que --asset-root en doclang.
	assetRoot := opts.AssetRoot
	if assetRoot == "" {
		assetRoot = filepath.Dir(opts.InputFile)
	}
	absAssetRoot, err := filepath.Abs(assetRoot)
	if err != nil {
		return fmt.Errorf("invalid asset root: %w", err)
	}
	// Re-chequear el tamaño DESPUÉS de expandir: @include anidados/repetidos
	// pueden inflar un archivo raíz pequeño muy por encima de su tamaño
	// original (amplificación estilo "billion laughs") — el límite de
	// profundidad (include.MaxDepth) no acota esto por sí solo.
	if err := util.CheckInputSize(len(content), maxInputBytes); err != nil {
		return fmt.Errorf("expanded content exceeds size limit: %w", err)
	}

	// 2. Auto-detección de modo y habilitación de normalización
	needsAI := opts.EnableNormalization || opts.EnableAI
	finalMode := opts.Mode

	if opts.Mode == "auto" {
		// En modo auto, primero verificar si hay frontmatter
		contentStr := string(content)
		if strings.HasPrefix(contentStr, "---") {
			// Hay frontmatter, extraer el modo
			lines := strings.Split(contentStr, "\n")
			for _, line := range lines[1:] {
				if strings.TrimSpace(line) == "---" {
					break
				}
				if strings.Contains(line, "mode:") {
					// Extraer modo del frontmatter
					parts := strings.Split(line, ":")
					if len(parts) >= 2 {
						detectedMode := strings.TrimSpace(parts[1])
						detectedMode = strings.Trim(detectedMode, `"'`)
						finalMode = detectedMode
						// Habilitar AI para modos que lo requieren
						// "flex-ai" se mantiene como alias deprecado de "flex-full"
						if detectedMode == "flex" || detectedMode == "flex-full" || detectedMode == "flex-ai" || detectedMode == "auto" {
							needsAI = true
						}
						util.Info("FILE", "Modo detectado → '%s' (desde frontmatter)", detectedMode)
						break
					}
				}
			}
		} else {
			// No hay frontmatter, asumir flex-full y habilitar AI
			finalMode = "flex-full"
			needsAI = true
			util.Info("FILE", "Modo detectado → 'flex-full' (sin frontmatter)")
		}
	} else {
		// Modo específico, verificar si requiere AI
		// "flex-ai" se mantiene como alias deprecado de "flex-full"
		if opts.Mode == "flex" || opts.Mode == "flex-full" || opts.Mode == "flex-ai" || opts.Mode == "auto" {
			needsAI = true
		}
		util.Info("FILE", "Modo configurado → '%s'", opts.Mode)
	}
	// 3. Configurar parser
	util.Info("PARSE", "Iniciando análisis del contenido...")
	p := parser.New(util.GetDefault())

	// Habilitar normalización si se requiere
	if needsAI {
		util.Info("NORMALIZE", "Habilitando normalización para modo '%s'", finalMode)
		p.SetNormalization(true)
	}

	// 4. Parsear el archivo (con recover + timeout: ver docs/SECURITY_AUDIT_2026-07.md, ME-8/BA-5)
	var astNode *ast.AST
	var diagnostics []diagnostics.Diagnostic
	if err := util.RunGuarded(util.DefaultParseTimeout, func() error {
		astNode, diagnostics = p.Parse(string(content), opts.InputFile)
		return nil
	}); err != nil {
		return err
	}
	// 5. Mostrar diagnósticos de parsing
	if len(diagnostics) > 0 {
		for _, diag := range diagnostics {
			if diag.IsError() {
				fmt.Fprintf(os.Stderr, "❌ %s\n", diag.String())
			} else {
				fmt.Fprintf(os.Stderr, "ℹ️  %s\n", diag.String())
			}
		}
	}

	// 6. Verificar errores fatales
	hasErrors := false
	for _, diag := range diagnostics {
		if diag.IsError() {
			hasErrors = true
			break
		}
	}

	if hasErrors {
		return fmt.Errorf("parsing failed with errors")
	}

	if astNode == nil {
		return fmt.Errorf("failed to parse input file")
	}

	// Mostrar información del parsing
	if astNode != nil && len(astNode.ContentBlocks) > 0 {
		util.Info("PARSE", "Parsing completado → %d slides detectados", len(astNode.ContentBlocks))
	}

	// 6.5. Etapa de transform (issue #240, decisión C): built-ins primero
	// (issue #239: numeración de figuras/tablas + resolución de \ref),
	// filtros de terceros (--filter) después, en orden. Corre ANTES del
	// lint deliberadamente (ver core/transform, doc del paquete):
	// el linter valida el AST YA transformado, así que un filtro que
	// produzca AST inválido queda rechazado, no aceptado en silencio.
	astNode, err = transform.RunBuiltins(astNode, []transform.Transform{xref.Transform})
	if err != nil {
		return fmt.Errorf("built-in transform stage failed: %w", err)
	}
	if len(opts.Filters) > 0 {
		util.Info("TRANSFORM", "Aplicando %d filtro(s) externo(s)...", len(opts.Filters))
		astNode, err = transform.RunFilters(astNode, opts.Filters, transform.DefaultFilterTimeout)
		if err != nil {
			return fmt.Errorf("filter stage failed: %w", err)
		}
	}

	// 7. Ejecutar linter
	util.Info("LINT", "Validando presentación...")
	var policy *linter.PolicyConfig
	if policyResolver != nil {
		p, err := policyResolver(opts.LintConfig, astNode.FrontMatter)
		if err != nil {
			return err
		}
		policy = p
	} else {
		p, err := linter.ResolvePolicyConfig(opts.LintConfig, astNode.FrontMatter)
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

	allDiagnostics := linterInstance.LintUnfiltered(astNode)

	activeDiags, waivedDiags := policy.Evaluate(allDiagnostics, astNode.FilePath, time.Now())

	if opts.ReportFormat != "" {
		outPath := opts.ReportOut
		if outPath == "" {
			outPath = fmt.Sprintf("slidelang-report.%s", opts.ReportFormat)
		}
		if err := report.WriteReport(opts.ReportFormat, outPath, activeDiags, waivedDiags, opts.InputFile); err != nil {
			return fmt.Errorf("failed to write report: %w", err)
		}
		util.Info("LINT", "Reporte de evidencia generado en '%s'", outPath)
	}

	if postLint != nil {
		if err := postLint(astNode, activeDiags, waivedDiags); err != nil {
			return fmt.Errorf("post-lint hook failed: %w", err)
		}
	}

	lintDiagnostics := activeDiags

	// Contar warnings y errores
	warningCount := 0
	errorCount := 0
	for _, diag := range lintDiagnostics {
		if diag.IsError() {
			errorCount++
		} else {
			warningCount++
		}
	}

	if len(lintDiagnostics) > 0 {
		util.Info("LINT", "Encontrados %d warnings, %d errores", warningCount, errorCount)
		for _, diag := range lintDiagnostics {
			if diag.IsError() {
				fmt.Fprintf(os.Stderr, "❌ %s\n", diag.String())
			} else {
				fmt.Fprintf(os.Stderr, "⚠️  %s\n", diag.String())
			}
		}
	} else {
		util.Info("LINT", "Validación completada sin problemas")
	}
	// Verificar errores de linter
	for _, diag := range lintDiagnostics {
		if diag.IsError() {
			hasErrors = true
			break
		}
	}

	if hasErrors {
		return fmt.Errorf("linting failed with errors")
	}

	// 8. Si solo lint, terminar aquí
	if opts.LintOnly {
		util.Summary("Linting", map[string]interface{}{
			"slides":   len(astNode.ContentBlocks),
			"warnings": warningCount,
			"errores":  errorCount,
		})
		return nil
	}

	// Validar el modo de rendering antes de escribir nada al disco (issue #92).
	// "" se trata como "browser" (default): los callers programáticos que no pasan
	// por cobra dejan el campo vacío, y todo el pipeline downstream ya interpreta
	// vacío == browser.
	if opts.RenderMode != "" && !validRenderModes[opts.RenderMode] {
		return fmt.Errorf("invalid render-mode: %s (valid: browser, offline-assets, offline-inline)", opts.RenderMode)
	}

	// 9. Generar salida (uno o varios formatos, mismo AST parseado una sola vez)
	formats := parseFormats(opts.Format)
	if len(formats) == 0 {
		return fmt.Errorf("no output format specified")
	}

	// --render-mode del operador solo afecta la salida HTML; ni el JSON
	// (serializa el AST sin pasar por el pipeline de rendering) ni el PDF
	// (issue #128: su HTML siempre se fuerza a offline-inline
	// internamente — ver generatePDF — sin importar --render-mode, para que
	// mermaid/chart/map queden pre-renderizados de forma determinística en
	// vez de correr contra la ventana fija de 500ms de RenderHTMLToPDF) lo
	// consultan. hasHTML también gatea más abajo si se arma el pipeline
	// offline (issue #92, review de PR #122: un build --format json con
	// --render-mode offline no debe disparar detección/instalación de
	// Chromium para un formato que nunca lo usa).
	hasHTML := false
	hasPDF := false
	for _, format := range formats {
		switch format {
		case "html":
			hasHTML = true
		case "pdf":
			hasPDF = true
		}
	}
	if isOfflineRenderMode(opts.RenderMode) && !hasHTML {
		// util.Warn(message, args...) — sin tag, a diferencia de util.Info.
		util.Warn("--render-mode=%s no tiene efecto sin --format html; el JSON y el PDF ignoran el modo de rendering", opts.RenderMode)
	}

	// image-format/webp-quality solo aplican en modos offline — Y siempre en
	// --format pdf (issue #128, hallazgo de code-review sobre PR #160): pdf
	// fuerza rasterización offline internamente sin importar --render-mode,
	// así que un build "--format pdf --image-format jpeg" (sin --render-mode
	// offline-*) saltaba esta validación por completo y el valor inválido
	// llegaba sin chequear hasta el fetcher. Se validan aquí para rechazar
	// valores mal formados en vez de emitir assets con una extensión que no
	// coincide con sus bytes (p. ej. .jpeg con contenido PNG) o defaultear en
	// silencio (issue #92). "" se acepta como el default "png".
	if isOfflineRenderMode(opts.RenderMode) || hasPDF {
		switch opts.ImageFormat {
		case "", "png", "webp":
		default:
			return fmt.Errorf("invalid image-format: %s (valid: png, webp)", opts.ImageFormat)
		}
		if opts.ImageFormat == "webp" && (opts.WebPQuality < 1 || opts.WebPQuality > 100) {
			return fmt.Errorf("invalid webp-quality: %d (must be 1-100)", opts.WebPQuality)
		}
	}

	// Validar TODOS los formatos antes de generar cualquiera: si un formato de
	// la lista es inválido y se detectara recién en el loop de abajo, los
	// formatos anteriores ya habrían escrito archivos reales al output dir,
	// dejando un build parcial en disco pese a que el comando falla.
	for _, format := range formats {
		if !config.IsValidBuildFormat(format) {
			return fmt.Errorf("unsupported format: %s (must be one of 'html', 'pdf', 'json', or a comma-separated combination)", format)
		}
	}
	// Un formato reconocido pero aún no implementado (p. ej. "pdf") combinado
	// con otros formatos reproduciría el mismo build-parcial-en-disco de
	// arriba: los formatos anteriores generarían igual antes de llegar al no
	// implementado. Para un solo formato lo dejamos llegar a
	// GenerateWithOptions, que da un mensaje de error más específico.
	if len(formats) > 1 {
		for _, format := range formats {
			if !generator.IsImplementedFormat(format) {
				return fmt.Errorf("cannot combine format %q with other formats: not yet implemented", format)
			}
		}
	}

	util.Info("GEN", "Generando output formato(s) '%s'...", strings.Join(formats, ", "))
	gen := generator.New(util.GetDefault())

	// Crear opciones de generador
	genOpts := generator.GeneratorOptions{
		Theme:           opts.Theme,
		EmbedAssets:     opts.EmbedAssets,
		NoNavigation:    opts.NoNavigation,
		NoUtilities:     opts.NoUtilities,
		Config:          cfg,
		RenderMode:      opts.RenderMode,
		ImageFormat:     opts.ImageFormat,
		WebPQuality:     opts.WebPQuality,
		ChromiumPath:    opts.ChromiumPath,
		InstallChromium: opts.InstallChromium,
		AssetRoot:       absAssetRoot,
	}

	// Configurar el pipeline offline UNA vez, envolviendo todo el loop de formatos,
	// para que un fallo de inicialización de Chromium ocurra ANTES de escribir
	// cualquier formato al disco — misma garantía de "no dejar salida parcial" que
	// la validación de formatos de arriba. No-op en browser, si el deck no tiene
	// mermaid/chart/map, o (issue #92, review de PR #122) si HTML no está en la
	// lista de formatos — un build --format json nunca usa el pipeline offline y
	// no debe forzar detección/instalación de Chromium para nada.
	offlineCleanup := func() {}
	renderCtx := renderer.NewDefaultRenderContext()
	if hasHTML {
		var err error
		renderCtx, offlineCleanup, err = gen.SetupOfflineRenderContext(astNode, opts.OutputDir, genOpts)
		if err != nil {
			return err
		}
	}
	defer offlineCleanup()

	for _, format := range formats {
		if err := gen.GenerateWithOptions(astNode, format, opts.OutputDir, genOpts, renderCtx); err != nil {
			return fmt.Errorf("failed to generate output (%s): %w", format, err)
		}
	}

	// Resumen final
	util.Summary("Build", map[string]interface{}{
		"slides":     len(astNode.ContentBlocks),
		"warnings":   warningCount,
		"formato":    strings.Join(formats, ","),
		"output_dir": opts.OutputDir,
	})

	return nil
}
