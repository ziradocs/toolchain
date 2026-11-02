// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/renderer/chromium"
	"go.ziradocs.com/core/v2/util"
	"go.ziradocs.com/doclang/v2/internal/pipeline"
)

// maxConcurrentExports acota cuántos `export` corren a la vez. Más bajo que
// maxConcurrentParses porque un export puede levantar Chromium. Misma
// adquisición no bloqueante que parseSemaphore (ver parse.go).
const maxConcurrentExports = 2

var exportSemaphore = make(chan struct{}, maxConcurrentExports)

// DefaultExportTimeout es el presupuesto de un `export` completo (parseo,
// lint, Chromium y escritura).
const DefaultExportTimeout = 3 * time.Minute

// exportRenderMode es el modo de render de `export`: el mismo default que
// `doclang build`. offline-assets queda fuera porque escribe una carpeta
// assets/ junto a la salida, fuera del control de `overwrite`.
const exportRenderMode = "browser"

// chromiumInstallHint es lo que se le dice al cliente cuando falta Chromium.
// El MCP nunca descarga nada: la instalación la hace el usuario, una vez,
// desde su terminal.
const chromiumInstallHint = "Install Chrome, Chromium or Edge, or run `doclang build --install-chromium` once from a terminal. doclang mcp never downloads Chromium itself."

var allowedExportFormats = map[string]bool{"docx": true, "pdf": true, "html": true}

type exportInput struct {
	Path      string `json:"path" jsonschema:"the document to export (.md, .markdown or .doclang), relative to the server root or absolute inside it"`
	Format    string `json:"format" jsonschema:"output format: docx, pdf or html"`
	Theme     string `json:"theme,omitempty" jsonschema:"theme name (see list_themes); defaults to the document's own theme or 'professional'"`
	Output    string `json:"output,omitempty" jsonschema:"output file, relative to the server root or absolute inside it; its directory must exist and its extension must match format. Defaults to the input path with the format's extension"`
	Overwrite bool   `json:"overwrite,omitempty" jsonschema:"replace the output file if it already exists (default false: refuse)"`
	TOC       *bool  `json:"toc,omitempty" jsonschema:"force the table of contents on or off; defaults to the document's front matter, or on when it has front matter"`
}

type exportOutput struct {
	OutputPath  string                   `json:"outputPath"`
	Format      string                   `json:"format"`
	Bytes       int64                    `json:"bytes"`
	Diagnostics []diagnostics.Diagnostic `json:"diagnostics"`
}

type exportError struct {
	Error       string                   `json:"error"`
	Diagnostics []diagnostics.Diagnostic `json:"diagnostics,omitempty"`
	// RequiresChromium lista qué partes del documento necesitan Chromium
	// cuando no se encontró; Hint dice cómo instalarlo.
	RequiresChromium []string `json:"requiresChromium,omitempty"`
	Hint             string   `json:"hint,omitempty"`
}

// exportConfig es la parte de Options que usa `export`.
type exportConfig struct {
	root         exportRoot
	chromiumPath string
	timeout      time.Duration
}

// registerExportTool registra el tool "export": escribe DOCX, PDF o HTML de
// un documento en disco con el mismo pipeline que `doclang build`
// (internal/pipeline). Todo lo que lee o escribe queda confinado a la raíz
// del servidor; ver confine.go y la nota de export en
// docs/SECURITY_AUDIT_2026-07.md.
func registerExportTool(server *sdkmcp.Server, logger util.Logger, cfg exportConfig) {
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name: "export",
		Description: "Export a DocLang/Markdown file on disk to DOCX, PDF or HTML, using the same pipeline as `doclang build`. " +
			"Paths are confined to the server root. Refuses to replace an existing file unless overwrite is true. " +
			"PDF, and DOCX with mermaid diagrams, math or non-native charts, need a local Chrome/Chromium/Edge.",
	}, func(ctx context.Context, req *sdkmcp.CallToolRequest, in exportInput) (*sdkmcp.CallToolResult, any, error) {
		out, errOut := runExportTool(ctx, logger, cfg, in)
		if errOut != nil {
			res, resErr := errorResult(errOut)
			return res, errOut, resErr
		}
		res, resErr := jsonResult(out)
		return res, out, resErr
	})
}

