// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"math/rand"
	"reflect"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/a11y"
	"go.ziradocs.com/core/v2/renderer"
)

// Oráculo: pptxSplitInline y pptxSplitBasic tal como estaban en v2.49.2, que
// en cada paso volvían a buscar todos los patterns sobre lo que quedaba del
// texto.

func oraclePptxSplitInline(content string) []pptxInlineSegment {
	var segments []pptxInlineSegment
	remaining := content
	// emphasis es remaining con los destinos tapados: ahí se buscan la
	// negrita y la cursiva; el código se busca en el texto tal cual.
	emphasis := pptxMaskLinkDestinations(content)
	pos := 0

	for pos < len(remaining) {
		type basicMatch struct {
			start, end         int
			inner              string
			bold, italic, code bool
		}
		var best *basicMatch
		// bestRelPos se mantiene relativo a remaining[pos:] durante toda
		// esta iteración (pos es constante aquí), así que se compara
		// directo contra loc[0] — sin re-relativizar una posición
		// absoluta a mitad de la comparación, que es lo que hacía frágil
		// agregar un patrón más.
		bestRelPos := len(remaining) - pos

		for _, re := range pptxBasicPatterns {
			haystack := emphasis
			if re.code {
				haystack = remaining
			}
			loc := re.re.FindStringSubmatchIndex(haystack[pos:])
			if loc == nil || loc[0] >= bestRelPos {
				continue
			}
			bestRelPos = loc[0]
			best = &basicMatch{
				start: pos + loc[0], end: pos + loc[1],
				inner: remaining[pos+loc[2] : pos+loc[3]],
				bold:  re.bold, italic: re.italic, code: re.code,
			}
		}

		var langLoc []int
		if loc := renderer.InlineLangSpanPattern.FindStringSubmatchIndex(remaining[pos:]); loc != nil && loc[0] < bestRelPos {
			langLoc = loc
			bestRelPos = loc[0]
			best = nil // el span de idioma gana sobre code/bold/italic en esta posición
		}

		// Token de clase `[texto]{.clase}`. Se evalúa después del span de
		// idioma y contra el mismo bestRelPos, así que el que empiece antes
		// gana; los dos comparten delimitador y no pueden anidarse (el
		// content-class de ambos excluye "[").
		if loc := pptxSpanTokenRe.FindStringSubmatchIndex(remaining[pos:]); loc != nil && loc[0] < bestRelPos {
			matchStart, matchEnd := pos+loc[0], pos+loc[1]
			inner := remaining[pos+loc[2] : pos+loc[3]]
			class := remaining[pos+loc[4] : pos+loc[5]]

			if matchStart > pos {
				segments = append(segments, pptxInlineSegment{text: remaining[pos:matchStart]})
			}
			// Un token DESCONOCIDO se emite literal, con corchetes y llaves.
			// Es lo que hace el HTML (ver el comentario de inlineSpanPattern en
			// core/renderer/sanitizer.go) y lo que hace acá al lado el span de
			// idioma con un tag inválido: quedarse solo con el texto interno
			// escondería el typo del autor.
			style, known := pptxSpanTokenStyle[class]
			if !known {
				segments = append(segments, pptxInlineSegment{text: remaining[matchStart:matchEnd]})
				pos = matchEnd
				continue
			}
			// El texto interno se procesa por code/bold/italic, igual que el de
			// un span de idioma, y el estilo del token se estampa encima de
			// cada segmento resultante.
			for _, seg := range oraclePptxSplitBasic(inner) {
				seg.underline = seg.underline || style.underline
				seg.code = seg.code || style.code
				if style.color != nil {
					seg.color = style.color
				}
				if style.fontSizePt != 0 {
					seg.fontSizePt = style.fontSizePt
				}
				segments = append(segments, seg)
			}
			pos = matchEnd
			continue
		}

		if langLoc != nil {
			matchStart, matchEnd := pos+langLoc[0], pos+langLoc[1]
			inner := remaining[pos+langLoc[2] : pos+langLoc[3]]
			lang := remaining[pos+langLoc[4] : pos+langLoc[5]]

			if matchStart > pos {
				segments = append(segments, pptxInlineSegment{text: remaining[pos:matchStart]})
			}
			if a11y.IsValidLangTag(lang) {
				// Procesar el texto interno recursivamente por
				// code/bold/italic (finding #2) en vez de emitirlo
				// verbatim con los asteriscos/backticks literales, y
				// estampar el idioma en cada segmento resultante.
				for _, inner := range oraclePptxSplitBasic(inner) {
					inner.lang = lang
					segments = append(segments, inner)
				}
			} else {
				// Tag inválido: degradar al texto LITERAL completo
				// (con corchetes/llaves — finding #5) en vez de
				// quedarse silenciosamente solo con el texto interno,
				// que escondería el error de tipeo del autor.
				segments = append(segments, pptxInlineSegment{text: remaining[matchStart:matchEnd]})
			}
			pos = matchEnd
			continue
		}

		if best == nil {
			segments = append(segments, pptxInlineSegment{text: remaining[pos:]})
			break
		}
		if best.start > pos {
			segments = append(segments, pptxInlineSegment{text: remaining[pos:best.start]})
		}
		segments = append(segments, applySpanTokens(best.inner,
			pptxInlineSegment{bold: best.bold, italic: best.italic, code: best.code})...)
		pos = best.end
	}

	return segments
}

