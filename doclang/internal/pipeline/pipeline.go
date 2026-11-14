// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

// Package pipeline es el pipeline de `doclang build` sin cobra: expansión de
// @include, parseo, transform, lint con el dialecto de documentos y
// generación. Lo comparten el comando `build` (internal/cli/build.go) y el
// tool `export` del servidor MCP (internal/mcp/export.go), para que un
// documento exportado desde un agente salga del mismo código que uno
// compilado en la terminal.
//
// Se parte en dos etapas porque `build` hace cosas entre una y otra que no
// le tocan al MCP (reporte de evidencia, hook post-lint, --lint-only) y
// porque el MCP necesita el AST ya parseado para decidir, antes de
// generar, si el documento requiere Chromium (ver ChromiumRequirements):
//
//   - Prepare: include → parse → transform → tema → lint.
//   - Generate: resolución de TOC/numeración, validación del modo de
//     render y llamada al generador.
//
// Los mensajes de log y los textos de error son los mismos que tenía
// build.go antes de extraerse: el CLI no cambia de comportamiento.
package pipeline

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/include"
	"go.ziradocs.com/core/v2/linter"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/report"
	"go.ziradocs.com/core/v2/transform"
	"go.ziradocs.com/core/v2/util"
	"go.ziradocs.com/core/v2/xref"
	"go.ziradocs.com/doclang/v2/internal/generator"
	"go.ziradocs.com/doclang/v2/themes/document"
)

// MaxInputSizeEnvVar ajusta el tope de tamaño de entrada sin recompilar
// (prioridad en `build`: --max-size > esta variable >
// util.DefaultMaxInputBytes; el MCP no tiene flag, así que usa esta
// variable o el default). Nombre distinto del de slidelang
// (SLIDELANG_MAX_SIZE): son binarios independientes.
const MaxInputSizeEnvVar = "DOCLANG_MAX_SIZE"

// Stage identifica en qué etapa falló el documento por su contenido.
type Stage string

const (
	StageParse Stage = "parse"
	StageLint  Stage = "lint"
)

// DiagnosticsError es el error de Prepare cuando el documento tiene errores
// de parseo o de lint. Error() devuelve el mismo texto que build.go
// devolvía antes ("parse errors found" / "lint errors found"); el MCP usa
// Diagnostics para devolverlos estructurados, igual que el tool `lint`.
type DiagnosticsError struct {
	Stage       Stage
	Diagnostics []diagnostics.Diagnostic
}

func (e *DiagnosticsError) Error() string {
	return string(e.Stage) + " errors found"
}

// PrepareOptions son las entradas de Prepare. Cada campo corresponde a un
// flag de `doclang build`; el valor cero es el default del flag.
type PrepareOptions struct {
	// InputFile es la ruta del documento, solo para el AST (FilePath, título
	// de respaldo, evaluación de la política). Prepare no lo lee: recibe el
	// contenido ya leído.
	InputFile string
	// MaxInputBytes es el tope de tamaño, que se vuelve a aplicar después de
	// expandir los @include.
	MaxInputBytes int
	// IncludeRoot es el directorio, absoluto, al que se confinan los
	// @include (util.ResolveConfinedPath dentro de include.Expand).
	IncludeRoot string
	// DocDir es el directorio del documento, en la misma forma que la ruta
	// que el reporte pone en document.path (DocumentDir): los diagnósticos
	// de un archivo incluido nombran su archivo relativo a él
	// (include.SourceMap.MapDiagnostics, ver sourcesFromDocDir). Vacío si el
	// documento no tiene ruta en disco: el archivo va absoluto.
	DocDir string
	// SourceDir es el directorio, absoluto, desde el que se resuelven las
	// fuentes de imagen relativas (el mismo que GenerateOptions.SourceDir).
	// Con él, una imagen escrita dentro de un archivo incluido se reescribe
	// para resolverse desde el directorio de ese archivo (ver
	// rebaseIncludedAssets). Vacío deja las fuentes como están.
	SourceDir string
	// Filters son binarios de filtro externo (--filter); el MCP no pasa
	// ninguno.
	Filters []string

	// ThemeName es el tema pedido por quien invoca (--theme del CLI, o el
	// `theme` de la tool MCP); vacío = el del frontmatter o "professional".
	// ThemeTrusted dice si ese nombre viene del operador: el CLI pasa true,
	// el MCP pasa false aunque ya lo haya validado (ver
	// document.ThemeLoader.LoadTheme, ME-2).
	ThemeName    string
	ThemeTrusted bool

	CustomRules       []linter.Rule
	RulePacks         []linter.RulePack
	ExternalRulepacks []string
	LintConfig        string
	PolicyResolver    func(flagPath string, fm *ast.FrontMatterNode) (*linter.PolicyConfig, error)
	PostLint          func(doc *ast.AST, active []diagnostics.Diagnostic, waived []linter.WaivedDiagnostic) error
	ReportFormat      string
	ReportOut         string
}

