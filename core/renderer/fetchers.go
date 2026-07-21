// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"context"

	"go.ziradocs.com/core/ast"
)

// Interfaces consumer-side (issue #134/G1b): renderer necesita nombrar la
// forma de cada fetcher para tipar los campos de RenderContext y llamar sus
// métodos desde html.go, pero la implementación concreta (chromedp/HTTP)
// vive en renderer/chromium, que importa renderer (para MapConfig, etc.) —
// si renderer importara chromium para referenciar los structs concretos,
// sería un ciclo. Go satisface estas interfaces estructuralmente: los
// structs *chromium.MermaidFetcher/ChartFetcher/MapFetcher/PlantUMLFetcher
// no necesitan declarar que las implementan.

// MermaidFetcher pre-renderiza un diagrama Mermaid a SVG/PNG para los modos
// offline (assets en disco o inline como data URI).
type MermaidFetcher interface {
	FetchAndSave(ctx context.Context, mermaidCode string, outputDir string) (string, error)
	FetchInline(ctx context.Context, mermaidCode string) (string, error)
}

// MathFetcher pre-renderiza una ecuación LaTeX a SVG para los modos offline
// (issue #239-B). Misma forma que MermaidFetcher — MathJax-SVG (motor
// elegido sobre KaTeX: su SVG es autocontenido, sin web-fonts, no requiere
// tocar renderer/csp.go) produce un SVG standalone igual que Mermaid, así
// que el mecanismo de fetch/cache es el mismo, solo cambia qué se manda a
// renderizar en el navegador headless.
type MathFetcher interface {
	FetchAndSave(ctx context.Context, latex string, outputDir string) (string, error)
	FetchInline(ctx context.Context, latex string) (string, error)
}

// ChartFetcher pre-renderiza un chart de Chart.js a PNG/WebP para los modos
// offline. GetImageFormat expone el formato configurado (html.go lo usa
// para decidir el media type del data URI inline).
type ChartFetcher interface {
	FetchAndSave(ctx context.Context, elem *ast.ChartElement, chartConfig string, outputDir string, width, height int) (string, error)
	FetchInline(ctx context.Context, elem *ast.ChartElement, chartConfig string, width, height int) ([]byte, error)
	GetImageFormat() string
}

// MapFetcher pre-renderiza un mapa Leaflet a PNG/WebP para los modos offline.
type MapFetcher interface {
	FetchAndSave(ctx context.Context, config MapConfig, outputDir string, width, height int) (string, error)
	FetchInline(ctx context.Context, config MapConfig, width, height int) ([]byte, error)
	GetImageFormat() string
}

// PlantUMLFetcher obtiene un diagrama PlantUML (servidor HTTP, no Chromium)
// para los modos offline.
type PlantUMLFetcher interface {
	FetchDiagramToAssets(ctx context.Context, content string) (string, error)
	FetchDiagramInline(ctx context.Context, content string) (string, error)
}
