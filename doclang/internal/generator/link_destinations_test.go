// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"bytes"
	"go/parser"
	"go/printer"
	"go/token"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"go.ziradocs.com/core/v2/renderer"
)

// Este archivo tiene su gemelo en slidelang/internal/generator.

// linkScannerCorpus son casos de la gramática de destinos de core.
var linkScannerCorpus = []string{
	"[a](img/a.png)", `[a](img/a.png "t")`, "[a](img/a.png 't')", "[a](img/a.png (t))", `[a](img/a.png   "t"  ) y`,
	"[a](<mi dir/a.png>)", `[a](<mi dir/a.png> "t")`, "[a](<a)b.png>)", "[a](foto_(1).png)", `[a](a.png "di \"hola\"")`,
	"[a]( a.png )", "[a]( )", "[a]()", `[a](a.png"t")`, "[a](t.png bare)", "[a](mi dir/a.png)", `[a](a.png "t" x)`,
	"[a](a (b c))", "[a](a(b c))", "[a](a.png (t(u)))", `[a](<a.png>"t")`, "[a](<a b", "[a](<a<b>)", "[a](<a b) c",
	"[a](foto_(1.png", "[a](b (c", "[a](a.png \"t\nu\")", "[a](fotos/a (1).png) fin", "[a](c d.png 'e)f') fin",
	`[a](x\)*y*) y`, "[a](b (c [a](b (c )", "[[[[a](b)", "[a](b[c](d)", "[a](b[c](d) e", "![[a](b)](c)",
	"[![a](b)](c)", "![a](b){width=30%}", strings.Repeat("[a](b", 30) + ")", strings.Repeat("[", 30) + "](b)",
	strings.Repeat("[a](", 20) + strings.Repeat(")", 20) + " x", strings.Repeat("![a](b (c ", 20) + ")",
}

