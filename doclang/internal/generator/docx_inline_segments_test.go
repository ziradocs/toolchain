// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"math/rand"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/renderer"
)

// Oráculo: el walker del DOCX tal como estaba en v2.49.2, que en cada paso
// volvía a buscar todos los patterns sobre lo que quedaba del texto, con sus
// búsquedas de imagen y de link.

var oracleDocxImageOpen = regexp.MustCompile(`!\[([^\]]*)\]\(`)
var oracleDocxImageAttrs = regexp.MustCompile(`^\{(?:\s*[A-Za-z][\w-]*=(?:"[^"}]*"|[^\s}]+))+\s*\}`)

func oracleDocxFindInlineImage(s string) []int {
	for search := 0; search < len(s); {
		loc := oracleDocxImageOpen.FindStringSubmatchIndex(s[search:])
		if loc == nil {
			return nil
		}
		start := search + loc[0]
		openEnd := search + loc[1]
		if dest, n, ok := renderer.ScanLinkDestination(s[openEnd:]); ok {
			end := openEnd + n
			if m := oracleDocxImageAttrs.FindStringIndex(s[end:]); m != nil {
				end += m[1]
			}
			return []int{start, end, search + loc[2], search + loc[3], openEnd, openEnd + len(dest)}
		}
		search = start + 1
	}
	return nil
}

func oracleDocxFindInlineLink(s string) []int {
	if !strings.Contains(s, "![") {
		return renderer.FindInlineLinkIndex(s)
	}
	masked := []byte(s)
	for pos := 0; pos < len(s); {
		loc := oracleDocxFindInlineImage(s[pos:])
		if loc == nil {
			break
		}
		for i := pos + loc[0]; i < pos+loc[1]; i++ {
			masked[i] = 'x'
		}
		pos += loc[1]
	}
	return renderer.FindInlineLinkIndex(string(masked))
}

// oracleDocxSegments es el bucle del walker viejo. straddled dice si en algún
// paso la posición quedó estrictamente adentro de una imagen (otro tramo,
// como una negrita, la cortó por la mitad): ahí el buscador de links viejo
// volvía a enmascarar desde esa posición y el nuevo no (ver
// docxScanInlineLinks).
func oracleDocxSegments(content string, index []func(s, masked string) []int) (segs []docxInlineSegment, straddled bool) {
	var images [][2]int
	for pos := 0; pos < len(content); {
		loc := oracleDocxFindInlineImage(content[pos:])
		if loc == nil {
			break
		}
		images = append(images, [2]int{pos + loc[0], pos + loc[1]})
		pos += loc[1]
	}
	masked := docxMaskLinkDestinations(content)
	pos := 0
	for pos < len(content) {
		for _, im := range images {
			if im[0] < pos && pos < im[1] {
				straddled = true
			}
		}
		best, bestLoc := -1, []int(nil)
		for i, f := range index {
			loc := f(content[pos:], masked[pos:])
			if loc != nil && (bestLoc == nil || loc[0] < bestLoc[0]) {
				best, bestLoc = i, loc
			}
		}
		if bestLoc == nil {
			segs = append(segs, docxInlineSegment{pattern: -1, start: pos, end: len(content)})
			break
		}
		if bestLoc[0] > 0 {
			segs = append(segs, docxInlineSegment{pattern: -1, start: pos, end: pos + bestLoc[0]})
		}
		seg := docxInlineSegment{pattern: best, start: pos + bestLoc[0], end: pos + bestLoc[1]}
		if len(bestLoc) >= 4 {
			seg.inner = content[pos+bestLoc[2] : pos+bestLoc[3]]
		}
		if len(bestLoc) >= 6 {
			seg.extra = content[pos+bestLoc[4] : pos+bestLoc[5]]
		}
		segs = append(segs, seg)
		pos = seg.end
	}
	return segs, straddled
}

