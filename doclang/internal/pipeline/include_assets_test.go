// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package pipeline

import (
	"testing"

	"go.ziradocs.com/core/v2/include"
)

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
	for _, tc := range cases {
		r := assetRebaser{
			lines:     []string{"raíz", tc.in},
			origins:   include.LineOrigins{"", "/p/a/doc.md"},
			sourceDir: "/p/docs",
		}
		if got := r.inline(tc.in, 2, "/p/a/doc.md", "/p/a"); got != tc.want {
			t.Errorf("inline(%q)\n got %q\nwant %q", tc.in, got, tc.want)
		}
	}
}

// TestAssetRebaserNeedsSourceInSameFile: si la fuente no aparece en las
// líneas del mismo archivo a partir de la posición, no se reescribe.
func TestAssetRebaserNeedsSourceInSameFile(t *testing.T) {
	r := assetRebaser{
		lines:     []string{"![a](x.png)", "otra cosa", "![b](y.png)"},
		origins:   include.LineOrigins{"/p/a/doc.md", "/p/a/doc.md", "/p/b/doc.md"},
		sourceDir: "/p/docs",
	}
	if got := r.source("x.png", 1, "/p/a/doc.md", "/p/a"); got != "../a/x.png" {
		t.Errorf("x.png = %q", got)
	}
	// y.png está en otro archivo: desde la línea 1 no se ve.
	if got := r.source("y.png", 1, "/p/a/doc.md", "/p/a"); got != "y.png" {
		t.Errorf("y.png se reescribió contra el archivo equivocado: %q", got)
	}
	if got := r.inline("![b](y.png)", 1, "/p/a/doc.md", "/p/a"); got != "![b](y.png)" {
		t.Errorf("inline y.png se reescribió contra el archivo equivocado: %q", got)
	}
}
