// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"math/rand"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// inlineAttrsPrefixOracle es el regex que attrsScanner reemplaza.
var inlineAttrsPrefixOracle = regexp.MustCompile(`^` + inlineAttrsBody)

// linkSyntaxAlphabet es el alfabeto de las cadenas aleatorias: toda la
// sintaxis que leen la apertura, el destino, el título y los atributos, con
// algunos bytes de relleno.
var linkSyntaxAlphabet = []string{
	"[", "]", "(", ")", "!", "<", ">", `"`, "'", `\`, "{", "}", "&", ";", "=",
	" ", "\t", "\n", "\x01", "a", "b", "x", "-", "_", "1", "%",
	"](", "[a](", "![a](", "&amp;", "&quot;", "{a=", "<zdc0/>", "<zde1/>",
}

func randomLinkSyntax(r *rand.Rand, n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteString(linkSyntaxAlphabet[r.Intn(len(linkSyntaxAlphabet))])
	}
	return b.String()
}

// compareFinders recorre text con el buscador nuevo y con el de v2.49.2
// (inline_links_oracle_test.go) como lo hace replaceInlineLinks, y para cada
// imagen compara además el bloque de atributos con el regex.
func compareFinders(t *testing.T, text string, image, escaped bool) {
	t.Helper()
	open := inlineLinkPattern
	if image {
		open = inlineImagePattern
	}
	got := newLinkFinder(text, image, escaped)
	want := newOracleLinkFinder(text, open, escaped)
	for from := 0; from <= len(text); {
		gl, gs := got.find(from)
		wl, ws := want.find(from)
		if !reflect.DeepEqual(gl, wl) || gs != ws {
			t.Fatalf("find(%d) en %q (image=%v escaped=%v): %v %v, quiero %v %v", from, text, image, escaped, gl, gs, wl, ws)
		}
		if gl == nil {
			break
		}
		if image {
			_, ga := got.attrs.blockEnd(gl[1])
			wa := escaped && inlineAttrsPrefixOracle.MatchString(text[gl[1]:])
			if escaped && ga != wa {
				t.Fatalf("atributos después de %v en %q: %v, quiero %v", gl, text, ga, wa)
			}
		}
		from = gl[1]
	}
	// Búsquedas que no avanzan (hacia atrás y salteadas) dan lo mismo.
	for _, from := range []int{len(text) / 2, 0, len(text) / 3, len(text)} {
		gl, gs := got.find(from)
		wl, ws := want.find(from)
		if !reflect.DeepEqual(gl, wl) || gs != ws {
			t.Fatalf("find(%d) fuera de orden en %q: %v %v, quiero %v %v", from, text, gl, gs, wl, ws)
		}
	}
}

func TestLinkFinderMatchesOracle(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	iters := 20000
	if testing.Short() {
		iters = 2000
	}
	for i := 0; i < iters; i++ {
		raw := randomLinkSyntax(r, 1+r.Intn(40))
		for _, image := range []bool{false, true} {
			compareFinders(t, raw, image, false)
			compareFinders(t, EscapeHTML(raw), image, true)
		}
	}
}

func TestLinkFinderMatchesOracleCases(t *testing.T) {
	cases := []string{
		"[a](b", "[a](b (c ) x", "[a](b (c", "[a](mi página.html)", "[a](<b c> \"t\")",
		`[a](b\) c)`, `[a](b\\) c)`, "[a](b \"t\" x)", "[a](b (t))", "[a](b (t)", "( )",
		"[a]( )", "[a]()", "[a](<>)", "[a](<>", "![a](b){width=30%}", "![a](b){a=x y}",
		"![a](b){a=&quot;x y&quot;}", "![a](b){ }", "![a](b){a=b=c}", "![a](b){a=xb=&quot;y z&quot;}",
		"[[[[a](b)", "[a](b[c](d)", "[a](b[c](d) e", "![[a](b)](c)", "[![a](b)](c)",
		strings.Repeat("[a](b", 50) + ")", strings.Repeat("[a](b (c ", 30) + ")",
		strings.Repeat("![a](b){a=", 20) + "}", strings.Repeat("![a](b){a=x", 20) + " !}",
		strings.Repeat("[", 40) + "](b)", strings.Repeat("[a](", 30) + strings.Repeat(")", 30) + " x",
	}
	for _, c := range cases {
		for _, image := range []bool{false, true} {
			compareFinders(t, c, image, false)
			compareFinders(t, EscapeHTML(c), image, true)
		}
	}
}

// TestAttrsScannerMatchesRegex compara attrsScanner con el regex en todas las
// posiciones de cadenas hechas de la sintaxis de los atributos, con las dos
// comillas.
func TestAttrsScannerMatchesRegex(t *testing.T) {
	raw := regexp.MustCompile(`^\{(?:\s*[A-Za-z][\w-]*=(?:"[^"}]*"|[^\s}]+))+\s*\}`)
	alphabet := []string{"{", "}", "=", "a", "b", "-", "_", "1", " ", "\t", "\n", `"`, "&quot;", "&", "!", "x="}
	r := rand.New(rand.NewSource(2))
	for i := 0; i < 20000; i++ {
		var b strings.Builder
		for n := 1 + r.Intn(30); n > 0; n-- {
			b.WriteString(alphabet[r.Intn(len(alphabet))])
		}
		s := b.String()
		for _, v := range []struct {
			quote string
			re    *regexp.Regexp
		}{{"&quot;", inlineAttrsPrefixOracle}, {`"`, raw}} {
			a := attrsScanner{text: s, quote: v.quote}
			for p := 0; p <= len(s); p++ {
				end, ok := a.blockEnd(p)
				loc := v.re.FindStringIndex(s[p:])
				if ok != (loc != nil) || ok && end != p+loc[1] {
					t.Fatalf("blockEnd(%d) en %q (quote %s): %d %v, regex %v", p, s, v.quote, end, ok, loc)
				}
			}
		}
	}
}

