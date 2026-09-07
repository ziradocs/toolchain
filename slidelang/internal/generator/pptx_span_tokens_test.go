// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
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
			segments := pptxSplitInline("antes [X]{." + class + "} después")

			var got strings.Builder
			for _, s := range segments {
				got.WriteString(s.text)
			}
			if strings.Contains(got.String(), "{."+class+"}") || strings.Contains(got.String(), "[X]") {
				t.Fatalf("el token salió literal: %q", got.String())
			}
			if !strings.Contains(got.String(), "X") {
				t.Fatalf("se perdió el texto del token: %q", got.String())
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
			if s.color == nil || s.color.R != 0xdc {
				t.Errorf("color = %+v, se esperaba el rojo del CSS", s.color)
			}
		}},
		// Los que PPTX no puede pintar: el texto sale, el estilo no, y ni uno
		// ni otro es un error.
		{"sub", expectPlain},
		{"sup", expectPlain},
		{"highlight-warning", expectPlain},
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
