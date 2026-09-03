// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package chromium

import (
	"strings"
	"testing"
)

// TestChartSurfaceCSS_RejectsStyleEscapes es el punto de seguridad de este
// cambio: chart-surface viene de un theme.json EXTERNO y se interpola SIN
// escapar dentro de un <style>. Un valor que pueda cerrar la declaración o el
// bloque inyectaría markup en la página que Chromium carga (audit BA-11, el
// mismo motivo por el que existe renderer.SanitizeCSSCustomProperty). Ante
// cualquier valor sospechoso se cae a "white", el comportamiento histórico.
func TestChartSurfaceCSS_RejectsStyleEscapes(t *testing.T) {
	malicious := []string{
		`red; } </style><script>fetch('http://evil')</script>`,
		"red; }",
		"red}",
		"red<script>",
		"red\n; background: blue",
		"red\r\n}",
		"#fff; --x: y",
	}
	for _, in := range malicious {
		if got := chartSurfaceCSS(in); got != "white" {
			t.Errorf("chartSurfaceCSS(%q) = %q, want \"white\" (debe rechazarse)", in, got)
		}
	}
}

func TestChartSurfaceCSS_AcceptsRealColors(t *testing.T) {
	for _, in := range []string{"#0b0b0b", "#fff", "rgb(1, 2, 3)", "rgba(1,2,3,0.5)", "hsl(120, 50%, 50%)", "cornflowerblue"} {
		if got := chartSurfaceCSS(in); got != in {
			t.Errorf("chartSurfaceCSS(%q) = %q, want el valor tal cual", in, got)
		}
	}
	if got := chartSurfaceCSS(""); got != "white" {
		t.Errorf("chartSurfaceCSS(\"\") = %q, want \"white\"", got)
	}
}

// TestBuildChartHTML_SurfaceReachesTheBody comprueba el cableado completo: el
// fondo NO es una opción de Chart.js (el canvas es transparente), así que el
// único lugar donde puede aterrizar es el body de la página temporal.
func TestBuildChartHTML_SurfaceReachesTheBody(t *testing.T) {
	html := buildChartHTML(`{"type":"bar","data":{}}`, 400, 300, "#0b0b0b")
	if !strings.Contains(html, "background: #0b0b0b;") {
		t.Errorf("el color de superficie no llegó al body:\n%s", html)
	}

	plain := buildChartHTML(`{"type":"bar","data":{}}`, 400, 300, "")
	if !strings.Contains(plain, "background: white;") {
		t.Error("sin tema el fondo debe seguir siendo blanco")
	}

	// Un valor peligroso no debe aparecer NUNCA en la página, ni siquiera
	// parcialmente.
	danger := buildChartHTML(`{"type":"bar","data":{}}`, 400, 300, `red; } </style><script>alert(1)</script>`)
	if strings.Contains(danger, "<script>alert(1)</script>") || strings.Contains(danger, "</style><script>") {
		t.Errorf("se inyectó markup desde el token de tema:\n%s", danger)
	}
	if !strings.Contains(danger, "background: white;") {
		t.Error("el valor rechazado debe caer a blanco")
	}
}