// docxPatternSets son los conjuntos de patterns del walker con la búsqueda
// vieja de cada uno.
func docxPatternSets() map[string]struct {
	patterns []docxInlinePattern
	index    []func(s, masked string) []int
} {
	g := &DOCXGenerator{}
	sets := map[string][]docxInlinePattern{
		"cuerpo":        g.docxInlinePatterns(),
		"énfasis":       g.docxEmphasisInnerPatterns(),
		"idioma":        g.docxLangInnerPatterns(),
		"solo imágenes": {g.docxImagePattern()},
	}
	out := map[string]struct {
		patterns []docxInlinePattern
		index    []func(s, masked string) []int
	}{}
	for name, pts := range sets {
		var index []func(s, masked string) []int
		for _, pt := range pts {
			pt := pt
			switch {
			case pt.regex != nil && pt.emphasis:
				index = append(index, func(_, masked string) []int { return pt.regex.FindStringSubmatchIndex(masked) })
			case pt.regex != nil:
				index = append(index, func(s, _ string) []int { return pt.regex.FindStringSubmatchIndex(s) })
			default:
				// Imagen o link: el link devuelve 8 índices sobre el
				// enmascarado; se distingue probando.
				if loc := pt.scan("[a](b)")(0); loc != nil {
					index = append(index, func(s, _ string) []int { return oracleDocxFindInlineLink(s) })
				} else {
					index = append(index, func(s, _ string) []int { return oracleDocxFindInlineImage(s) })
				}
			}
		}
		out[name] = struct {
			patterns []docxInlinePattern
			index    []func(s, masked string) []int
		}{pts, index}
	}
	return out
}

var docxSegmentAlphabet = []string{
	"*", "**", "`", "a", "b", " ", "x", "[", "]", "(", ")", "!", "{", "}", "=", `"`, "<", ">",
	"![a](i.png)", "[l](u)", "[t]{lang=fr}", "[k]{.kbd}", "[a](b", "![a](b c)", "{width=3}", "\\",
}

// El walker nuevo parte el texto igual que el viejo, salvo cuando otro tramo
// corta una imagen por la mitad (ver docxScanInlineLinks).
func TestDocxInlineSegmentsMatchOldWalker(t *testing.T) {
	r := rand.New(rand.NewSource(3))
	sets := docxPatternSets()
	compared := 0
	for i := 0; i < 20000; i++ {
		var b strings.Builder
		for n := 1 + r.Intn(25); n > 0; n-- {
			b.WriteString(docxSegmentAlphabet[r.Intn(len(docxSegmentAlphabet))])
		}
		s := b.String()
		for name, set := range sets {
			want, straddled := oracleDocxSegments(s, set.index)
			if straddled {
				continue
			}
			compared++
			got := docxInlineSegments(s, set.patterns)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("%s: %q\n nuevo %+v\n viejo %+v", name, s, got, want)
			}
		}
	}
	if compared < 70000 {
		t.Fatalf("solo %d comparaciones sin imágenes cortadas", compared)
	}
}

// Cuando una negrita corta una imagen, el resto de la imagen ya no se vuelve
// a leer como link: es el único cambio del walker nuevo.
func TestDocxInlineSegmentsStraddledImage(t *testing.T) {
	g := &DOCXGenerator{}
	s := "**x ![a** [l](u)](b)"
	segs := docxInlineSegments(s, g.docxInlinePatterns())
	for _, seg := range segs {
		if seg.pattern == 4 {
			t.Fatalf("link dentro del resto de una imagen cortada: %+v", segs)
		}
	}
}

// docxInlineSegments es lineal: con 4n tarda ~4 veces lo de n (antes, un
// párrafo con muchos tramos era cuadrático).
func TestDocxInlineSegmentsScaleLinearly(t *testing.T) {
	if testing.Short() {
		t.Skip("mide tiempos")
	}
	g := &DOCXGenerator{}
	patterns := g.docxInlinePatterns()
	for name, gen := range map[string]func(n int) string{
		"tramos":          func(n int) string { return strings.Repeat("Texto [a](https://e.com/p?a=1&b=2) y ![i](img/a.png \"t\") *é* ", n) },
		"[a](b":           func(n int) string { return strings.Repeat("[a](b", n) },
		"![a](b){a=":      func(n int) string { return strings.Repeat("![a](b){a=", n) },
		"énfasis y links": func(n int) string { return strings.Repeat("*a* [b](c) ", n) },
		"imagen cortada":  func(n int) string { return strings.Repeat("**x ![a**](b) ", n) },
	} {
		assertLinear(t, name, func(s string) { docxInlineSegments(s, patterns) }, gen)
	}
}