// Prepared es el resultado de Prepare.
type Prepared struct {
	Doc   *ast.AST
	Theme document.Theme
	// Sources es el origen de cada línea del contenido expandido, y DocDir
	// el de PrepareOptions: con ellos Generate lleva a su archivo lo que
	// --fail-on-unavailable reporta.
	Sources include.SourceMap
	DocDir  string
	// ParseDiagnostics y LintDiagnostics son los avisos que no frenaron el
	// build (warnings/info), con la política de lint aplicada (EvaluatePolicy);
	// con errores, Prepare devuelve DiagnosticsError.
	ParseDiagnostics []diagnostics.Diagnostic
	LintDiagnostics  []diagnostics.Diagnostic
}

// Diagnostics devuelve los diagnósticos de parseo y de lint juntos.
func (p *Prepared) Diagnostics() []diagnostics.Diagnostic {
	out := make([]diagnostics.Diagnostic, 0, len(p.ParseDiagnostics)+len(p.LintDiagnostics))
	out = append(out, p.ParseDiagnostics...)
	return append(out, p.LintDiagnostics...)
}

// Prepare corre include → parse → transform → tema → lint sobre content.
func Prepare(log util.Logger, content []byte, opts PrepareOptions) (*Prepared, error) {
	inputFile := opts.InputFile
	maxInputBytes := opts.MaxInputBytes

	// Expandir @include (issue #238, decisión 3) — build-time, fuera
	// del parser (ver core/include, doc del paquete): así
	// `fmt` preserva @include verbatim en vez de expandirlo.
	//
	// Un @include escrito dentro de un bloque de código es código (issue
	// #487): queda como está, como lo muestran get_ast y el lint, y se avisa
	// con INCLUDE001. Para saber qué es código, core parsea el raíz y cada
	// archivo incluido (parser.ParseDocument), así que la expansión corre bajo
	// el mismo guard de recover/timeout que el parseo de abajo (ME-8/BA-5).
	var expansion include.Expansion
	if err := util.RunGuarded(util.DefaultParseTimeout, func() error {
		var err error
		expansion, err = include.ExpandWithOptions(string(content), opts.IncludeRoot, readIncludedFile, include.ExpandOptions{KeepIncludesInCode: true})
		return err
	}); err != nil {
		return nil, fmt.Errorf("failed to expand includes: %w", err)
	}
	origins, sources := expansion.Origins, sourcesFromDocDir(expansion.Sources, opts.DocDir)
	content = []byte(expansion.Text)
	// Re-chequear el tamaño DESPUÉS de expandir — ver el mismo
	// comentario en slidelang/internal/cli/build.go (amplificación
	// estilo "billion laughs"; include.MaxDepth no acota esto solo).
	if err := util.CheckInputSize(len(content), maxInputBytes); err != nil {
		return nil, fmt.Errorf("expanded content exceeds size limit: %w", err)
	}

	// Parse document
	log.Info("PARSER", "Parsing document...")

	contentStr := string(content)

	var doc *ast.AST
	var parseDiags []diagnostics.Diagnostic

	// ParseDocument despacha por el `mode:` del frontmatter entre los
	// dos dialectos documentales (ver core/parser):
	// - flex (y sus sinónimos flex-full/auto, y un archivo sin
	//   frontmatter): Markdown, con la jerarquía # → ## → ### donde
	//   ## y ### son subsecciones y NO secciones separadas, más la
	//   normalización automática.
	// - strict: bloques SECTION declarados, sin normalizar nunca.
	// La normalización corre dentro de ParseDocument, así que el guard
	// de recover/timeout debe cubrirlo entero (ME-8/BA-5).
	log.Info("PARSER", "Parsing DocLang document")
	if err := util.RunGuarded(util.DefaultParseTimeout, func() error {
		doc, parseDiags = parser.New(log).ParseDocument(contentStr, inputFile)
		return nil
	}); err != nil {
		return nil, err
	}
	parseDiags = append(parseDiags, keptIncludeDiagnostics(strings.Split(contentStr, "\n"), expansion.KeptInCode)...)
	parseDiags = append(parseDiags, openIncludeDiagnostics(expansion, opts.DocDir)...)
	// Los avisos del parser que son de una regla del linter (LINK001 de una
	// imagen de bloque) van con los del linter: política, reporte y log de
	// LINT. Ver SplitParserFindings.
	parseDiags, parserFindings := SplitParserFindings(parseDiags)
	// Las posiciones cuentan sobre el contenido expandido: se llevan al
	// archivo y la línea que las escribió antes de la política, el reporte y
	// el log, para que todos nombren la línea que ve el autor. Los
	// parserFindings se llevan después de juntarlos con los del linter, que
	// los compara con las posiciones del AST (MergeParserFindings).
	parseDiags = MapToSource(sources, parseDiags, opts.DocDir)

	// Check for errors in diagnostics; print warnings too (e.g. CHART002
	// for malformed chart JSON).
	hasErrors := false
	for _, diag := range parseDiags {
		if diag.IsError() {
			log.Error("PARSER: Error at %s: %s", Where(diag), diag.Message)
			hasErrors = true
		} else if diag.IsWarning() {
			log.Warn("PARSER: Warning at %s: %s", Where(diag), diag.Message)
		}
	}
	if hasErrors {
		return nil, &DiagnosticsError{Stage: StageParse, Diagnostics: append(parseDiags, MapToSource(sources, parserFindings, opts.DocDir)...)}
	}

	// Las imágenes de un archivo incluido se resuelven desde el directorio
	// de ese archivo, no desde el del raíz. Antes de los transforms, para
	// que filtros, lint y generadores vean ya la ruta efectiva. Lo que eso le
	// cambia a LINK001 se deshace en el lint (linkFindingsUnderRebase).
	rewrites := rebaseIncludedAssets(doc, strings.Split(contentStr, "\n"), origins, opts.SourceDir)

	// Etapa de transform (issue #240, decisión C): built-ins primero
	// (issue #239: numeración de figuras/tablas + resolución de
	// \ref), filtros de terceros (--filter) después, en orden. Corre
	// ANTES del lint deliberadamente (ver core/transform,
	// doc del paquete): el linter valida el AST YA transformado.
	doc, err := transform.RunBuiltins(doc, []transform.Transform{xref.Transform})
	if err != nil {
		return nil, fmt.Errorf("built-in transform stage failed: %w", err)
	}
	if len(opts.Filters) > 0 {
		log.Info("TRANSFORM", "Aplicando %d filtro(s) externo(s)...", len(opts.Filters))
		doc, err = transform.RunFilters(doc, opts.Filters, transform.DefaultFilterTimeout)
		if err != nil {
			return nil, fmt.Errorf("filter stage failed: %w", err)
		}
	}

	// Derivar LangRuns (issue #63) ANTES del lint, mismo criterio de
	// orden que el bloque de arriba: tiene que estar disponible para
	// el linter y, vía él, para un rulepack externo (recibe el AST
	// json.Marshal'd tal cual, core/linter/external.go). Corre
	// DESPUÉS de RunFilters y se re-deriva siempre desde Content,
	// nunca se confía en lo que un filtro externo haya dejado en
	// LangRuns — ver renderer.PopulateLangRuns. BuildVariables()
	// (issue #63 code review, finding #10) para que un span que solo
	// existe tras sustituir {{variable}} también se derive.
	// Con el código literal (issue #487), como el HTML y get_ast.
	renderer.PopulateLangRunsWithOptions(doc, doc.FrontMatter.BuildVariables(), renderer.InlineHTMLOptions{LiteralCode: true})

	// Cargar el tema ANTES del lint (issue #30 — seam de contraste
	// WCAG): el nombre solo depende de doc.FrontMatter y del tema pedido,
	// ambos disponibles desde este punto, así que el tema se resuelve aquí
	// para poder pasar sus variables CSS ya resueltas al linter vía
	// WithThemeVariables (ver core/linter.ThemeAware).
	themeName, themeTrusted := ThemeName(doc, opts.ThemeName, opts.ThemeTrusted)
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

	// Ejecutar el linter compartido (issue "doclang a la par" — antes
	// solo slidelang lo corría; el core es el mismo, así que
	// cablearlo acá es barato). Mismo motor de políticas configurable
	// que slidelang --lint-config (ver linter.PolicyConfig).
	log.Info("LINT", "Validating document...")
	var policy *linter.PolicyConfig
	if opts.PolicyResolver != nil {
		p, err := opts.PolicyResolver(opts.LintConfig, doc.FrontMatter)
		if err != nil {
			return nil, err
		}
		policy = p
	} else {
		p, err := linter.ResolvePolicyConfig(opts.LintConfig, doc.FrontMatter)
		if err != nil {
			return nil, err
		}
		policy = p
	}
	// WithDialect(DialectDocuments) apaga las reglas que solo tienen
	// sentido para slides: FRONT003 (el frontmatter es opcional en un
	// documento), el schema de layout "title" que disparaba LAYOUT002
	// sobre la primera sección, y el closing automático; y hace que
	// CORE001 hable de documento. Las fija el mismo seam de core que
	// slidelang no usa, así que su lint no cambia. Mismo cableado en
	// internal/mcp/lint.go.
	linterInstance := linter.New().WithDialect(linter.DialectDocuments).WithPolicy(policy).WithThemeVariables(theme.Variables)
	for _, rule := range opts.CustomRules {
		linterInstance.AddRule(rule)
	}
	for _, pack := range opts.RulePacks {
		for _, rule := range pack.Rules() {
			linterInstance.AddRule(rule)
		}
	}
	if len(opts.ExternalRulepacks) > 0 {
		linterInstance.WithRulepacks(opts.ExternalRulepacks, 30*time.Second)
	}
	linkDelta := linkFindingsUnderRebase(doc, rewrites)
	allDiagnostics, extDescriptors := linterInstance.LintUnfilteredWithDescriptors(doc)
	allDiagnostics = linkDelta.apply(allDiagnostics)
	allDiagnostics = MergeParserFindings(doc, parserFindings, allDiagnostics)
	allDiagnostics = MapToSource(sources, allDiagnostics, opts.DocDir)
	// Los avisos del parser (SPECIAL002, TABLE007…) pasan por la política
	// como los del linter: un warning se puede apagar, y si la política le
	// sube la severidad cuenta como un error de lint. El mismo camino que la
	// tool `lint` del MCP (EvaluatePolicy).
	evaluated := EvaluatePolicy(policy, parseDiags, allDiagnostics, doc.FilePath, time.Now())
	parserActive, activeDiags := evaluated.ParserActive, evaluated.LintActive

	if opts.ReportFormat != "" {
		outPath := opts.ReportOut
		descriptors := linter.CollectDescriptors(opts.CustomRules, opts.RulePacks)
		descriptors = append(descriptors, extDescriptors...)
		descriptors = linter.NormalizeDescriptors(descriptors)
		// El reporte lleva también los avisos del parser, como la tool `lint`
		// del MCP; antes solo salían por log. Ordenados por línea.
		if err := report.WriteReport(opts.ReportFormat, outPath, evaluated.Active(), evaluated.Waived(), descriptors, doc, content, opts.ExternalRulepacks); err != nil {
			return nil, fmt.Errorf("failed to write report: %w", err)
		}
		if outPath != "" && outPath != "-" {
			log.Info("LINT", "Reporte de evidencia generado en '%s'", outPath)
		}
	}

	if opts.PostLint != nil {
		// El hook recibe lo mismo que el reporte: parser y linter, con la
		// política aplicada y por posición.
		if err := opts.PostLint(doc, evaluated.Active(), evaluated.Waived()); err != nil {
			return nil, fmt.Errorf("post-lint hook failed: %w", err)
		}
	}

	lintErrors := false
	// Los avisos del parser ya salieron en el log de PARSER; acá van los que
	// la política cambió: los que subió a error y el POLICY001 de un waiver
	// vencido.
	for _, diag := range parserActive {
		if diag.IsError() {
			log.Error("LINT: Error at %s: %s", Where(diag), diag.Message)
			lintErrors = true
		} else if isPolicyNotice(diag) {
			log.Warn("LINT: Warning at %s: %s", Where(diag), diag.Message)
		}
	}
	for _, diag := range activeDiags {
		if diag.IsError() {
			log.Error("LINT: Error at %s: %s", Where(diag), diag.Message)
			lintErrors = true
		} else if diag.IsWarning() {
			log.Warn("LINT: Warning at %s: %s", Where(diag), diag.Message)
		}
	}
	if lintErrors {
		// Los del parser con la política aplicada: si la política subió un
		// aviso a error, quien recibe el error (el export del MCP) lo ve como
		// error.
		return nil, &DiagnosticsError{Stage: StageLint, Diagnostics: append(append([]diagnostics.Diagnostic{}, parserActive...), activeDiags...)}
	}

	return &Prepared{
		Doc:              doc,
		Theme:            theme,
		Sources:          sources,
		DocDir:           opts.DocDir,
		ParseDiagnostics: parserActive,
		LintDiagnostics:  activeDiags,
	}, nil
}

