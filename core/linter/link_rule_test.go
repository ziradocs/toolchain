// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package linter

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/util"
)

// linkRuleInputs son las 19 fuentes de la reproducción, las mismas que
// fija core/renderer/inline_links_test.go (linkDestinationCases).
var linkRuleInputs = []string{
	"[Elegir](javascript:alert(1))",
	"[Elegir](https://example.com/a)",
	"[Up](JavaScript:alert(1))",
	"[Sp]( javascript:alert(1) )",
	"[Dat](data:text/html,hola)",
	"[Vb](vbscript:msgbox)",
	"[Ent](&#106;avascript:alert(1))",
	"[Pct](java%73cript:alert)",
	"[Void](javascript:void(0))",
	"[NoParen](javascript:alert)",
	"[Wiki](https://es.wikipedia.org/wiki/Foo_(bar))",
	"[Unk](foo:bar)",
	"[File](file:///etc/passwd)",
	"[Mail](mailto:a@b.c)",
	"[Tel](tel:+5215555555555)",
	"[Anc](#seccion)",
	"[Rel](./otro.html)",
	"![Img](javascript:alert(1))",
	"Antes [Elegir](javascript:alert(1)) despues.",
}

// linkRuleExtraInputs cubre lo que la tabla original no: entidades sin ";"
// en un query string (que no deben decodificarse ni marcarse) y spans dentro
// de la etiqueta de un enlace bloqueado (que el renderer procesa antes del
// enlace).
var linkRuleExtraInputs = []string{
	"[s](https://x.com/p?id=1&section=intro)",
	"[p](https://x.com/p?a=1&param=2&region=us)",
	"[n](https://x.com/p?a=1&notit;)",
	"[c](&copy;)",
	"[e](&#106;avascript:alert(1))",
	"[k](javascript&colon;alert(1))",
	"[Ver [aquí]{.danger}](javascript:alert(1))",
	"[Ver [ici]{lang=fr}](javascript:alert(1))",
	"[Ver [aquí]{.danger}](https://x.com)",
}

func strictTextSource(texts ...string) string {
	var b strings.Builder
	b.WriteString("---\nmode: strict\ntitle: \"Probe\"\n---\n\nSLIDE content\n  title: \"Probe\"\n")
	for _, text := range texts {
		b.WriteString("  TEXT\n    " + text + "\n")
	}
	return b.String()
}

func link001(diags []diagnostics.Diagnostic) []diagnostics.Diagnostic {
	var out []diagnostics.Diagnostic
	for _, d := range diags {
		if d.RuleID == "LINK001" {
			out = append(out, d)
		}
	}
	return out
}

// LINK001 marca un destino si y solo si el renderer lo descarta. Sin esta
// equivalencia la regla mentiría en alguna de las dos direcciones: avisaría
// de enlaces que sí se emiten, o callaría ante uno que se pierde.
func TestLinkDestinationRuleMatchesRenderer(t *testing.T) {
	for _, input := range append(append([]string{}, linkRuleInputs...), linkRuleExtraInputs...) {
		t.Run(input, func(t *testing.T) {
			html := renderer.ProcessInlineMarkdownSecure(input)
			dropped := !strings.Contains(html, "<a href=") && !strings.Contains(html, "<img ")
			found := link001(lintSource(t, strictTextSource(input)))
			if dropped != (len(found) > 0) {
				t.Fatalf("renderer dropped=%v (html %q) but LINK001=%v", dropped, html, found)
			}
			for _, d := range found {
				if d.Severity != diagnostics.Warning {
					t.Errorf("LINK001 severity = %v, want warning", d.Severity)
				}
			}
		})
	}
}

func TestLinkDestinationRuleMessagesAndPosition(t *testing.T) {
	src := strictTextSource("[Elegir](javascript:alert(1))", "[Pct](java%73cript:alert)", "![Img](javascript:alert(1))", "[Ent](&#106;avascript:alert(1))")
	found := link001(lintSource(t, src))
	if len(found) != 4 {
		t.Fatalf("got %d LINK001, want 4: %v", len(found), found)
	}
	for i, want := range []string{
		`Link destination "javascript:alert(1)" uses the "javascript:" scheme, which is not allowed`,
		`Link destination "java%73cript:alert" is not a valid URL`,
		`Image destination "javascript:alert(1)" uses the "javascript:" scheme`,
		// El mensaje cita el destino como lo escribió el autor, no la
		// versión decodificada que se valida.
		`Link destination "&#106;avascript:alert(1)" uses the "javascript:" scheme`,
	} {
		if !strings.Contains(found[i].Message, want) {
			t.Errorf("message %d = %q, want it to contain %q", i, found[i].Message, want)
		}
	}
	// El primer TEXT abre en la línea 8 de la fuente.
	if found[0].Position.Line != 8 {
		t.Errorf("first LINK001 at line %d, want 8", found[0].Position.Line)
	}
}

func TestLinkDestinationRuleSkips(t *testing.T) {
	for _, text := range []string{
		"Escribe `[x](javascript:alert(1))` para ver el problema.",
		"[Sitio]({{url}})",
		"[Wiki](https://es.wikipedia.org/wiki/Foo_(bar))",
	} {
		if found := link001(lintSource(t, strictTextSource(text))); len(found) != 0 {
			t.Errorf("%q: unexpected LINK001 %v", text, found)
		}
	}
}

// DocLang comparte el linter: un enlace en prosa, en una viñeta y en una
// celda, y una imagen de bloque con destino bloqueado (que el renderer no
// dibuja), deben reportarse en un .doclang.
func TestLinkDestinationRuleDocument(t *testing.T) {
	src := "---\ntitle: \"Probe\"\n---\n\n# Probe\n\n" +
		"Texto con [a](javascript:alert(1)).\n\n" +
		"- Viñeta con [b](vbscript:x)\n\n" +
		"| Col |\n| --- |\n| [c](data:text/html,x) |\n\n" +
		"![Img](javascript:alert(1))\n\n" +
		"![Foto](img/foto_(1).png)\n"
	doc, parseDiags := parser.New(util.NewNoop()).ParseDocument(src, "test.doclang")
	if doc == nil {
		t.Fatalf("parse returned no AST: %v", parseDiags)
	}
	found := link001(New().WithDialect(DialectDocuments).Lint(doc))
	var msgs []string
	for _, d := range found {
		msgs = append(msgs, d.Message)
	}
	joined := strings.Join(msgs, "\n")
	for _, want := range []string{`"javascript:alert(1)" uses`, `"vbscript:x"`, `"data:text/html,x"`, "the image is not rendered"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing LINK001 containing %q; got:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "foto_(1)") {
		t.Errorf("block image with balanced parentheses flagged:\n%s", joined)
	}
}
