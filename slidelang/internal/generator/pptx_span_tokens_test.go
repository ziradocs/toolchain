// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/renderer"
)

// documentedSpanTokens son los catorce tokens `[texto]{.clase}` que el toolchain
// declara. Es la lista contra la que se mide la cobertura de PPTX; el test de
// deriva de más abajo comprueba que core efectivamente los conoce a todos, para
// que esta constante no se vuelva una lista de deseos.
var documentedSpanTokens = []string{
	"danger", "info", "success", "warning", "accent",
	"highlight-warning", "highlight-info", "highlight-success",
	"underline", "sub", "sup", "kbd", "small", "large",
}

// Ningún token declarado puede llegar a la diapositiva con su sintaxis a la
// vista.
//
// Este es el defecto que el test fija: PPTX no conocía los `[x]{.clase}`, así
// que los catorce salían LITERALES —corchetes, llaves y nombre de clase— en
// medio del texto. Que PPTX no pueda pintar un `sub` es una limitación; que
// imprima "[2]{.sub}" es un bug, y son cosas distintas.
func TestPptxSplitInline_DocumentedTokensNeverReachTheSlideLiterally(t *testing.T) {
	for _, class := range documentedSpanTokens {
		t.Run(class, func(t *testing.T) {
			// Las tres formas en que un token aparece de verdad: pelado y
			// envuelto en el énfasis con el que se escribe naturalmente
			// (`**[CRÍTICO]{.danger}**`). La revisión encontró que las dos
			// envueltas seguían saliendo literales — las ramas de bold/italic
			// emitían su texto interno verbatim y el token no se volvía a
			// mirar—, o sea que el invariante que este test declara se
			// cumplía solo en el caso más fácil.
			for _, form := range []string{
				"antes [X]{.%s} después",
				"antes **[X]{.%s}** después",
				"antes *[X]{.%s}* después",
			} {
				in := fmt.Sprintf(form, class)
				var got strings.Builder
				for _, s := range pptxSplitInline(in) {
					got.WriteString(s.text)
				}
				if strings.Contains(got.String(), "{."+class+"}") || strings.Contains(got.String(), "[X]") {
					t.Errorf("%q → el token salió literal: %q", in, got.String())
				}
				if !strings.Contains(got.String(), "X") {
					t.Errorf("%q → se perdió el texto del token: %q", in, got.String())
				}
			}
		})
	}
}

// Un token INVENTADO sí conserva su sintaxis. Es lo que hace el HTML —ver el
// comentario de inlineSpanPattern en core/renderer/sanitizer.go— y es lo que
// convierte un typo en algo visible en vez de en un borrado silencioso. Sin
// este caso, "no dejar nada literal" se podría cumplir tragándose todo.
func TestPptxSplitInline_UnknownTokenStaysLiteral(t *testing.T) {
	segments := pptxSplitInline("antes [X]{.mystery} después")

	var got strings.Builder
	for _, s := range segments {
		got.WriteString(s.text)
	}
	if !strings.Contains(got.String(), "[X]{.mystery}") {
		t.Errorf("un token desconocido debe quedar literal para que se vea el typo; salió %q", got.String())
	}
}

// El estilo concreto de los nueve tokens que PPTX sí puede expresar, y el texto
// plano de los cinco que no.
func TestPptxSplitInline_TokenStyles(t *testing.T) {
	for _, tc := range []struct {
		class string
		check func(t *testing.T, s pptxInlineSegment)
	}{
		{"underline", func(t *testing.T, s pptxInlineSegment) {
			if !s.underline {
				t.Error("no quedó subrayado")
			}
		}},
		{"kbd", func(t *testing.T, s pptxInlineSegment) {
			if !s.code {
				t.Error("no quedó en monoespaciada")
			}
		}},
		{"small", func(t *testing.T, s pptxInlineSegment) {
			if s.fontSizePt >= pptxBodyFontSizePt {
				t.Errorf("fontSizePt = %v, se esperaba menor que el cuerpo (%v)", s.fontSizePt, pptxBodyFontSizePt)
			}
		}},
		{"large", func(t *testing.T, s pptxInlineSegment) {
			if s.fontSizePt <= pptxBodyFontSizePt {
				t.Errorf("fontSizePt = %v, se esperaba mayor que el cuerpo (%v)", s.fontSizePt, pptxBodyFontSizePt)
			}
		}},
		{"danger", func(t *testing.T, s pptxInlineSegment) {
			if s.color == nil || s.color.R != 0xef {
				t.Errorf("color = %+v, se esperaba el rojo del bundle por defecto", s.color)
			}
		}},
		// `sub`/`sup` necesitan una línea base que pptxgo no expone: el texto
		// sale, el estilo no, y ni uno ni otro es un error.
		{"sub", expectPlain},
		{"sup", expectPlain},
		// Un `highlight-*` pierde el FONDO —eso es lo inexpresable— pero
		// conserva su color de texto, que la API sí soporta.
		{"highlight-warning", func(t *testing.T, s pptxInlineSegment) {
			if s.color == nil || s.color.R != 0x92 {
				t.Errorf("color = %+v, se esperaba el color de texto del highlight", s.color)
			}
			if s.underline || s.code || s.fontSizePt != 0 {
				t.Errorf("no debería llevar ningún otro estilo: %+v", s)
			}
		}},
	} {
		t.Run(tc.class, func(t *testing.T) {
			segments := pptxSplitInline("[X]{." + tc.class + "}")
			if len(segments) != 1 {
				t.Fatalf("se esperaba 1 segmento, hay %d: %+v", len(segments), segments)
			}
			if segments[0].text != "X" {
				t.Fatalf("text = %q, se esperaba %q", segments[0].text, "X")
			}
			tc.check(t, segments[0])
		})
	}
}