// ThemeName resuelve el tema con prioridad: el pedido por quien invoca >
// frontmatter > default. El segundo valor dice si el nombre es confiable:
// el del frontmatter nunca lo es, porque es contenido del documento (ver
// ThemeLoader.LoadTheme); el pedido lo es solo si requestedTrusted.
func ThemeName(doc *ast.AST, requested string, requestedTrusted bool) (name string, trusted bool) {
	if requested != "" {
		return requested, requestedTrusted
	}
	if doc.FrontMatter != nil && doc.FrontMatter.Theme != "" {
		return doc.FrontMatter.Theme, false
	}
	return "professional", true
}

// GenerateOptions son las entradas de Generate. TOC y Numbering en nil
// significan "no se pidió explícitamente" (el equivalente de
// !cmd.Flags().Changed): manda el frontmatter o el default de documentos.
type GenerateOptions struct {
	// Ctx acota la generación: al cancelarse, Chromium se cierra y la
	// generación termina. nil equivale a context.Background().
	Ctx        context.Context
	Format     string
	OutputFile string
	TOC        *bool
	Numbering  *bool
	PageBreaks bool

	RenderMode     string
	ImageFormat    string
	WebPQuality    int
	PlantUMLServer string
	PlantUMLFormat string
	DiagramBackend string
	KrokiServer    string

	ChromiumPath    string
	InstallChromium bool
	// AssetRoot es el directorio, absoluto, al que se confinan las fuentes
	// de imagen locales (AL-4).
	AssetRoot string
	// SourceDir es el directorio, absoluto, del archivo fuente: una fuente
	// de imagen relativa se resuelve contra él y después se confina a
	// AssetRoot. Ver generator.GeneratorOptions.SourceDir.
	SourceDir string
	// AllowRemoteImages: ver generator.GeneratorOptions.AllowRemoteImages.
	AllowRemoteImages bool
	// FailOnUnavailable, NoNetwork, TileServer y TileUserAgent: ver los
	// campos del mismo nombre de generator.GeneratorOptions.
	FailOnUnavailable bool
	NoNetwork         bool
	TileServer        string
	TileUserAgent     string
	TileAttribution   string
	// ExpandedLines deja los avisos del generador con la línea del contenido
	// expandido ("line N"), para quien las lleva a su archivo por su cuenta
	// (la tool MCP). Sin él, el aviso nombra "archivo:N" o la línea del
	// documento, como el reporte de --fail-on-unavailable.
	ExpandedLines bool
}