func FuzzLinkFinderMatchesOracle(f *testing.F) {
	for _, s := range []string{"[a](b", "[a](b (c )", "![a](b){a=x}", "[a](<b> \"t\")", `[a](b\)c)`} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if len(s) > 2000 {
			return
		}
		for _, image := range []bool{false, true} {
			compareFinders(t, s, image, false)
			compareFinders(t, EscapeHTML(s), image, true)
		}
	})
}

// linearInputs son las líneas que eran cuadráticas (o casi) en v2.49.2: una
// apertura cuyo destino no cierra repetida, con y sin paréntesis, escapes,
// títulos, atributos de imagen y etiquetas sin "]".
var linearInputs = map[string]func(n int) string{
	"[a](b":               func(n int) string { return strings.Repeat("[a](b", n) },
	"[a](b (c + )":        func(n int) string { return strings.Repeat("[a](b (c ", n) + ")" },
	"[a](b&c":             func(n int) string { return strings.Repeat("[a](b&c ", n) },
	"[[[ ](b":             func(n int) string { return strings.Repeat("[", n) + "](b" },
	"![a](b){a=":          func(n int) string { return strings.Repeat("![a](b){a=", n) },
	"![a](b){a=x + !}":    func(n int) string { return strings.Repeat("![a](b){a=x", n) + " !}" },
	"![a](b":              func(n int) string { return strings.Repeat("![a](b", n) },
	"![[a](b":             func(n int) string { return strings.Repeat("![[a](b", n) },
	"[x](":                func(n int) string { return strings.Repeat("[x](", n) },
	`[a](b\(`:             func(n int) string { return strings.Repeat(`[a](b\(`, n) },
	"[a](`b":              func(n int) string { return strings.Repeat("[a](`b", n) },
	"[a](<b":              func(n int) string { return strings.Repeat("[a](<b", n) },
	`[a](b "x`:            func(n int) string { return strings.Repeat(`[a](b "x`, n) },
	"[a]( + ))) x":        func(n int) string { return strings.Repeat("[a](", n) + strings.Repeat(")", n) + " x" },
	"válidos":             func(n int) string { return strings.Repeat("[a](https://x.com/p?a=1&b=2) ", n) },
	"imágenes con título": func(n int) string { return strings.Repeat(`![a](<mi dir/a.png> "t") `, n) },
	"rutas con espacios":  func(n int) string { return strings.Repeat("[a](mi página.html) ", n) },
	"paréntesis":          func(n int) string { return strings.Repeat("[a](b (c ", n) },
	"énfasis":             func(n int) string { return strings.Repeat(`*a ![i](x*y.png "*t*") [b](u*v) `, n) },
}

// linkFinderSteps recorre s como replaceInlineLinks (imágenes y enlaces,
// atributos incluidos) y devuelve los bytes que recorrieron los escaneos.
func linkFinderSteps(s string) int {
	text := EscapeHTML(s)
	steps := 0
	for _, image := range []bool{true, false} {
		f := newLinkFinder(text, image, true)
		for from := 0; from < len(text); {
			loc, _ := f.find(from)
			if loc == nil {
				break
			}
			if image {
				f.attrs.blockEnd(loc[1])
			}
			from = loc[1]
		}
		steps += f.steps + f.attrs.steps
	}
	return steps
}

// TestLinkFinderStepsAreLinear cuenta trabajo, no tiempo, así que no depende
// de la carga de la máquina: los bytes recorridos tienen que crecer como el
// texto (4n contra n da ~4x; cuadrático daría ~16x) y quedarse en unas pocas
// pasadas por byte.
func TestLinkFinderStepsAreLinear(t *testing.T) {
	const n = 2000
	for name, gen := range linearInputs {
		small, large := gen(n), gen(4*n)
		s1, s4 := linkFinderSteps(small), linkFinderSteps(large)
		if s4 > 5*s1 {
			t.Errorf("%s: %d pasos con n=%d y %d con 4n (más que lineal)", name, s1, n, s4)
		}
		if per := float64(s4) / float64(len(EscapeHTML(large))); per > 24 {
			t.Errorf("%s: %.1f pasos por byte", name, per)
		}
	}
}

func BenchmarkLinkFinderPathological(b *testing.B) {
	for _, name := range []string{"[a](b", "[a](b (c + )", "[a](b&c", "[[[ ](b", "![a](b){a=", "válidos"} {
		line := linearInputs[name](20000)
		b.Run(name, func(b *testing.B) {
			b.SetBytes(int64(len(line)))
			for i := 0; i < b.N; i++ {
				ProcessInlineMarkdownSecure(line)
			}
		})
	}
}
