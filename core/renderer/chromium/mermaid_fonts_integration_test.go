// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package chromium

import (
	"bytes"
	"context"
	"encoding/base64"
	"os"
	"regexp"
	"strconv"
	"testing"

	"go.ziradocs.com/core/v2/renderer"
)

// monoFontCandidates son fuentes MONOESPACIADAS con métricas muy distintas a
// cualquier fallback proporcional — es justamente ese contraste lo que hace
// medible si la fuente llegó a tiempo. Se prueba una lista porque el test
// corre tanto en macOS como en los runners de Linux.
var monoFontCandidates = []struct{ path, mime string }{
	{"/System/Library/Fonts/Supplemental/Andale Mono.ttf", "font/ttf"},
	{"/System/Library/Fonts/Supplemental/Courier New.ttf", "font/ttf"},
	{"/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf", "font/ttf"},
	{"/usr/share/fonts/dejavu/DejaVuSansMono.ttf", "font/ttf"},
	{"/usr/share/fonts/truetype/liberation/LiberationMono-Regular.ttf", "font/ttf"},
}

func probeMonoFontDataURI(t *testing.T) string {
	t.Helper()
	for _, c := range monoFontCandidates {
		data, err := os.ReadFile(c.path)
		if err != nil {
			continue
		}
		return "data:" + c.mime + ";base64," + base64.StdEncoding.EncodeToString(data)
	}
	// Ruidoso a propósito: estos son los ÚNICOS tests que prueban que el
	// arreglo sirve (el resto verifica que la página emita lo correcto, que
	// es necesario pero no suficiente). Si se saltan, el verde de CI no dice
	// nada sobre las métricas.
	t.Skip("SIN COBERTURA EMPÍRICA: no hay ninguna fuente monoespaciada conocida en esta máquina, así que la única prueba de que la fuente llega al layout no corrió")
	return ""
}

var svgViewBoxRe = regexp.MustCompile(`viewBox="[-\d.]+ [-\d.]+ ([\d.]+) ([\d.]+)"`)

func svgViewBoxWidth(t *testing.T, svg string) float64 {
	t.Helper()
	m := svgViewBoxRe.FindStringSubmatch(svg)
	if m == nil {
		t.Fatalf("el SVG no trae viewBox:\n%s", svg)
	}
	w, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		t.Fatalf("viewBox con ancho no numérico (%q): %v", m[1], err)
	}
	return w
}

// TestRenderMermaidToSVGWithTheme_FontChangesMeasuredLayout es la ÚNICA
// prueba que demuestra que el arreglo sirve. Que la página emita el
// @font-face y el document.fonts.load() correctos es necesario pero no
// suficiente: lo que estaba roto es que Mermaid MEDÍA con la fuente
// equivocada, y eso solo se ve en las métricas del SVG que produce.
//
// Las dos corridas declaran la MISMA familia. La única diferencia es si el
// recurso viaja o no:
//
//   - con recurso: la familia existe, Mermaid mide con métricas monoespaciadas
//   - sin recurso: la familia no resuelve a nada (es un nombre inventado) y
//     Mermaid cae al fallback proporcional — el comportamiento de antes de
//     este PR, aun pasándole el nombre vía themeVariables
//
// El label está hecho de 'i': el carácter donde más se separan un
// monoespaciado y un proporcional, para que la diferencia no dependa de qué
// fuente concreta encontró probeMonoFontDataURI.
func TestRenderMermaidToSVGWithTheme_FontChangesMeasuredLayout(t *testing.T) {
	src := probeMonoFontDataURI(t)
	r := newTestChromiumRenderer(t)

	const familia = "ZiradocsProbeMono"
	const code = "graph TD; A[iiiiiiiiiiiiiiiiiiiiiiiiiiiiii] --> B[iiiiiiiiiiiiiiiiiiiiiiiiiiiiii]"

	conRecurso := renderer.DiagramThemeColors{
		FontFamily: familia,
		Fonts:      []renderer.DiagramFontFace{{Family: familia, Src: src}},
	}
	soloNombre := renderer.DiagramThemeColors{FontFamily: familia}

	svgCon, err := r.RenderMermaidToSVGWithTheme(context.Background(), code, conRecurso)
	if err != nil {
		t.Fatalf("render con la fuente: %v", err)
	}
	svgSin, err := r.RenderMermaidToSVGWithTheme(context.Background(), code, soloNombre)
	if err != nil {
		t.Fatalf("render sin la fuente: %v", err)
	}

	anchoCon := svgViewBoxWidth(t, svgCon)
	anchoSin := svgViewBoxWidth(t, svgSin)
	t.Logf("ancho medido: con la fuente=%.2f, solo con el nombre=%.2f", anchoCon, anchoSin)

	if anchoCon == anchoSin {
		t.Errorf("Mermaid midió igual con y sin la fuente (%.2f): el recurso no llegó a tiempo al layout", anchoCon)
	}
	// Un monoespaciado da a 'i' el ancho completo, así que tiene que salir
	// MÁS ancho que el fallback proporcional. La desigualdad estricta
	// descarta que la diferencia venga de ruido de redondeo.
	if anchoCon <= anchoSin {
		t.Errorf("con la fuente monoespaciada el diagrama debía salir más ancho: con=%.2f sin=%.2f", anchoCon, anchoSin)
	}
}