var validRenderModes = map[string]bool{
	"browser":        true,
	"offline-assets": true,
	"offline-inline": true,
}

var validDiagramBackends = map[string]bool{"chromium": true, "kroki": true}

// ValidateRenderOptions revisa el modo de render y el backend de diagramas,
// con los mismos mensajes que build.go.
func ValidateRenderOptions(renderMode, diagramBackend string) error {
	if !validRenderModes[renderMode] {
		return fmt.Errorf("invalid render-mode: %s (valid: browser, offline-assets, offline-inline)", renderMode)
	}
	if !validDiagramBackends[diagramBackend] {
		return fmt.Errorf("invalid diagram-backend: %s (valid: chromium, kroki)", diagramBackend)
	}
	return nil
}

// ValidateNetworkOptions revisa --no-network, --tile-server,
// --tile-user-agent y --tile-attribution: --no-network no se combina con lo
// que pediría algo a la red (--allow-remote-images, --install-chromium,
// --tile-server), y el servidor de tiles tiene que ser un template válido
// que este build sepa usar.
func ValidateNetworkOptions(noNetwork, allowRemoteImages, installChromium bool, tileServer, tileUserAgent, tileAttribution string) error {
	if noNetwork && tileServer != "" {
		return fmt.Errorf("--no-network and --tile-server contradict each other (with --no-network no map tile is downloaded)")
	}
	if noNetwork && allowRemoteImages {
		return fmt.Errorf("--no-network and --allow-remote-images contradict each other")
	}
	if noNetwork && installChromium {
		return fmt.Errorf("--no-network and --install-chromium contradict each other (installing Chromium downloads it)")
	}
	return generator.ValidateTileServer(tileServer, tileUserAgent, tileAttribution)
}