func runExportTool(ctx context.Context, logger util.Logger, cfg exportConfig, in exportInput) (*exportOutput, *exportError) {
	out, errOut := runExport(ctx, logger, cfg, in)
	if errOut != nil {
		// Los errores de confinamiento citan la raíz absoluta; el cliente
		// trabaja con rutas relativas a ella y no necesita la de la máquina.
		errOut.Error = cfg.root.redact(errOut.Error)
	}
	return out, errOut
}

func runExport(ctx context.Context, logger util.Logger, cfg exportConfig, in exportInput) (*exportOutput, *exportError) {
	fail := func(format string, args ...any) (*exportOutput, *exportError) {
		return nil, &exportError{Error: fmt.Sprintf(format, args...)}
	}

	format := strings.ToLower(strings.TrimSpace(in.Format))
	if !allowedExportFormats[format] {
		return fail("unsupported format %q (valid: docx, pdf, html)", in.Format)
	}
	// in.Theme es input de un cliente MCP: solo nombres de tema conocidos
	// (ME-2). Después se pasa como no confiable al pipeline igual.
	if err := validateThemeName(in.Theme); err != nil {
		return fail("%v", err)
	}

	maxBytes := util.DefaultMaxInputBytes
	inputAbs, content, err := cfg.root.resolveInput(in.Path, maxBytes)
	if err != nil {
		return fail("%v", err)
	}
	outReq := in.Output
	if outReq == "" {
		outReq = cfg.root.defaultOutput(inputAbs, format)
	}
	outputAbs, err := cfg.root.resolveOutput(outReq, format)
	if err != nil {
		return fail("%v", err)
	}
	// BA-13: sin overwrite no se reemplaza nada. Este chequeo es para fallar
	// antes de generar; el que vale es el os.Link de commitExport.
	if !in.Overwrite {
		if _, err := os.Lstat(outputAbs); err == nil {
			return fail("output already exists: %s (pass overwrite: true to replace it)", cfg.root.display(outputAbs))
		}
	}

	select {
	case exportSemaphore <- struct{}{}:
	default:
		return fail("server busy: export concurrency limit reached (%d), try again shortly", maxConcurrentExports)
	}

	// Se genera a un temporal en el mismo directorio y solo al final se
	// mueve al destino: un export cancelado o vencido nunca deja un archivo
	// a medio escribir en la ruta pedida.
	tmp, err := os.CreateTemp(filepath.Dir(outputAbs), ".doclang-export-*."+format)
	if err != nil {
		<-exportSemaphore
		return fail("cannot create temporary output: %v", err)
	}
	tmpPath := tmp.Name()
	tmp.Close()

	timeout := cfg.timeout
	if timeout <= 0 {
		timeout = DefaultExportTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	type result struct {
		out    *exportOutput
		errOut *exportError
	}
	done := make(chan result, 1)
	go func() {
		// El slot se libera cuando el trabajo termina de verdad, no cuando
		// el handler deja de esperarlo (mismo criterio que parse.go): el
		// generador no recibe contexto y sigue corriendo tras un timeout.
		defer func() { <-exportSemaphore }()
		var r result
		if err := util.RecoverGuard(func() error {
			r.out, r.errOut = generateExport(logger, cfg, in, format, inputAbs, content, tmpPath)
			return nil
		}); err != nil {
			r = result{errOut: &exportError{Error: err.Error()}}
		}
		done <- r
	}()

	var r result
	select {
	case r = <-done:
	case <-ctx.Done():
		go func() {
			<-done
			os.Remove(tmpPath)
		}()
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return fail("export timed out after %s", timeout)
		}
		return fail("export cancelled: %v", ctx.Err())
	}
	if r.errOut != nil {
		os.Remove(tmpPath)
		return nil, r.errOut
	}

	size, err := commitExport(tmpPath, outputAbs, in.Overwrite)
	if err != nil {
		os.Remove(tmpPath)
		if errors.Is(err, fs.ErrExist) {
			return fail("output already exists: %s (pass overwrite: true to replace it)", cfg.root.display(outputAbs))
		}
		return fail("cannot write output: %v", err)
	}
	r.out.OutputPath = cfg.root.display(outputAbs)
	r.out.Bytes = size
	return r.out, nil
}

