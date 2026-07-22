// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/util"
)

func nativePos() diagnostics.Position { return diagnostics.Position{Line: 1, Column: 1} }

func nativeBarChart() *ast.ChartElement {
	c := ast.NewChartElement(nativePos(), "bar")
	c.Data = [][]interface{}{{"A", 10.0}, {"B", 20.0}}
	c.Labels = []string{"A", "B"}
	return c
}

func astWithElements(elements ...ast.Element) *ast.AST {
	doc := ast.NewAST(nativePos())
	block := ast.NewContentBlock(nativePos(), "content")
	block.Elements = append(block.Elements, elements...)
	doc.ContentBlocks = append(doc.ContentBlocks, *block)
	return doc
}

func TestTryBuildNativeContext(t *testing.T) {
	nativeMap := ast.NewMapElement(nativePos(), "default")

	tests := []struct {
		name string
		doc  *ast.AST
		opts GeneratorOptions
		want bool
	}{
		{
			name: "native-capable chart alone",
			doc:  astWithElements(nativeBarChart()),
			want: true,
		},
		{
			// Un mapa rutea a Chromium aunque go-staticmaps pueda
			// rasterizarlo: el render nativo de un mapa puede fallar en
			// runtime (tiles/red/timeout) y sin Chromium eso sería un
			// <div map-error> permanente. El camino Chromium igual intenta
			// go-staticmaps primero (mapa nativo en el happy path) pero con
			// fallback — ver el doc de tryBuildNativeContext (2º hallazgo de
			// la review de PR #259).
			name: "chart + map forces Chromium (maps kept off the fallback-less native path)",
			doc:  astWithElements(nativeBarChart(), nativeMap),
			want: false,
		},
		{
			name: "mermaid forces Chromium",
			doc:  astWithElements(nativeBarChart(), ast.NewMermaidElement(nativePos(), "flowchart", "graph TD; A-->B")),
			want: false,
		},
		{
			name: "math forces Chromium",
			doc:  astWithElements(nativeBarChart(), ast.NewMathElement(nativePos(), "E = mc^2")),
			want: false,
		},
		{
			name: "unsupported chart type (scatter) forces Chromium",
			doc: astWithElements(func() *ast.ChartElement {
				c := ast.NewChartElement(nativePos(), "scatter")
				c.Data = [][]interface{}{{1.0, 2.0}}
				return c
			}()),
			want: false,
		},
		{
			name: "chart with Options forces Chromium",
			doc: astWithElements(func() *ast.ChartElement {
				c := nativeBarChart()
				c.Options = map[string]interface{}{"responsive": true}
				return c
			}()),
			want: false,
		},
		{
			name: "chart in IsJSONMode forces Chromium",
			doc: astWithElements(func() *ast.ChartElement {
				c := nativeBarChart()
				c.IsJSONMode = true
				return c
			}()),
			want: false,
		},
		{
			// Regresión de code-review (bot Codex sobre la primera versión de
			// este PR): un chart type-eligible (bar, sin Options/JSON) pero
			// SIN datos pasa SupportsNativeChartRendering (chequeo de tipo)
			// y aun así falla el render real -- RenderChartNativePNG.ok=true,
			// err!=nil (ver TestRenderChartNativePNG_NoDataFallsBackWithError
			// en core). El gate debe rechazarlo (routear a
			// Chromium, que sí tiene el fallback a Chart.js), no aprobarlo
			// solo por elegibilidad de tipo.
			name: "type-eligible chart with no data forces Chromium (does not just check type)",
			doc:  astWithElements(ast.NewChartElement(nativePos(), "bar")),
			want: false,
		},
		{
			name: "webp requested forces Chromium even for native-capable chart",
			doc:  astWithElements(nativeBarChart()),
			opts: GeneratorOptions{ImageFormat: "webp"},
			want: false,
		},
		{
			name: "no interactive elements is trivially all-native",
			doc:  astWithElements(ast.NewTextElement(nativePos(), "just text")),
			want: true,
		},
	}

	g := New(util.NewNoop())
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, got := g.tryBuildNativeContext(tt.doc, t.TempDir(), tt.opts)
			if got != tt.want {
				t.Errorf("tryBuildNativeContext() ok = %v, want %v", got, tt.want)
			}
			if got && ctx == nil {
				t.Error("tryBuildNativeContext() returned ok=true but a nil context")
			}
			if !got && ctx != nil {
				t.Error("tryBuildNativeContext() returned ok=false but a non-nil context")
			}
		})
	}
}