func oraclePptxSplitBasic(content string) []pptxInlineSegment {
	var segments []pptxInlineSegment
	remaining := content
	pos := 0

	for pos < len(remaining) {
		var best *struct {
			start, end         int
			inner              string
			bold, italic, code bool
		}
		bestRelPos := len(remaining) - pos

		for _, re := range pptxBasicPatterns {
			loc := re.re.FindStringSubmatchIndex(remaining[pos:])
			if loc == nil || loc[0] >= bestRelPos {
				continue
			}
			bestRelPos = loc[0]
			best = &struct {
				start, end         int
				inner              string
				bold, italic, code bool
			}{
				start: pos + loc[0], end: pos + loc[1],
				inner: remaining[pos+loc[2] : pos+loc[3]],
				bold:  re.bold, italic: re.italic, code: re.code,
			}
		}

		if best == nil {
			segments = append(segments, pptxInlineSegment{text: remaining[pos:]})
			break
		}
		if best.start > pos {
			segments = append(segments, pptxInlineSegment{text: remaining[pos:best.start]})
		}
		segments = append(segments, applySpanTokens(best.inner,
			pptxInlineSegment{bold: best.bold, italic: best.italic, code: best.code})...)
		pos = best.end
	}

	return segments
}

var pptxSplitAlphabet = []string{
	"*", "**", "`", "a", "b", " ", "[", "]", "(", ")", "!", "{", "}", "=", "\n",
	"![a](i*.png)", "[l](u*)", "[t]{lang=fr}", "[t]{lang=x_y}", "[k]{.kbd}", "[k]{.nope}", "[a](b", "[*a*]{lang=es}",
}

// pptxSplitInline y pptxSplitBasic parten igual que antes de guardar el
// próximo match de cada patrón.
func TestPptxSplitInlineMatchesOld(t *testing.T) {
	r := rand.New(rand.NewSource(4))
	for i := 0; i < 30000; i++ {
		var b strings.Builder
		for n := 1 + r.Intn(25); n > 0; n-- {
			b.WriteString(pptxSplitAlphabet[r.Intn(len(pptxSplitAlphabet))])
		}
		s := b.String()
		if got, want := pptxSplitInline(s), oraclePptxSplitInline(s); !reflect.DeepEqual(got, want) {
			t.Fatalf("pptxSplitInline(%q)\n nuevo %+v\n viejo %+v", s, got, want)
		}
		if got, want := pptxSplitBasic(s), oraclePptxSplitBasic(s); !reflect.DeepEqual(got, want) {
			t.Fatalf("pptxSplitBasic(%q)\n nuevo %+v\n viejo %+v", s, got, want)
		}
	}
}

// Un párrafo con muchos tramos es lineal (antes, cuadrático).
func TestPptxSplitInlineScalesLinearly(t *testing.T) {
	if testing.Short() {
		t.Skip("mide tiempos")
	}
	for name, gen := range map[string]func(n int) string{
		"tramos": func(n int) string {
			return strings.Repeat("Texto [a](https://e.com/p?a=1&b=2) y ![i](img/a.png \"t\") *é* ", n)
		},
		"[a](b":        func(n int) string { return strings.Repeat("[a](b", n) },
		"énfasis":      func(n int) string { return strings.Repeat("*a* **b** ", n) },
		"idioma largo": func(n int) string { return "[" + strings.Repeat("*a* ", n) + "]{lang=fr}" },
	} {
		assertLinear(t, name, func(s string) { pptxSplitInline(s) }, gen)
	}
}
