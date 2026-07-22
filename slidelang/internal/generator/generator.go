// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"fmt"
	"os"
	"path/filepath"

	"go.ziradocs.com/core/ast"
	"go.ziradocs.com/core/config"
	"go.ziradocs.com/core/renderer"
	"go.ziradocs.com/core/util"
	"go.ziradocs.com/slidelang/internal/generator/data"
	"go.ziradocs.com/slidelang/internal/serializer"
)

// GeneratorOptions define las opciones de generación
type GeneratorOptions struct {
	Theme        string
	EmbedAssets  bool
	NoNavigation bool
	NoUtilities  bool
	Config       *config.SlideLangConfig
	// Offline rendering (issue #92) — solo afecta la salida HTML. Vacío/"browser"
	// mantiene el rendering client-side actual contra CDNs.
	RenderMode      string // "browser" | "offline-assets" | "offline-inline"
	ImageFormat     string // "png" | "webp"
	WebPQuality     int    // 1-100
	ChromiumPath    string
	InstallChromium bool
	// AssetRoot confina las fuentes de imagen locales de --format pptx a
	// este directorio (mismo mecanismo que doclang, ver
	// docs/SECURITY_AUDIT_2026-07.md AL-4); vacío desactiva la confinación
	// (no usado por los demás formatos, que nunca leen bytes de imagen del
	// disco del lado del servidor — HTML solo emite una URL relativa).
	AssetRoot string
}

// IsOffline indica si el modo de rendering pre-renderiza en build time.
func (o GeneratorOptions) IsOffline() bool {
	return renderer.IsOfflineRenderMode(o.RenderMode)
}

type Generator struct {
	serializer *serializer.JSONSerializer
	logger     util.Logger
}

func New(log util.Logger) *Generator {
	return &Generator{
		serializer: serializer.New(),
		logger:     log,
	}
}

// implementedFormats son los formatos que GenerateWithOptions puede generar
// hoy. "slidelang" es un nombre reconocido por el switch de abajo, pero no
// está implementado (retorna error) - por eso NO está aquí. Única fuente de
// verdad reutilizada por IsImplementedFormat, para que un caller (p. ej. la
// validación de --format en el CLI) pueda rechazar un formato no
// implementado ANTES de generar nada, en vez de descubrirlo a mitad de un
// build con varios formatos y dejar salida parcial en disco.
var implementedFormats = map[string]bool{
	"json": true,
	"html": true,
	"pdf":  true,
	"pptx": true,
}

// IsImplementedFormat indica si GenerateWithOptions puede generar `format`
// hoy (a diferencia de un formato simplemente reconocido pero pendiente de
// implementar, como "pdf" o "slidelang").
func IsImplementedFormat(format string) bool {
	return implementedFormats[format]
}

// GenerateWithOptions crea archivos de salida con opciones específicas. ctx
// controla el modo de rendering offline de mermaid/chart/map (issue #92) —
// pasado explícitamente por el caller (issue #134/G1a) en vez de leído de un
// RenderContext global; solo el case "html" lo usa. "json" lo ignora (nunca
// pre-renderiza esos elementos, ver generateJSON) y "pdf" también lo ignora
// por una razón distinta: generatePDF arma su propio RenderContext interno
// (siempre forzado a offline-inline, con su propia instancia de Chromium,
// ver pdf.go) en vez de reusar el que recibe este método.
func (g *Generator) GenerateWithOptions(astNode *ast.AST, format string, outputDir string, opts GeneratorOptions, ctx *renderer.RenderContext) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory %s: %w", outputDir, err)
	}

	switch format {
	case "json":
		return g.generateJSON(astNode, outputDir)
	case "html":
		return g.generateHTMLWithOptions(astNode, outputDir, opts, ctx)
	case "pdf":
		return g.generatePDF(astNode, outputDir, opts)
	case "pptx":
		return g.generatePPTX(astNode, outputDir, opts)
	case "slidelang":
		return fmt.Errorf("SlideLang pretty-print not yet implemented")
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

// generateJSON crea un archivo JSON del AST
func (g *Generator) generateJSON(astNode *ast.AST, outputDir string) error {
	// Generar nombre de archivo basado en el archivo de entrada
	filename := "presentation.json"
	if astNode.FilePath != "" {
		base := filepath.Base(astNode.FilePath)
		ext := filepath.Ext(base)
		name := base[:len(base)-len(ext)]
		filename = name + ".json"
	}

	outputPath := filepath.Join(outputDir, filename)

	// Poblar los campos "*HTML" (issue #64) antes de serializar, reusando el
	// mismo ensamblado de variables y las mismas funciones de render inline
	// que usa --format html, para que el JSON no obligue al consumidor a
	// reimplementar el dialecto inline del CLI.
	variables := data.BuildVariables(astNode)
	renderer.PopulateInlineHTML(astNode, variables)

	// Serializar el AST a JSON
	jsonData, err := g.serializer.SerializeToJSON(astNode)
	if err != nil {
		return fmt.Errorf("failed to serialize AST: %w", err)
	}

	// Escribir JSON al archivo
	if err := os.WriteFile(outputPath, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write JSON file: %w", err)
	}

	g.logger.Info("FILE", "Generated JSON file: %s", outputPath)
	return nil
}

// RenderASTJSON serializa astNode al mismo JSON que produce generateJSON, sin
// escribir a disco — para callers en proceso (p. ej. el servidor MCP) que
// necesitan el AST serializado como valor, no como archivo.
func (g *Generator) RenderASTJSON(astNode *ast.AST) ([]byte, error) {
	variables := data.BuildVariables(astNode)
	renderer.PopulateInlineHTML(astNode, variables)
	return g.serializer.SerializeToJSONCompact(astNode)
}

// RenderHTMLPreview renderiza astNode a un string HTML autocontenido sin
// tocar disco, para callers en proceso (p. ej. el servidor MCP) que
// necesitan una vista previa en memoria en vez de archivos en outputDir.
// Fuerza EmbedAssets a true sin importar opts: como nunca se escribe ningún
// asset a disco, un HTML con referencias a CSS/JS externos quedaría roto
// para quien lo reciba.
func (g *Generator) RenderHTMLPreview(astNode *ast.AST, opts GeneratorOptions, ctx *renderer.RenderContext) (string, error) {
	opts.EmbedAssets = true
	presentationConfig, err := g.preparePresentationConfig(astNode, "", opts, ctx)
	if err != nil {
		return "", fmt.Errorf("failed to prepare presentation config: %w", err)
	}
	htmlContent := presentationConfig.Builder.Build()
	return g.renderHTML(presentationConfig, htmlContent)
}