// generateExport corre el pipeline de `doclang build` y escribe en tmpPath.
func generateExport(logger util.Logger, cfg exportConfig, in exportInput, format, inputAbs string, content []byte, tmpPath string) (*exportOutput, *exportError) {
	inputName := cfg.root.display(inputAbs)
	prepared, err := pipeline.Prepare(logger, content, pipeline.PrepareOptions{
		InputFile:     inputName,
		MaxInputBytes: util.DefaultMaxInputBytes,
		IncludeRoot:   cfg.root.dir, // @include confinado a la raíz
		ThemeName:     in.Theme,
		ThemeTrusted:  false, // input de un cliente MCP, aunque ya validado
	})
	if err != nil {
		var diagErr *pipeline.DiagnosticsError
		if errors.As(err, &diagErr) {
			return nil, &exportError{Error: err.Error(), Diagnostics: diagErr.Diagnostics}
		}
		return nil, &exportError{Error: err.Error()}
	}

	// El generador DOCX, sin Chromium, solo deja un warning y omite
	// diagramas y ecuaciones; acá no hay log visible, así que se exige
	// antes de generar.
	if reasons := pipeline.ChromiumRequirements(prepared.Doc, format, exportRenderMode, "chromium"); len(reasons) > 0 {
		mgr := chromium.NewChromiumManager(cfg.chromiumPath, false, &renderer.ChromiumLoggerAdapter{Logger: logger})
		if _, err := mgr.Detect(); err != nil {
			return nil, &exportError{
				Error:            fmt.Sprintf("Chromium is required for %s but was not found: %v", strings.Join(reasons, ", "), err),
				Diagnostics:      prepared.Diagnostics(),
				RequiresChromium: reasons,
				Hint:             chromiumInstallHint,
			}
		}
	}

	if err := pipeline.Generate(logger, prepared, inputName, pipeline.GenerateOptions{
		Format:          format,
		OutputFile:      tmpPath,
		TOC:             in.TOC,
		RenderMode:      exportRenderMode,
		ImageFormat:     "png",
		WebPQuality:     85,
		PlantUMLFormat:  "svg",
		DiagramBackend:  "chromium",
		ChromiumPath:    cfg.chromiumPath,
		InstallChromium: false, // nunca se descarga nada desde el MCP
		AssetRoot:       cfg.root.dir,
	}); err != nil {
		return nil, &exportError{Error: err.Error(), Diagnostics: prepared.Diagnostics()}
	}

	diags := prepared.Diagnostics()
	if diags == nil {
		diags = []diagnostics.Diagnostic{}
	}
	return &exportOutput{Format: format, Diagnostics: diags}, nil
}

// commitExport mueve tmpPath al destino. Sin overwrite usa os.Link, que
// falla con EEXIST si el destino apareció mientras se generaba (BA-13, sin
// carrera entre el chequeo y la escritura); con overwrite, os.Rename, que
// reemplaza el archivo de forma atómica y nunca sigue un symlink.
func commitExport(tmpPath, outputAbs string, overwrite bool) (int64, error) {
	if err := os.Chmod(tmpPath, 0o644); err != nil {
		return 0, err
	}
	info, err := os.Stat(tmpPath)
	if err != nil {
		return 0, err
	}
	if overwrite {
		if err := os.Rename(tmpPath, outputAbs); err != nil {
			return 0, err
		}
		return info.Size(), nil
	}
	if err := os.Link(tmpPath, outputAbs); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return 0, err
		}
		// Un filesystem sin hard links: O_EXCL da la misma garantía de no
		// pisar un archivo existente.
		if err := copyExclusive(tmpPath, outputAbs); err != nil {
			return 0, err
		}
	}
	os.Remove(tmpPath)
	return info.Size(), nil
}

func copyExclusive(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(dst)
		return err
	}
	return out.Close()
}
