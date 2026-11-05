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
// búsquedas de imagen y de link. El código sí usa el emparejamiento nuevo
// (markdownCodeSpans, como el HTML): ese cambio es a propósito y no es lo que
// compara esta prueba.

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

// oracleDocxSegments es el bucle del walker viejo. emphasisCut dice si en
// algún paso una negrita o una cursiva terminó estrictamente adentro de una
// imagen o de un code span (los cortó por la mitad, como en "**x ![a**](b)"):
// ahí el buscador de links viejo volvía a enmascarar desde esa posición, con
// lo que quedaba de la imagen o del código, y el nuevo no, a propósito (ver
// docxScanInlineLinks). emphasis[i] marca los patterns de
// énfasis.
// oracleContent es el texto completo que recorre oracleDocxSegments, para el
// patrón de código, que se empareja sobre todo el texto.
var oracleContent string

func oracleDocxSegments(content string, index []func(s, masked string) []int, emphasis []bool) (segs []docxInlineSegment, emphasisCut bool) {
	oracleContent = content
	var images [][2]int
	for pos := 0; pos < len(content); {
		loc := oracleDocxFindInlineImage(content[pos:])
		if loc == nil {
			break
		}
		images = append(images, [2]int{pos + loc[0], pos + loc[1]})
		pos += loc[1]
	}
	codeSpans := markdownCodeSpans(content)
	masked := docxMaskLinkDestinations(content)
	pos := 0
	for pos < len(content) {
		if n := len(segs); n > 0 && segs[n-1].pattern >= 0 && emphasis[segs[n-1].pattern] {
			for _, im := range images {
				if im[0] < pos && pos < im[1] {
					emphasisCut = true
				}
			}
			for _, sp := range codeSpans {
				if sp[0] < pos && pos < sp[1] {
					emphasisCut = true
				}
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
	return segs, emphasisCut
}

// docxPatternSets son los conjuntos de patterns del walker con la búsqueda
// vieja de cada uno.
func docxPatternSets() map[string]struct {
	patterns []docxInlinePattern
	index    []func(s, masked string) []int
	emphasis []bool
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
		emphasis []bool
	}{}
	for name, pts := range sets {
		var index []func(s, masked string) []int
		var emphasis []bool
		for _, pt := range pts {
			pt := pt
			emphasis = append(emphasis, pt.emphasis)
			switch {
			case pt.regex != nil && pt.emphasis:
				index = append(index, func(_, masked string) []int { return pt.regex.FindStringSubmatchIndex(masked) })
			case pt.regex != nil:
				index = append(index, func(s, _ string) []int { return pt.regex.FindStringSubmatchIndex(s) })
			case pt.scan("`a`")(0) != nil:
				// Código: el emparejamiento de CommonMark (markdownCodeSpans)
				// sobre todo el texto, como el HTML; s es lo que queda desde
				// la posición del walker.
				index = append(index, func(s, _ string) []int {
					from := len(oracleContent) - len(s)
					loc := docxScanCodeSpans(oracleContent)(from)
					for i := range loc {
						loc[i] -= from
					}
					return loc
				})
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
			emphasis []bool
		}{pts, index, emphasis}
	}
	return out
}

var docxSegmentAlphabet = []string{
	"*", "**", "`", "a", "b", " ", "x", "[", "]", "(", ")", "!", "{", "}", "=", `"`, "<", ">",
	"![a](i.png)", "[l](u)", "[t]{lang=fr}", "[k]{.kbd}", "[a](b", "![a](b c)", "{width=3}", "\\",
}

// El walker nuevo parte el texto igual que el viejo, salvo cuando una
// negrita o una cursiva corta una imagen o un code span por la mitad (ver
// docxScanInlineLinks). Un code span que corta una imagen sí tiene que dar lo
// mismo.
func TestDocxInlineSegmentsMatchOldWalker(t *testing.T) {
	r := rand.New(rand.NewSource(3))
	sets := docxPatternSets()
	compared, skipped := 0, 0
	for i := 0; i < 40000; i++ {
		var b strings.Builder
		for n := 1 + r.Intn(25); n > 0; n-- {
			b.WriteString(docxSegmentAlphabet[r.Intn(len(docxSegmentAlphabet))])
		}
		s := b.String()
		for name, set := range sets {
			want, emphasisCut := oracleDocxSegments(s, set.index, set.emphasis)
			if emphasisCut {
				skipped++
				continue
			}
			compared++
			got := docxInlineSegments(s, set.patterns)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("%s: %q\n nuevo %+v\n viejo %+v", name, s, got, want)
			}
		}
	}
	t.Logf("%d comparaciones, %d saltadas por una negrita o cursiva que corta una imagen o un code span", compared, skipped)
	if compared < 150000 {
		t.Fatalf("solo %d comparaciones", compared)
	}
}

// Un code span que corta una imagen no se traga los links que siguen: el
// "![" del código no abre ninguna imagen.
func TestDocxInlineSegmentsCodeSpanCutsImage(t *testing.T) {
	g := &DOCXGenerator{}
	for _, s := range []string{
		"El prefijo `![` abre una imagen; ver [la guía](https://e.com/g).",
		"a``b![c``d[l](u)",
		// Del segundo review: un backtick escapado o un code span doble antes
		// corría el emparejamiento del regex viejo y la imagen se tragaba el
		// link.
		"Con ``a`b`` se muestra un backtick y con `![` empieza una imagen como ![logo](img.png); ver la `sintaxis` en [la guía](https://d.example).",
		"Usa \\` y `![` así: ![logo](img.png) `code` [la guía](https://g7.example).",
		"Escapa con \\`. Usa `![` ![captura](img.png) y luego [la `guía`](https://g4.example).",
		"Un \\` suelto no abre código; `![` tampoco abre imagen si está en código: ![ok](img.png) [link](https://g9.example) y [otro `x`](https://g10.example).",
		"![`x`}é\\\\\"[l](u)",
	} {
		segs := docxInlineSegments(s, g.docxInlinePatterns())
		// El último link del texto ([…](u) o […](https://…)) tiene que
		// salir como link.
		if k := strings.LastIndex(s, "]("); k >= 0 && !strings.HasPrefix(s, "![") {
			dest := s[k+2 : k+2+strings.IndexByte(s[k+2:], ')')]
			found := false
			for _, seg := range segs {
				found = found || seg.pattern == 4 && seg.extra == dest
			}
			if !found {
				t.Errorf("%q: sin el link a %s: %+v", s, dest, segs)
			}
		}
		want, _ := oracleDocxSegments(s, docxPatternSets()["cuerpo"].index, docxPatternSets()["cuerpo"].emphasis)
		if !reflect.DeepEqual(segs, want) {
			t.Errorf("%q: distinto del walker de v2.49.2\n nuevo %+v\n viejo %+v", s, segs, want)
		}
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
		"tramos": func(n int) string {
			return strings.Repeat("Texto [a](https://e.com/p?a=1&b=2) y ![i](img/a.png \"t\") *é* ", n)
		},
		"[a](b":                       func(n int) string { return strings.Repeat("[a](b", n) },
		"![a](b){a=":                  func(n int) string { return strings.Repeat("![a](b){a=", n) },
		"énfasis y links":             func(n int) string { return strings.Repeat("*a* [b](c) ", n) },
		"imagen cortada":              func(n int) string { return strings.Repeat("**x ![a**](b) ", n) },
		"código que corta una imagen": func(n int) string { return strings.Repeat("`![`](u) ", n) },
		"ejemplo del review": func(n int) string {
			return strings.Repeat("El prefijo `![` abre una imagen; ver [la guía](https://e.com/g). ", n)
		},
	} {
		assertLinear(t, name, func(s string) { docxInlineSegments(s, patterns) }, gen)
	}
}

// Con code spans que abren imágenes, el walker viejo daba links que dependían
// de por dónde pasaba; el nuevo, en estos casos, da los mismos links que el
// HTML (renderer.FindInlineLinks), que es la referencia. Que el DOCX resuelva
// los links con la misma lógica que el HTML en todos los casos va aparte.
func TestDocxInlineLinksMatchHTMLWithCodeSpans(t *testing.T) {
	g := &DOCXGenerator{}
	for _, s := range []string{
		"\"`b[t]{lang=fr}![a](b c)`![`>*{\\]<`>[l](u) [l](u){a**![k]{.kbd}![a](i.png){width=3}=",
		"[a](b{width=3}`![`)[l](u)[l](u)",
		"![a](b c)*a{`{([g](https://e.com/g){width=3}[g](https://e.com/g)\\=\\[k]{.kbd}`![`\"<)x**x){width=3}[l](u))\"",
	} {
		var got, want []string
		for _, seg := range docxInlineSegments(s, g.docxInlinePatterns()) {
			if seg.pattern == 4 {
				got = append(got, strings.NewReplacer("`", "").Replace(seg.extra))
			}
		}
		for _, l := range renderer.FindInlineLinks(s) {
			if !l.Image && !l.Dropped {
				want = append(want, strings.NewReplacer("<code>", "", "</code>", "").Replace(l.Destination))
			}
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%q: links del DOCX %q, del HTML %q", s, got, want)
		}
	}
}
