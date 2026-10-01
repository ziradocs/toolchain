// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"regexp"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/a11y"
	"golang.org/x/net/html"
)

// inlineAllowedAttrs es el conjunto cerrado de etiquetas y atributos que las
// pasadas inline (y el envoltorio de listas de ProcessInlineMarkdownSecure)
// pueden emitir. Cualquier otra etiqueta o atributo en la salida significa
// que texto del autor terminó interpretado como markup.
var inlineAllowedAttrs = map[string]map[string]bool{
	"a":      {"href": true},
	"img":    {"src": true, "alt": true},
	"strong": {},
	"em":     {},
	"mark":   {"class": true},
	"del":    {},
	"code":   {},
	"span":   {"class": true, "lang": true},
	"u":      {},
	"sub":    {},
	"sup":    {},
	"kbd":    {"class": true},
	"small":  {"class": true},
	"br":     {},
	"ul":     {},
	"li":     {},
}

var inlineClassAttrPattern = regexp.MustCompile(`class="([^"]+)"`)

// inlineAllowedClasses son los valores de class que aparecen en
// inlineSpanTokens, la única fuente de clases de las pasadas inline.
func inlineAllowedClasses() map[string]bool {
	classes := map[string]bool{}
	for _, tags := range inlineSpanTokens {
		for _, m := range inlineClassAttrPattern.FindAllStringSubmatch(tags[0], -1) {
			classes[m[1]] = true
		}
	}
	return classes
}

// inlineHTMLProblems tokeniza out como lo haría un navegador y devuelve cada
// etiqueta o atributo fuera de la allowlist, cada clase desconocida, cada
// lang inválido y cada href/src con un esquema que el filtro no permite.
func inlineHTMLProblems(out string) []string {
	classes := inlineAllowedClasses()
	var problems []string
	z := html.NewTokenizer(strings.NewReader(out))
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			return problems
		}
		if tt != html.StartTagToken && tt != html.SelfClosingTagToken {
			continue
		}
		tok := z.Token()
		allowed, ok := inlineAllowedAttrs[tok.Data]
		if !ok {
			problems = append(problems, "etiqueta <"+tok.Data+">")
			continue
		}
		for _, attr := range tok.Attr {
			switch {
			case strings.HasPrefix(strings.ToLower(attr.Key), "on"):
				problems = append(problems, "atributo de evento "+attr.Key+" en <"+tok.Data+">")
			case !allowed[attr.Key]:
				problems = append(problems, "atributo "+attr.Key+" en <"+tok.Data+">")
			case attr.Key == "class" && !classes[attr.Val]:
				problems = append(problems, "clase "+attr.Val+" en <"+tok.Data+">")
			case attr.Key == "lang" && !a11y.IsValidLangTag(attr.Val):
				problems = append(problems, "lang "+attr.Val)
			case attr.Key == "href" || attr.Key == "src":
				if p, _ := ClassifyURL(attr.Val); p != URLAllowed {
					problems = append(problems, attr.Key+" "+attr.Val)
				}
			}
		}
	}
}

func assertInlineHTMLAllowlisted(t *testing.T, input string) {
	t.Helper()
	for name, fn := range map[string]func(string) string{
		"Secure":          ProcessInlineMarkdownSecure,
		"SecureLine":      ProcessInlineMarkdownSecureLine,
		"SecureMultiline": ProcessInlineMarkdownSecureMultiline,
	} {
		out := fn(input)
		if problems := inlineHTMLProblems(out); len(problems) > 0 {
			t.Errorf("%s(%q) = %q\nfuera de la allowlist: %v", name, input, out, problems)
		}
	}
}