// TestSetupOfflineRenderContext_NativeDeckSkipsChromium es la prueba
// decisiva de issue #164: un ChromiumPath deliberadamente inválido (y
// InstallChromium=false, para no intentar descargar nada) debe seguir
// produciendo un build offline exitoso — SIN error — para un deck cuyos
// charts son todos nativo-capaces, porque el gate nunca llega a instanciar
// Chromium. Si SetupOfflineRenderContext intentara instanciar Chromium
// igual, fallaría con el path roto; que retorne sin error ES la prueba de
// que Chromium nunca se tocó. Se verifica además que el chart resultante
// renderiza como una imagen PNG real (no un placeholder de error).
func TestSetupOfflineRenderContext_NativeDeckSkipsChromium(t *testing.T) {
	g := New(util.NewNoop())
	doc := astWithElements(nativeBarChart())
	opts := GeneratorOptions{
		RenderMode:      "offline-inline",
		ChromiumPath:    "/nonexistent/definitely-not-a-real-chromium-binary",
		InstallChromium: false,
	}

	ctx, cleanup, err := g.SetupOfflineRenderContext(doc, t.TempDir(), opts)
	defer cleanup()
	if err != nil {
		t.Fatalf("expected no error for an all-native deck even with a broken Chromium path, got: %v", err)
	}

	if _, ok := ctx.ChartFetcher.(*renderer.NativeChartFetcher); !ok {
		t.Fatalf("expected ctx.ChartFetcher to be a *renderer.NativeChartFetcher, got %T", ctx.ChartFetcher)
	}

	html := renderer.RenderElementToHTML(doc.ContentBlocks[0].Elements[0], nil, ctx)
	if strings.Contains(html, "chart-error") {
		t.Errorf("expected a real rendered chart, got an error placeholder: %s", html)
	}
	if !strings.Contains(html, "data:image/png;base64,") {
		t.Errorf("expected an inline base64 PNG image, got: %s", html)
	}
}

// TestSetupOfflineRenderContext_MermaidDeckStillNeedsChromium confirma el
// otro lado del gate: un deck con mermaid (Chromium-only sin excepción)
// debe seguir intentando instanciar Chromium de verdad, y por tanto fallar
// con el mismo ChromiumPath roto — si esto NO fallara, el gate estaría
// clasificando mermaid como nativo-capaz por error.
func TestSetupOfflineRenderContext_MermaidDeckStillNeedsChromium(t *testing.T) {
	g := New(util.NewNoop())
	doc := astWithElements(ast.NewMermaidElement(nativePos(), "flowchart", "graph TD; A-->B"))
	opts := GeneratorOptions{
		RenderMode:      "offline-inline",
		ChromiumPath:    "/nonexistent/definitely-not-a-real-chromium-binary",
		InstallChromium: false,
	}

	_, cleanup, err := g.SetupOfflineRenderContext(doc, t.TempDir(), opts)
	defer cleanup()
	if err == nil {
		t.Fatal("expected an error initializing Chromium for a mermaid deck with a broken ChromiumPath — the gate must not treat mermaid as native-capable")
	}
}

// TestSetupOfflineRenderContext_BadChartDataStillNeedsChromium es la
// regresión de la review de PR #259 (bot Codex): un chart type-eligible
// (bar) pero sin datos debe seguir routeando a Chromium — antes del fix,
// SupportsNativeChartRendering (solo TIPO) aprobaba este chart, el gate
// declaraba el deck 100% nativo-capaz, y el render real fallaba con
// <div class="chart-error"> sin ningún fallback disponible (Chromium nunca
// se instanció). Con el fix, tryBuildNativeContext intenta el render real
// durante el gate y lo rechaza, así que este deck SÍ debe intentar
// Chromium — y por tanto fallar con el mismo ChromiumPath roto que las
// otras pruebas, igual que el caso de mermaid.
func TestSetupOfflineRenderContext_BadChartDataStillNeedsChromium(t *testing.T) {
	g := New(util.NewNoop())
	doc := astWithElements(ast.NewChartElement(nativePos(), "bar")) // sin Data
	opts := GeneratorOptions{
		RenderMode:      "offline-inline",
		ChromiumPath:    "/nonexistent/definitely-not-a-real-chromium-binary",
		InstallChromium: false,
	}

	_, cleanup, err := g.SetupOfflineRenderContext(doc, t.TempDir(), opts)
	defer cleanup()
	if err == nil {
		t.Fatal("expected an error initializing Chromium for a chart with no data and a broken ChromiumPath — the gate must not approve a chart by type alone, only after proving the native render actually succeeds")
	}
}

// TestSetupOfflineRenderContext_MapDeckStillNeedsChromium es la regresión de
// la review de PR #259 (2º hallazgo): un deck con un mapa debe seguir
// routeando a Chromium, no al fetcher nativo sin fallback. Antes del fix, el
// gate declaraba los mapas "siempre nativo-capaces" y el deck se armaba con
// renderer.NativeMapFetcher (ya eliminado) — un render nativo fallido de
// mapa (tiles/red/timeout) habría dado un <div map-error> permanente sin el
// fallback a Chromium que chromium.MapFetcher provee. Con el fix, cualquier
// mapa fuerza el camino Chromium, y por tanto un ChromiumPath roto falla
// igual que el caso de mermaid.
func TestSetupOfflineRenderContext_MapDeckStillNeedsChromium(t *testing.T) {
	g := New(util.NewNoop())
	doc := astWithElements(ast.NewMapElement(nativePos(), "default"))
	opts := GeneratorOptions{
		RenderMode:      "offline-inline",
		ChromiumPath:    "/nonexistent/definitely-not-a-real-chromium-binary",
		InstallChromium: false,
	}

	_, cleanup, err := g.SetupOfflineRenderContext(doc, t.TempDir(), opts)
	defer cleanup()
	if err == nil {
		t.Fatal("expected an error initializing Chromium for a map deck with a broken ChromiumPath — maps must route to Chromium (which keeps the go-staticmaps→Chromium fallback), not to a fallback-less native fetcher")
	}
}