// TestRenderMermaidToSVGWithTheme_UnusableFontStillRenders fija el
// degradado: una fuente que el navegador nunca podrá cargar no debe colgar ni
// romper el render. La cara se descarta en Go antes de llegar a la página
// (ver DiagramFontFace.valid), así que este caso recorre el camino de
// siempre.
func TestRenderMermaidToSVGWithTheme_UnusableFontStillRenders(t *testing.T) {
	r := newTestChromiumRenderer(t)

	svg, err := r.RenderMermaidToSVGWithTheme(context.Background(), "graph TD; A-->B", renderer.DiagramThemeColors{
		FontFamily: "Rota",
		Fonts:      []renderer.DiagramFontFace{{Family: "Rota", Src: "data:font/woff2;base64,no-es-base64!"}},
	})
	if err != nil {
		t.Fatalf("una fuente inutilizable no debe romper el render: %v", err)
	}
	if svgViewBoxWidth(t, svg) <= 0 {
		t.Errorf("el diagrama salió vacío:\n%s", svg)
	}
}

// TestRenderMermaidToPNGWithTheme_FontsDoNotBreakTheReadySignal es la SEGUNDA
// puerta, y la que más fácil se rompería en silencio: el camino PNG no espera
// al nodo svg como el SVG, espera a #renderComplete, cuyo setTimeout arranca
// al evaluar el script. Al diferir el render detrás de las fuentes, esa señal
// tuvo que moverse dentro del then() de mermaid.run(); si se hubiera quedado
// donde estaba, chromedp capturaría antes de que el diagrama exista y saldría
// un PNG en blanco — sin error, sin timeout, sin nada que lo delate.
//
// Que los dos PNG difieran es lo que prueba que el de la izquierda se dibujó
// CON la fuente: mismo diagrama, misma familia declarada, única diferencia el
// recurso.
func TestRenderMermaidToPNGWithTheme_FontsDoNotBreakTheReadySignal(t *testing.T) {
	src := probeMonoFontDataURI(t)
	r := newTestChromiumRenderer(t)

	const familia = "ZiradocsProbeMono"
	const code = "graph TD; A[iiiiiiiiiiiiiiiiiiiiiiiiiiiiii] --> B[iiiiiiiiiiiiiiiiiiiiiiiiiiiiii]"

	conRecurso, err := r.RenderMermaidToPNGWithTheme(context.Background(), code, 600, 400, renderer.DiagramThemeColors{
		FontFamily: familia,
		Fonts:      []renderer.DiagramFontFace{{Family: familia, Src: src}},
	})
	if err != nil {
		t.Fatalf("render PNG con la fuente: %v", err)
	}
	soloNombre, err := r.RenderMermaidToPNGWithTheme(context.Background(), code, 600, 400, renderer.DiagramThemeColors{FontFamily: familia})
	if err != nil {
		t.Fatalf("render PNG sin la fuente: %v", err)
	}

	if len(conRecurso) == 0 {
		t.Fatal("el PNG salió vacío: la señal de #renderComplete se adelantó al render")
	}
	t.Logf("PNG: con la fuente=%d bytes, solo con el nombre=%d bytes", len(conRecurso), len(soloNombre))
	if bytes.Equal(conRecurso, soloNombre) {
		t.Error("los dos PNG son idénticos: la fuente no llegó al layout del camino PNG")
	}
}