// ValidateFailOnUnavailable rechaza --fail-on-unavailable con markdown: esa
// salida no resuelve imágenes, diagramas ni mapas (los deja como fuente o
// referencia), así que no hay nada que pueda faltar y la bandera se habría
// ignorado en silencio.
func ValidateFailOnUnavailable(failOnUnavailable bool, format string) error {
	if failOnUnavailable && (format == "markdown" || format == "md") {
		return fmt.Errorf("--fail-on-unavailable does not apply to --format %s: markdown output keeps images, diagrams and maps as references and resolves none of them", format)
	}
	return nil
}

// Generate genera p.Doc en opts.OutputFile. inputFile solo se usa para el
// título de respaldo de un documento sin frontmatter.
func Generate(log util.Logger, p *Prepared, inputFile string, opts GenerateOptions) error {
	doc := p.Doc
	theme := p.Theme
	format := opts.Format

	// Determine if theme is page-view
	isPageView := theme.Name == "page-view"

	gen := generator.New(log)

	tocEnabled, numberingEnabled := ResolveTOCAndNumbering(doc, opts.TOC, opts.Numbering)

	// toc.depth has no CLI flag (--toc only talks about
	// enabled/disabled), so its resolution stays OUTSIDE the
	// TOC override above — otherwise `doclang build --toc`
	// would silently revert an explicit `toc.depth` back to the
	// hardcoded default. Markdown has no notion of depth (its TOC
	// loop only walks top-level ContentBlocks), so this only
	// affects HTML/PDF's extractSubsections.
	tocDepth := 3
	if doc.FrontMatter != nil && doc.FrontMatter.TOC != nil && doc.FrontMatter.TOC.Depth != nil {
		tocDepth = *doc.FrontMatter.TOC.Depth
	}

	if err := ValidateRenderOptions(opts.RenderMode, opts.DiagramBackend); err != nil {
		return err
	}
	if err := ValidateNetworkOptions(opts.NoNetwork, opts.AllowRemoteImages, opts.InstallChromium, opts.TileServer, opts.TileUserAgent, opts.TileAttribution); err != nil {
		return err
	}
	if err := ValidateFailOnUnavailable(opts.FailOnUnavailable, format); err != nil {
		return err
	}

	var pageConfig *ast.PageConfig
	var headerFooterConfig *ast.HeaderFooterConfig
	var watermarkConfig *ast.WatermarkConfig
	if doc.FrontMatter != nil {
		pageConfig = doc.FrontMatter.Page
		headerFooterConfig = doc.FrontMatter.HeaderFooter
		watermarkConfig = doc.FrontMatter.Watermark
	}

	renderMode := opts.RenderMode
	genOpts := generator.GeneratorOptions{
		Format:            format,
		Theme:             theme.Name,
		ThemeVariables:    theme.Variables,
		ThemeExternal:     theme.IsExternal,
		ShowHeaders:       isPageView, // Only for page-view
		ShowFooters:       isPageView, // Only for page-view
		InteractiveViewer: tocEnabled, // Enable viewer when TOC is enabled
		TOC:               tocEnabled,
		TOCDepth:          tocDepth,
		Numbering:         numberingEnabled,
		Title:             generator.DocumentTitle(doc, inputFile), // respaldo para Markdown sin frontmatter
		PageBreaks:        opts.PageBreaks,
		Page:              pageConfig,         // page: front matter (size/margins), PDF only — see resolvePDFOptions
		HeaderFooter:      headerFooterConfig, // header:/footer:/layout_defaults: front matter (issue #117)
		Watermark:         watermarkConfig,    // watermark: front matter (issue #179)
		PlantUMLMode:      renderMode,
		PlantUMLServer:    opts.PlantUMLServer,
		PlantUMLFormat:    opts.PlantUMLFormat,
		MermaidMode:       renderMode,
		DiagramBackend:    opts.DiagramBackend, // "chromium" (default) or "kroki"
		KrokiServer:       opts.KrokiServer,
		ChartMode:         renderMode,
		MapMode:           renderMode,
		MathMode:          renderMode, // issue #239-B
		ImageMode:         renderMode, // issue #167
		ImageFormat:       opts.ImageFormat,
		WebPQuality:       opts.WebPQuality,
		ChromiumPath:      opts.ChromiumPath,
		InstallChromium:   opts.InstallChromium,
		AssetRoot:         opts.AssetRoot, // Confinamiento de fuentes de imagen (AL-4)
		SourceDir:         opts.SourceDir, // Base de las fuentes de imagen relativas
		AllowRemoteImages: opts.AllowRemoteImages,
		FailOnUnavailable: opts.FailOnUnavailable,
		NoNetwork:         opts.NoNetwork,
		TileServer:        opts.TileServer,
		TileUserAgent:     opts.TileUserAgent,
		TileAttribution:   opts.TileAttribution,
		Ctx:               opts.Ctx,
	}
	if !opts.ExpandedLines {
		genOpts.LocateLine = func(line int) (string, int) {
			ue := &generator.UnavailableError{Items: []generator.UnavailableContent{{Line: line}}}
			mapUnavailable(ue, p.Sources, p.DocDir)
			return ue.Items[0].File, ue.Items[0].Line
		}
	}
	log.Info("GENERATOR", "Generating %s document...", format)
	if err := gen.Generate(doc, opts.OutputFile, genOpts); err != nil {
		// Con --fail-on-unavailable la salida no se escribe; tampoco debe
		// quedar la de un build anterior con el mismo nombre, que alguien
		// tomaría por la de este.
		var ue *generator.UnavailableError
		if errors.As(err, &ue) {
			// Las líneas cuentan sobre el contenido expandido: cada una se
			// lleva al archivo que escribió el elemento.
			mapUnavailable(ue, p.Sources, p.DocDir)
			// Solo un archivo regular: un enlace simbólico o un directorio con
			// ese nombre no es una salida de doclang.
			if info, err := os.Lstat(opts.OutputFile); err == nil && info.Mode().IsRegular() {
				if rmErr := os.Remove(opts.OutputFile); rmErr != nil {
					log.Warn("could not remove the previous %s: %v", opts.OutputFile, rmErr)
				}
			}
		}
		return fmt.Errorf("generation failed: %w", err)
	}
	return nil
}

