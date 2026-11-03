// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package pipeline

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/include"
	"go.ziradocs.com/core/v2/renderer"
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

// TestAssetRebaserReadsDestinationLikeCore: la ruta reescrita es la que lee
// core en el destino original, y el título y el texto que sigue no cambian.
// Se reescribe, se vuelve a leer con renderer.ParseLinkDestination y se
// compara. El incluido está en "cap uno/" para que una ruta mal partida no
// pueda rearmarse por casualidad: con el espacio de la carpeta, la ruta nueva
// va entre <...>.
func TestAssetRebaserReadsDestinationLikeCore(t *testing.T) {
	cases := []struct {
		name string
		// in es lo que sigue a "![x](" en la línea.
		in string
	}{
		// Paréntesis en el nombre: "(1).png" no es un título, porque después
		// del ")" no viene el cierre. Core lee la ruta entera (con LINK001).
		{"paren-en-ruta-suelta", "fotos/a (1).png) fin"},
		{"paren-sin-espacio", "fotos/a(1).png) fin"},
		// Lo que parece un título y no cierra: la ruta va hasta el primer ")"
		// que balancea, y "f')" queda como texto.
		{"titulo-que-no-cierra", "c d.png 'e)f') fin"},
		// Títulos con escapes y referencias de carácter: la ruta no los
		// lleva, así que se reescribe, y el título queda tal cual.
		{"titulo-con-escape", `a.png "t\"x") fin`},
		{"titulo-con-entidad", `a.png "a &amp; b") fin`},
		{"titulo-entre-parentesis", "a.png (t) ) fin"},
		{"titulo-con-espacios-en-ruta", `<a b.png> "t\"x") fin`},
		{"titulo-simple", "img/a.png 't') fin"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			line := "Ver ![x](" + tc.in
			before, ok := renderer.ParseLinkDestination(tc.in)
			if !ok {
				t.Fatalf("core no lee %q como destino", tc.in)
			}
			r := testRebaser([]string{"raíz", line}, include.LineOrigins{"", "/p/cap uno/doc.md"})
			got := r.inline(line, 2)
			const open = "Ver ![x]("
			if !strings.HasPrefix(got, open) {
				t.Fatalf("got %q", got)
			}
			after, ok := renderer.ParseLinkDestination(got[len(open):])
			if !ok {
				t.Fatalf("la reescritura %q ya no es una imagen", got)
			}
			wantPath := "../cap uno/" + renderer.LocalPathFromURL(before.Dest)
			if gotPath := renderer.LocalPathFromURL(after.Dest); gotPath != wantPath {
				t.Errorf("ruta: got %q, want %q (línea %q)", gotPath, wantPath, got)
			}
			if after.Title != before.Title {
				t.Errorf("título: got %q, want %q (línea %q)", after.Title, before.Title, got)
			}
			if rest, wantRest := got[len(open)+after.Consumed:], tc.in[before.Consumed:]; rest != wantRest {
				t.Errorf("texto que sigue: got %q, want %q", rest, wantRest)
			}
		})
	}
}

// TestAssetRebaserKeepsTitleVerbatim: el título se copia byte a byte, con sus
// escapes y referencias de carácter; solo una ruta con "\" o "&" se deja sin
// reescribir.
func TestAssetRebaserKeepsTitleVerbatim(t *testing.T) {
	cases := []struct{ in, want string }{
		{`![a](a.png "t\"x")`, `![a](../a/a.png "t\"x")`},
		{`![a](a.png "a &amp; b")`, `![a](../a/a.png "a &amp; b")`},
		{`![a](a.png 'it\'s')`, `![a](../a/a.png 'it\'s')`},
		{`![a](a&amp;b.png "t")`, `![a](a&amp;b.png "t")`},
		{`![a](a\(1\).png "t")`, `![a](a\(1\).png "t")`},
	}
	for _, tc := range cases {
		r := testRebaser([]string{"raíz", tc.in}, include.LineOrigins{"", "/p/a/doc.md"})
		if got := r.inline(tc.in, 2); got != tc.want {
			t.Errorf("inline(%q)\n got %q\nwant %q", tc.in, got, tc.want)
		}
	}
}

// TestAssetRebaserSpacedPathWithTitle: una ruta suelta con espacios seguida
// de un título bien formado. Core lee todo como ruta (con LINK001), pero el
// autor escribió una ruta y un título: se reescribe la ruta entre <...> y el
// título queda afuera, tal cual, como hasta v2.49.1.
func TestAssetRebaserSpacedPathWithTitle(t *testing.T) {
	cases := []struct{ in, want string }{
		{`![f](sub dir/a.png 'T')`, `![f](<../cap uno/sub dir/a.png> 'T')`},
		{`![f](sub dir/a.png "t\"x")`, `![f](<../cap uno/sub dir/a.png> "t\"x")`},
		{`![f](sub dir/a.png (T))`, `![f](<../cap uno/sub dir/a.png> (T))`},
	}
	for _, tc := range cases {
		r := testRebaser([]string{"raíz", tc.in}, include.LineOrigins{"", "/p/cap uno/doc.md"})
		if got := r.inline(tc.in, 2); got != tc.want {
			t.Errorf("inline(%q)\n got %q\nwant %q", tc.in, got, tc.want)
		}
	}
}
