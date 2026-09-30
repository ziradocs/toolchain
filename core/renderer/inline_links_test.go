// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"reflect"
	"testing"
)

// linkDestinationCases son las fuentes con las que se reprodujo el bug: un
// destino con esquema no permitido y paréntesis internos
// (`[Elegir](javascript:alert(1))`) salía como "Elegir)" sin ningún aviso, y
// un destino legítimo con paréntesis balanceados salía con el href truncado.
// El linter reutiliza esta misma tabla (link_rule_test.go) para comprobar que
// marca un destino exactamente cuando el renderer lo descarta.
var linkDestinationCases = []struct {
	name  string
	input string
	want  string
}{
	{"javascript con paréntesis", "[Elegir](javascript:alert(1))", "Elegir"},
	{"https", "[Elegir](https://example.com/a)", `<a href="https://example.com/a">Elegir</a>`},
	{"esquema en mayúsculas", "[Up](JavaScript:alert(1))", "Up"},
	{"espacios alrededor", "[Sp]( javascript:alert(1) )", "Sp"},
	{"data", "[Dat](data:text/html,hola)", "Dat"},
	{"vbscript", "[Vb](vbscript:msgbox)", "Vb"},
	{"entidad numérica", "[Ent](&#106;avascript:alert(1))", "Ent"},
	{"porcentaje en el esquema", "[Pct](java%73cript:alert)", "Pct"},
	{"javascript void", "[Void](javascript:void(0))", "Void"},
	{"javascript sin paréntesis", "[NoParen](javascript:alert)", "NoParen"},
	{"paréntesis balanceados legítimos", "[Wiki](https://es.wikipedia.org/wiki/Foo_(bar))", `<a href="https://es.wikipedia.org/wiki/Foo_(bar)">Wiki</a>`},
	{"esquema desconocido", "[Unk](foo:bar)", "Unk"},
	{"file", "[File](file:///etc/passwd)", "File"},
	{"mailto", "[Mail](mailto:a@b.c)", `<a href="mailto:a@b.c">Mail</a>`},
	{"tel", "[Tel](tel:+5215555555555)", `<a href="tel:+5215555555555">Tel</a>`},
	{"ancla", "[Anc](#seccion)", `<a href="#seccion">Anc</a>`},
	{"relativo", "[Rel](./otro.html)", `<a href="./otro.html">Rel</a>`},
	{"imagen inline con javascript", "![Img](javascript:alert(1))", "Img"},
	{"enlace en medio de una oración", "Antes [Elegir](javascript:alert(1)) despues.", "Antes Elegir despues."},
}