func expectPlain(t *testing.T, s pptxInlineSegment) {
	t.Helper()
	if s.underline || s.code || s.bold || s.italic || s.color != nil || s.fontSizePt != 0 {
		t.Errorf("PPTX no puede expresar este token; se esperaba texto plano, quedó %+v", s)
	}
}

// El mapa de PPTX no puede acumular nombres que el toolchain no reconoce.
//
// pptxSpanTokenStyle es una lista escrita a mano, y core no exporta la suya
// (inlineSpanTokens es privado), así que la deriva se detecta por
// comportamiento: cada clave del mapa tiene que producir markup real al pasar
// por el renderer de core. Una clase que core no conoce sale literal de ahí, y
// este test la caza.
//
// La dirección contraria —un token que core agregue y este mapa no tenga— NO
// queda cubierta y no puede quedarlo sin que core exporte su tabla; el síntoma
// sería que ese token nuevo se vea como un typo en PPTX. Está anotado en el PR.
func TestPptxSpanTokenStyle_EveryKeyIsATokenCoreRenders(t *testing.T) {
	keys := make([]string, 0, len(pptxSpanTokenStyle))
	for k := range pptxSpanTokenStyle {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, class := range keys {
		t.Run(class, func(t *testing.T) {
			html := renderer.ProcessInlineMarkdownFormatsSecure("[X]{." + class + "}")
			if strings.Contains(html, "{."+class+"}") {
				t.Errorf("core no reconoce %q: lo dejó literal (%q). "+
					"Un nombre que solo existe en el mapa de PPTX no estiliza nada.", class, html)
			}
		})
	}

	// Y los catorce documentados están todos cubiertos: si core reconoce uno
	// que el mapa no tiene, ese token sale literal a la diapositiva.
	for _, class := range documentedSpanTokens {
		if _, ok := pptxSpanTokenStyle[class]; !ok {
			t.Errorf("el token documentado %q no está en pptxSpanTokenStyle: saldría literal en PPTX", class)
		}
	}
}

// Dentro de un span de código el token NO se interpreta, y eso espeja al HTML.
//
// core emite “ `[c]{.success}` “ como `<code>[c]{.success}</code>` —literal—
// mientras `**[b]{.danger}**` sí compone. Un span de código es texto que se
// muestra tal cual; ese es el punto de escribirlo, y PPTX tiene que decir lo
// mismo que el HTML sobre el mismo documento.
func TestPptxSplitInline_TokensAreNotInterpretedInsideCode(t *testing.T) {
	segments := pptxSplitInline("a `[c]{.success}` b")

	var found bool
	for _, s := range segments {
		if s.text == "[c]{.success}" {
			found = true
			if !s.code {
				t.Error("el segmento literal debería seguir siendo código")
			}
			if s.color != nil {
				t.Errorf("no debería llevar color: %+v", s.color)
			}
		}
	}
	if !found {
		var got strings.Builder
		for _, s := range segments {
			got.WriteString(s.text)
		}
		t.Errorf("el token dentro de código se interpretó; salió %q", got.String())
	}
}

// Los colores son los del bundle por DEFECTO, no los fallbacks del CSS.
//
// El comentario original de pptxSpanTokenStyle decía que replicaban los
// fallbacks "para que una diapositiva y su HTML no se vean de dos colores
// distintos". Era falso en los dos sentidos: esos fallbacks nunca se usan
// —el bundle siempre define la custom property— así que los cinco colores
// diferían del HTML incluso sin tema. Este test ancla los valores correctos.
//
// Lo que NO fija, porque el generador no lo puede cumplir: que coincidan con un
// tema. PPTX es ciego a los temas.
func TestPptxSpanTokenStyle_ColorsMatchTheDefaultBundle(t *testing.T) {
	for class, want := range map[string][3]uint8{
		"danger":  {0xef, 0x44, 0x44},
		"info":    {0x3b, 0x82, 0xf6},
		"success": {0x10, 0xb9, 0x81},
		"warning": {0xf5, 0x9e, 0x0b},
		"accent":  {0x06, 0xb6, 0xd4},
	} {
		got := pptxSpanTokenStyle[class].color
		if got == nil {
			t.Errorf("%s: sin color", class)
			continue
		}
		if got.R != want[0] || got.G != want[1] || got.B != want[2] {
			t.Errorf("%s = #%02X%02X%02X, se esperaba #%02X%02X%02X (el valor del bundle por defecto)",
				class, got.R, got.G, got.B, want[0], want[1], want[2])
		}
	}
}
