// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package chromium

import (
	"reflect"
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

	if !reflect.DeepEqual(ctx.DiagramThemeColors, diagramTheme()) {
		t.Errorf("el RenderContext devuelto no conservó el tema: %#v", ctx.DiagramThemeColors)
	}
	mf, ok := ctx.MermaidFetcher.(*MermaidFetcher)
	if !ok {
		t.Fatalf("esperaba un *MermaidFetcher, got %T", ctx.MermaidFetcher)
	}
	if !reflect.DeepEqual(mf.themeColors, diagramTheme()) {
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

// ---------------------------------------------------------------------------
// Fuentes auto-hospedadas en la página temporal
// ---------------------------------------------------------------------------

const probeFontSrc = "data:font/woff2;base64,QUJD" // "ABC"

func fontThemeFixture() renderer.DiagramThemeColors {
	return renderer.DiagramThemeColors{
		NodeBG:     "#101010",
		FontFamily: "Probe Sans",
		Fonts: []renderer.DiagramFontFace{
			{Family: "Probe Sans", Weight: "700", Src: probeFontSrc},
		},
	}
}

// TestBuildMermaidHTML_FontsReachBothPages: el nombre de familia ya llegaba
// vía themeVariables, pero un nombre no basta — la página temporal es un
// documento aparte que no hereda los @font-face del deck, así que sin el
// recurso Mermaid mide con la fuente fallback y hornea esas métricas en el
// SVG. Se afirma sobre LAS DOS puertas, igual que los colores.
func TestBuildMermaidHTML_FontsReachBothPages(t *testing.T) {
	const code = "graph TD; A-->B"
	páginas := map[string]string{
		"SVG": buildMermaidSVGHTML(code, fontThemeFixture()),
		"PNG": buildMermaidPNGHTML(code, 400, 300, fontThemeFixture()),
	}

	for name, html := range páginas {
		for _, want := range []string{
			"@font-face {",
			`font-family: "Probe Sans";`,
			"src: url(" + probeFontSrc + `) format("woff2");`,
			"font-weight: 700;",
		} {
			if !strings.Contains(html, want) {
				t.Errorf("la página %s no recibió el recurso de fuente (%q):\n%s", name, want, html)
			}
		}
		// Y la CSP tiene que permitirlo: default-src 'none' bloquea la carga
		// de una fuente aunque el @font-face esté ahí.
		if !strings.Contains(html, "font-src data:;") {
			t.Errorf("la página %s no declara font-src, su CSP bloquearía la fuente:\n%s", name, html)
		}
	}
}

// TestBuildMermaidHTML_FontsInvertTheRenderOrder es el corazón del arreglo, y
// la razón de que no alcance con inyectar el @font-face: un @font-face NO se
// descarga hasta que algo lo usa, así que con el orden de siempre
// (startOnLoad y a dibujar) Mermaid mediría antes de que la fuente exista.
//
// Con fuentes: startOnLoad pasa a false, se piden explícitamente vía
// document.fonts.load() y recién después se llama mermaid.run().
func TestBuildMermaidHTML_FontsInvertTheRenderOrder(t *testing.T) {
	for name, html := range map[string]string{
		"SVG": buildMermaidSVGHTML("graph TD; A-->B", fontThemeFixture()),
		"PNG": buildMermaidPNGHTML("graph TD; A-->B", 400, 300, fontThemeFixture()),
	} {
		if !strings.Contains(html, "startOnLoad: false") {
			t.Errorf("la página %s dejó startOnLoad activo: Mermaid dibujaría antes de las fuentes:\n%s", name, html)
		}
		for _, want := range []string{"document.fonts.load(", "mermaid.run()", "Promise.race("} {
			if !strings.Contains(html, want) {
				t.Errorf("la página %s no espera las fuentes (%q falta):\n%s", name, want, html)
			}
		}
		// El shorthand tiene que ser uno que document.fonts.load() acepte:
		// un shorthand inválido hace que load() rechace en vez de cargar, y
		// el .catch seguiría a dibujar igual — o sea, fallaría en silencio y
		// exactamente igual que no haber hecho nada.
		if !strings.Contains(html, `700 16px \"Probe Sans\"`) {
			t.Errorf("la página %s no pide la familia con un shorthand válido:\n%s", name, html)
		}
	}
}

// TestBuildMermaidPNGHTML_SignalMovesAfterRender cubre la consecuencia menos
// obvia de diferir el render: #renderComplete es lo que espera chromedp antes
// de capturar, y su setTimeout arranca al evaluar el script. Si se queda al
// nivel de siempre, se marca listo mientras el diagrama todavía no existe y
// la captura sale vacía. Tiene que vivir DENTRO del then() de run().
func TestBuildMermaidPNGHTML_SignalMovesAfterRender(t *testing.T) {
	html := buildMermaidPNGHTML("graph TD; A-->B", 400, 300, fontThemeFixture())

	done := strings.Index(html, "function done() {")
	señal := strings.Index(html, "setAttribute('data-ready'")
	draw := strings.Index(html, "function draw() {")
	if done < 0 || señal < 0 || draw < 0 {
		t.Fatalf("faltan piezas del bootstrap (done=%d señal=%d draw=%d):\n%s", done, señal, draw, html)
	}
	if done >= señal || señal >= draw {
		t.Errorf("la señal de #renderComplete no quedó dentro de done() (done=%d señal=%d draw=%d):\n%s", done, señal, draw, html)
	}
	if !strings.Contains(html, ".then(done)") {
		t.Errorf("done() no está encadenado al render:\n%s", html)
	}
}

// TestBuildMermaidHTML_WithoutFontsKeepsTheOldMechanism: un tema sin fuentes
// —o sin tema— no paga nada de lo anterior. El camino de siempre queda
// intacto, que es la otra mitad del contrato "sin tema, byte por byte igual".
func TestBuildMermaidHTML_WithoutFontsKeepsTheOldMechanism(t *testing.T) {
	casos := map[string]renderer.DiagramThemeColors{
		"sin tema":         {},
		"tema sin fuentes": diagramTheme(),
		"fuente no utilizable": {FontFamily: "X", Fonts: []renderer.DiagramFontFace{
			{Family: "X", Src: "https://fonts.example/x.woff2"},
		}},
	}
	for nombre, tema := range casos {
		for name, html := range map[string]string{
			"SVG": buildMermaidSVGHTML("graph TD; A-->B", tema),
			"PNG": buildMermaidPNGHTML("graph TD; A-->B", 400, 300, tema),
		} {
			if !strings.Contains(html, "startOnLoad: true") {
				t.Errorf("%s/%s debía conservar startOnLoad:\n%s", nombre, name, html)
			}
			for _, noWant := range []string{"document.fonts", "mermaid.run()", "@font-face"} {
				if strings.Contains(html, noWant) {
					t.Errorf("%s/%s no debía traer %q:\n%s", nombre, name, noWant, html)
				}
			}
		}
	}
}

// TestMermaidFetcher_CacheKeyIncludesFontBytes extiende la lección de #250 al
// recurso: dos temas que declaran la MISMA familia pero distinto archivo
// producen diagramas con métricas distintas, así que no pueden compartir
// entrada de cache.
func TestMermaidFetcher_CacheKeyIncludesFontBytes(t *testing.T) {
	const code = "graph TD; A-->B"
	conFuente := func(src string) *MermaidFetcher {
		f := &MermaidFetcher{}
		f.SetDiagramThemeColors(renderer.DiagramThemeColors{
			FontFamily: "Probe Sans",
			Fonts:      []renderer.DiagramFontFace{{Family: "Probe Sans", Src: src}},
		})
		return f
	}

	a := conFuente("data:font/woff2;base64,QUJD") // "ABC"
	b := conFuente("data:font/woff2;base64,WFla") // "XYZ"
	gemelo := conFuente("data:font/woff2;base64,QUJD")

	if a.cacheKey(code) == b.cacheKey(code) {
		t.Error("dos archivos de fuente distintos deben producir claves distintas")
	}
	if a.cacheKey(code) != gemelo.cacheKey(code) {
		t.Error("el mismo archivo debe producir la misma clave")
	}
	if a.cacheKey(code) == GenerateContentHash(code) {
		t.Error("un tema con solo fuentes debe cambiar la clave")
	}
}
