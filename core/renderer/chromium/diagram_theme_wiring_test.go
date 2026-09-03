// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package chromium

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/renderer"
)

func diagramTheme() renderer.DiagramThemeColors {
	return renderer.DiagramThemeColors{NodeBG: "#101010", Edge: "#202020", FontFamily: "Inter"}
}

// TestBuildMermaidHTML_ThemeReachesBothPages cubre LAS DOS puertas de render
// de Mermaid. Una revisión de #250 encontró un P1 justo por cubrir solo una
// de dos puertas equivalentes, así que acá se afirman las dos desde el
// principio: el SVG (offline-assets/inline) y el PNG (que usa doclang).
func TestBuildMermaidHTML_ThemeReachesBothPages(t *testing.T) {
	const code = "graph TD; A-->B"

	svg := buildMermaidSVGHTML(code, diagramTheme())
	png := buildMermaidPNGHTML(code, 400, 300, diagramTheme())

	for name, html := range map[string]string{"SVG": svg, "PNG": png} {
		for _, want := range []string{"#101010", "#202020", "Inter", "themeVariables"} {
			if !strings.Contains(html, want) {
				t.Errorf("la página %s no recibió %q:\n%s", name, want, html)
			}
		}
		// La invariante de seguridad del issue #85 sigue en pie con tema.
		if !strings.Contains(html, "securityLevel: 'strict'") || !strings.Contains(html, "htmlLabels: false") {
			t.Errorf("la página %s perdió el par de seguridad", name)
		}
	}
}

// TestBuildMermaidHTML_ZeroValueUnchanged es la garantía byte por byte: sin
// tema, las dos páginas son exactamente las de antes.
func TestBuildMermaidHTML_ZeroValueUnchanged(t *testing.T) {
	const code = "graph TD; A-->B"
	zero := renderer.DiagramThemeColors{}

	if html := buildMermaidSVGHTML(code, zero); strings.Contains(html, "themeVariables") {
		t.Errorf("sin tema no debe emitirse themeVariables:\n%s", html)
	}
	if html := buildMermaidPNGHTML(code, 400, 300, zero); strings.Contains(html, "themeVariables") {
		t.Errorf("sin tema no debe emitirse themeVariables:\n%s", html)
	}
}

// TestMermaidFetcher_CacheKeyIncludesTheme es la lección de #250 aplicada
// desde el principio y no después de que la encuentren: BaseFetcher devuelve
// el archivo ya existente vía os.Stat sin re-rasterizar, así que un tema que
// cambia el SVG SIN cambiar el código fuente tiene que cambiar el hash. Si
// no, dos builds al mismo outputDir con temas distintos sirven el primero.
//
// Se afirma sobre las DOS puertas del fetcher, que derivan la clave del mismo
// método justamente para no poder desincronizarse.
func TestMermaidFetcher_CacheKeyIncludesTheme(t *testing.T) {
	const code = "graph TD; A-->B"

	sinTema := &MermaidFetcher{}
	conTema := &MermaidFetcher{}
	conTema.SetDiagramThemeColors(diagramTheme())
	otroTema := &MermaidFetcher{}
	otroTema.SetDiagramThemeColors(renderer.DiagramThemeColors{NodeBG: "#ffffff"})

	base := sinTema.cacheKey(code)
	if base != GenerateContentHash(code) {
		t.Errorf("sin tema la clave debe ser la histórica: %q vs %q", base, GenerateContentHash(code))
	}
	if conTema.cacheKey(code) == base {
		t.Error("un tema debe cambiar la clave de cache")
	}
	if conTema.cacheKey(code) == otroTema.cacheKey(code) {
		t.Error("dos temas distintos deben producir claves distintas")
	}
	// Mismo tema y mismo código en DOS fetchers distintos: la clave tiene que
	// coincidir, o el cache no serviría de nada entre corridas. Se comparan
	// dos instancias y no una consigo misma —lo que sería una tautología— para
	// que el test sí detecte cualquier cosa dependiente de la instancia que se
	// colara en la derivación.
	gemelo := &MermaidFetcher{}
	gemelo.SetDiagramThemeColors(diagramTheme())
	if conTema.cacheKey(code) != gemelo.cacheKey(code) {
		t.Error("dos fetchers con el mismo tema deben producir la misma clave")
	}
}

// TestNewRenderContext_ForwardsDiagramTheme cubre el equivalente al "call
// site real" de #250: de nada sirve el campo si NewRenderContext no lo
// reenvía al fetcher que de verdad renderiza. Ese cableado no tenía test
// para NINGÚN grupo de tokens, ni siquiera para los de charts.
func TestNewRenderContext_ForwardsDiagramTheme(t *testing.T) {
	ctx := NewRenderContext(&ChromiumRenderer{}, RenderContextOptions{
		MermaidMode:        "offline-assets",
		DiagramThemeColors: diagramTheme(),
	})

	if ctx.DiagramThemeColors != diagramTheme() {
		t.Errorf("el RenderContext devuelto no conservó el tema: %#v", ctx.DiagramThemeColors)
	}
	mf, ok := ctx.MermaidFetcher.(*MermaidFetcher)
	if !ok {
		t.Fatalf("esperaba un *MermaidFetcher, got %T", ctx.MermaidFetcher)
	}
	if mf.themeColors != diagramTheme() {
		t.Errorf("el tema no llegó al MermaidFetcher: %#v", mf.themeColors)
	}
	// Y la consecuencia observable: la clave de cache ya lo refleja.
	if mf.cacheKey("graph TD; A-->B") == GenerateContentHash("graph TD; A-->B") {
		t.Error("el fetcher construido por NewRenderContext no está usando el tema en su clave")
	}
}

// TestNewRenderContext_KrokiIsNotThemed fija el hueco DECLARADO: el backend
// Kroki renderiza en un servidor remoto, sin mermaid.initialize() que
// interceptar, así que no recibe el tema. Se afirma para que sea una
// decisión visible y no una sorpresa: el día que se implemente vía directiva
// %%{init}%%, este test es el que hay que cambiar a propósito.
func TestNewRenderContext_KrokiIsNotThemed(t *testing.T) {
	ctx := NewRenderContext(nil, RenderContextOptions{
		MermaidMode:        "offline-assets",
		DiagramBackend:     "kroki",
		DiagramThemeColors: diagramTheme(),
	})

	if _, esKroki := ctx.MermaidFetcher.(*KrokiFetcher); !esKroki {
		t.Fatalf("esperaba un *KrokiFetcher, got %T", ctx.MermaidFetcher)
	}
	// El contexto SÍ lleva el tema (otros consumidores lo usan); lo que no
	// ocurre es que el fetcher remoto lo aplique.
	if ctx.DiagramThemeColors.IsZero() {
		t.Error("el RenderContext debe seguir llevando el tema aunque este backend no lo use")
	}
}