// linkScannerAlphabet es el alfabeto de las cadenas aleatorias.
var linkScannerAlphabet = []string{
	"[", "]", "(", ")", "!", "<", ">", `"`, "'", `\`, "{", "}", "&", " ", "\t", "\n", "\x01",
	"a", "b", "*", "`", "](", "[a](", "![a](", "{a=",
}

func randomLinkText(r *rand.Rand, n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteString(linkScannerAlphabet[r.Intn(len(linkScannerAlphabet))])
	}
	return b.String()
}

// coreImageOpen es la apertura de una imagen inline de core
// (inlineImagePattern, no exportado).
var coreImageOpen = regexp.MustCompile(`!\[([^\]]*)\]\(`)

// coreFindImage es la búsqueda de imágenes de core hecha con su API
// exportada: la apertura con el regex desde cada candidato y el destino con
// renderer.ScanLinkDestination y renderer.ParseLinkDestination. Devuelve los
// seis primeros índices de linkScanner.find (inicio, fin, etiqueta, destino)
// y el título ya sin escapes.
func coreFindImage(s string, from int) ([]int, string) {
	for search := from; search < len(s); {
		m := coreImageOpen.FindStringSubmatchIndex(s[search:])
		if m == nil {
			return nil, ""
		}
		start, openEnd := search+m[0], search+m[1]
		if raw, n, ok := renderer.ScanLinkDestination(s[openEnd:]); ok {
			d, _ := renderer.ParseLinkDestination(s[openEnd:])
			i := skipSpaceTab(raw, 0)
			if i < len(raw) && raw[i] == '<' && !d.Spaced {
				i++
			}
			destStart := openEnd + i
			return []int{start, openEnd + n, search + m[2], search + m[3], destStart, destStart + len(d.Dest)}, d.Title
		}
		search = start + 1
	}
	return nil, ""
}

func unescapeTitle(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) && isASCIIPunct(s[i+1]) {
			i++
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// compareWithCore recorre s con linkScanner y con core, avanzando como
// maskLinkDestinations.
func compareWithCore(t *testing.T, s string) {
	t.Helper()
	links := newLinkScanner(s, false)
	for from := 0; from <= len(s); {
		got, _ := links.find(from)
		want := renderer.FindInlineLinkIndex(s[from:])
		for i := range want {
			if want[i] >= 0 {
				want[i] += from
			}
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("enlace desde %d en %q: %v, core %v", from, s, got, want)
		}
		if got == nil {
			break
		}
		from = got[1]
	}
	images := newLinkScanner(s, true)
	for from := 0; from <= len(s); {
		got, _ := images.find(from)
		want, title := coreFindImage(s, from)
		if (got == nil) != (want == nil) || got != nil && !reflect.DeepEqual(got[:6], want) {
			t.Fatalf("imagen desde %d en %q: %v, core %v", from, s, got, want)
		}
		if got == nil {
			break
		}
		gotTitle := ""
		if got[6] >= 0 {
			gotTitle = unescapeTitle(s[got[6]:got[7]])
		}
		if gotTitle != title {
			t.Fatalf("título de la imagen desde %d en %q: %q, core %q", from, s, gotTitle, title)
		}
		from = got[1]
	}
}

// linkScanner lee lo mismo que core (renderer.FindInlineLinkIndex para los
// enlaces, el regex de apertura y renderer.ScanLinkDestination para las
// imágenes): sobre el corpus y sobre entradas aleatorias de su alfabeto.
func TestLinkScannerMatchesCore(t *testing.T) {
	for _, s := range linkScannerCorpus {
		compareWithCore(t, s)
	}
	r := rand.New(rand.NewSource(1))
	for i := 0; i < 20000; i++ {
		compareWithCore(t, randomLinkText(r, 1+r.Intn(30)))
	}
}

// openAt y destEnd leen lo mismo que el regex de apertura anclado y que
// renderer.ScanLinkDestination, con las consultas en orden.
func TestLinkScannerOpenAtAndDestEnd(t *testing.T) {
	open := regexp.MustCompile(`^!\[[^\]]*\]\(`)
	r := rand.New(rand.NewSource(6))
	for i := 0; i < 20000; i++ {
		s := randomLinkText(r, 1+r.Intn(30))
		opens := newLinkScanner(s, true)
		for p := 0; p < len(s); p++ {
			got, ok := opens.openAt(p)
			m := open.FindStringIndex(s[p:])
			if ok != (m != nil) || ok && got != p+m[1] {
				t.Fatalf("openAt(%d) en %q: %d %v, regex %v", p, s, got, ok, m)
			}
		}
		dests := newLinkScanner(s, true)
		for v := 1; v <= len(s); v++ {
			if s[v-1] != '(' {
				continue
			}
			n, ok := dests.destEnd(v)
			_, want, wantOK := renderer.ScanLinkDestination(s[v:])
			if ok != wantOK || ok && n != want {
				t.Fatalf("destEnd(%d) en %q: %d %v, core %d %v", v, s, n, ok, want, wantOK)
			}
		}
	}
}

// attrsScanner con comillas crudas es el regex de atributos de los
// generadores, en todas las posiciones.
func TestAttrsScannerMatchesRegex(t *testing.T) {
	re := regexp.MustCompile(`^\{(?:\s*[A-Za-z][\w-]*=(?:"[^"}]*"|[^\s}]+))+\s*\}`)
	alphabet := []string{"{", "}", "=", "a", "b", "-", "_", "1", " ", "\t", "\n", `"`, "&", "!", "x="}
	r := rand.New(rand.NewSource(2))
	for i := 0; i < 20000; i++ {
		var b strings.Builder
		for n := 1 + r.Intn(30); n > 0; n-- {
			b.WriteString(alphabet[r.Intn(len(alphabet))])
		}
		s := b.String()
		a := attrsScanner{text: s, quote: `"`}
		for p := 0; p <= len(s); p++ {
			end, ok := a.blockEnd(p)
			loc := re.FindStringIndex(s[p:])
			if ok != (loc != nil) || ok && end != p+loc[1] {
				t.Fatalf("blockEnd(%d) en %q: %d %v, regex %v", p, s, end, ok, loc)
			}
		}
	}
}

// scaleInputs son las líneas que eran cuadráticas en el enmascarado, en la
// búsqueda de imágenes del DOCX, en la salida Markdown o en el rebase de
// @include: aperturas cuyo destino no cierra, con y sin paréntesis, etiquetas
// sin "]" y atributos de imagen sin cerrar.
var scaleInputs = map[string]func(n int) string{
	"[a](b":                 func(n int) string { return strings.Repeat("[a](b", n) },
	"![a](b":                func(n int) string { return strings.Repeat("![a](b", n) },
	"aperturas y un cierre": func(n int) string { return strings.Repeat("[a](b (c ", n) + ")" },
	"imágenes y un cierre":  func(n int) string { return strings.Repeat("![a](b (c ", n) + ")" },
	"sin cierre":            func(n int) string { return strings.Repeat("[a](b (c ![d](e ", n) },
	"![![![":                func(n int) string { return strings.Repeat("![", n) + "](b" },
	"![a](b){a=":            func(n int) string { return strings.Repeat("![a](b){a=", n) },
	"![a](b){a=x + !}":      func(n int) string { return strings.Repeat("![a](b){a=x", n) + " !}" },
	"enlaces e imágenes":    func(n int) string { return strings.Repeat(`*a* ![i](x*y.png "t") [b](u*v) `, n) },
	"con espacios":          func(n int) string { return strings.Repeat("[a](mi página.html) ![b](mi foto.png) ", n) },
}

// TestLinkScannerStepsAreLinear cuenta bytes recorridos, no tiempo: con 4n
// tienen que ser ~4 veces los de n (cuadrático daría ~16).
func TestLinkScannerStepsAreLinear(t *testing.T) {
	steps := func(s string) int {
		total := 0
		for _, image := range []bool{true, false} {
			f := newLinkScanner(s, image)
			for from := 0; from < len(s); {
				loc, _ := f.find(from)
				if loc == nil {
					break
				}
				from = loc[1]
			}
			total += f.steps
		}
		return total
	}
	for name, gen := range scaleInputs {
		s1, s4 := steps(gen(2000)), steps(gen(8000))
		if s4 > 5*s1 {
			t.Errorf("%s: %d pasos con n=2000 y %d con 4n (más que lineal)", name, s1, s4)
		}
	}
}

func TestMaskLinkDestinationsScalesLinearly(t *testing.T) {
	if testing.Short() {
		t.Skip("mide tiempos")
	}
	for name, gen := range scaleInputs {
		assertLinear(t, name, func(s string) { maskLinkDestinations(s, docxCodeSpanRe) }, gen)
	}
}

// assertLinear mide f sobre gen(n) y gen(4n), con n creciendo hasta que
// gen(n) tarde al menos 3 ms, y falla si 4n tarda 10 veces más o peor
// (lineal da ~4x, cuadrático ~16x). Cada medida es la mejor de cinco,
// después de un GC, y una comparación que falla se repite antes de
// reportarla: una pausa ajena puede inflar una medida, no las tres.
func assertLinear(t *testing.T, name string, f func(string), gen func(int) string) {
	t.Helper()
	best := func(s string) time.Duration {
		b := time.Duration(1 << 62)
		for i := 0; i < 5; i++ {
			runtime.GC()
			start := time.Now()
			f(s)
			if d := time.Since(start); d < b {
				b = d
			}
		}
		return b
	}
	n := 250
	for n < 1<<16 && best(gen(n)) < 3*time.Millisecond {
		n *= 2
	}
	small, large := gen(n), gen(4*n)
	var t1, t4 time.Duration
	for attempt := 0; attempt < 3; attempt++ {
		t1, t4 = best(small), best(large)
		if t4 < 10*t1 {
			return
		}
	}
	t.Errorf("%s: %v con n=%d y %v con 4n (más que lineal)", name, t1, n, t4)
}

// link_destinations.go tiene una copia en slidelang/internal/generator: las dos
// tienen que ser el mismo código, salvo los comentarios. Fuera del monorepo
// (el módulo descargado solo) la copia no está y la prueba se salta.
//
// Las dos copias existen porque los CLIs compilan contra el core publicado,
// que no exporta su buscador. Cuando los dos se suban a core v2.49.3 (que ya
// es lineal), maskLinkDestinations y los recorridos de imágenes pueden volver
// a la API de core y las copias se borran.
func TestLinkDestinationsTwinIsIdentical(t *testing.T) {
	twin := filepath.Join("..", "..", "..", "slidelang", "internal", "generator", "link_destinations.go")
	if _, err := os.Stat(twin); err != nil {
		t.Skipf("sin la copia gemela: %v", err)
	}
	code := func(path string) string {
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		var b bytes.Buffer
		if err := printer.Fprint(&b, fset, f); err != nil {
			t.Fatal(err)
		}
		return b.String()
	}
	if code("link_destinations.go") != code(twin) {
		t.Fatalf("link_destinations.go difiere de %s fuera de los comentarios", twin)
	}
}