// mapUnavailable lleva la línea de cada elemento de ue (del contenido
// expandido) a su archivo y su línea de origen, con el mismo
// include.SourceMap.MapDiagnostics que los diagnósticos.
func mapUnavailable(ue *generator.UnavailableError, sources include.SourceMap, docDir string) {
	diags := make([]diagnostics.Diagnostic, len(ue.Items))
	for i, it := range ue.Items {
		diags[i] = diagnostics.Diagnostic{Position: diagnostics.NewPosition(it.Line, 1)}
	}
	for i, d := range sources.MapDiagnostics(diags, docDir) {
		if ue.Items[i].Line > 0 {
			ue.Items[i].Line, ue.Items[i].File = d.Position.Line, d.File
		}
	}
}

// ResolveTOCAndNumbering aplica los defaults de documentos: un valor
// explícito (flag del CLI, `toc` de la tool MCP) gana; si no lo hay y el
// documento tiene frontmatter, manda su `toc.enabled`/`numbering:` y, sin
// ellos, ambos quedan activados. Sin frontmatter y sin valor explícito
// quedan apagados: es el comportamiento que build.go tenía con los
// defaults de sus flags (--toc=false, --numbering=false).
func ResolveTOCAndNumbering(doc *ast.AST, toc, numbering *bool) (tocEnabled, numberingEnabled bool) {
	if toc != nil {
		tocEnabled = *toc
	}
	if numbering != nil {
		numberingEnabled = *numbering
	}

	if doc.FrontMatter != nil {
		if toc == nil {
			// An explicit `toc.enabled` in front matter is the
			// default; absent that, TOC stays on by default for
			// documents (issue #115 follow-up, same opt-out gap
			// #100 closed for `numbering:`).
			if doc.FrontMatter.TOC != nil && doc.FrontMatter.TOC.Enabled != nil {
				tocEnabled = *doc.FrontMatter.TOC.Enabled
			} else {
				tocEnabled = true
			}
		}
		if numbering == nil {
			// An explicit `numbering:` in front matter is the
			// default; absent that, numbering stays on by default
			// for documents (issue #100).
			if doc.FrontMatter.Numbering != nil {
				numberingEnabled = *doc.FrontMatter.Numbering
			} else {
				numberingEnabled = true
			}
		}
	}
	return tocEnabled, numberingEnabled
}

// isPolicyNotice dice si d es el POLICY001 que la política agrega por un
// waiver vencido.
func isPolicyNotice(d diagnostics.Diagnostic) bool {
	return d.RuleID == "POLICY001" || d.Code == "POLICY001"
}

// readIncludedFile lee cada archivo que trae un @include; es una variable
// para que las pruebas simulen un fallo dentro de la expansión.
var readIncludedFile = os.ReadFile
