// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package pipeline

import (
	"testing"

	"go.ziradocs.com/core/v2/include"
)

func testRebaser(lines []string, origins include.LineOrigins) *assetRebaser {
	return &assetRebaser{
		lines:      lines,
		origins:    origins,
		sourceDir:  "/p/docs",
		realSource: "/p/docs",
		realDirs:   map[string]string{"/p/a": "/p/a", "/p/docs": "/p/docs", "/p/cap uno": "/p/cap uno"},
	}
}

// TestAssetRebaserInline: solo cambia la ruta del destino de cada
// ![alt](dest) local y relativo; respeta títulos, <...>, code spans,
// remotas, absolutas, {{variables}} y destinos con escapes.
func TestAssetRebaserInline(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Ver ![a](x.png) aquí.", "Ver ![a](../a/x.png) aquí."},
		{"![a](img/x.png \"Título\")", "![a](../a/img/x.png \"Título\")"},
		{"![a](<mi foto.png>)", "![a](<../a/mi foto.png>)"},
		{"Dos ![a](x.png) y ![b](./y.png).", "Dos ![a](../a/x.png) y ![b](../a/y.png)."},
		{"Código `![a](x.png)` y ![b](x.png)", "Código `![a](x.png)` y ![b](../a/x.png)"},
		{"Doble ``![a](x.png) ` `` fin", "Doble ``![a](x.png) ` `` fin"},
		{"![r](https://e.com/x.png)", "![r](https://e.com/x.png)"},
		{"![r](/abs/x.png)", "![r](/abs/x.png)"},
		{"![r]({{img}}/x.png)", "![r]({{img}}/x.png)"},
		{`![r](x\_y.png)`, `![r](x\_y.png)`},
		{"![r](x.png", "![r](x.png"},
		{"Enlace [t](x.png) sin imagen", "Enlace [t](x.png) sin imagen"},
	}
	if commonMarkDestinations {
		// Una ruta suelta con espacios se reescribe entre <...>.
		cases = append(cases, struct{ in, want string }{"![a](mi foto.png)", "![a](<../a/mi foto.png>)"})
	} else {
		cases = append(cases, struct{ in, want string }{"![a](mi foto.png)", "![a](../a/mi foto.png)"})
	}
	for _, tc := range cases {
		r := testRebaser([]string{"raíz", tc.in}, include.LineOrigins{"", "/p/a/doc.md"})
		if got := r.inline(tc.in, 2); got != tc.want {
			t.Errorf("inline(%q)\n got %q\nwant %q", tc.in, got, tc.want)
		}
	}
}

// TestAssetRebaserPerOccurrence: cada ocurrencia se decide por la línea
// donde aparece. Un párrafo que empieza en un incluido sin salto de línea
// final y sigue en el archivo raíz reescribe solo la imagen del incluido,
// aunque las dos digan (logo.png).
func TestAssetRebaserPerOccurrence(t *testing.T) {
	lines := []string{"Texto ![i](logo.png)", "sigue ![r](logo.png)"}
	r := testRebaser(lines, include.LineOrigins{"/p/a/doc.md", ""})
	got := r.inline("Texto ![i](logo.png) sigue ![r](logo.png)", 1)
	if want := "Texto ![i](../a/logo.png) sigue ![r](logo.png)"; got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
	// Al revés: empieza en el raíz y sigue en el incluido.
	r = testRebaser(lines, include.LineOrigins{"", "/p/a/doc.md"})
	got = r.inline("Texto ![i](logo.png) sigue ![r](logo.png)", 1)
	if want := "Texto ![i](logo.png) sigue ![r](../a/logo.png)"; got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

// TestAssetRebaserFieldBounds: una fuente que no aparece en las líneas del
// campo (hasta la siguiente posición conocida) no se reescribe, aunque
// aparezca más abajo en otro archivo.
func TestAssetRebaserFieldBounds(t *testing.T) {
	lines := []string{"![a](x.png)", "otra cosa", "![b](y.png)"}
	r := testRebaser(lines, include.LineOrigins{"", "", "/p/a/doc.md"})
	r.starts = []int{1, 3}
	if got := r.newField(1).source("y.png"); got != "y.png" {
		t.Errorf("y.png se reescribió desde un campo que no la contiene: %q", got)
	}
	if got := r.newField(3).source("y.png"); got != "../a/y.png" {
		t.Errorf("y.png = %q", got)
	}
}

// TestAssetRebaserHeadingHTML: el <img> que el parser deja en el HTML de un
// encabezado se reescribe buscando "(src" en el texto fuente.
func TestAssetRebaserHeadingHTML(t *testing.T) {
	lines := []string{"### Head ![h](h.png)"}
	r := testRebaser(lines, include.LineOrigins{"/p/cap uno/doc.md"})
	got := r.headingHTML(`<h3 id="head-hhpng">Head <img src="h.png" alt="h"></h3>`, 1)
	want := `<h3 id="head-hhpng">Head <img src="../cap uno/h.png" alt="h"></h3>`
	if commonMarkDestinations {
		// El src es una URL: el espacio va codificado.
		want = `<h3 id="head-hhpng">Head <img src="../cap%20uno/h.png" alt="h"></h3>`
	}
	if got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}