func TestProcessInlineMarkdownSecure_LinkDestinations(t *testing.T) {
	for _, tc := range linkDestinationCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ProcessInlineMarkdownSecure(tc.input); got != tc.want {
				t.Errorf("ProcessInlineMarkdownSecure(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestProcessInlineMarkdownSecure_LinkDestinationEdges(t *testing.T) {
	for _, tc := range []struct {
		name, input, want string
	}{
		// Cambio de comportamiento: sin cierre balanceado no es un enlace y
		// queda literal (antes se cortaba en el primer ")").
		{"desbalanceado queda literal", "[a](b (c)", "[a](b (c)"},
		{"desbalanceado no impide el enlace siguiente", "[a](b (c) y [d](https://e.com)", `[a](b (c) y <a href="https://e.com">d</a>`},
		{"destino vacío queda literal", "[a]()", "[a]()"},
		{"paréntesis anidados en dos niveles", "[a](https://x.com/(a(b)))", `<a href="https://x.com/(a(b))">a</a>`},
		{"texto después del cierre se conserva", "[a](https://x.com/f(1)) y más)", `<a href="https://x.com/f(1)">a</a> y más)`},
		{"imagen con paréntesis en la fuente", "![alt](img/foto_(1).png)", `<img src="img/foto_(1).png" alt="alt">`},
		{"imagen dentro de enlace", "[![alt](img.png)](https://x.com)", `<a href="https://x.com"><img src="img.png" alt="alt"></a>`},
		{"code span como destino", "[a](`url`)", `<a href="&lt;code&gt;url&lt;/code&gt;">a</a>`},
		{"entidad &amp; del autor en la query", "[q](https://x.com/?a=1&amp;b=2)", `<a href="https://x.com/?a=1&amp;b=2">q</a>`},
		{"entidad &colon; que arma el esquema", "[c](javascript&colon;alert(1))", "c"},
		{"query con &section sin punto y coma", "[s](https://x.com/p?id=1&section=intro)", `<a href="https://x.com/p?id=1&amp;section=intro">s</a>`},
		{"query con &param y &region", "[p](https://x.com/p?a=1&param=2&region=us)", `<a href="https://x.com/p?a=1&amp;param=2&amp;region=us">p</a>`},
		{"span de clase en la etiqueta de un enlace bloqueado", "[Ver [aquí]{.danger}](javascript:alert(1))", `Ver <span class="slidelang-text-danger">aquí</span>`},
		{"span de idioma en la etiqueta de un enlace bloqueado", "[Ver [ici]{lang=fr}](javascript:alert(1))", `Ver <span lang="fr">ici</span>`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ProcessInlineMarkdownSecure(tc.input); got != tc.want {
				t.Errorf("ProcessInlineMarkdownSecure(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestScanLinkDestination(t *testing.T) {
	for _, tc := range []struct {
		in       string
		dest     string
		consumed int
		ok       bool
	}{
		{"url) resto", "url", 4, true},
		{"javascript:alert(1)) resto", "javascript:alert(1)", 20, true},
		{"a(b(c))d)", "a(b(c))d", 9, true},
		{"a(b", "", 0, false},
		{")", "", 0, false},
		{"a\nb)", "", 0, false},
	} {
		dest, consumed, ok := ScanLinkDestination(tc.in)
		if dest != tc.dest || consumed != tc.consumed || ok != tc.ok {
			t.Errorf("ScanLinkDestination(%q) = (%q, %d, %v), want (%q, %d, %v)",
				tc.in, dest, consumed, ok, tc.dest, tc.consumed, tc.ok)
		}
	}
}

func TestFindInlineLinks(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want []InlineLink
	}{
		{"enlace bloqueado con paréntesis", "ver [x](javascript:alert(1)) ya", []InlineLink{{Text: "x", Destination: "javascript:alert(1)", Dropped: true}}},
		{"imagen dentro de enlace", "[![a](i.png)](https://x.com)", []InlineLink{
			{Image: true, Text: "a", Destination: "i.png"},
			{Text: `<img src="i.png" alt="a">`, Destination: "https://x.com"},
		}},
		{"span de clase en la etiqueta", "[Ver [aquí]{.danger}](javascript:alert(1))", []InlineLink{
			{Text: `Ver <span class="slidelang-text-danger">aquí</span>`, Destination: "javascript:alert(1)", Dropped: true},
		}},
		{"destino como lo escribió el autor", "[q](https://x.com/?a=1&b=2)", []InlineLink{{Text: "q", Destination: "https://x.com/?a=1&b=2"}}},
		{"enlace dentro de code span se ignora", "`[x](javascript:alert(1))`", nil},
		{"desbalanceado no es enlace", "[a](b (c)", nil},
		{"una línea por vez", "[a](b\nc)", nil},
		{"varias líneas", "[a](b)\n[c](d)", []InlineLink{{Text: "a", Destination: "b"}, {Text: "c", Destination: "d"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := FindInlineLinks(tc.in); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("FindInlineLinks(%q) = %#v, want %#v", tc.in, got, tc.want)
			}
		})
	}
}

func TestFindInlineLinkIndex(t *testing.T) {
	s := "[a](b (c) y [Wiki](https://x.org/Foo_(bar)) z"
	loc := FindInlineLinkIndex(s)
	if len(loc) != 6 {
		t.Fatalf("FindInlineLinkIndex(%q) = %v", s, loc)
	}
	if got := s[loc[0]:loc[1]]; got != "[Wiki](https://x.org/Foo_(bar))" {
		t.Errorf("match = %q", got)
	}
	if got := s[loc[2]:loc[3]]; got != "Wiki" {
		t.Errorf("label = %q", got)
	}
	if got := s[loc[4]:loc[5]]; got != "https://x.org/Foo_(bar)" {
		t.Errorf("destination = %q", got)
	}
	if FindInlineLinkIndex("sin enlaces (x)") != nil {
		t.Error("expected nil without links")
	}
}

// html.UnescapeString decodifica por prefijo entidades heredadas sin ";",
// así que usarlo tal cual convertía "&section=" en "§ion=". Solo se decodifican
// referencias completas terminadas en ";".
func TestDecodeLinkDestination(t *testing.T) {
	for in, want := range map[string]string{
		"https://x.com/p?id=1&section=intro":    "https://x.com/p?id=1&section=intro",
		"https://x.com/p?a=1&param=2&region=us": "https://x.com/p?a=1&param=2&region=us",
		"https://x.com/p?copy=1&timestamp=2":    "https://x.com/p?copy=1&timestamp=2",
		"https://x.com/p?a=1&notit;":            "https://x.com/p?a=1&notit;",
		"https://x.com/p?a=1&amp;b=2":           "https://x.com/p?a=1&b=2",
		"&copy;":                                "©",
		"&#106;avascript:x":                     "javascript:x",
		"&#x6A;avascript:x":                     "javascript:x",
		"javascript&colon;x":                    "javascript:x",
		"&#59;":                                 ";",
	} {
		if got := DecodeLinkDestination(in); got != want {
			t.Errorf("DecodeLinkDestination(%q) = %q, want %q", in, got, want)
		}
	}
}

// ValidateURLScheme se reimplementó encima de ClassifyURL; esta prueba fija
// que el contrato de siempre (URL recortada, o "" si se rechaza) no cambió.
func TestClassifyURLMatchesValidateURLScheme(t *testing.T) {
	for _, in := range []string{
		"https://x.com", " https://x.com ", "mailto:a@b.c", "#a", "./r.html", "",
		"javascript:alert(1)", "DATA:x", "foo:bar", "java%73cript:x", "http://[::1",
	} {
		problem, _ := ClassifyURL(in)
		got := ValidateURLScheme(in)
		if (problem == URLAllowed) != (got != "" || in == "") {
			t.Errorf("ClassifyURL(%q) = %v but ValidateURLScheme = %q", in, problem, got)
		}
	}
	if p, s := ClassifyURL("javascript:alert(1)"); p != URLSchemeNotAllowed || s != "javascript" {
		t.Errorf("ClassifyURL(javascript:) = (%v, %q)", p, s)
	}
	if p, _ := ClassifyURL("java%73cript:x"); p != URLInvalid {
		t.Errorf("ClassifyURL(java%%73cript:x) = %v, want URLInvalid", p)
	}
}