// inlineAttributeCases mezclan las pasadas que emiten atributos (imagen y
// enlace) con las que corren antes o después de ellas. Antes, una pasada
// posterior volvía a leer como Markdown el texto que una anterior ya había
// metido en un atributo (el alt o el src de una imagen, el href de un
// enlace), y las comillas del HTML que emitía cerraban ese atributo.
var inlineAttributeCases = []struct {
	name  string
	input string
	want  string
}{
	{
		"enlace dentro del alt",
		"![[x](y)](a onerror=alert(1)//)",
		`<img src="y" alt="[x">](a onerror=alert(1)//)`,
	},
	{
		"enlace dentro del alt con texto alrededor",
		"ver ![[x](y)](a onclick=f//) fin",
		`ver <img src="y" alt="[x">](a onclick=f//) fin`,
	},
	{
		"imagen dentro del alt de otra imagen",
		"![![a](b)](c)",
		`<img src="b" alt="![a">](c)`,
	},
	{
		"span de clase dentro del alt",
		"![[x]{.danger}](y)",
		`<img src="y" alt="x">`,
	},
	{
		"span de idioma dentro del alt",
		"![[bonjour]{lang=fr}](y)",
		`<img src="y" alt="bonjour">`,
	},
	{
		"énfasis y código dentro del alt",
		"![**a** *b* `c` ==d== ~~e~~](y)",
		`<img src="y" alt="a b c d e">`,
	},
	{
		"imagen como destino de un enlace",
		"[a](![b](c))",
		`<a href="![b](c)">a</a>`,
	},
	{
		"imagen dentro de un enlace",
		"[![a](i.png)](https://x.com)",
		`<a href="https://x.com"><img src="i.png" alt="a"></a>`,
	},
	{
		"imagen y texto dentro de un enlace",
		"[ver ![a](i.png) **ya**](https://x.com)",
		`<a href="https://x.com">ver <img src="i.png" alt="a"> <strong>ya</strong></a>`,
	},
	{
		"código en la etiqueta de un enlace",
		"[`c`](https://x.com)",
		`<a href="https://x.com"><code>c</code></a>`,
	},
	{
		// El code span se restaura dentro del destino antes de validarlo:
		// "<code>0%0X0</code>" no es una URL válida y el enlace se descarta.
		// Antes se validaba el centinela y el <code> entraba después al href.
		"código inválido como destino de un enlace",
		"[0](`0%0X0`)",
		"0",
	},
	{
		"código como destino de un enlace",
		"[a](`url`)",
		`<a href="&lt;code&gt;url&lt;/code&gt;">a</a>`,
	},
	{
		"código como destino de una imagen",
		"![a](`u`)",
		`<img src="&lt;code&gt;u&lt;/code&gt;" alt="a">`,
	},
	{
		// No hay sintaxis de título: lo que sigue al destino es parte del
		// destino, escapado dentro del mismo href.
		"destino con algo que parece un título",
		`[x](u "t")`,
		`<a href="u &quot;t&quot;">x</a>`,
	},
}

func TestInlinePasses_DoNotRewriteEmittedAttributes(t *testing.T) {
	for _, tc := range inlineAttributeCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ProcessInlineMarkdownSecure(tc.input); got != tc.want {
				t.Errorf("ProcessInlineMarkdownSecure(%q)\n got %q\nwant %q", tc.input, got, tc.want)
			}
			assertInlineHTMLAllowlisted(t, tc.input)
		})
	}
}

// FindInlineLinks corre las mismas pasadas, así que tampoco puede reportar un
// enlace que empiece adentro del alt de una imagen ya emitida.
func TestFindInlineLinks_IgnoresTextInsideEmittedAttributes(t *testing.T) {
	links := FindInlineLinks("![[x](y)](a onerror=alert(1)//)")
	if len(links) != 1 || !links[0].Image || links[0].Destination != "y" {
		t.Errorf("FindInlineLinks = %#v, quería solo la imagen con destino y", links)
	}
}

// FuzzInlineMarkdownAllowlist comprueba la propiedad general: para cualquier
// entrada, la salida de las tres entradas públicas solo tiene etiquetas y
// atributos de la allowlist, clases de inlineSpanTokens, lang válidos y
// href/src que el filtro de esquemas acepta.
func FuzzInlineMarkdownAllowlist(f *testing.F) {
	for _, tc := range inlineAttributeCases {
		f.Add(tc.input)
	}
	for _, tc := range linkDestinationCases {
		f.Add(tc.input)
	}
	for _, s := range []string{
		"**[a**]{.danger}",
		"[See [important]{.danger}](url)",
		"- ![a](b)\n- [c](d)",
		"`<zdc0/>` y &lt;zdc0/&gt;",
		"[x](`u`)",
		"![a](`u`)",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, input string) {
		assertInlineHTMLAllowlisted(t, input)
	})
}
